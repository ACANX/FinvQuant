package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/baidu/finscope"
)

// 本文件实现「BDFS（百度股市通 FinScope）板块成分股实时行情同步」（需求 11262）：
// 取板块 GN2309（北交所上市企业）全部成分股，一页即带行情快照，
// 字段转换后写入 cdeu.finv_quote_secu_realtime，provider 固定 'BDFS'。
//
// 与 moomoo 列表采集的差异（刻意保持独立，不复用 StockListCollector）：
//   - 来源是「板块成分股」接口，无市场 / 品种配置维度，也无配置表行；
//   - 接口按偏移量翻页（pn += rn），一次可取全量，无需逐页间隔；
//   - 落库表是跨 provider 的 cdeu.finv_quote_secu_realtime（主键 usc），
//     usc 需按 (exchange, code) 从 public.finv_security 解析，解析不到的行跳过并计数；
//   - 进度与日志沿用 StockListRunner，前端「行情快照同步」页可原样轮询。

const (
	// BdfsInstrumentSymbol 北交所股票（板块 GN2309）在前端 Tab 上的品种符号。
	BdfsInstrumentSymbol = "bsestock"
	// BdfsEtfSymbol 沪深京 ETF（国证ETF 指数 399380）在前端 Tab 上的品种符号。
	// 注意：占位 Tab 的「ETF」用的是 etf 符号，两者刻意分开（需求 11265）。
	BdfsEtfSymbol = "cnetf"
	// BdfsProvider 写入 cdeu.finv_quote_secu_realtime.provider 的固定值。
	BdfsProvider = "BDFS"
	// BdfsBseBlockCode 北交所上市企业板块代码（百度股市通 block）。
	BdfsBseBlockCode = "GN2309"
	// BdfsEtfIndexCode 国证ETF 指数代码（覆盖沪深京 ETF 池）。
	BdfsEtfIndexCode = "399380"
	// BdfsDefaultPageSize 单页条数默认值：实测 rn=5000 可一次返回全部，500 足够且更稳。
	BdfsDefaultPageSize = 500
	// BdfsMaxPages 翻页上限，防止服务端忽略偏移量时死循环。
	BdfsMaxPages = 200
	// BdfsDbBatchSize 单次落库行数：控制单条 SQL 的参数数量。
	BdfsDbBatchSize = 200
	// BdfsDefaultTimeout 单次接口请求超时。
	BdfsDefaultTimeout = 20 * time.Second
	// BdfsUnresolvedLogLimit 未解析 usc 的成分股逐条打印上限（防止大板块一次刷满日志面板；
	// 运行日志本身有 500 行上限，超出会丢弃最早的行）。
	BdfsUnresolvedLogLimit = 200
)

// BdfsConstituent 是一条成分股的入库所需字段（已从接口结构里取过 rawData）。
//
// 接口同时返回带单位的展示字符串与 rawData 原始数值，本结构只保留 rawData，
// 缺值时用 HasRaw 标记，避免把「无行情」写成 0。
type BdfsConstituent struct {
	// Code 证券代码（去重键）。
	Code string
	// Name 证券简称。
	Name string
	// Exchange 交易所：SH / SZ / BJ。
	Exchange string
	// HasQuote 是否有行情（1 / 0）。
	HasQuote int
	// HasRaw rawData 是否可用（决定数值列写 NULL 还是写值）。
	HasRaw bool
	// PriceNominal 最新价（元）。
	PriceNominal float64
	// ChangePrice 涨跌额（元，可负）。
	ChangePrice float64
	// ChangeRatio 涨跌幅（百分数，0.83 表示 +0.83%）。
	ChangeRatio float64
	// Volume 成交量（手）。
	Volume float64
	// Amount 成交额（元）。
	Amount float64
	// MarketValue 总市值（元）。
	MarketValue float64
}

// BdfsFetchResult 一次板块采集的结果。
type BdfsFetchResult struct {
	// Items 去重后的全部成分股。
	Items []BdfsConstituent
	// Pages 实际请求页数。
	Pages int
}

// BdfsFetchRequest 一次成分股拉取的来源与分页参数。
type BdfsFetchRequest struct {
	// FinanceType 成分类型：finscope.FinanceTypeBlock（板块）/ FinanceTypeIndex（指数成分）。
	FinanceType string
	// Code 来源标识：板块 GNxxxx / 指数 6 位代码（如 399380 国证ETF）。
	Code string
	// PageSize 单页条数（接口 rn）。
	PageSize int
}

// BdfsSource 一个 BDFS 采集来源（对应前端一个 Tab）。
type BdfsSource struct {
	// Symbol 前端 Tab 的品种符号（请求体 instrument_symbol）。
	Symbol string
	// FinanceType 成分类型（block / index）。
	FinanceType string
	// Code 来源标识（板块 GN2309 / 指数 399380）。
	Code string
	// InstrumentType 写入目标表 instrument_type 的值（富途品种类型键值：3=股票 4=ETF）。
	//
	// 需求 11302：历史上这里写的是 `stock` / `etf` 文本，与 FTMM 的 3 / 4 不一致；
	// 现统一为字典键值（V300220 已把存量文本行归一）。
	InstrumentType string
	// Label 来源中文说明（写日志）。
	Label string
	// MarketHint 日志里的市场段（与前端日志按「品种/市场」过滤对齐）。
	MarketHint string
}

// BdfsSources 已接入的 BDFS 来源清单（新增来源在此登记，前端另需补 Tab）。
var BdfsSources = []BdfsSource{
	{
		Symbol:         BdfsInstrumentSymbol,
		FinanceType:    finscope.FinanceTypeBlock,
		Code:           BdfsBseBlockCode,
		InstrumentType: "3",
		Label:          "北交所上市企业（板块 GN2309）",
		MarketHint:     "BJ",
	},
	{
		Symbol:         BdfsEtfSymbol,
		FinanceType:    finscope.FinanceTypeIndex,
		Code:           BdfsEtfIndexCode,
		InstrumentType: "4",
		Label:          "国证ETF（指数 399380，覆盖沪深京 ETF）",
		MarketHint:     "4",
	},
}

// BdfsSourceBySymbol 按前端 Tab 符号取来源定义。
func BdfsSourceBySymbol(symbol string) (BdfsSource, bool) {
	symbol = strings.TrimSpace(symbol)
	for _, src := range BdfsSources {
		if src.Symbol == symbol {
			return src, true
		}
	}
	return BdfsSource{}, false
}

// BdfsConstituentsFetcher 成分股拉取能力（默认实现走 MetaExt finscope；测试可注入）。
type BdfsConstituentsFetcher interface {
	FetchConstituents(ctx context.Context, req BdfsFetchRequest) (BdfsFetchResult, error)
}

// FinscopeConstituentsFetcher 基于 MetaExt component/quote/baidu/finscope 的默认实现。
type FinscopeConstituentsFetcher struct {
	client *finscope.Client
}

// NewFinscopeConstituentsFetcher 创建默认拉取器；零值配置按 finscope 协议默认值补齐。
func NewFinscopeConstituentsFetcher() *FinscopeConstituentsFetcher {
	return &FinscopeConstituentsFetcher{client: finscope.New(finscope.Config{})}
}

// FetchConstituents 按偏移量翻页取全部成分股，并按 code 去重。
func (f *FinscopeConstituentsFetcher) FetchConstituents(ctx context.Context, req BdfsFetchRequest) (BdfsFetchResult, error) {
	if f.client == nil {
		return BdfsFetchResult{}, fmt.Errorf("finscope 客户端未初始化")
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = BdfsDefaultPageSize
	}
	financeType := strings.TrimSpace(req.FinanceType)
	if financeType == "" {
		financeType = finscope.FinanceTypeBlock
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = BdfsBseBlockCode
	}

	gen, err := f.client.GetAllConstituents(ctx, finscope.GetConstituentsRequest{
		FinanceType: financeType,
		Market:      finscope.MarketA,
		Code:        code,
		SortKey:     finscope.SortByMarketValue,
		SortType:    finscope.SortDesc,
		Limit:       pageSize,
	})
	if err != nil {
		return BdfsFetchResult{}, err
	}

	items := make([]BdfsConstituent, 0, len(gen.Items))
	for _, item := range gen.Items {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		raw := item.RawData
		items = append(items, BdfsConstituent{
			Code:         code,
			Name:         strings.TrimSpace(item.Name),
			Exchange:     strings.ToUpper(strings.TrimSpace(item.Exchange)),
			HasQuote:     item.HasQuote,
			HasRaw:       item.HasQuote == 1,
			PriceNominal: raw.LastPrice,
			ChangePrice:  raw.ChangePrice,
			ChangeRatio:  raw.ChangeRatio,
			Volume:       raw.Volume,
			Amount:       raw.Amount,
			MarketValue:  raw.MarketValue,
		})
	}
	return BdfsFetchResult{Items: items, Pages: gen.Pages}, nil
}

// SecuUscKey 一条「交易所 + 证券代码」键，用于解析 Finv 统一编码 usc。
type SecuUscKey struct {
	// Exchange 交易所标志（finv_exchange.exchange_flag）：SH / SZ / BJ。
	Exchange string
	// Code 证券代码（finv_security.security_code）。
	Code string
}

// SecuUscResolver usc 解析能力（由 Store 实现；测试可注入）。
type SecuUscResolver interface {
	// ResolveSecuUsc 批量解析 usc；未命中的键不出现在返回 map 中。
	ResolveSecuUsc(ctx context.Context, keys []SecuUscKey) (map[SecuUscKey]string, error)
}

// QuoteSecuRealtimeSink cdeu.finv_quote_secu_realtime 落库能力（由 Store 实现）。
type QuoteSecuRealtimeSink interface {
	// SaveQuoteSecuRealtimeRows 批量 upsert（按 usc 主键覆盖），返回新增 / 更新行数。
	SaveQuoteSecuRealtimeRows(ctx context.Context, rows []QuoteSecuRealtimeRow) (inserted int64, updated int64, err error)
}

// QuoteSecuRealtimeRow 是 cdeu.finv_quote_secu_realtime 的一行（列与 V300209 一致）。
//
// 数值列统一用 *json.Number：与既有 realtime stock 落库口径一致，由 pgx 以文本传，
// 避免 float64 二次格式化丢精度；nil 表示该字段无来源，写 NULL。
type QuoteSecuRealtimeRow struct {
	USC                       string
	StockID                   *string
	Name                      *string
	StockCode                 *string
	MarketType                *string
	MarketLabel               *string
	InstrumentType            *string
	Currency                  *string
	MarketValue               *json.Number
	TotalShares               *json.Number
	CirculationTotalShares    *json.Number
	TradeTrunover             *json.Number
	TradeVolumn               *json.Number
	CirculationMarketValue    *json.Number
	DtUpdate                  *int64
	PriceNominal              *json.Number
	ChangeRatio               *json.Number
	TrustAum                  *json.Number
	TradeChangeRatio          *json.Number
	PriceAmplitude            *json.Number
	VolumnRatio               *json.Number
	BuysellRatio              *json.Number
	ChangePrice               *json.Number
	PriceDirect               *string
	Leverage                  *json.Number
	Page                      *int64
	RmbExchangeRate           *json.Number
	RmbMarketValue            *json.Number
	RmbCirculationMarketValue *json.Number
	RmbTradeTrunover          *json.Number
	RmbPriceNominal           *json.Number
	Remark                    *string
	Provider                  string
}

// BdfsCollectOptions BDFS 采集控制参数（零值按默认值补齐）。
type BdfsCollectOptions struct {
	// Source 采集来源（板块 / 指数），零值按北交所上市企业板块补齐。
	Source BdfsSource
	// PageSize 单页条数，默认 BdfsDefaultPageSize。
	PageSize int
	// Timeout 单次接口请求超时，默认 BdfsDefaultTimeout。
	Timeout time.Duration
}

// normalize 补齐默认值。
func (o BdfsCollectOptions) normalize() BdfsCollectOptions {
	if o.Source.Symbol == "" {
		o.Source, _ = BdfsSourceBySymbol(BdfsInstrumentSymbol)
	}
	if o.PageSize <= 0 || o.PageSize > BdfsMaxPages*1000 {
		o.PageSize = BdfsDefaultPageSize
	}
	if o.Timeout <= 0 {
		o.Timeout = BdfsDefaultTimeout
	}
	return o
}

// BdfsSecuRealtimeCollector BDFS 采集器（无状态，可并发调用）。
type BdfsSecuRealtimeCollector struct {
	fetcher  BdfsConstituentsFetcher
	resolver SecuUscResolver
	sink     QuoteSecuRealtimeSink
	logger   *log.Logger
	hook     StockListCollectHook
}

// NewBdfsSecuRealtimeCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewBdfsSecuRealtimeCollector(fetcher BdfsConstituentsFetcher, resolver SecuUscResolver, sink QuoteSecuRealtimeSink, logger *log.Logger) *BdfsSecuRealtimeCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &BdfsSecuRealtimeCollector{fetcher: fetcher, resolver: resolver, sink: sink, logger: logger}
}

// WithHook 设置过程事件回调（链式）。
func (c *BdfsSecuRealtimeCollector) WithHook(hook StockListCollectHook) *BdfsSecuRealtimeCollector {
	c.hook = hook
	return c
}

// emit 发送一条过程事件（供运行日志面板消费）。
func (c *BdfsSecuRealtimeCollector) emit(src BdfsSource, stage string, page int, rows int, inserted, updated int64, message string) {
	if c.hook == nil {
		return
	}
	c.hook(StockListCollectEvent{
		Stage:   stage,
		Config:  StockListCollectConfig{InstrumentSymbol: src.Symbol, MarketType: src.MarketHint},
		Page:    page,
		Rows:    rows,
		Insert:  inserted,
		Update:  updated,
		Message: message,
		Time:    time.Now(),
	})
}

// Collect 执行一次完整采集：拉取成分股 → 解析 usc → 分批 upsert → 返回统计。
func (c *BdfsSecuRealtimeCollector) Collect(ctx context.Context, opts BdfsCollectOptions) (StockListCollectResult, error) {
	opts = opts.normalize()
	src := opts.Source
	started := time.Now()
	result := StockListCollectResult{InstrumentSymbol: src.Symbol, MarketType: src.MarketHint}

	finish := func(err error) (StockListCollectResult, error) {
		result.ElapsedMs = time.Since(started).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			c.emit(src, "done", 0, 0, result.RowsInsert, result.RowsUpdate, "采集失败："+err.Error())
			return result, err
		}
		c.emit(src, "done", result.Pages, int(result.RowsReceived), result.RowsInsert, result.RowsUpdate,
			fmt.Sprintf("采集完成：%s，%d 页 / %d 条，新增 %d、覆盖 %d",
				src.Label, result.Pages, result.RowsReceived, result.RowsInsert, result.RowsUpdate))
		return result, nil
	}

	reqCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	c.emit(src, "begin", 0, 0, 0, 0, fmt.Sprintf("开始采集 BDFS %s（provider=%s）", src.Label, BdfsProvider))
	fetched, err := c.fetcher.FetchConstituents(reqCtx, BdfsFetchRequest{
		FinanceType: src.FinanceType,
		Code:        src.Code,
		PageSize:    opts.PageSize,
	})
	if err != nil {
		return finish(fmt.Errorf("拉取 %s 失败: %w", src.Label, err))
	}
	if len(fetched.Items) == 0 {
		return finish(fmt.Errorf("%s 未返回任何数据", src.Label))
	}
	result.Pages = fetched.Pages
	result.RowsReceived = int64(len(fetched.Items))
	result.CountExists = len(fetched.Items)
	result.TotalAvailable = len(fetched.Items)
	c.emit(src, "page", fetched.Pages, len(fetched.Items), 0, 0,
		fmt.Sprintf("拉取完成：%d 页 / %d 条", fetched.Pages, len(fetched.Items)))

	// 解析 usc：只查一次，避免逐行往返。
	keys := make([]SecuUscKey, 0, len(fetched.Items))
	seenKey := make(map[SecuUscKey]struct{}, len(fetched.Items))
	for _, item := range fetched.Items {
		key := SecuUscKey{Exchange: strings.ToUpper(item.Exchange), Code: item.Code}
		if _, ok := seenKey[key]; ok {
			continue
		}
		seenKey[key] = struct{}{}
		keys = append(keys, key)
	}
	uscMap, err := c.resolver.ResolveSecuUsc(reqCtx, keys)
	if err != nil {
		return finish(fmt.Errorf("解析 usc 失败: %w", err))
	}

	now := time.Now().UnixMilli()
	rows := make([]QuoteSecuRealtimeRow, 0, len(fetched.Items))
	unresolved := make([]BdfsConstituent, 0)
	for idx, item := range fetched.Items {
		key := SecuUscKey{Exchange: strings.ToUpper(item.Exchange), Code: item.Code}
		usc := uscMap[key]
		if usc == "" {
			unresolved = append(unresolved, item)
			continue
		}
		pageNo := int64(idx/opts.PageSize) + 1
		rows = append(rows, BdfsRealtimeRow(item, usc, now, pageNo, src.InstrumentType))
	}
	if len(unresolved) > 0 {
		c.emit(src, "retry", 0, len(unresolved), 0, 0,
			fmt.Sprintf("有 %d 条成分股未能在 public.finv_security 解析到 usc，已跳过（清单见下方逐条日志）", len(unresolved)))
		// 逐条打印，便于人工登记入库：日志面板与 docker logs 都能直接复制
		// exchange + code 是登记 public.finv_security 所需的最小键，name 便于人工核对。
		for i, item := range unresolved {
			if i >= BdfsUnresolvedLogLimit {
				c.emit(src, "retry", 0, 0, 0, 0,
					fmt.Sprintf("其余 %d 条未解析成分股不再逐条打印（仅保留前 %d 条）",
						len(unresolved)-BdfsUnresolvedLogLimit, BdfsUnresolvedLogLimit))
				break
			}
			line := fmt.Sprintf("未解析 usc：exchange=%s code=%s name=%s",
				strings.ToUpper(item.Exchange), item.Code, item.Name)
			c.emit(src, "retry", 0, 0, 0, 0, line)
			c.logger.Printf("[cdeu][bdfs] unresolved usc: exchange=%s code=%s name=%s source=%s",
				strings.ToUpper(item.Exchange), item.Code, item.Name, src.Symbol)
		}
	}
	if len(rows) == 0 {
		return finish(fmt.Errorf("%s 的 %d 条数据均未解析到 usc，未写入任何行",
			src.Label, len(fetched.Items)))
	}

	for start := 0; start < len(rows); start += BdfsDbBatchSize {
		if err := ctx.Err(); err != nil {
			return finish(err)
		}
		end := start + BdfsDbBatchSize
		if end > len(rows) {
			end = len(rows)
		}
		inserted, updated, saveErr := c.sink.SaveQuoteSecuRealtimeRows(ctx, rows[start:end])
		result.RowsInsert += inserted
		result.RowsUpdate += updated
		if saveErr != nil {
			return finish(fmt.Errorf("写入 cdeu.finv_quote_secu_realtime 失败（第 %d 批）: %w",
				start/BdfsDbBatchSize+1, saveErr))
		}
		c.emit(src, "page", int(rows[end-1].PageValue()), end-start, inserted, updated,
			fmt.Sprintf("写入第 %d 批：新增 %d、覆盖 %d", start/BdfsDbBatchSize+1, inserted, updated))
	}

	c.logger.Printf("[cdeu][bdfs] source=%s code=%s items=%d insert=%d update=%d unresolved=%d elapsed=%dms",
		src.Symbol, src.Code, len(rows), result.RowsInsert, result.RowsUpdate, len(unresolved),
		time.Since(started).Milliseconds())
	return finish(nil)
}

// PageValue 返回该行记录的页码（供日志展示；nil 记 0）。
func (r QuoteSecuRealtimeRow) PageValue() int64 {
	if r.Page == nil {
		return 0
	}
	return *r.Page
}

// BdfsMarketType 写入 cdeu.finv_quote_secu_realtime.market_type 的固定值。
//
// 与参考表 cdeu.finv_cdeu_secu_realtime_stock 口径一致：market_type 是「源系统市场分类编码」，
// 4 = CN A 股（1 = HK、2 = US）；北交所板块的成分股全部落在 A 股，直接固定 4。
const BdfsMarketType = "4"

// bdfsExchangeMeta BDFS 交易所标志 → 新表 market_label / 币种。
//
// market_label 用交易所标志本身（BJ / SH / SZ，与参考表数据一致：BJ 行即 market_type=4 + market_label=BJ），
// 北交所板块固定为 BJ；保留 SH / SZ 分支以便该采集器将来复用到其它板块。
var bdfsExchangeMeta = map[string]struct {
	Label    string
	Currency string
}{
	"BJ": {Label: "BJ", Currency: "CNY"},
	"SH": {Label: "SH", Currency: "CNY"},
	"SZ": {Label: "SZ", Currency: "CNY"},
}

// BdfsRealtimeRow 把一条 BDFS 成分股转换成 cdeu.finv_quote_secu_realtime 的一行。
//
// 字段映射（其余列无来源，写 NULL）：
//
//	stock_code        ← code
//	name              ← name
//	market_type       ← 固定 "4"（CN A 股，与参考表口径一致）
//	market_label      ← 交易所标志（BJ / SH / SZ；北交所板块即 BJ）
//	instrument_type   ← "3"（股票键值；接口对本板块只返回股票）
//	currency          ← CNY（A 股）
//	price_nominal     ← rawData.lastPx
//	change_price      ← rawData.pxChange
//	change_ratio      ← rawData.pxChangeRate / 100（表内口径为小数比例）
//	trade_volumn      ← rawData.volume × 100（接口单位为手，表内按股）
//	trade_trunover    ← rawData.amount（元）
//	market_value      ← rawData.marketValue（元）
//	price_direct      ← pxChange 符号（+ / - / =；持平为 =）
//	page              ← 该行在本次采集里的页码（从 1 起）
//	dt_update         ← 采集时刻（Unix 毫秒）
//	provider          ← "BDFS"
//	stock_id          ← NULL（BDFS 不提供源系统证券 ID）
//	remark            ← 来源说明，便于事后追溯
func BdfsRealtimeRow(item BdfsConstituent, usc string, dtUpdateMs int64, page int64, instrumentType string) QuoteSecuRealtimeRow {
	meta, ok := bdfsExchangeMeta[strings.ToUpper(item.Exchange)]
	if !ok {
		meta = bdfsExchangeMeta["BJ"]
	}
	marketType := BdfsMarketType
	marketLabel := meta.Label
	currency := meta.Currency
	if strings.TrimSpace(instrumentType) == "" {
		instrumentType = "3"
	}
	stockCode := item.Code
	name := item.Name
	remark := fmt.Sprintf("来源 BDFS（exchange=%s）", item.Exchange)
	pageValue := page
	dtUpdate := dtUpdateMs

	row := QuoteSecuRealtimeRow{
		USC:            usc,
		StockCode:      &stockCode,
		Name:           &name,
		MarketType:     &marketType,
		MarketLabel:    &marketLabel,
		InstrumentType: &instrumentType,
		Currency:       &currency,
		DtUpdate:       &dtUpdate,
		Page:           &pageValue,
		Remark:         &remark,
		Provider:       BdfsProvider,
	}
	if !item.HasRaw {
		// hasQuote=0：只写身份列，行情列保持 NULL，避免写成 0 被下游当成真实报价。
		return row
	}
	row.PriceNominal = bdfsNumber(item.PriceNominal)
	row.ChangePrice = bdfsNumber(item.ChangePrice)
	row.ChangeRatio = bdfsNumber(item.ChangeRatio / 100)
	row.TradeVolumn = bdfsNumber(item.Volume * 100)
	row.TradeTrunover = bdfsNumber(item.Amount)
	row.MarketValue = bdfsNumber(item.MarketValue)
	direct := "="
	switch {
	case item.ChangePrice > 0:
		direct = "+"
	case item.ChangePrice < 0:
		direct = "-"
	}
	row.PriceDirect = &direct
	return row
}

// bdfsNumber float64 → *json.Number（保留最短可逆表示）。
func bdfsNumber(v float64) *json.Number {
	n := json.Number(strconv.FormatFloat(v, 'f', -1, 64))
	return &n
}
