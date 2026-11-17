package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/acnxo/MetaExt/component/quote/citics/xetapp"
)

// 本文件实现「沪深京指数」实时行情采集（需求 11294）：
// 调 MetaExt `xetapp.GetQuoteSnapshotCNAIndex` 拉全量指数 → 内部码映射成 usc →
// 字段转换后 upsert 到 cdeu.finv_quote_secu_realtime（主键 usc，provider='ZXET'）。
//
// 与 BDFS 采集的关系：
//   - 落库表与 sink/resolver 完全复用（QuoteSecuRealtimeSink / SecuUscResolver）；
//   - 行情字段口径一致（涨跌幅等按小数比例、成交量按股）；
//   - 差异：指数没有分页（一次 6 池合并），没有配置表行（固定来源），
//     usc 优先查 public.finv_security，未登记时按编码域构造（见 xet_index_code_mapping.go）。
//
// 池：默认 = 上游 4 池（中证 1100 / 国证 / 股转 / 大盘） + 深市 1200 + 北交所 1400。
// 上游 4 池覆盖 879 只；补 1200/1400 后覆盖 1239 只（`IndexCodeMapping.md` §A）。

const (
	// XetIndexDbBatchSize 单次落库行数（控制单条 SQL 参数数量）。
	XetIndexDbBatchSize = 200
	// XetIndexDefaultTimeout 一次采集的总超时（6 次请求，串行约 2~4 秒）。
	XetIndexDefaultTimeout = 120 * time.Second
	// XetIndexUnknownLogLimit 「映射表里没有的接口码」逐条日志上限。
	XetIndexUnknownLogLimit = 50
)

// XetIndexSnapshotFetcher 指数快照拉取能力（默认走 MetaExt xetapp；测试可注入）。
type XetIndexSnapshotFetcher interface {
	GetQuoteSnapshotCNAIndex(ctx context.Context, req xetapp.IndexSnapshotRequest) (*xetapp.IndexSnapshotResult, error)
}

// XetIndexDefaultFetcher 基于 MetaExt component/quote/citics/xetapp 的默认实现。
type XetIndexDefaultFetcher struct {
	client *xetapp.Client
}

// NewXetIndexFetcher 创建默认拉取实现（生产环境用无参版本）。
func NewXetIndexFetcher() XetIndexSnapshotFetcher {
	return newXetIndexFetcher(xetapp.Config{})
}

// newXetIndexFetcher 按给定配置创建（单测注入 GatewayURL 用）。
func newXetIndexFetcher(cfg xetapp.Config) *XetIndexDefaultFetcher {
	return &XetIndexDefaultFetcher{client: xetapp.New(cfg)}
}

// GetQuoteSnapshotCNAIndex 拉取沪深京全量指数快照。
func (f *XetIndexDefaultFetcher) GetQuoteSnapshotCNAIndex(ctx context.Context, req xetapp.IndexSnapshotRequest) (*xetapp.IndexSnapshotResult, error) {
	return f.client.GetQuoteSnapshotCNAIndex(ctx, req)
}

// XetIndexCollectOptions 指数采集控制参数（零值按默认值补齐）。
type XetIndexCollectOptions struct {
	// MaxCount 每个池的返回行数上限，0 取上游默认值。
	MaxCount int
	// Timeout 本次采集总超时，0 取 XetIndexDefaultTimeout。
	Timeout time.Duration
	// Boards 自定义池；为空使用默认 6 池（上游 4 池 + 深市 1200 + 北交所 1400）。
	Boards []xetapp.IndexBoard
}

// normalize 补齐默认值。
func (o XetIndexCollectOptions) normalize() XetIndexCollectOptions {
	if o.Timeout <= 0 {
		o.Timeout = XetIndexDefaultTimeout
	}
	if len(o.Boards) == 0 {
		o.Boards = xetIndexDefaultBoards()
	}
	return o
}

// xetIndexDefaultBoards 默认池：上游 4 池 + 深市 1200 + 北交所 1400。
func xetIndexDefaultBoards() []xetapp.IndexBoard {
	boards := make([]xetapp.IndexBoard, 0, len(xetapp.IndexBoards)+2)
	boards = append(boards, xetapp.IndexBoards...)
	boards = append(boards,
		xetapp.IndexBoard{Name: "深市指数", Action: xetapp.ActionQuoteSnapshotByMarket, Board: "1200"},
		xetapp.IndexBoard{Name: "北交所指数", Action: xetapp.ActionQuoteSnapshotByMarket, Board: "1400"},
	)
	return boards
}

// XetIndexCollector 指数采集器（无状态，可并发调用）。
type XetIndexCollector struct {
	fetcher  XetIndexSnapshotFetcher
	resolver SecuUscResolver
	sink     QuoteSecuRealtimeSink
	logger   *log.Logger
	hook     StockListCollectHook
}

// NewXetIndexCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewXetIndexCollector(fetcher XetIndexSnapshotFetcher, resolver SecuUscResolver, sink QuoteSecuRealtimeSink, logger *log.Logger) *XetIndexCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &XetIndexCollector{fetcher: fetcher, resolver: resolver, sink: sink, logger: logger}
}

// WithHook 设置过程事件回调（链式）。
func (c *XetIndexCollector) WithHook(hook StockListCollectHook) *XetIndexCollector {
	c.hook = hook
	return c
}

// emit 发送一条过程事件（供运行日志面板消费）。
func (c *XetIndexCollector) emit(stage string, pools, rows int, inserted, updated int64, message string) {
	if c.hook == nil {
		return
	}
	c.hook(StockListCollectEvent{
		Stage: stage,
		Config: StockListCollectConfig{
			InstrumentSymbol: XetIndexSymbol,
			MarketType:       XetIndexMarketType,
		},
		Page:    pools,
		Rows:    rows,
		Insert:  inserted,
		Update:  updated,
		Message: message,
		Time:    time.Now(),
	})
}

// Collect 执行一次完整采集：拉全量指数 → 映射 usc → 分批 upsert → 返回统计。
func (c *XetIndexCollector) Collect(ctx context.Context, opts XetIndexCollectOptions) (StockListCollectResult, error) {
	opts = opts.normalize()
	started := time.Now()
	result := StockListCollectResult{InstrumentSymbol: XetIndexSymbol, MarketType: XetIndexMarketType}

	finish := func(err error) (StockListCollectResult, error) {
		result.ElapsedMs = time.Since(started).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			c.emit("done", result.Pages, 0, result.RowsInsert, result.RowsUpdate, "采集失败："+err.Error())
			return result, err
		}
		c.emit("done", result.Pages, int(result.RowsReceived), result.RowsInsert, result.RowsUpdate,
			fmt.Sprintf("采集完成：%d 池 / %d 条，新增 %d、覆盖 %d",
				result.Pages, result.RowsReceived, result.RowsInsert, result.RowsUpdate))
		return result, nil
	}

	reqCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	c.emit("begin", len(opts.Boards), 0, 0, 0, fmt.Sprintf("开始采集沪深京指数（provider=%s）", XetIndexProvider))

	mapping, err := XetIndexCodeMapping()
	if err != nil {
		return finish(fmt.Errorf("加载指数代码映射失败: %w", err))
	}

	fetched, err := c.fetcher.GetQuoteSnapshotCNAIndex(reqCtx, xetapp.IndexSnapshotRequest{
		Boards:   opts.Boards,
		MaxCount: opts.MaxCount,
	})
	if err != nil {
		return finish(fmt.Errorf("拉取沪深京指数失败: %w", err))
	}
	if fetched == nil || len(fetched.Rows) == 0 {
		return finish(fmt.Errorf("沪深京指数未返回任何数据"))
	}
	result.Pages = len(fetched.Pools)
	if result.Pages == 0 {
		result.Pages = len(opts.Boards)
	}
	result.RowsReceived = int64(len(fetched.Rows))
	result.CountExists = len(fetched.Rows)
	result.TotalAvailable = len(fetched.Rows)
	for _, pool := range fetched.Pools {
		c.logger.Printf("[cdeu][xetindex] pool=%s action=%s board=%s max=%d fetched=%d added=%d",
			pool.Name, pool.Action, pool.Board, pool.MaxCount, pool.Fetched, pool.Added)
	}
	c.emit("page", result.Pages, len(fetched.Rows), 0, 0,
		fmt.Sprintf("拉取完成：%d 池 / %d 条（去重后）", result.Pages, len(fetched.Rows)))

	// 先批量解析 usc：只查有交易所键的行（SH/SZ/BJ），非交易所域直接构造。
	entries := make([]XetIndexCodeEntry, 0, len(fetched.Rows))
	rows := make([]xetapp.Row, 0, len(fetched.Rows))
	keys := make([]SecuUscKey, 0, len(fetched.Rows))
	seenKey := make(map[SecuUscKey]struct{}, len(fetched.Rows))
	unknown := make([]string, 0)
	for _, row := range fetched.Rows {
		entry, ok := mapping[row.Code]
		if !ok {
			unknown = append(unknown, row.Code)
			continue
		}
		entries = append(entries, entry)
		rows = append(rows, row)
		if key, ok := XetIndexSecuKey(entry); ok {
			if _, dup := seenKey[key]; !dup {
				seenKey[key] = struct{}{}
				keys = append(keys, key)
			}
		}
	}
	if len(unknown) > 0 {
		for i, code := range unknown {
			if i >= XetIndexUnknownLogLimit {
				c.emit("retry", 0, 0, 0, 0,
					fmt.Sprintf("其余 %d 条未知接口码不再逐条打印", len(unknown)-XetIndexUnknownLogLimit))
				break
			}
			c.logger.Printf("[cdeu][xetindex] unknown code: %s", code)
			c.emit("retry", 0, 0, 0, 0, "映射表无此接口码，已跳过："+code)
		}
	}
	if len(rows) == 0 {
		return finish(fmt.Errorf("全部 %d 条数据的接口码都不在映射表中", len(fetched.Rows)))
	}

	uscMap, err := c.resolver.ResolveSecuUsc(reqCtx, keys)
	if err != nil {
		return finish(fmt.Errorf("解析 usc 失败: %w", err))
	}

	now := time.Now().UnixMilli()
	out := make([]QuoteSecuRealtimeRow, 0, len(rows))
	unregistered := make([]XetIndexCodeEntry, 0)
	for i := range rows {
		entry := entries[i]
		usc := ""
		if key, ok := XetIndexSecuKey(entry); ok {
			usc = uscMap[key]
		}
		if usc == "" {
			usc = XetIndexUSC(entry)
			unregistered = append(unregistered, entry)
		}
		out = append(out, XetIndexRealtimeRow(entry, rows[i], usc, now))
	}
	if len(unregistered) > 0 {
		c.emit("retry", 0, len(unregistered), 0, 0,
			fmt.Sprintf("%d 只指数未在 public.finv_security 登记，按编码域构造 usc 写入（清单见逐条日志）",
				len(unregistered)))
		for i, entry := range unregistered {
			if i >= XetIndexUnregisteredLogLimit {
				c.emit("retry", 0, 0, 0, 0,
					fmt.Sprintf("其余 %d 条未登记指数不再逐条打印", len(unregistered)-XetIndexUnregisteredLogLimit))
				break
			}
			line := fmt.Sprintf("未登记 usc：code=%s name=%s domain=%s secu=%s usc=%s",
				entry.Code, entry.Name, entry.Domain, entry.SecuCode, XetIndexUSC(entry))
			c.emit("retry", 0, 0, 0, 0, line)
			c.logger.Printf("[cdeu][xetindex] unregistered usc: code=%s name=%s domain=%s secu=%s usc=%s",
				entry.Code, entry.Name, entry.Domain, entry.SecuCode, XetIndexUSC(entry))
		}
	}

	for start := 0; start < len(out); start += XetIndexDbBatchSize {
		if err := ctx.Err(); err != nil {
			return finish(err)
		}
		end := start + XetIndexDbBatchSize
		if end > len(out) {
			end = len(out)
		}
		inserted, updated, saveErr := c.sink.SaveQuoteSecuRealtimeRows(ctx, out[start:end])
		result.RowsInsert += inserted
		result.RowsUpdate += updated
		if saveErr != nil {
			return finish(fmt.Errorf("写入 cdeu.finv_quote_secu_realtime 失败（第 %d 批）: %w",
				start/XetIndexDbBatchSize+1, saveErr))
		}
		c.emit("page", result.Pages, end-start, inserted, updated,
			fmt.Sprintf("写入第 %d 批：新增 %d、覆盖 %d", start/XetIndexDbBatchSize+1, inserted, updated))
	}

	c.logger.Printf("[cdeu][xetindex] pools=%d rows=%d insert=%d update=%d unknown=%d unregistered=%d elapsed=%dms",
		result.Pages, len(out), result.RowsInsert, result.RowsUpdate, len(unknown), len(unregistered),
		time.Since(started).Milliseconds())
	return finish(nil)
}

// XetIndexRealtimeRow 把一条指数快照转换成 cdeu.finv_quote_secu_realtime 的一行。
//
// 字段映射（其余列无来源，写 NULL）：
//
//	name                      ← Row.Name
//	stock_code                ← 映射表的交易所/官方代码（不含市场前缀）
//	market_type               ← 固定 "4"（CN A 股）
//	market_label              ← 站点市场号：1100→SH、1200→SZ、1400→BJ、其它→CN
//	instrument_type           ← 固定 "4"
//	currency                  ← 名称含 HKD/USD 取对应币种，其余 CNY
//	price_nominal             ← Row.Last（点）
//	change_price              ← Row.ChangePrice
//	change_ratio              ← Row.ChangeRatio / 100（表内小数比例）
//	price_amplitude           ← Row.Amplitude / 100
//	buysell_ratio             ← Row.BidAskRatio / 100
//	volumn_ratio              ← Row.VolumeRatio
//	trade_volumn              ← Row.Volume × 100（手 → 股）
//	trade_trunover            ← Row.Amount（元）
//	market_value              ← Row.MarketCap（成分合计）
//	total_shares              ← Row.TotalShares（成分合计）
//	circulation_total_shares  ← Row.FloatShares（成分合计）
//	circulation_market_value  ← Row.FloatMarketCap（成分合计）
//	price_direct              ← ChangePrice 符号（+ / - / =）
//	page                      ← NULL（指数无分页）
//	dt_update                 ← 采集时刻（Unix 毫秒）
//	provider                  ← "ZXET"
//	remark                    ← 来源与口径说明
func XetIndexRealtimeRow(entry XetIndexCodeEntry, row xetapp.Row, usc string, dtUpdateMs int64) QuoteSecuRealtimeRow {
	name := row.Name
	stockCode := entry.SecuCode
	marketType := XetIndexMarketType
	marketLabel := XetIndexMarketLabel(entry.MarketNo)
	instrumentType := XetIndexInstrumentType
	currency := xetIndexCurrency(row.Name)
	dtUpdate := dtUpdateMs
	remark := fmt.Sprintf("来源 ZXET（接口码 %s，编码域 %s，成分市场 %s，市场号 %s；市值/股本为成分合计）",
		entry.Code, entry.Domain, entry.ComponentMarket, entry.MarketNo)

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
		Provider:       XetIndexProvider,
	}
	// 指数无行情时上游给零值：用 Last 判定「有无行情」，有行情才写行情列（含 0 值，如持平）。
	if row.Last != 0 {
		out.PriceNominal = xetIndexNumber(row.Last)
		out.ChangePrice = xetIndexNumber(row.ChangePrice)
		out.ChangeRatio = xetIndexNumber(row.ChangeRatio / 100)
		out.PriceAmplitude = xetIndexNumber(row.Amplitude / 100)
		out.BuysellRatio = xetIndexNumber(row.BidAskRatio / 100)
		out.VolumnRatio = xetIndexNumber(row.VolumeRatio)
		out.TradeVolumn = xetIndexIntNumber(row.Volume * 100)
		out.TradeTrunover = xetIndexNumber(row.Amount)
		direct := "="
		switch {
		case row.ChangePrice > 0:
			direct = "+"
		case row.ChangePrice < 0:
			direct = "-"
		}
		out.PriceDirect = &direct
	}
	if row.MarketCap != 0 || row.TotalShares != 0 || row.FloatShares != 0 || row.FloatMarketCap != 0 {
		out.MarketValue = xetIndexNumber(row.MarketCap)
		out.TotalShares = xetIndexIntNumber(row.TotalShares)
		out.CirculationTotalShares = xetIndexIntNumber(row.FloatShares)
		out.CirculationMarketValue = xetIndexNumber(row.FloatMarketCap)
	}
	return out
}

// xetIndexNumber float64 → *json.Number（保留最短可逆表示）。
func xetIndexNumber(v float64) *json.Number {
	n := json.Number(strconv.FormatFloat(v, 'f', -1, 64))
	return &n
}

// xetIndexIntNumber int64 → *json.Number。
func xetIndexIntNumber(v int64) *json.Number {
	n := json.Number(strconv.FormatInt(v, 10))
	return &n
}

// 编译期断言：默认实现满足采集器接口。
var _ XetIndexSnapshotFetcher = (*XetIndexDefaultFetcher)(nil)
