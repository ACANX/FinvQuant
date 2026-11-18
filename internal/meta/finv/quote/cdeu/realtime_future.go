package cdeu

// 本文件是「期货行情排行」的领域模型与筛选参数（需求 11322）。
//
// 表：cdeu.finv_quote_secu_realtime_future（主键 stock_id，一行一合约，整行覆盖）。
// 采集口径见 quote_list_futures_snapshot_sync.go：结构化列只有
// 代码/名称/现价/涨跌幅（字段 1/2/3/4），其余字段原样留档在 fields_json，
// 字段定义留档在 field_list_json —— 因此本页**不臆造** fields_json 的编号含义，
// 只把两段 JSON 原样返回，由前端按 field_list_json 渲染明细。

// 表名复用 store_futures_list.go 的 tableFutureList / tableFutureSyncConfig
// （cdeu.finv_quote_secu_realtime_future / cdeu.finv_quote_cdeu_future_sync_config）。

// 需求 11324：期货排行展示「持仓量」「日增仓」两个指标列。
//
// 这两个指标不是主表结构化列，而在字段留档里：field_list_json 给出
// fieldId → fieldName（实测 11=持仓量、12=日增仓），fields_json 给出
// fieldId → 原文。**按字段名查编号**而不是硬编码 11/12，避免上游字段顺序变化后读错值。
const (
	futureFieldNameOpenInterest = "持仓量"
	futureFieldNameDayIncrease  = "日增仓"
)

// RealtimeFutureOrderColumns 期货排行排序白名单。
//
// 持仓量 / 日增仓同样可排序（走 realtimeFutureOrderExprs 里的表达式）。
var RealtimeFutureOrderColumns = map[string]bool{
	"price_nominal": true,
	"change_ratio":  true,
	"open_interest": true,
	"day_increase":  true,
	"dt_update":     true,
	"stock_id":      true,
	"contract_code": true,
	"futures_type":  true,
	"market_label":  true,
	"name":          true,
}

// RealtimeFutureListFilter 期货排行查询条件。
type RealtimeFutureListFilter struct {
	// Keyword 匹配 contract_code / name / usc / stock_id（ILIKE）。
	Keyword string
	// FuturesType 期货分类（1~8），空或 all 表示不限。
	FuturesType string
	// MarketLabel 市场标签（US / HK / SG / JP），空或 all 表示不限。
	MarketLabel string
	// OnlyOnline 只列在线合约（flag_online = '1'）。
	OnlyOnline bool
	// OrderBy 见 RealtimeFutureOrderColumns，空默认 change_ratio。
	OrderBy string
	Desc    bool
	Limit   int
	Offset  int
}

// RealtimeFutureListRow 期货排行一行（数值列以文本返回，保住 NUMERIC(20,6) 精度）。
type RealtimeFutureListRow struct {
	StockID         string `json:"stock_id"`
	USC             string `json:"usc"`
	FuturesType     string `json:"futures_type"`
	FuturesTypeName string `json:"futures_type_name"`
	MarketType      string `json:"market_type"`
	MarketLabel     string `json:"market_label"`
	ContractCode    string `json:"contract_code"`
	Name            string `json:"name"`
	PriceNominal    string `json:"price_nominal"`
	ChangeRatio     string `json:"change_ratio"`
	// OpenInterest 持仓量（字段留档里 fieldName=持仓量 的原文，非数值时为空）。
	OpenInterest string `json:"open_interest"`
	// DayIncrease 日增仓（同上，fieldName=日增仓）。
	DayIncrease string `json:"day_increase"`
	PriceDirect string `json:"price_direct"`
	FlagOnline  string `json:"flag_online"`
	Provider    string `json:"provider"`
	Remark      string `json:"remark"`
	DtUpdate    int64  `json:"dt_update"`
	// FieldsJSON 该合约的原始字段留档（编号 → 原文，jsonb 文本）。
	FieldsJSON string `json:"fields_json"`
	// FieldListJSON 字段定义留档（编号 → 字段名/类型/精度，jsonb 文本）。
	FieldListJSON string `json:"field_list_json"`
}

// RealtimeFutureTypeOption 期货分类下拉项（来自配置表的 futures_type / futures_type_name）。
type RealtimeFutureTypeOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
