// Package secuft 提供 Supabase `public.finv_quote_secu_ft`（五日分钟行情采集配置）的只读分页查询
// （需求 11344）。
//
// 表在 Supabase 侧，本地库没有副本，只能经 PostgREST 读；因此本包只做三件事：
//
//  1. 列元数据（columns.go）：21 列的中文表头、类型、是否可排序 —— 由实测 PostgREST
//     逐列探测得出（表当前 0 行，无法从数据推断），见 docs。
//  2. 查询构造（store.go）：关键字模糊 + 5 个精确筛选 + 白名单排序 + 分页。
//  3. 复用 secukey.Client 的 PostgREST 通道（鉴权 / 重试 / 错误映射与证券键表同步一致）。
//
// 安全：20013 的 APIKEY 只在进程内使用，不下发前端、不写日志；前端只拿到掩码后的 ProjectID。
package secuft

// TableName Supabase 侧的源表（public schema）。
const TableName = "finv_quote_secu_ft"

// ColumnKind 列的数据类型（决定前端对齐方式与筛选控件）。
type ColumnKind string

// 列类型取值。
const (
	// KindText 文本列。
	KindText ColumnKind = "text"
	// KindNumber 数值列（bigint / integer / smallint）。
	KindNumber ColumnKind = "number"
	// KindTimestamp 时间列（timestamptz）。
	KindTimestamp ColumnKind = "timestamp"
)

// Column 单列元数据。
type Column struct {
	// Name 列名（PostgREST 的 select/order 用它）。
	Name string `json:"name"`
	// Title 中文表头。
	Title string `json:"title"`
	// Kind 数据类型。
	Kind ColumnKind `json:"kind"`
	// Sortable 是否允许按该列排序（全部列都允许；保留字段便于后续收紧）。
	Sortable bool `json:"sortable"`
	// Note 取值口径说明（悬浮提示用，空串表示无需说明）。
	Note string `json:"note,omitempty"`
}

// columns 表 `public.finv_quote_secu_ft` 的全部列（21 列，顺序即页面展示顺序）。
//
// 来源：2026-11-19 对 PostgREST 逐列 `select=<col>` 探测（存在的列返回 200，
// 不存在的列返回 `42703 column ... does not exist`），类型用 `?<col>=eq.@@@`
// 的类型转换报错反推（text / bigint / integer / smallint / timestamptz）。
// 列顺序按「标识 → 名称 → 分类 → 富途编码 → 开关/备注 → 时间」组织。
var columns = []Column{
	{Name: "usc", Title: "统一编号", Kind: KindText, Sortable: true, Note: "证券统一编码，与本地 finv_security.usc 同口径"},
	{Name: "symbol", Title: "证券代码", Kind: KindText, Sortable: true},
	{Name: "futu_symbol", Title: "富途代码", Kind: KindText, Sortable: true, Note: "富途侧证券代码原始形态"},
	{Name: "name", Title: "证券名称", Kind: KindText, Sortable: true},
	{Name: "stock_id", Title: "富途ID", Kind: KindNumber, Sortable: true, Note: "富途证券内部 ID"},
	{Name: "market", Title: "市场", Kind: KindText, Sortable: true, Note: "市场缩写（HK/SH/SZ/BJ/US 等）"},
	{Name: "market_label", Title: "行情市场", Kind: KindText, Sortable: true, Note: "行情市场标识：HK/SH/SZ/BJ/US/FX/CC"},
	{Name: "market_type", Title: "市场类型", Kind: KindNumber, Sortable: true, Note: "富途市场类型：1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密"},
	{Name: "market_code", Title: "市场代码", Kind: KindNumber, Sortable: true, Note: "富途市场代码（不是 finv_market.market_code）"},
	{Name: "instrument", Title: "品种", Kind: KindText, Sortable: true, Note: "stock/etfs/index/currency/crypto/futures/sectors"},
	{Name: "instrument_type", Title: "品种类型", Kind: KindNumber, Sortable: true, Note: "3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密"},
	{Name: "sub_instrument_type", Title: "品种子类型", Kind: KindNumber, Sortable: true},
	{Name: "type_secu", Title: "证券分类", Kind: KindText, Sortable: true},
	{Name: "type_secu_sub", Title: "证券子类", Kind: KindText, Sortable: true},
	{Name: "req_section", Title: "订阅段", Kind: KindNumber, Sortable: true, Note: "富途行情订阅段，0 表示未设"},
	{Name: "provider", Title: "提供方", Kind: KindText, Sortable: true},
	{Name: "flag_enable", Title: "采集开关", Kind: KindNumber, Sortable: true, Note: "1 启用 / 0 禁用"},
	{Name: "remark", Title: "备注", Kind: KindText, Sortable: true},
	{Name: "gmt_create", Title: "创建时间", Kind: KindTimestamp, Sortable: true},
	{Name: "gmt_update", Title: "更新时间", Kind: KindTimestamp, Sortable: true},
	{Name: "dt_update", Title: "数据更新", Kind: KindTimestamp, Sortable: true, Note: "表内业务时间列"},
}

// Columns 返回全部列元数据（副本，调用方可安全修改）。
func Columns() []Column {
	out := make([]Column, len(columns))
	copy(out, columns)
	return out
}

// columnNames 返回 select 用的列名列表。
func columnNames() []string {
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		out = append(out, column.Name)
	}
	return out
}

// sortableColumns 可排序列白名单（列名 → 是否允许）。
var sortableColumns = func() map[string]bool {
	out := make(map[string]bool, len(columns))
	for _, column := range columns {
		if column.Sortable {
			out[column.Name] = true
		}
	}
	return out
}()

// DefaultSortBy 默认排序列。
//
// 用 usc（统一编号）而不是市场或时间：usc 在同一市场内单调，分页稳定且与其它
// 证券列表页的默认观感一致。
const DefaultSortBy = "usc"

// TextFilters 文本精确筛选白名单（请求参数名 → 列名）。
//
// 只有基数低的维度才做下拉：市场 / 品种 / 分类 / 提供方；
// 高基数列（代码、名称、ID）走关键字模糊匹配。
var TextFilters = []struct {
	Param   string // 请求参数名
	Column  string // 列名
	Title   string // 中文名
	Options bool   // 是否提供下拉候选（FilterOptions 接口）
}{
	{Param: "marketLabel", Column: "market_label", Title: "行情市场", Options: true},
	{Param: "instrument", Column: "instrument", Title: "品种", Options: true},
	{Param: "typeSecu", Column: "type_secu", Title: "证券分类", Options: true},
	{Param: "provider", Column: "provider", Title: "提供方", Options: true},
}

// KeywordColumns 关键字模糊匹配的列（PostgREST 的 or=(...) 表达式）。
var KeywordColumns = []string{"usc", "symbol", "futu_symbol", "name"}
