package cdeu

// 本文件是「加密货币行情排行」的领域模型与筛选参数（需求 11323）。
//
// 表：cdeu.finv_quote_secu_realtime_crypto（V300225 创建，主键 usc，一行一币种）。
// 结构见 V300226 的列注释：价格/涨跌/振幅/成交量额/盘口/52 周高低等均为结构化列，
// 无需解析 JSON，因此本页直接按列白名单查询与排序。

// 表名复用 store_crypto_realtime.go 的 tableCryptoRealtime。

// RealtimeCryptoOrderColumns 加密排行排序白名单。
var RealtimeCryptoOrderColumns = map[string]bool{
	"price_nominal":   true,
	"change_price":    true,
	"change_ratio":    true,
	"price_amplitude": true,
	"trade_volumn":    true,
	"trade_trunover":  true,
	"volumn_ratio":    true,
	"buysell_ratio":   true,
	"bid_price":       true,
	"ask_price":       true,
	"highest52weeks":  true,
	"lowest52weeks":   true,
	"dt_update":       true,
	"code":            true,
	"usc":             true,
}

// RealtimeCryptoListFilter 加密排行查询条件。
type RealtimeCryptoListFilter struct {
	// Keyword 匹配 usc / code / name（ILIKE）。
	Keyword string
	// QuoteCurrency 计价币种（如 USD / HKD），空或 all 表示不限。
	QuoteCurrency string
	// SecStatus 证券状态（原文），空或 all 表示不限。
	SecStatus string
	// Suspension 停牌过滤：空 = 不限，"0" = 未停牌，"1" = 停牌。
	Suspension string
	// OrderBy 见 RealtimeCryptoOrderColumns，空默认 change_ratio。
	OrderBy string
	Desc    bool
	Limit   int
	Offset  int
}

// RealtimeCryptoListRow 加密排行一行（数值列以文本返回，保住 NUMERIC 精度）。
type RealtimeCryptoListRow struct {
	USC            string `json:"usc"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	QuoteCurrency  string `json:"quote_currency"`
	PriceNominal   string `json:"price_nominal"`
	PriceOpen      string `json:"price_open"`
	PriceHighest   string `json:"price_highest"`
	PriceLowest    string `json:"price_lowest"`
	PriceLastClose string `json:"price_last_close"`
	PriceAvg       string `json:"price_avg"`
	ChangePrice    string `json:"change_price"`
	ChangeRatio    string `json:"change_ratio"`
	PriceAmplitude string `json:"price_amplitude"`
	PriceDirect    string `json:"price_direct"`
	TradeVolumn    string `json:"trade_volumn"`
	TradeTrunover  string `json:"trade_trunover"`
	VolumnRatio    string `json:"volumn_ratio"`
	BidPrice       string `json:"bid_price"`
	AskPrice       string `json:"ask_price"`
	BidVol         string `json:"bid_vol"`
	AskVol         string `json:"ask_vol"`
	BuysellRatio   string `json:"buysell_ratio"`
	Highest52Weeks string `json:"highest52weeks"`
	Lowest52Weeks  string `json:"lowest52weeks"`
	HighestHistory string `json:"highest_history"`
	LowestHistory  string `json:"lowest_history"`
	SecStatus      string `json:"sec_status"`
	Suspension     string `json:"suspension"`
	DataDate       string `json:"data_date"`
	Provider       string `json:"provider"`
	DtUpdate       int64  `json:"dt_update"`
}
