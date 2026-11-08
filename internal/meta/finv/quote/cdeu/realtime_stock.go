package cdeu

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// 本文件是「证券实时行情快照批量 Upsert」的领域模型与入参校验（需求 10957）。
//
// 表：cdeu.finv_cdeu_secu_realtime_stock（V300115 创建，一行一证券，主键 stock_id）。
// 语义：实时快照，只保留最新状态，不保留历史；写入按字段级覆盖（COALESCE）。

// tableRealtimeStock 证券实时行情快照表（在 cdeu schema）。
const tableRealtimeStock = "cdeu.finv_cdeu_secu_realtime_stock"

// MaxRealtimeStockBatchRows 单请求 items 条数上限。
//
// 一个批次在单条 SQL 内用 jsonb_to_recordset 展开写入，500 行只产生一次往返；
// 上限固定（不做成环境变量，避免配置面膨胀）。
const MaxRealtimeStockBatchRows = 500

// ErrRealtimeStockInvalid 快照入参非法（handler 映射为 HTTP 400）。
//
// 用 errors.Is 判定：所有入参问题都用 %w 包装它，业务码固定 400，
// 不与数据库错误（2006）混淆。
var ErrRealtimeStockInvalid = errors.New("实时行情快照入参非法")

// RealtimeStockItem 一行证券实时行情快照。
//
// 数值列一律用 json.Number 承载原文（再以 text 传给数据库转 NUMERIC(24,6)），
// 避免 float64 往返造成的精度漂移；字段为 nil 表示本次未提供，落库为 NULL。
// 落库列名与 JSON 字段同名，便于与建表脚本对照。
type RealtimeStockItem struct {
	USC                    *string      `json:"usc"`
	StockID                *int64       `json:"stock_id"`
	Name                   *string      `json:"name"`
	StockCode              *string      `json:"stock_code"`
	MarketType             *string      `json:"market_type"`
	MarketLabel            *string      `json:"market_label"`
	InstrumentType         *string      `json:"instrument_type"`
	Currency               *string      `json:"currency"`
	MarketValue            *json.Number `json:"market_value"`
	TotalShares            *json.Number `json:"total_shares"`
	CirculationTotalShares *json.Number `json:"circulation_total_shares"`
	TradeTrunover          *json.Number `json:"trade_trunover"`
	TradeVolumn            *json.Number `json:"trade_volumn"`
	CirculationMarketValue *json.Number `json:"circulation_market_value"`
	DtUpdate               *int64       `json:"dt_update"`
	PriceNominal           *json.Number `json:"price_nominal"`
	ChangeRatio            *json.Number `json:"change_ratio"`
	TrustAum               *json.Number `json:"trust_aum"`
	TradeChangeRatio       *json.Number `json:"trade_change_ratio"`
	PriceAmplitude         *json.Number `json:"price_amplitude"`
	VolumnRatio            *json.Number `json:"volumn_ratio"`
	BuysellRatio           *json.Number `json:"buysell_ratio"`
	ChangePrice            *json.Number `json:"change_price"`
	PriceDirect            *string      `json:"price_direct"`
	Leverage               *json.Number `json:"leverage"`
	// Remark 备注（需求 11028）：人工标注或采集端说明，不参与换算/排序；缺省不清空已有值。
	Remark *string `json:"remark"`
	Page   *int64  `json:"page"`
}

// RealtimeStockUpsertRequest 批量 upsert 请求体。
type RealtimeStockUpsertRequest struct {
	Items []RealtimeStockItem `json:"items"`
}

// RealtimeStockUpsertStat 写入统计（按主键是否命中区分新增/更新）。
type RealtimeStockUpsertStat struct {
	Total    int64 `json:"total"`
	Inserted int64 `json:"insert"`
	Updated  int64 `json:"update"`
}

// ValidateRealtimeStockItems 校验入参：非空、条数上限、stock_id 必填且为正、
// 批内 stock_id 不重复、dt_update 非负、page 为正（提供时）。
//
// stock_id 由源系统（富途）的 64 位证券编号充当，表列为 BIGINT（V300116 由 INTEGER 扩宽，
// 富途实测最大 148739012639708），因此这里只校验 > 0，上限交给 int64 与数据库。
//
// 为什么批内禁止重复 stock_id：整批是一条 INSERT ... ON CONFLICT 语句，
// 同一行在同一条语句里被影响两次会直接报错（"cannot affect row a second time"），
// 提前拦截可以给出可读的错误位置。
func ValidateRealtimeStockItems(items []RealtimeStockItem) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: items 不能为空", ErrRealtimeStockInvalid)
	}
	if len(items) > MaxRealtimeStockBatchRows {
		return fmt.Errorf("%w: items 条数 %d 超过上限 %d",
			ErrRealtimeStockInvalid, len(items), MaxRealtimeStockBatchRows)
	}
	seen := make(map[int64]int, len(items))
	for index, item := range items {
		position := index + 1
		if item.StockID == nil {
			return fmt.Errorf("%w: 第 %d 条缺少 stock_id", ErrRealtimeStockInvalid, position)
		}
		stockID := *item.StockID
		if stockID <= 0 {
			return fmt.Errorf("%w: 第 %d 条 stock_id 非法（%d），须为正整数",
				ErrRealtimeStockInvalid, position, stockID)
		}
		if first, ok := seen[stockID]; ok {
			return fmt.Errorf("%w: stock_id=%d 在第 %d 条与第 %d 条重复",
				ErrRealtimeStockInvalid, stockID, first+1, position)
		}
		seen[stockID] = index
		if item.DtUpdate != nil && *item.DtUpdate < 0 {
			return fmt.Errorf("%w: 第 %d 条 dt_update 不能为负（%d）",
				ErrRealtimeStockInvalid, position, *item.DtUpdate)
		}
		// page 为「市值排行接口的分页页码」，从 1 开始；表侧有 CHECK (page IS NULL OR page > 0)，
		// 这里提前拦截可给出可读的行号，不必等数据库报 23514。
		if item.Page != nil {
			if *item.Page <= 0 {
				return fmt.Errorf("%w: 第 %d 条 page 非法（%d），须为正整数",
					ErrRealtimeStockInvalid, position, *item.Page)
			}
			if *item.Page > math.MaxInt32 {
				return fmt.Errorf("%w: 第 %d 条 page 越界（%d），须 ≤ %d",
					ErrRealtimeStockInvalid, position, *item.Page, math.MaxInt32)
			}
		}
	}
	return nil
}

// realtimeStockText 文本列取值：nil → JSON null（落库 NULL）。
//
// 不做 TrimSpace：名称/代码里的前后空格属上游原样数据，交给数据库侧 btrim 统一处理，
// 这里保持一致转换，避免 Go 与 SQL 两处裁剪规则不一致。
func realtimeStockText(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// realtimeStockNumber 数值列取值：nil → JSON null；有值时保留原始十进制文本，
// 由数据库按 NUMERIC(24,6) 定点转换（不经过 float64）。
func realtimeStockNumber(value *json.Number) any {
	if value == nil {
		return nil
	}
	return value.String()
}

// realtimeStockInt 整数列取值：nil → JSON null。
func realtimeStockInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// RealtimeStockListFilter 列表查询条件（URL 参数由 handler 解析并校验）。
type RealtimeStockListFilter struct {
	Keyword     string // 匹配 usc / stock_code / name（ILIKE）
	MarketLabel string // 空或 all 表示不限
	Currency    string // 空或 all 表示不限
	OrderBy     string // 见 RealtimeStockOrderColumns，空默认 rmb_market_value（需求 11025）
	Desc        bool   // true 降序（默认），false 升序
	// USCMissing 为 true 时只查 usc 为空的行，即「待录入证券」（需求 10995）。
	USCMissing bool
	Limit      int
	Offset     int
}

// RealtimeStockOrderColumns 允许排序的列白名单（列名直接拼进 SQL，必须先校验）。
var RealtimeStockOrderColumns = map[string]bool{
	"stock_id":                 true,
	"market_value":             true,
	"circulation_market_value": true,
	"change_price":             true,
	"change_ratio":             true,
	"trust_aum":                true,
	"trade_trunover":           true,
	"trade_volumn":             true,
	"volumn_ratio":             true,
	"dt_update":                true,
	// 人民币计价口径（需求 11025，11027 去掉汇率）：只开放「指标」列参与排行；
	// rmb_exchange_rate 是换算参数而非指标，不作为排序字段（仍随行返回用于展示）。
	// 未补全的行（NULL）按既有规则排在最后。
	"rmb_market_value":             true,
	"rmb_circulation_market_value": true,
	"rmb_trade_trunover":           true,
	"rmb_price_nominal":            true,
}

// RealtimeStockListRow 列表行（wire 字段 snake_case）。
//
// 数值列统一以文本输出（SQL 侧 ::text），保住 NUMERIC(24,6) 的精度；
// 展示层的万/亿折算与小数位裁剪由前端负责。
type RealtimeStockListRow struct {
	StockID                int64  `json:"stock_id"`
	USC                    string `json:"usc"`
	Name                   string `json:"name"`
	StockCode              string `json:"stock_code"`
	MarketType             string `json:"market_type"`
	MarketLabel            string `json:"market_label"`
	InstrumentType         string `json:"instrument_type"`
	Currency               string `json:"currency"`
	PriceNominal           string `json:"price_nominal"`
	ChangePrice            string `json:"change_price"`
	ChangeRatio            string `json:"change_ratio"`
	PriceDirect            string `json:"price_direct"`
	MarketValue            string `json:"market_value"`
	TotalShares            string `json:"total_shares"`
	CirculationTotalShares string `json:"circulation_total_shares"`
	TradeTrunover          string `json:"trade_trunover"`
	TradeVolumn            string `json:"trade_volumn"`
	CirculationMarketValue string `json:"circulation_market_value"`
	TrustAum               string `json:"trust_aum"`
	TradeChangeRatio       string `json:"trade_change_ratio"`
	PriceAmplitude         string `json:"price_amplitude"`
	VolumnRatio            string `json:"volumn_ratio"`
	BuysellRatio           string `json:"buysell_ratio"`
	Leverage               string `json:"leverage"`
	// 人民币计价字段（V300126 / 需求 11023，由「人民币计价指标更新」补全，需求 11024/11025）：
	// 空串表示尚未补全（库里为 NULL），不是 0。
	RmbExchangeRate           string `json:"rmb_exchange_rate"`
	RmbMarketValue            string `json:"rmb_market_value"`
	RmbCirculationMarketValue string `json:"rmb_circulation_market_value"`
	RmbTradeTrunover          string `json:"rmb_trade_trunover"`
	RmbPriceNominal           string `json:"rmb_price_nominal"`
	// Remark 备注（需求 11028）：空串表示无备注。
	Remark string `json:"remark"`
	// GmtCreate / GmtUpdate 库侧审计时间（需求 11029，RFC 3339）：
	// 前者入库时刻（不再变化），后者由触发器自动刷新。
	GmtCreate time.Time `json:"gmt_create"`
	GmtUpdate time.Time `json:"gmt_update"`
	DtUpdate  int64     `json:"dt_update"`
}
