package cdeu

// 收藏的美股/港股指数行情快照采集（需求 11333）。
//
// 链路：quote.finv_quote_secu_fav（收藏表）→ public.finv_security / finv_exchange
// 过滤出 security_type ∈ {StockIndex, Index} 且 exchange_flag ∈ {HK, US} 的指数 →
// 拼 moomoo code（港股 `HK.<security_code>`、美股 `US..<security_code>`）→
// moomoo OpenAPI `POST /api/v1.0/quote/snapshot`（一次最多 400 个）→ 字段/类型转换 →
// 按 usc upsert 到 cdeu.finv_quote_secu_realtime（有值才覆盖，见 quoteSecuRealtimeUpdateClause）。
//
// moomoo code 形式（实测 2026-11-18）：
//   - 港股指数用 `HK.<代码>`，如 `HK.800000`（恒生指数）、`HK.800700`（恒生科技指数）；
//   - 美股指数用 **双点** `US..<代码>`，如 `US..DJI` / `US..NDX` / `US..SPX` / `US..VIX`；
//     单点形态 `US.DJI` 查不到（会被当成同代码的 ETF，`US.DJIA` 就是 ETF）；
//   - 港股别名形态 `HK.HSI` / `HK.HSTECH` 不受支持，必须用官方数字代码。
//
// 凭据复用「加密货币」Tab 保存的 moomoo 配置（sys.sys_config 20016，见 moomoo_config.go）。
//
// 失败语义：单个批次失败即返回错误（不写半批）；收藏表里没有美股/港股指数时返回空跑结果、不报错。

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// 指数收藏采集的业务常量。
const (
	// IndexFavSymbol 前端 Tab 符号（instrument_symbol=index，需求 11333）。
	IndexFavSymbol = "index"
	// IndexFavProvider 指数快照落库的 provider。
	IndexFavProvider = "MMAPI"
	// IndexFavInstrumentType 指数在富途字典里的 instrument_type（6=指数）。
	IndexFavInstrumentType = "6"
	// MoomooHKIndexCodePrefix 港股指数 code 前缀（HK.800000）。
	MoomooHKIndexCodePrefix = "HK."
	// MoomooUSIndexCodePrefix 美股指数 code 前缀（双点：US..DJI）。
	MoomooUSIndexCodePrefix = "US.."
	// indexFavBatchSize 单批请求 code 数（与 moomoo 接口上限一致，不开放配置）。
	indexFavBatchSize = webapi.MaxCodes
	// indexFavDbBatchSize 单批落库行数（控制单条 SQL 参数数量）。
	indexFavDbBatchSize = 200
	// indexFavDefaultTimeout 一次采集的默认总超时。
	indexFavDefaultTimeout = 60 * time.Second
)

// IndexFavTarget 收藏表里的一个美股/港股指数（拼 code 与落库都用它）。
type IndexFavTarget struct {
	// USC public.finv_security.usc（收藏表主键，落库主键）。
	USC string
	// SecurityCode 官方代码（港股 800000，美股 DJI），拼 moomoo code 用。
	SecurityCode string
	// ExchangeFlag 交易所标志（HK / US）。
	ExchangeFlag string
	// Currency 计价货币（currency_type，缺省按交易所兜底）。
	Currency string
}

// MoomooCode 拼 moomoo OpenAPI 的 code；不支持的交易所返回空串。
func (t IndexFavTarget) MoomooCode() string {
	code := strings.TrimSpace(t.SecurityCode)
	if code == "" {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(t.ExchangeFlag)) {
	case "HK":
		return MoomooHKIndexCodePrefix + strings.ToUpper(code)
	case "US":
		return MoomooUSIndexCodePrefix + strings.ToUpper(code)
	}
	return ""
}

// indexFavMarket 交易所 → 落库用的 (market_type, market_label)。
//
// 与 cdeu.finv_quote_secu_realtime 现有数据同口径（实测：美股 2/US、港股 1/HK）。
func (t IndexFavTarget) market() (marketType, marketLabel string, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(t.ExchangeFlag)) {
	case "HK":
		return "1", "HK", true
	case "US":
		return "2", "US", true
	}
	return "", "", false
}

// IndexFavSnapshotCollectOptions 采集控制参数（零值按默认值补齐）。
type IndexFavSnapshotCollectOptions struct {
	// Timeout 本次采集总超时，0 取 indexFavDefaultTimeout。
	Timeout time.Duration
}

// normalize 补齐默认值。
func (o IndexFavSnapshotCollectOptions) normalize() IndexFavSnapshotCollectOptions {
	if o.Timeout <= 0 {
		o.Timeout = indexFavDefaultTimeout
	}
	return o
}

// IndexFavSnapshotResult 一次采集的结果统计。
type IndexFavSnapshotResult struct {
	// USCTotal 收藏表里的美股/港股指数数量（拼出 code 之前）。
	USCTotal int
	// CodesRequested 实际请求的 code 数（去重后）。
	CodesRequested int
	// RowsReceived 接口返回的快照行数。
	RowsReceived int
	// RowsInsert / RowsUpdate 新增 / 覆盖行数。
	RowsInsert int64
	RowsUpdate int64
	// Batches 请求批次数。
	Batches int
	// Missing 请求了但接口没返回的 code（多为上游未覆盖的指数，不报错）。
	Missing []string
	// ElapsedMs 总耗时。
	ElapsedMs int64
}

// IndexFavSnapshotStore 采集器依赖的存储能力（Store 实现；测试可注入）。
type IndexFavSnapshotStore interface {
	ListFavoriteExchangeIndexTargets(ctx context.Context) ([]IndexFavTarget, error)
	SaveQuoteSecuRealtimeRows(ctx context.Context, rows []QuoteSecuRealtimeRow) (int64, int64, error)
}

// IndexFavSnapshotCollector 收藏的美股/港股指数快照采集器。
type IndexFavSnapshotCollector struct {
	fetcher webapi.Fetcher
	store   IndexFavSnapshotStore
	logger  *log.Logger
	hook    StockListCollectHook
}

// NewIndexFavSnapshotCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewIndexFavSnapshotCollector(
	fetcher webapi.Fetcher, store IndexFavSnapshotStore, logger *log.Logger,
) *IndexFavSnapshotCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &IndexFavSnapshotCollector{fetcher: fetcher, store: store, logger: logger}
}

// WithHook 设置过程事件回调（链式；供运行日志面板消费）。
func (c *IndexFavSnapshotCollector) WithHook(hook StockListCollectHook) *IndexFavSnapshotCollector {
	c.hook = hook
	return c
}

// emit 发送一条过程事件。
func (c *IndexFavSnapshotCollector) emit(stage string, pages, rows int, inserted, updated int64, message string) {
	if c.hook == nil {
		return
	}
	c.hook(StockListCollectEvent{
		Stage: stage,
		Config: StockListCollectConfig{
			InstrumentSymbol: IndexFavSymbol,
		},
		Page:    pages,
		Rows:    rows,
		Insert:  inserted,
		Update:  updated,
		Message: message,
		Time:    time.Now(),
	})
}

// Collect 执行一次采集。
func (c *IndexFavSnapshotCollector) Collect(
	ctx context.Context, opts IndexFavSnapshotCollectOptions,
) (IndexFavSnapshotResult, error) {
	var result IndexFavSnapshotResult
	opts = opts.normalize()
	started := time.Now()

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	targets, err := c.store.ListFavoriteExchangeIndexTargets(runCtx)
	if err != nil {
		return result, err
	}
	result.USCTotal = len(targets)
	c.emit("begin", 0, 0, 0, 0,
		fmt.Sprintf("开始采集收藏的美股/港股指数行情快照（provider=%s，收藏=%d）", IndexFavProvider, len(targets)))
	if len(targets) == 0 {
		c.logger.Printf("[cdeu][index] 收藏表里没有美股/港股指数，跳过采集")
		c.emit("page", 0, 0, 0, 0, "收藏表里没有美股/港股指数，跳过采集")
		return result, nil
	}

	// code → 目标（同一 code 只请求一次；返回行按 code 反查 usc，不靠 code 反推）。
	codeTargets := make(map[string]IndexFavTarget, len(targets))
	codes := make([]string, 0, len(targets))
	for _, target := range targets {
		code := target.MoomooCode()
		if code == "" {
			c.logger.Printf("[cdeu][index] 跳过无法拼 code 的收藏指数：usc=%s exchange=%s code=%s",
				target.USC, target.ExchangeFlag, target.SecurityCode)
			continue
		}
		if _, exists := codeTargets[code]; exists {
			continue
		}
		codeTargets[code] = target
		codes = append(codes, code)
	}
	result.CodesRequested = len(codes)
	if len(codes) == 0 {
		c.emit("page", 0, 0, 0, 0, "收藏的指数都不支持 moomoo code（仅支持港股/美股）")
		return result, nil
	}

	for start := 0; start < len(codes); start += indexFavBatchSize {
		if err := runCtx.Err(); err != nil {
			return result, err
		}
		end := start + indexFavBatchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[start:end]
		snapshots, err := c.fetcher.GetQuoteSnapshots(runCtx, batch)
		if err != nil {
			c.emit("retry", start/indexFavBatchSize+1, 0, 0, 0,
				fmt.Sprintf("第 %d 批拉取失败：%v", start/indexFavBatchSize+1, err))
			return result, fmt.Errorf("拉取美股/港股指数快照失败（第 %d 批）: %w", start/indexFavBatchSize+1, err)
		}
		result.Batches++
		result.RowsReceived += len(snapshots)

		now := time.Now().UnixMilli()
		rows := make([]QuoteSecuRealtimeRow, 0, len(snapshots))
		returned := make(map[string]bool, len(snapshots))
		for _, snapshot := range snapshots {
			code := strings.ToUpper(strings.TrimSpace(snapshot.Code))
			returned[code] = true
			target, ok := codeTargets[code]
			if !ok {
				c.logger.Printf("[cdeu][index] 忽略未请求的 code：%s", snapshot.Code)
				continue
			}
			row, ok := indexFavRealtimeRow(target, snapshot, now)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		for _, code := range batch {
			if !returned[code] {
				result.Missing = append(result.Missing, code)
			}
		}
		if len(rows) == 0 {
			continue
		}

		for dbStart := 0; dbStart < len(rows); dbStart += indexFavDbBatchSize {
			dbEnd := dbStart + indexFavDbBatchSize
			if dbEnd > len(rows) {
				dbEnd = len(rows)
			}
			inserted, updated, saveErr := c.store.SaveQuoteSecuRealtimeRows(runCtx, rows[dbStart:dbEnd])
			result.RowsInsert += inserted
			result.RowsUpdate += updated
			if saveErr != nil {
				return result, saveErr
			}
			c.emit("page", start/indexFavBatchSize+1, dbEnd-dbStart, inserted, updated,
				fmt.Sprintf("第 %d 批：codes=%d rows=%d 新增=%d 覆盖=%d",
					start/indexFavBatchSize+1, len(batch), dbEnd-dbStart, inserted, updated))
		}
		c.logger.Printf("[cdeu][index] 第 %d 批：codes=%d rows=%d", start/indexFavBatchSize+1, len(batch), len(rows))
	}

	if len(result.Missing) > 0 {
		c.logger.Printf("[cdeu][index] 接口未返回 %d 个 code（上游未覆盖）：%s",
			len(result.Missing), strings.Join(result.Missing, ", "))
		c.emit("retry", result.Batches, 0, result.RowsInsert, result.RowsUpdate,
			fmt.Sprintf("接口未返回 %d 个 code（上游未覆盖）：%s",
				len(result.Missing), strings.Join(result.Missing, ", ")))
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	c.logger.Printf("[cdeu][index] 采集完成：收藏指数=%d codes=%d rows=%d 新增=%d 覆盖=%d 批次数=%d 耗时=%dms",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, result.Batches, result.ElapsedMs)
	return result, nil
}

// roundFloat 按给定小数位四舍五入（消除浮点相减/相除的尾差）。
func roundFloat(v float64, digits int) float64 {
	if digits <= 0 {
		return math.Round(v)
	}
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// indexFavRealtimeRow 把一条 moomoo 指数快照转成 cdeu.finv_quote_secu_realtime 的一行。
//
// 字段映射（其余列无来源，写 NULL —— 落库是「有值才覆盖」，不会清掉库里已有值）：
//
//	usc             ← 收藏表 usc（主键）
//	name            ← 快照 name
//	stock_code      ← finv_security.security_code（港股 800000 / 美股 DJI）
//	market_type     ← HK→"1"、US→"2"
//	market_label    ← HK / US
//	instrument_type ← "6"（指数，富途字典）
//	currency        ← finv_security.currency_type（缺省 HKD / USD）
//	price_nominal   ← last_price
//	change_price    ← last_price − prev_close_price
//	change_ratio    ← 上式 / prev_close_price（表内是小数比例，不是百分数）
//	price_amplitude ← amplitude / 100（表内是小数比例）
//	volumn_ratio    ← volume_ratio（倍数，与 ZXET 同口径）
//	buysell_ratio   ← bid_ask_ratio / 100（表内是小数比例）
//	trade_volumn    ← volume
//	trade_trunover  ← turnover
//	dt_update       ← 采集时刻（Unix 毫秒）
//	provider        ← "MMAPI"
//	remark          ← 来源与口径说明
func indexFavRealtimeRow(target IndexFavTarget, snapshot webapi.Snapshot, dtUpdateMs int64) (QuoteSecuRealtimeRow, bool) {
	marketType, marketLabel, ok := target.market()
	if !ok {
		return QuoteSecuRealtimeRow{}, false
	}
	usc := strings.TrimSpace(target.USC)
	if usc == "" {
		return QuoteSecuRealtimeRow{}, false
	}

	name := snapshot.Name
	if strings.TrimSpace(name) == "" {
		name = usc
	}
	stockCode := strings.TrimSpace(target.SecurityCode)
	instrumentType := IndexFavInstrumentType
	currency := strings.TrimSpace(target.Currency)
	if currency == "" {
		if marketLabel == "HK" {
			currency = "HKD"
		} else {
			currency = "USD"
		}
	}
	dtUpdate := dtUpdateMs
	remark := fmt.Sprintf("来源 MMAPI（moomoo code %s，交易所 %s；指数快照按 usc 覆盖，涨跌额/涨跌幅由 last 与 prev_close 计算）",
		target.MoomooCode(), strings.ToUpper(target.ExchangeFlag))

	out := QuoteSecuRealtimeRow{
		USC:            usc,
		Name:           &name,
		StockCode:      &stockCode,
		MarketType:     &marketType,
		MarketLabel:    &marketLabel,
		InstrumentType: &instrumentType,
		Currency:       &currency,
		DtUpdate:       &dtUpdate,
		Remark:         &remark,
		Provider:       IndexFavProvider,
	}

	// 无行情（last 为 0）时只写身份列，行情列留 NULL（不覆盖库里已有值）。
	if snapshot.LastPrice == 0 {
		return out, true
	}
	out.PriceNominal = cryptoNumber(snapshot.LastPrice)
	out.TradeVolumn = cryptoNumber(snapshot.Volume)
	out.TradeTrunover = cryptoNumber(snapshot.Turnover)
	out.VolumnRatio = cryptoNumber(snapshot.VolumeRatio)
	out.PriceAmplitude = cryptoNumber(roundFloat(snapshot.Amplitude/100, 8))
	out.BuysellRatio = cryptoNumber(roundFloat(snapshot.BidAskRatio/100, 8))

	if snapshot.PrevClosePrice != 0 {
		change := snapshot.LastPrice - snapshot.PrevClosePrice
		// 浮点相减会带尾差（23294.75-23508.17=-213.41999999999825），落 NUMERIC 前按
		// 价格精度四舍五入；涨跌幅是小数比例，多留两位。
		out.ChangePrice = cryptoNumber(roundFloat(change, 6))
		out.ChangeRatio = cryptoNumber(roundFloat(change/snapshot.PrevClosePrice, 8))
		direct := "="
		switch {
		case change > 0:
			direct = "+"
		case change < 0:
			direct = "-"
		}
		out.PriceDirect = &direct
	}
	return out, true
}
