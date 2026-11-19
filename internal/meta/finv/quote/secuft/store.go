package secuft

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	syscfg "github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 分页参数默认值与上限。
const (
	// DefaultPageSize 默认每页条数（需求 11344 指定 15）。
	DefaultPageSize = 15
	// MaxPageSize 每页条数上限。
	MaxPageSize = 200
)

// Query 分页查询条件。
type Query struct {
	// Page 页码（≥1）。
	Page int
	// PageSize 每页条数（1–MaxPageSize）。
	PageSize int
	// Keyword 关键字：模糊匹配 KeywordColumns。
	Keyword string
	// TextFilters 文本精确筛选（参数名 → 值；仅 TextFilters 白名单内的参数生效）。
	TextFilters map[string]string
	// FlagEnable 采集开关（"" 不限 / "1" / "0"）。
	FlagEnable string
	// SortBy 排序列（白名单内，非法回退 DefaultSortBy）。
	SortBy string
	// SortDesc 是否降序。
	SortDesc bool
}

// Normalize 把缺省/越界参数收敛到合法值。
func (q *Query) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	if q.SortBy == "" || !sortableColumns[q.SortBy] {
		q.SortBy = DefaultSortBy
	}
	// 采集开关只接受 1/0，其它（含 "all"）视为不限。
	q.FlagEnable = strings.TrimSpace(q.FlagEnable)
	if q.FlagEnable != "0" && q.FlagEnable != "1" {
		q.FlagEnable = ""
	}
}

// Offset 计算 PostgREST 的 offset。
func (q Query) Offset() int { return (q.Page - 1) * q.PageSize }

// Row 页面返回的一行（列名与原表一致，前端按 Columns() 渲染）。
type Row map[string]json.RawMessage

// Result 一次分页查询的结果。
type Result struct {
	// Rows 本页数据。
	Rows []Row
	// Total 满足条件的总行数（PostgREST 的 Content-Range 尾段；取不到时为 0）。
	Total int64
	// ElapsedMs 上游耗时（毫秒）。
	ElapsedMs int64
}

// Store 查询入口。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建查询入口。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Page 执行一次分页查询。
//
// 20013 未配置 / 非法时直接返回可读错误（不访问 Supabase）。
func (s *Store) Page(ctx context.Context, query Query) (*Result, error) {
	query.Normalize()
	conn, err := s.supabaseConn(ctx)
	if err != nil {
		return nil, err
	}

	client := secukey.NewClient(conn)
	values := buildQueryValues(query)
	started := time.Now()
	rows, total, err := client.FetchQuery(ctx, TableName, values)
	if err != nil {
		return nil, err
	}
	result := &Result{Total: total, ElapsedMs: time.Since(started).Milliseconds()}
	result.Rows = make([]Row, 0, len(rows))
	for _, raw := range rows {
		row := Row{}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("解析 Supabase 数据行失败: %w", err)
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

// FilterOptions 各下拉维度的候选值（表内 DISTINCT）。
//
// 表为空时全部返回空切片（页面下拉只有「全部」一项），不报错。
func (s *Store) FilterOptions(ctx context.Context) (map[string][]string, error) {
	conn, err := s.supabaseConn(ctx)
	if err != nil {
		return nil, err
	}
	client := secukey.NewClient(conn)
	out := make(map[string][]string, len(TextFilters))
	for _, filter := range TextFilters {
		if !filter.Options {
			continue
		}
		values := url.Values{}
		values.Set("select", filter.Column)
		values.Set("order", filter.Column+".asc")
		values.Set("limit", "500")
		rows, _, err := client.FetchQuery(ctx, TableName, values)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool, len(rows))
		options := make([]string, 0, len(rows))
		for _, raw := range rows {
			var row Row
			if err := json.Unmarshal(raw, &row); err != nil {
				continue
			}
			value := jsonText(row[filter.Column])
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			options = append(options, value)
		}
		out[filter.Param] = options
	}
	return out, nil
}

// supabaseConn 读 20013（带缓存的 syscfg Store）。
func (s *Store) supabaseConn(ctx context.Context) (syscfg.SupabaseConn, error) {
	values, err := syscfg.NewStore(s.pool).Load(ctx)
	if err != nil {
		return syscfg.SupabaseConn{}, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		return syscfg.SupabaseConn{}, fmt.Errorf("Supabase 连接不可用: %w", values.SupabaseConnErr)
	}
	return values.SupabaseConn, nil
}

// buildQueryValues 组装 PostgREST 查询参数（包内可见，便于单测）。
func buildQueryValues(query Query) url.Values {
	values := url.Values{}
	values.Set("select", strings.Join(columnNames(), ","))
	// 排序追加 usc 作为稳定兜底键，避免同值行在翻页时重复/丢失。
	order := query.SortBy + ".asc"
	if query.SortDesc {
		order = query.SortBy + ".desc"
	}
	if query.SortBy != "usc" {
		order += ",usc.asc"
	}
	values.Set("order", order)
	values.Set("limit", strconv.Itoa(query.PageSize))
	values.Set("offset", strconv.Itoa(query.Offset()))

	if query.Keyword != "" {
		values.Set("or", keywordExpression(query.Keyword))
	}
	for _, filter := range TextFilters {
		value := strings.TrimSpace(query.TextFilters[filter.Param])
		if value == "" || value == "all" {
			continue
		}
		values.Set(filter.Column, "eq."+value)
	}
	if query.FlagEnable != "" {
		values.Set("flag_enable", "eq."+query.FlagEnable)
	}
	return values
}

// keywordExpression 拼接 `(a.ilike.*kw*,b.ilike.*kw*,...)`。
//
// PostgREST 的 `*` 是 `%` 的等价写法；值里的 `*` `,` `(` `)` 需要转义成 `\*` 等，
// 否则会被当作通配符或分隔符（与 REST 语义一致，防注入也靠这个）。
func keywordExpression(keyword string) string {
	escaped := strings.NewReplacer(
		`\`, `\\`,
		`*`, `\*`,
		`,`, `\,`,
		`(`, `\(`,
		`)`, `\)`,
	).Replace(keyword)
	parts := make([]string, 0, len(KeywordColumns))
	for _, column := range KeywordColumns {
		parts = append(parts, column+".ilike.*"+escaped+"*")
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// jsonText 把 JSON 原始值转成展示文本（字符串去引号、数字原样、null 为空串）。
func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	text := strings.TrimSpace(string(raw))
	if text == "null" {
		return ""
	}
	if unquoted, err := strconv.Unquote(text); err == nil {
		return unquoted
	}
	return text
}

// LogQuery 记录一次查询（不含 APIKEY 与数据内容），供 handler 复用统一日志格式。
func LogQuery(query Query, result *Result, err error) {
	if err != nil {
		log.Printf("[secuft] query failed page=%d pageSize=%d keyword=%q err=%v", query.Page, query.PageSize, query.Keyword, err)
		return
	}
	log.Printf("[secuft] query ok page=%d pageSize=%d total=%d rows=%d elapsed_ms=%d",
		query.Page, query.PageSize, result.Total, len(result.Rows), result.ElapsedMs)
}
