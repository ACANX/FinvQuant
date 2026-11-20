// Package secuft 提供 Supabase `public.finv_quote_secu_ft`（五日分钟行情采集配置）的只读分页查询
// 与「新增采集证券」的元数据补全（需求 11344 / 11345）。
//
// 表在 Supabase 侧，本地库没有副本，只能经 PostgREST 访问；因此本包做三件事：
//
//  1. 列元数据（本文件）：32 列的中文表头、类型、是否可排序；
//  2. 查询构造（store.go）：关键字模糊 + 精确筛选 + 白名单排序 + 分页；
//  3. 元数据补全与写入（lookup.go / create.go）：联查本地 4 张表，写回收集配置表。
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
	// KindNumber 数值列（bigint / integer / smallint / numeric）。
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

// columns 表 `public.finv_quote_secu_ft` 的全部列（32 列，顺序即页面展示顺序）。
//
// 来源：2026-11-19 抓取 `select=*` 的 CSV 表头（32 列），并用 `?<col>=eq.@@@`
// 的类型转换报错逐列反推类型；列口径由同日 465 行真实数据与本地 4 张表比对得出
// （一致率见 lookup.go 注释）。
//
// 顺序：标识 → 名称 → 市场/交易所 → 证券分类 → 品种/开关/富途代码 → 富途ID → 富途编码
//
//	→ 权重/采集统计 → 备注 → 时间。
var columns = []Column{
	{Name: "usc", Title: "统一编号", Kind: KindText, Sortable: true, Note: "证券统一编码，与本地 finv_security.usc 同口径"},
	{Name: "name", Title: "证券名称", Kind: KindText, Sortable: true},
	{Name: "region", Title: "区域", Kind: KindText, Sortable: true, Note: "CN / HK / US / FX / CC"},
	{Name: "market", Title: "市场", Kind: KindText, Sortable: true, Note: "细分市场：SH/SZ/HK/BJ 或 NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX"},
	{Name: "market_label", Title: "行情市场", Kind: KindText, Sortable: true, Note: "行情市场标识：HK/SH/SZ/BJ/US/FX/CC"},
	{Name: "symbol", Title: "证券全码", Kind: KindText, Sortable: true, Note: "富途全码（ftmm_symbol），如 HK.00700、US..BKX"},
	{Name: "currency", Title: "计价货币", Kind: KindText, Sortable: true},
	{Name: "timezone", Title: "时区", Kind: KindText, Sortable: true, Note: "IANA 时区名，如 Asia/Shanghai"},
	{Name: "provider", Title: "提供方", Kind: KindText, Sortable: true},
	{Name: "type_secu", Title: "证券分类", Kind: KindText, Sortable: true},
	{Name: "type_secu_sub", Title: "证券子类", Kind: KindText, Sortable: true},
	{Name: "instrument", Title: "品种", Kind: KindText, Sortable: true, Note: "stock/etf/index/forex/crypto/future"},
	{Name: "flag_enable", Title: "采集开关", Kind: KindNumber, Sortable: true, Note: "1 启用 / 0 禁用"},
	{Name: "futu_symbol", Title: "富途代码", Kind: KindText, Sortable: true, Note: "富途侧证券代码裸码，如 00700、.BKX"},
	{Name: "stock_id", Title: "富途ID", Kind: KindNumber, Sortable: true, Note: "富途证券内部 ID"},
	{Name: "market_type", Title: "市场类型", Kind: KindNumber, Sortable: true, Note: "富途市场类型：1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密"},
	{Name: "market_code", Title: "市场代码", Kind: KindNumber, Sortable: true, Note: "富途市场代码（不是 finv_market.market_code）"},
	{Name: "instrument_type", Title: "品种类型", Kind: KindNumber, Sortable: true, Note: "3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密"},
	{Name: "sub_instrument_type", Title: "品种子类型", Kind: KindNumber, Sortable: true},
	{Name: "req_section", Title: "订阅段", Kind: KindNumber, Sortable: true, Note: "富途行情订阅段，0 表示未设"},
	{Name: "weight_priority", Title: "采集权重", Kind: KindNumber, Sortable: true, Note: "采集优先级，默认 50"},
	{Name: "weight_frequency", Title: "采集频率", Kind: KindNumber, Sortable: true, Note: "采集频率权重，默认 10"},
	{Name: "ts_latest_data", Title: "最新数据时间戳", Kind: KindNumber, Sortable: true},
	{Name: "count_last_fetch", Title: "累计采集次数", Kind: KindNumber, Sortable: true},
	{Name: "count_fail", Title: "累计失败次数", Kind: KindNumber, Sortable: true},
	{Name: "count_stale", Title: "连续陈旧次数", Kind: KindNumber, Sortable: true},
	{Name: "dt_update", Title: "数据更新时间", Kind: KindTimestamp, Sortable: true},
	{Name: "dt_last_check", Title: "最近检查时间", Kind: KindTimestamp, Sortable: true},
	{Name: "dt_last_fetch", Title: "最近采集时间", Kind: KindTimestamp, Sortable: true},
	{Name: "gmt_update", Title: "更新时间", Kind: KindTimestamp, Sortable: true},
	{Name: "gmt_create", Title: "创建时间", Kind: KindTimestamp, Sortable: true},
	{Name: "remark", Title: "备注", Kind: KindText, Sortable: true},
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
// 只有基数低的维度才做下拉：区域 / 行情市场 / 品种 / 分类 / 提供方；
// 高基数列（代码、名称、细分市场）走关键字模糊匹配。
var TextFilters = []struct {
	Param   string // 请求参数名
	Column  string // 列名
	Title   string // 中文名
	Options bool   // 是否提供下拉候选（FilterOptions 接口）
}{
	{Param: "region", Column: "region", Title: "区域", Options: true},
	{Param: "marketLabel", Column: "market_label", Title: "行情市场", Options: true},
	{Param: "instrument", Column: "instrument", Title: "品种", Options: true},
	{Param: "typeSecu", Column: "type_secu", Title: "证券分类", Options: true},
	{Param: "provider", Column: "provider", Title: "提供方", Options: true},
}

// KeywordColumns 关键字模糊匹配的列（PostgREST 的 or=(...) 表达式）。
var KeywordColumns = []string{"usc", "symbol", "futu_symbol", "name"}
