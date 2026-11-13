package indicator

// 证券指标表（cdeu.finv_quote_secu_indicator，132 列）的**只读**查询（需求 11249/11250）。
//
// 与同步链路（service.go / runner.go / store.go）相互独立：本文件只做 SELECT，不写任何表、
// 不触发触发器，供前端「行情 → 实时行情 → 证券指标」页面使用。
//
// 两个关键口径：
//  1. 列清单以 **数据库实际列** 为准（pg_attribute + 列注释），不硬编码 Go 列数组：
//     上游补列后重新执行同步即可在 `/Columns` 里看到新列，前端无需改代码；
//     同时这份清单就是 SQL 标识符白名单——拼接进 SQL 的列名永远来自数据库目录，
//     不来自请求参数（避免注入）。
//  2. 所有列值一律 `::text` 取回（含 numeric(30,10)、bigint、timestamptz）：数值不经 float64，
//     万/亿折算与小数位裁剪由前端负责，与「证券行情排行」同口径。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrQueryInvalid 查询参数非法（Handler 映射为 HTTP 400）。
var ErrQueryInvalid = errors.New("指标查询参数非法")

// DefaultPageSize 默认每页条数（与页面默认值一致）。
const DefaultPageSize = 50

// MaxPageSize 每页条数上限：132 列 × 500 行已经足够撑满一次表格渲染，
// 再大只会把响应体做大，对宽表没有收益。
const MaxPageSize = 500

// indicatorDefaultColumns 请求未指定 columns 时的默认列集（概览口径，23 列）。
//
// 前端「证券指标」页面一般会显式传当前选中的列；这里的默认值只服务于 curl / 脚本，
// 保证不带参数也能看到一眼能读懂的一行。
var indicatorDefaultColumns = []string{
	"usc", "ftmm_symbol", "name",
	"price", "change_price", "change_ratio", "price_direct",
	"price_open", "price_highest", "price_lowest", "price_last_close",
	"volume", "turnover", "ratio_volume", "ratio_turnover",
	"total_market_cap", "outstanding_market_cap",
	"pe_ttm", "pb_ratio", "dividend_ratio",
	"market_status_text", "ts", "dt_update",
}

// indicatorKeyColumns 行身份列：无论请求什么列集都必须返回，
// 否则前端拿到的行既不能点开详情也不能排序定位。
var indicatorKeyColumns = []string{"usc", "ftmm_symbol"}

// IndicatorColumn 列元数据（前端据此生成列标题、列分组与格式化口径）。
type IndicatorColumn struct {
	// Name 列名（snake_case，与响应行里的键一致）。
	Name string `json:"name"`
	// Title 列中文名（取自表列注释；注释缺失时回退列名）。
	Title string `json:"title"`
	// DataType 简化类型：numeric / integer / text / timestamp / boolean / other。
	DataType string `json:"dataType"`
	// Sortable 是否允许作为 orderBy（当前全部可排序；保留字段便于将来排除大文本列）。
	Sortable bool `json:"sortable"`
}

// IndicatorPageFilter 分页查询条件。
type IndicatorPageFilter struct {
	// Keyword 模糊匹配 usc / ftmm_symbol / name（ILIKE）。
	Keyword string
	// Market 市场前缀（ftmm_symbol 的第一段，如 HK / US）；空或 all 不限。
	Market string
	// Columns 请求列（已在 Handler 层拆分逗号）；空表示使用默认列集。
	Columns []string
	// OrderBy 排序字段（必须是实际存在的列）；空默认 dt_update。
	OrderBy string
	// Desc 是否降序。
	Desc bool
	// Limit 每页条数（<=0 取默认值，超过上限截断）。
	Limit int
	// Offset 偏移量（页码换算）。
	Offset int
}

// Reader 指标表只读查询器。
type Reader struct {
	pool *pgxpool.Pool
}

// NewReader 创建只读查询器。
func NewReader(pool *pgxpool.Pool) *Reader {
	return &Reader{pool: pool}
}

// indicatorCatalogSQL 读出表的全部列（按建表列序）及其中文注释与实际类型。
//
// to_regclass 返回 NULL 时（表不存在）不报错而是零行，调用方按「无列」处理。
const indicatorCatalogSQL = `
SELECT a.attname,
       COALESCE(col_description(a.attrelid, a.attnum), ''),
       format_type(a.atttypid, a.atttypmod)
  FROM pg_attribute a
 WHERE a.attrelid = to_regclass($1)
   AND a.attnum > 0
   AND NOT a.attisdropped
 ORDER BY a.attnum`

// Columns 返回指标表的全部列元数据（按建表列序）。
func (r *Reader) Columns(ctx context.Context) ([]IndicatorColumn, error) {
	rows, err := r.pool.Query(ctx, indicatorCatalogSQL, IndicatorTable)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 列元数据失败: %w", IndicatorTable, err)
	}
	defer rows.Close()

	columns := make([]IndicatorColumn, 0, 140)
	for rows.Next() {
		var name, comment, typeName string
		if err := rows.Scan(&name, &comment, &typeName); err != nil {
			return nil, fmt.Errorf("解析 %s 列元数据失败: %w", IndicatorTable, err)
		}
		columns = append(columns, newIndicatorColumn(name, comment, typeName))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 列元数据失败: %w", IndicatorTable, err)
	}
	return columns, nil
}

// newIndicatorColumn 由 catalog 原始三元组构造列元数据（抽出来便于单测）。
func newIndicatorColumn(name, comment, typeName string) IndicatorColumn {
	title := strings.TrimSpace(comment)
	if title == "" {
		title = name
	}
	return IndicatorColumn{
		Name:     name,
		Title:    title,
		DataType: indicatorDataType(typeName),
		Sortable: true,
	}
}

// indicatorDataType 把 PostgreSQL 的 format_type 结果收敛成前端可判断的几种。
func indicatorDataType(typeName string) string {
	switch {
	case strings.HasPrefix(typeName, "numeric"), strings.HasPrefix(typeName, "decimal"),
		strings.HasPrefix(typeName, "double"), strings.HasPrefix(typeName, "real"):
		return "numeric"
	case strings.HasPrefix(typeName, "bigint"), strings.HasPrefix(typeName, "integer"),
		strings.HasPrefix(typeName, "smallint"):
		return "integer"
	case strings.HasPrefix(typeName, "timestamp"), strings.HasPrefix(typeName, "date"):
		return "timestamp"
	case strings.HasPrefix(typeName, "text"), strings.HasPrefix(typeName, "character"):
		return "text"
	case strings.HasPrefix(typeName, "boolean"):
		return "boolean"
	default:
		return "other"
	}
}

// Page 分页查询指标行：返回满足条件的总行数与当前页（每行按请求列返回文本值）。
//
// 未在请求列里的列不会出现在结果中；`usc` / `ftmm_symbol` 始终返回。
func (r *Reader) Page(ctx context.Context, f IndicatorPageFilter) (int64, []map[string]string, error) {
	columns, err := r.Columns(ctx)
	if err != nil {
		return 0, nil, err
	}
	known := make(map[string]bool, len(columns))
	for _, column := range columns {
		known[column.Name] = true
	}
	if len(known) == 0 {
		return 0, nil, fmt.Errorf("%w: 表 %s 不存在或没有列", ErrQueryInvalid, IndicatorTable)
	}

	selected, err := resolveIndicatorColumns(f.Columns, known)
	if err != nil {
		return 0, nil, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	// 排序字段：默认按最后更新时间（新同步的行在前），空值一律排最后，再用 usc 稳定次序。
	orderBy := strings.TrimSpace(f.OrderBy)
	if orderBy == "" {
		orderBy = "dt_update"
	}
	if !known[orderBy] {
		return 0, nil, fmt.Errorf("%w: orderBy 非法（%s）", ErrQueryInvalid, orderBy)
	}
	direction := "DESC"
	if !f.Desc {
		direction = "ASC"
	}

	where, args := indicatorWhere(f)

	var total int64
	if err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM "+IndicatorTable+" t"+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("统计 %s 行数失败: %w", IndicatorTable, err)
	}
	if total == 0 {
		return 0, nil, nil
	}

	// 列名来自数据库目录（known 白名单），不来自请求参数；值一律 ::text 保住精度。
	selectList := make([]string, 0, len(selected))
	for _, name := range selected {
		selectList = append(selectList,
			fmt.Sprintf("COALESCE(t.%s::text, '') AS %s", name, name))
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(
		"SELECT %s FROM %s t%s ORDER BY t.%s %s NULLS LAST, t.usc ASC LIMIT $%d OFFSET $%d",
		strings.Join(selectList, ", "), IndicatorTable, where, orderBy, direction,
		len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("查询 %s 失败: %w", IndicatorTable, err)
	}
	defer rows.Close()

	list := make([]map[string]string, 0, limit)
	for rows.Next() {
		values := make([]*string, len(selected))
		scans := make([]any, len(selected))
		for i := range values {
			scans[i] = &values[i]
		}
		if err := rows.Scan(scans...); err != nil {
			return 0, nil, fmt.Errorf("解析 %s 行失败: %w", IndicatorTable, err)
		}
		item := make(map[string]string, len(selected))
		for i, name := range selected {
			// COALESCE 已兜住 NULL，这里只需防 nil 指针。
			if values[i] == nil {
				item[name] = ""
				continue
			}
			item[name] = *values[i]
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("读取 %s 行失败: %w", IndicatorTable, err)
	}
	return total, list, nil
}

// resolveIndicatorColumns 归一请求列集：去空、去重、补身份列，并校验每一列真实存在。
//
// 返回顺序 = 请求顺序（身份列缺失时前置补齐），便于前端按自己的列顺序渲染。
func resolveIndicatorColumns(requested []string, known map[string]bool) ([]string, error) {
	selected := make([]string, 0, len(requested)+len(indicatorKeyColumns))
	seen := make(map[string]bool, len(requested)+len(indicatorKeyColumns))
	appendColumn := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		selected = append(selected, name)
	}

	for _, name := range indicatorKeyColumns {
		if !known[name] {
			return nil, fmt.Errorf("%w: 表缺少身份列 %s", ErrQueryInvalid, name)
		}
		appendColumn(name)
	}

	names := requested
	if len(names) == 0 {
		names = indicatorDefaultColumns
	}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !known[name] {
			return nil, fmt.Errorf("%w: columns 里的列不存在（%s）", ErrQueryInvalid, name)
		}
		appendColumn(name)
	}
	return selected, nil
}

// indicatorWhere 拼装筛选条件（关键字 / 市场前缀），返回 WHERE 子句与参数。
//
// 关键字连 ftmm_symbol 一起匹配：可以直接拿 `HK.00700` 这种键去搜。
func indicatorWhere(f IndicatorPageFilter) (string, []any) {
	conds := make([]string, 0, 2)
	args := make([]any, 0, 2)

	if keyword := strings.TrimSpace(f.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(t.usc ILIKE $%d OR t.ftmm_symbol ILIKE $%d OR COALESCE(t.name, '') ILIKE $%d)",
			idx, idx, idx))
	}
	if market := strings.TrimSpace(f.Market); market != "" && !strings.EqualFold(market, "all") {
		args = append(args, strings.ToUpper(market))
		conds = append(conds, fmt.Sprintf("split_part(t.ftmm_symbol, '.', 1) = $%d", len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
