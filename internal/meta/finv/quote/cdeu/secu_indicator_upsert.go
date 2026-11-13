package cdeu

// 证券指标 Upsert（需求 11248）。
//
// 场景：采集端（Cloudflare Worker）抓完一只证券后推一条 webhook 过来，服务端一次事务写三张表：
//
//	1. cdeu.finv_quote_secu_indicator —— 指标本体（按 usc / ftmm_symbol 关联，不存在则插入）
//	2. futu.finv_futu_quote_collect   —— 采集参数（同名关联字段全部刷新，不存在则插入）
//	3. futu.finv_futu_secu_key        —— status 置 DONE，采集端后续跳过该证券
//
// usc 解析顺序（需求 11248 用户口径：「usc 查不到的话就基于 ftmm_symbol 字段来关联」）：
//
//	① futu.finv_futu_secu_key.usc（权威来源，73,212 行全部有值；采集目标就是从这张表导出来的）
//	② cdeu.finv_quote_secu_indicator.ftmm_symbol 反查已有行的 usc
//	③ futu.finv_futu_quote_collect.ftmm_symbol 反查已有行的 usc
//	④ 兜底：直接把归一后的 ftmm_symbol 当 usc
//
// 为什么不能只靠 usc 做键：indicator 表 18,019 行里 17,962 行的 usc 是裸代码
// （`SNTETH`），而 ftmm_symbol 是 `CC.SNTETH` —— 采集端只知道后者。
//
// 字段映射口径（需求 11248 用户口径）：
//   - 无对应列的字段（is_option / first_seen_at / updated_at / *_raw 等）**不做处理**，直接丢弃；
//   - 已有行只更新请求里给了值的列（SQL 用 `COALESCE($n, t.col)`），没给的保留原值；
//   - 数值走 json.Number 原文，不经过 float64，避免 numeric(30,10) 丢精度。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	symbolpkg "github.com/acanx/finvquant/internal/meta/finv/quote/symbol"
)

// secuIndicatorUpsertColumns 参与 Upsert 的指标列，顺序即 SQL 参数顺序（$1..$N）。
//
// 不含 `dt_create`：它由服务端在 INSERT 时单独给（UPDATE 不覆盖创建时间），
// 也不含触发器维护的 `dt_update`。
var secuIndicatorUpsertColumns = []string{
	"usc", "ftmm_symbol",
	"name", "lot_size", "spread_code", "ex_type", "price_accuracy",
	"volume_precision", "order_volume_precision",
	"price", "change_price", "change_ratio", "price_direct",
	"price_open", "price_highest", "price_lowest", "price_last_close", "price_nominal",
	"hight_direct", "open_direct", "low_direct",
	"volume", "turnover", "ratio_volume", "ratio_turnover", "amplitude_price",
	"price_average", "change_speed_price", "ratio_bid_ask",
	"total_shares", "total_market_cap", "outstanding_shares", "outstanding_market_cap",
	"price_highest_week52", "price_lowest_week52", "price_highest_history", "price_lowest_history",
	"volume_sold", "volume_bought", "price_limit_up", "price_limit_down",
	"kcb_static_volume", "kcb_static_turnover",
	"pe_ttm", "pe_lyr", "pb_ratio", "eps_ttm", "eps_lyr", "ps_ttm", "ps_lyr",
	"ts", "market_status", "market_status_text", "delay_time",
	"ts_exchange_dt", "ts_server_send_to_client", "ts_server_recv_from_exchange",
	"secu_url", "source_channel", "dt_collect", "count_update", "json_unmapped", "stock_id",
}

// secuIndicatorUpsertTable 目标表名（日志与错误信息复用）。
const secuIndicatorUpsertTable = "cdeu.finv_quote_secu_indicator"

// SecuIndicatorUpsertRequest 采集端推送体。
//
// 结构对应采集端直接发送的内层 JSON（`symbol` / `data` 同级）。
// 同时兼容 Cloudflare Worker 的外层包装（{timestamp, method, url, headers, body, $workers, $metadata}）：
// 若顶层带 `body` 且其中含 `data`，自动取内层（需求 11248 D6）。
type SecuIndicatorUpsertRequest struct {
	Body *SecuIndicatorUpsertRequest `json:"body,omitempty"`

	Timestamp      string `json:"timestamp"`
	Symbol         string `json:"symbol"`
	Market         string `json:"market"`
	Code           string `json:"code"`
	PageType       string `json:"page_type"`
	CollectedAt    string `json:"collected_at"`
	SourceURL      string `json:"source_url"`
	RunID          string `json:"run_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Mode           string `json:"mode"`

	Data *SecuIndicatorPayload `json:"data"`
}

// SecuIndicatorPayload 指标的字段集合（采集端 `data` 节点）。
//
// 只声明「指标表有对应列」或「collect 表需要」的字段；其余字段（is_option、is_futures、
// is_plate、is_etf_theme、first_seen_at、updated_at、underlying_stock_raw、spark_info_raw、
// kcb_stock_static_raw、page_type、en_name）按需求口径直接丢弃，不解析也不落库。
type SecuIndicatorPayload struct {
	Symbol    string `json:"symbol"`
	Market    string `json:"market"`
	StockCode string `json:"stock_code"`

	// StockID 采集端给的是字符串（"46377058462214"），也可能给数字，两种都接受。
	StockID *jsonInt64 `json:"stock_id"`

	Name        *string `json:"name"`
	SpreadCode  *string `json:"spread_code"`
	PriceDirect *string `json:"price_direct"`
	HightDirect *string `json:"hight_direct"`
	OpenDirect  *string `json:"open_direct"`
	LowDirect   *string `json:"low_direct"`

	MarketStatusText *string `json:"market_status_text"`
	SourceURL        *string `json:"source_url"`
	SourceChannel    *string `json:"source_channel"`
	UnmappedJSON     *string `json:"unmapped_json"`

	CollectedAt string `json:"collected_at"`

	// 采集参数（写 futu.finv_futu_quote_collect，不进指标表）
	MarketType        *json.Number `json:"market_type"`
	MarketCode        *json.Number `json:"market_code"`
	InstrumentType    *json.Number `json:"instrument_type"`
	SubInstrumentType *json.Number `json:"sub_instrument_type"`

	// 指标表整型列
	LotSize              *json.Number `json:"lot_size"`
	ExType               *json.Number `json:"ex_type"`
	PriceAccuracy        *json.Number `json:"price_accuracy"`
	VolumePrecision      *json.Number `json:"volume_precision"`
	OrderVolumePrecision *json.Number `json:"order_volume_precision"`
	MarketStatus         *json.Number `json:"market_status"`
	DelayTime            *json.Number `json:"delay_time"`
	UpdateCount          *json.Number `json:"update_count"`

	// 指标表 bigint 列（采集端给毫秒时间戳）
	Time                     *json.Number `json:"time"`
	ExchangeDataTimeMs       *json.Number `json:"exchange_data_time_ms"`
	ServerSendToClientTimeMs *json.Number `json:"server_send_to_client_time_ms"`
	ServerRecvFromExchangeMs *json.Number `json:"server_recv_from_exchange_time_ms"`

	// 指标表 numeric 列
	Price                *json.Number `json:"price"`
	Change               *json.Number `json:"change"`
	ChangeRatio          *json.Number `json:"change_ratio"`
	PriceOpen            *json.Number `json:"price_open"`
	PriceHighest         *json.Number `json:"price_highest"`
	PriceLowest          *json.Number `json:"price_lowest"`
	PriceLastClose       *json.Number `json:"price_last_close"`
	PriceNominal         *json.Number `json:"price_nominal"`
	Volume               *json.Number `json:"volume"`
	Turnover             *json.Number `json:"turnover"`
	RatioVolume          *json.Number `json:"ratio_volume"`
	RatioTurnover        *json.Number `json:"ratio_turnover"`
	AmplitudePrice       *json.Number `json:"amplitude_price"`
	PriceAverage         *json.Number `json:"price_average"`
	ChangeSpeedPrice     *json.Number `json:"change_speed_price"`
	RatioBidAsk          *json.Number `json:"ratio_bid_ask"`
	TotalShares          *json.Number `json:"total_shares"`
	TotalMarketCap       *json.Number `json:"total_market_cap"`
	OutstandingShares    *json.Number `json:"outstanding_shares"`
	OutstandingMarketCap *json.Number `json:"outstanding_market_cap"`
	PriceHighest52Week   *json.Number `json:"price_highest_52week"`
	PriceLowest52Week    *json.Number `json:"price_lowest_52week"`
	PriceHighestHistory  *json.Number `json:"price_highest_history"`
	PriceLowestHistory   *json.Number `json:"price_lowest_history"`
	VolumeSold           *json.Number `json:"volume_sold"`
	VolumeBought         *json.Number `json:"volume_bought"`
	PriceLimitUp         *json.Number `json:"price_limit_up"`
	PriceLimitDown       *json.Number `json:"price_limit_down"`
	KcbStaticVolume      *json.Number `json:"kcb_stock_static_volume"`
	KcbStaticTurnover    *json.Number `json:"kcb_stock_static_turnover"`
	PeTTM                *json.Number `json:"pe_ttm"`
	PeLYR                *json.Number `json:"pe_lyr"`
	PbRatio              *json.Number `json:"pb_ratio"`
	EpsTTM               *json.Number `json:"eps_ttm"`
	EpsLYR               *json.Number `json:"eps_lyr"`
	PsTTM                *json.Number `json:"ps_ttm"`
	PsLYR                *json.Number `json:"ps_lyr"`
}

// SecuIndicatorUpsertResult 写库结果（响应 data 节点）。
type SecuIndicatorUpsertResult struct {
	USC        string `json:"usc"`
	FtmmSymbol string `json:"ftmm_symbol"`
	// USCSource usc 的判定来源：KEY_TABLE / INDICATOR / COLLECT / FTMM_SYMBOL。
	USCSource string `json:"usc_source"`
	// Indicator 指标表动作：INSERTED / UPDATED。
	Indicator string `json:"indicator"`
	// Collect 采集参数表动作：INSERTED / UPDATED / SKIPPED_NO_STOCK_ID / SKIPPED_NO_MARKET_CODE。
	Collect string `json:"collect"`
	// Key key 表动作：DONE / NOT_FOUND。
	Key       string `json:"key"`
	RunID     string `json:"run_id,omitempty"`
	StockID   *int64 `json:"stock_id,omitempty"`
	DtCollect string `json:"dt_collect,omitempty"`
}

// jsonInt64 兼容「字符串或数字」的整数（采集端 stock_id 是字符串）。
type jsonInt64 struct {
	Value int64
	Valid bool
}

// UnmarshalJSON 接受 "46377058462214" / 46377058462214 / null。
func (v *jsonInt64) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if raw == "" || raw == "null" {
		v.Valid = false
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("stock_id 必须是整数，实际 %q", raw)
	}
	v.Value, v.Valid = parsed, true
	return nil
}

// unwrap 取真正的请求体：顶层带 body 且内含 data 时用内层（兼容 CF Worker 外壳）。
func (r *SecuIndicatorUpsertRequest) unwrap() *SecuIndicatorUpsertRequest {
	if r.Body != nil && r.Body.Data != nil {
		return r.Body
	}
	return r
}

// rtmmSymbol 归一后的 ftmm_symbol（优先级：data.symbol → 顶层 symbol）。
func (r *SecuIndicatorUpsertRequest) rtmmSymbol() string {
	if r.Data != nil && strings.TrimSpace(r.Data.Symbol) != "" {
		return symbolpkg.Normalize(r.Data.Symbol)
	}
	return symbolpkg.Normalize(r.Symbol)
}

// sourceURL 来源链接（优先级：data.source_url → 顶层 source_url）。
func (r *SecuIndicatorUpsertRequest) sourceURL() string {
	if r.Data != nil && r.Data.SourceURL != nil && strings.TrimSpace(*r.Data.SourceURL) != "" {
		return strings.TrimSpace(*r.Data.SourceURL)
	}
	return strings.TrimSpace(r.SourceURL)
}

// collectedAt 采集时刻（优先级：data.collected_at → 顶层 collected_at）。
func (r *SecuIndicatorUpsertRequest) collectedAt() string {
	if r.Data != nil && strings.TrimSpace(r.Data.CollectedAt) != "" {
		return strings.TrimSpace(r.Data.CollectedAt)
	}
	return strings.TrimSpace(r.CollectedAt)
}

// marketLabel 市场标签（data.market → data.market 之外的顶层 market），统一大写。
func (r *SecuIndicatorUpsertRequest) marketLabel() string {
	if r.Data != nil && strings.TrimSpace(r.Data.Market) != "" {
		return strings.ToUpper(strings.TrimSpace(r.Data.Market))
	}
	return strings.ToUpper(strings.TrimSpace(r.Market))
}

// ValidateSecuIndicatorUpsert 入参校验（handler 与 service 共用一份口径）。
func ValidateSecuIndicatorUpsert(req *SecuIndicatorUpsertRequest) error {
	if req == nil {
		return fmt.Errorf("%w: 请求体为空", ErrRealtimeStockInvalid)
	}
	req = req.unwrap()
	if req.Data == nil {
		return fmt.Errorf("%w: 缺少 data 节点", ErrRealtimeStockInvalid)
	}
	if req.rtmmSymbol() == "" {
		return fmt.Errorf("%w: 缺少 symbol", ErrRealtimeStockInvalid)
	}
	if mode := strings.TrimSpace(req.Mode); mode != "" && !strings.EqualFold(mode, "upsert") {
		return fmt.Errorf("%w: mode=%q 不支持（当前只接受 upsert）", ErrRealtimeStockInvalid, req.Mode)
	}
	return nil
}

// numericArg numeric 列参数：保留 JSON 原文（string 交给 pgx 的 numeric 编码），避免 float64 丢精度。
func numericArg(n *json.Number) any {
	if n == nil {
		return nil
	}
	raw := strings.TrimSpace(n.String())
	if raw == "" {
		return nil
	}
	return raw
}

// intArg 整型列参数；非整值（如 3.5）视为无效，返回 nil（该列保持原值）。
func intArg(n *json.Number) any {
	if n == nil {
		return nil
	}
	raw := strings.TrimSpace(n.String())
	if raw == "" {
		return nil
	}
	if iv, err := n.Int64(); err == nil {
		return iv
	}
	fv, err := n.Float64()
	if err != nil {
		return nil
	}
	iv := int64(fv)
	if float64(iv) != fv {
		return nil
	}
	return iv
}

// intOrZero 取整数，取不到给 0（用于 collect 表的派生判断）。
func intOrZero(n *json.Number) int {
	value := intArg(n)
	if iv, ok := value.(int64); ok {
		return int(iv)
	}
	return 0
}

// textArg 文本列参数：空白视为未提供（NULL），避免把已有值刷成空串。
func textArg(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

// collectedTime 解析采集时刻；解析失败返回 nil（该列保留原值，不阻断写入）。
func collectedTime(raw string) any {
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts
		}
	}
	return nil
}

// stockIDArg 返回 stock_id 参数（nil 表示请求没给）。
func (p *SecuIndicatorPayload) stockIDArg() any {
	if p == nil || p.StockID == nil || !p.StockID.Valid {
		return nil
	}
	return p.StockID.Value
}

// stockIDValue 返回 stock_id 的可空值（用于 collect 表 NOT NULL 判断与响应回显）。
func (p *SecuIndicatorPayload) stockIDValue() *int64 {
	if p == nil || p.StockID == nil || !p.StockID.Valid {
		return nil
	}
	value := p.StockID.Value
	return &value
}

// indicatorArgs 按 secuIndicatorUpsertColumns 的顺序给值。
//
// 长度必须与 secuIndicatorUpsertColumns 一致（单测 secu_indicator_upsert_test.go 有断言）。
func (p *SecuIndicatorPayload) indicatorArgs(usc, ftmm string, collected any) []any {
	return []any{
		usc, ftmm,
		textArg(p.Name), intArg(p.LotSize), textArg(p.SpreadCode), intArg(p.ExType),
		intArg(p.PriceAccuracy), intArg(p.VolumePrecision), intArg(p.OrderVolumePrecision),
		numericArg(p.Price), numericArg(p.Change), numericArg(p.ChangeRatio), textArg(p.PriceDirect),
		numericArg(p.PriceOpen), numericArg(p.PriceHighest), numericArg(p.PriceLowest),
		numericArg(p.PriceLastClose), numericArg(p.PriceNominal),
		textArg(p.HightDirect), textArg(p.OpenDirect), textArg(p.LowDirect),
		numericArg(p.Volume), numericArg(p.Turnover), numericArg(p.RatioVolume),
		numericArg(p.RatioTurnover), numericArg(p.AmplitudePrice),
		numericArg(p.PriceAverage), numericArg(p.ChangeSpeedPrice), numericArg(p.RatioBidAsk),
		numericArg(p.TotalShares), numericArg(p.TotalMarketCap),
		numericArg(p.OutstandingShares), numericArg(p.OutstandingMarketCap),
		numericArg(p.PriceHighest52Week), numericArg(p.PriceLowest52Week),
		numericArg(p.PriceHighestHistory), numericArg(p.PriceLowestHistory),
		numericArg(p.VolumeSold), numericArg(p.VolumeBought),
		numericArg(p.PriceLimitUp), numericArg(p.PriceLimitDown),
		numericArg(p.KcbStaticVolume), numericArg(p.KcbStaticTurnover),
		numericArg(p.PeTTM), numericArg(p.PeLYR), numericArg(p.PbRatio),
		numericArg(p.EpsTTM), numericArg(p.EpsLYR), numericArg(p.PsTTM), numericArg(p.PsLYR),
		intArg(p.Time), intArg(p.MarketStatus), textArg(p.MarketStatusText), intArg(p.DelayTime),
		intArg(p.ExchangeDataTimeMs), intArg(p.ServerSendToClientTimeMs),
		intArg(p.ServerRecvFromExchangeMs),
		textArg(p.SourceURL), textArg(p.SourceChannel), collected,
		intArg(p.UpdateCount), textArg(p.UnmappedJSON), p.stockIDArg(),
	}
}

// secuIndicatorInsertSQL 生成 INSERT 语句：参数 $1..$N 对应 secuIndicatorUpsertColumns，
// 末位多一个 $N+1 给 dt_create。
func secuIndicatorInsertSQL() string {
	placeholders := make([]string, 0, len(secuIndicatorUpsertColumns)+1)
	for i := range secuIndicatorUpsertColumns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	placeholders = append(placeholders, fmt.Sprintf("$%d", len(secuIndicatorUpsertColumns)+1))
	return fmt.Sprintf("INSERT INTO %s (%s, dt_create) VALUES (%s)",
		secuIndicatorUpsertTable,
		strings.Join(secuIndicatorUpsertColumns, ", "),
		strings.Join(placeholders, ", "))
}

// secuIndicatorUpdateSQL 生成 UPDATE 语句：除 usc 外的列都用
// `COALESCE($n, 列)` —— 请求没给的字段保留原值（需求口径：能关联到的字段更新，未给的不动）。
func secuIndicatorUpdateSQL() string {
	sets := make([]string, 0, len(secuIndicatorUpsertColumns))
	for i, column := range secuIndicatorUpsertColumns {
		if column == "usc" {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = COALESCE($%d, t.%s)", column, i+1, column))
	}
	return fmt.Sprintf("UPDATE %s AS t SET %s WHERE t.usc = $1",
		secuIndicatorUpsertTable, strings.Join(sets, ", "))
}

// resolveSecuIndicatorUSC 按「key 表 → indicator → collect → ftmm_symbol 兜底」定 usc。
//
// 返回 usc 与来源标记，来源写进响应便于采集端对账。
func resolveSecuIndicatorUSC(ctx context.Context, tx pgx.Tx, ftmm string) (string, string, error) {
	var usc string
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(btrim(usc), ''), '')
		   FROM futu.finv_futu_secu_key WHERE ftmm_symbol = $1`, ftmm).Scan(&usc); err == nil && usc != "" {
		return usc, "KEY_TABLE", nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("读取 finv_futu_secu_key 失败: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`SELECT usc FROM cdeu.finv_quote_secu_indicator
		  WHERE ftmm_symbol = $1 ORDER BY dt_update DESC NULLS LAST LIMIT 1`, ftmm).Scan(&usc); err == nil && usc != "" {
		return usc, "INDICATOR", nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("按 ftmm_symbol 读取 cdeu.finv_quote_secu_indicator 失败: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`SELECT usc FROM futu.finv_futu_quote_collect
		  WHERE ftmm_symbol = $1 LIMIT 1`, ftmm).Scan(&usc); err == nil && usc != "" {
		return usc, "COLLECT", nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("按 ftmm_symbol 读取 futu.finv_futu_quote_collect 失败: %w", err)
	}

	// 兜底：三处都没有时直接把 ftmm_symbol 当 usc（用户口径：基于 ftmm_symbol 关联）。
	return ftmm, "FTMM_SYMBOL", nil
}

// upsertSecuIndicator 写指标表：先按 usc 查，有则 UPDATE（只覆盖给了值的列），无则 INSERT。
func upsertSecuIndicator(ctx context.Context, tx pgx.Tx, usc, ftmm string,
	payload *SecuIndicatorPayload, collected any) (string, error) {
	args := payload.indicatorArgs(usc, ftmm, collected)

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM cdeu.finv_quote_secu_indicator WHERE usc = $1)`,
		usc).Scan(&exists); err != nil {
		return "", fmt.Errorf("查询指标表是否存在失败: %w", err)
	}

	if exists {
		if _, err := tx.Exec(ctx, secuIndicatorUpdateSQL(), args...); err != nil {
			return "", fmt.Errorf("更新 %s 失败: %w", secuIndicatorUpsertTable,
				explainUniqueViolation(err, secuIndicatorUpsertTable))
		}
		return "UPDATED", nil
	}

	// INSERT：dt_create 与 dt_collect 同源（都没有就 now()）。
	dtCreate := collected
	if dtCreate == nil {
		dtCreate = time.Now()
	}
	if _, err := tx.Exec(ctx, secuIndicatorInsertSQL(), append(args, dtCreate)...); err != nil {
		return "", fmt.Errorf("插入 %s 失败: %w", secuIndicatorUpsertTable,
			explainUniqueViolation(err, secuIndicatorUpsertTable))
	}
	return "INSERTED", nil
}

// secuCollectArgs collect 表的同名关联字段。
type secuCollectArgs struct {
	StockID           any
	FutuSymbol        string
	MarketType        any
	MarketCode        any
	InstrumentType    any
	SubInstrumentType any
	MarketLabel       string
	Instrument        string
}

// deriveCollectArgs 由 payload 推出采集参数；缺失项按已有缺省推断函数补齐。
func (p *SecuIndicatorPayload) deriveCollectArgs(marketLabel string) secuCollectArgs {
	instrumentType := normalizeInstrumentType(intOrZero(p.InstrumentType))

	// futu_symbol 是「不带市场标签的富途代码」：优先用 stock_code，
	// 缺失时从 ftmm_symbol 去掉 "市场标签." 前缀（`CC.SNTETH` → `SNTETH`）。
	futuSymbol := strings.TrimSpace(p.StockCode)
	if futuSymbol == "" {
		ftmm := p.rtmmSymbolRaw()
		if dot := strings.Index(ftmm, "."); dot >= 0 {
			futuSymbol = ftmm[dot+1:]
		} else {
			futuSymbol = ftmm
		}
	}

	subType := intOrZero(p.SubInstrumentType)
	if subType <= 0 {
		subType = subInstrumentType(instrumentType)
	}
	marketType := intOrZero(p.MarketType)
	if marketType <= 0 {
		marketType = marketTypeByLabel(marketLabel)
	}

	return secuCollectArgs{
		StockID:           p.stockIDArg(),
		FutuSymbol:        futuSymbol,
		MarketType:        marketType,
		MarketCode:        intArg(p.MarketCode),
		InstrumentType:    instrumentType,
		SubInstrumentType: subType,
		MarketLabel:       marketLabel,
		Instrument:        instrumentByInstrumentType(instrumentType),
	}
}

// rtmmSymbolRaw 归一后的 ftmm_symbol（不做空判断，供 deriveCollectArgs 拆前缀）。
func (p *SecuIndicatorPayload) rtmmSymbolRaw() string {
	return symbolpkg.Normalize(p.Symbol)
}

// upsertSecuCollect 写采集参数表。
//
// 口径（需求 11248 D3）：能关联到的同名字段全部更新；请求没给的保持原值（列 NOT NULL 的除外）。
// 两个前置条件不满足时跳过而不是报错：
//   - 无 stock_id 且表中无该行 → 新插入会撞 NOT NULL，跳过（其余两表照写）；
//   - 无 market_code 且表中无该行 → 同上，按同市场众数兜底，仍取不到才跳过。
func upsertSecuCollect(ctx context.Context, tx pgx.Tx, usc string, args secuCollectArgs,
	sourceURL, runID string) (string, error) {
	var existingUSC string
	err := tx.QueryRow(ctx,
		`SELECT usc FROM futu.finv_futu_quote_collect
		  WHERE usc = $1 OR ftmm_symbol = $2 LIMIT 1`, usc, args.MarketLabel+"."+args.FutuSymbol).Scan(&existingUSC)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("查询 futu.finv_futu_quote_collect 失败: %w", err)
	}

	if existingUSC != "" {
		if _, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_quote_collect AS t SET
       stock_id            = COALESCE($2, t.stock_id),
       futu_symbol         = COALESCE(NULLIF($3, ''), t.futu_symbol),
       market_type         = COALESCE($4, t.market_type),
       market_code         = COALESCE($5, t.market_code),
       instrument_type     = COALESCE($6, t.instrument_type),
       sub_instrument_type = COALESCE($7, t.sub_instrument_type),
       market_label        = COALESCE(NULLIF($8, ''), t.market_label),
       instrument          = COALESCE(NULLIF($9, ''), t.instrument)
 WHERE t.usc = $1`,
			existingUSC, args.StockID, args.FutuSymbol, args.MarketType, args.MarketCode,
			args.InstrumentType, args.SubInstrumentType, args.MarketLabel, args.Instrument); err != nil {
			return "", fmt.Errorf("更新 futu.finv_futu_quote_collect 失败: %w",
				explainUniqueViolation(err, "futu.finv_futu_quote_collect"))
		}
		return "UPDATED", nil
	}

	if args.StockID == nil {
		return "SKIPPED_NO_STOCK_ID", nil
	}
	marketCode := args.MarketCode
	if marketCode == nil {
		var reference int
		if err := tx.QueryRow(ctx, `
SELECT market_code FROM futu.finv_futu_quote_collect
 WHERE market_label = $1 AND market_code > 0
 GROUP BY market_code ORDER BY count(*) DESC LIMIT 1`, args.MarketLabel).Scan(&reference); err == nil && reference > 0 {
			marketCode = reference
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("推断 market_code 失败: %w", err)
		}
	}
	if marketCode == nil {
		return "SKIPPED_NO_MARKET_CODE", nil
	}

	remark := "OpenAPI " + strings.TrimSpace(sourceURL)
	if runID != "" {
		remark = "OpenAPI run_id=" + strings.TrimSpace(runID)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_quote_collect AS t
       (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
        sub_instrument_type, market_label, instrument, flag_enable, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, '1', $10)
ON CONFLICT (usc) DO NOTHING`,
		usc, args.StockID, args.FutuSymbol, args.MarketType, marketCode,
		args.InstrumentType, args.SubInstrumentType, args.MarketLabel, args.Instrument, remark); err != nil {
		return "", fmt.Errorf("插入 futu.finv_futu_quote_collect 失败: %w",
			explainUniqueViolation(err, "futu.finv_futu_quote_collect"))
	}
	return "INSERTED", nil
}

// markSecuKeyDone 把 key 表该证券置 DONE（采集端后续跳过）。
//
// 需求口径：已是 DONE 也照写（刷新 gmt_update，能反映最后一次采集时间）；
// stock_id / url 有值才覆盖（不回写 usc —— 它是本接口的 usc 来源）。
func markSecuKeyDone(ctx context.Context, tx pgx.Tx, ftmm string, stockID any, url string) (string, error) {
	tag, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_secu_key SET
       status   = 'DONE',
       stock_id = COALESCE($2, stock_id),
       url      = COALESCE(NULLIF($3, ''), url)
 WHERE ftmm_symbol = $1`, ftmm, stockID, url)
	if err != nil {
		return "", fmt.Errorf("更新 futu.finv_futu_secu_key 失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "NOT_FOUND", nil
	}
	return "DONE", nil
}

// UpsertSecuIndicator 写三张表（单事务）。
//
// 事务边界故意包住三张表：任意一张失败整体回滚，避免出现「指标写了但 key 没置 DONE，
// 采集端下次又抓一遍」或「key 置了 DONE 但指标没落库，证券被永久跳过」的半成品状态。
func (s *Store) UpsertSecuIndicator(ctx context.Context,
	req *SecuIndicatorUpsertRequest) (SecuIndicatorUpsertResult, error) {
	var result SecuIndicatorUpsertResult
	if err := ValidateSecuIndicatorUpsert(req); err != nil {
		return result, err
	}
	req = req.unwrap()

	ftmm := req.rtmmSymbol()
	payload := req.Data
	result.FtmmSymbol = ftmm
	result.RunID = strings.TrimSpace(req.RunID)
	result.StockID = payload.stockIDValue()

	sourceURL := req.sourceURL()
	collected := collectedTime(req.collectedAt())
	if ts, ok := collected.(time.Time); ok {
		result.DtCollect = ts.Format(time.RFC3339)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("UpsertSecuIndicator：开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	usc, uscSource, err := resolveSecuIndicatorUSC(ctx, tx, ftmm)
	if err != nil {
		return result, err
	}
	result.USC = usc
	result.USCSource = uscSource

	indicatorAction, err := upsertSecuIndicator(ctx, tx, usc, ftmm, payload, collected)
	if err != nil {
		return result, err
	}
	result.Indicator = indicatorAction

	collectAction, err := upsertSecuCollect(ctx, tx, usc,
		payload.deriveCollectArgs(req.marketLabel()), sourceURL, result.RunID)
	if err != nil {
		return result, err
	}
	result.Collect = collectAction

	keyAction, err := markSecuKeyDone(ctx, tx, ftmm, payload.stockIDArg(), sourceURL)
	if err != nil {
		return result, err
	}
	result.Key = keyAction

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("UpsertSecuIndicator：提交事务失败: %w", err)
	}
	return result, nil
}
