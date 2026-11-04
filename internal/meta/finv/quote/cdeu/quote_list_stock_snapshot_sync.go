package cdeu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件实现「行情列表股票快照同步」（quote list stock snapshot sync）：
// 按市场分页拉取 moomoo get-stock-list，
// 逐页交给 StockListSink 落库，并把进度回写配置表。
//
// 迁移说明（刻意约束）：
//   - 本文件只依赖标准库与 quoteapiv2，**不引用本项目其它文件的类型**；
//   - 落库（StockListSink）与配置读写（StockListConfigStore）都通过接口注入，
//     后续整体搬到 MetaExt 第三方包时，本文件连同三个接口一起搬即可。
//
// 采集范围与采集控制参数全部**运行时传入**：范围来自配置表（市场/品种/计划数量），
// 控制参数来自 StockListCollectOptions（页大小、页间间隔、重试、超时、页数上限等），
// 未新增控制项时通过 Options.Extra 透传，不必改结构体签名。

// 采集控制参数的默认值（Options 零值字段按这些补齐）。
const (
	// StockListDefaultPageSize 每页条数默认值（接口观测默认 50）。
	StockListDefaultPageSize = 50
	// StockListDefaultPlateType 板块类型默认值（接口缺该键会报 Params Error）。
	StockListDefaultPlateType = "1"
	// StockListDefaultRankType 排行类型默认值（3 = 市值排行）。
	StockListDefaultRankType = "3"
	// StockListDefaultPageStart 页码起始值：从 1 开始。
	//
	// 已实测（2026-11，marketType=1、pageSize=5）：
	//   page=0 → 服务端报错「moomoo page 不能小于 1，实际 0」
	//   page=1 → 港股第一条（HSBC HOLDINGS），pagination{page:1, count:668, total:3337}
	//   page=2 / page=3 → 依次为后续页
	// 早期 Python 版传的是 page-1（0 起），按此口径第 1 页就会失败，属错误实现。
	// 该值保留可配置（Options.PageStart），便于将来服务端口径变化时调整；<=0 一律回落到本默认值。
	StockListDefaultPageStart = 1
	// StockListDefaultInterval 页间间隔默认值（避免触发服务端限流）。
	StockListDefaultInterval = 6 * time.Second
	// StockListDefaultMaxRetries 单页失败重试次数默认值。
	StockListDefaultMaxRetries = 2
	// StockListDefaultRetryBackoff 重试退避基数默认值（第 n 次重试等待 n×该值）。
	StockListDefaultRetryBackoff = 2 * time.Second
	// StockListDefaultRequestTimeout 单次请求超时默认值。
	StockListDefaultRequestTimeout = 20 * time.Second
)

// 日志明细排版：每行 10 个证券、每格固定显示宽度（中文按 2 列计），便于对齐阅读。
const (
	stockListDetailPerLine   = 10
	stockListDetailCellWidth = 22
)

// StockListCollectConfig 一份采集范围配置（对应配置表一行）。
type StockListCollectConfig struct {
	// InstrumentSymbol 证券品种符号：stock / etf / future。
	InstrumentSymbol string `json:"instrument_symbol"`
	// MarketType 市场类型编码（接口 marketType 原值）：1=HK 2=US 4=CN。
	MarketType string `json:"market_type"`
	// CountPlan 计划采集数量；nil 表示按接口返回的总数全量采集。
	CountPlan *int `json:"count_plan"`
	// CountExists 该市场当前存在的证券总数（配置表回写值，仅展示用）。
	CountExists int `json:"count_exists"`
	// TotalAvailable 最近一轮实际可采集数量（配置表回写值，仅展示用）。
	TotalAvailable int `json:"total_available"`
}

// String 便于日志输出。
func (c StockListCollectConfig) String() string {
	plan := "全部"
	if c.CountPlan != nil {
		plan = strconv.Itoa(*c.CountPlan)
	}
	return fmt.Sprintf("%s/%s(计划 %s)", c.InstrumentSymbol, c.MarketType, plan)
}

// StockListCollectOptions 采集控制参数（运行时动态传入）。
//
// 零值字段一律按 StockListDefault* 常量补齐；新增控制项优先用 Extra 透传，
// 避免每次扩展都改结构体与调用方。
type StockListCollectOptions struct {
	// PlateType 板块类型（接口 plateType），空取 StockListDefaultPlateType。
	PlateType string
	// RankType 排行类型（接口 rankType），空取 StockListDefaultRankType。
	RankType string
	// PageSize 每页条数，0 取默认。
	PageSize int
	// PageStart 页码起始值，0 取默认（默认 1）。
	PageStart int
	// Interval 页间间隔（限速），0 取默认。
	Interval time.Duration
	// MaxRetries 单页失败重试次数，0 取默认；负数表示不重试。
	MaxRetries int
	// RetryBackoff 重试退避基数，0 取默认。
	RetryBackoff time.Duration
	// RequestTimeout 单次请求超时，0 取默认。
	RequestTimeout time.Duration
	// MaxPages 页数上限，0 表示不限（仍受计划数量约束）。
	MaxPages int
	// ContinueOnError 单个市场失败后是否继续采集后续市场。
	ContinueOnError bool
	// FilterInstrumentSymbol 只采集该品种（空=全部）。
	FilterInstrumentSymbol string
	// FilterMarketType 只采集该市场（空=全部）。
	FilterMarketType string
	// OverrideCountPlan 覆盖配置里的计划数量（nil=用配置值）。
	OverrideCountPlan *int
	// Extra 预留扩展参数（当前未消费，供后续新增控制项透传）。
	Extra map[string]string
}

// Normalize 补齐默认值，返回可直接使用的新副本（不修改调用方入参）。
func (o StockListCollectOptions) Normalize() StockListCollectOptions {
	if o.PlateType == "" {
		o.PlateType = StockListDefaultPlateType
	}
	if o.RankType == "" {
		o.RankType = StockListDefaultRankType
	}
	if o.PageSize <= 0 {
		o.PageSize = StockListDefaultPageSize
	}
	if o.PageStart <= 0 {
		o.PageStart = StockListDefaultPageStart
	}
	if o.Interval <= 0 {
		o.Interval = StockListDefaultInterval
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = StockListDefaultMaxRetries
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = 0
	}
	if o.RetryBackoff <= 0 {
		o.RetryBackoff = StockListDefaultRetryBackoff
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = StockListDefaultRequestTimeout
	}
	return o
}

// StockListCollectEvent 采集过程中的事件（供日志面板、SSE、前端轮询等消费）。
//
// Stage 取值：
//   - begin：某个市场开始采集（Message 含计划数量与总页数）
//   - page：某一页采集完成（Page/Rows/Insert/Update 有效）
//   - retry：某一页失败后重试（Message 含失败原因）
//   - done：某个市场采集结束（成功或失败）
type StockListCollectEvent struct {
	Stage   string
	Config  StockListCollectConfig
	Page    int
	Rows    int
	Insert  int64
	Update  int64
	Message string
	Time    time.Time
}

// StockListCollectHook 采集事件回调；nil 表示不关心过程事件。
type StockListCollectHook func(event StockListCollectEvent)

// StockListFetcher 单页拉取能力（*quoteapiv2.Client 已实现；测试可注入假实现）。
type StockListFetcher interface {
	GetStockList(ctx context.Context, req quoteapiv2.GetStockListRequest) (quoteapiv2.StockListPage, error)
}

// StockListSavedRow 一行的落库结果（供日志展示「哪些证券、是新增还是更新」）。
type StockListSavedRow struct {
	// StockID 标的编号。
	StockID int64
	// StockCode 代码（如 00005 / AAPL）。
	StockCode string
	// Name 名称。
	Name string
	// Inserted true=新增（此前不存在），false=更新。
	Inserted bool
}

// StockListSink 单页落库能力（由调用方实现，采集逻辑不关心存储细节）。
type StockListSink interface {
	// SaveStockListPage 保存一页列表数据，按行返回新增/更新标记（空切片表示该页无可写行）。
	SaveStockListPage(ctx context.Context, cfg StockListCollectConfig, page int, items []quoteapiv2.StockListItem) (rows []StockListSavedRow, err error)
}

// StockListProgress 采集进度与结果（回写配置表）。
type StockListProgress struct {
	// PageDone 已完成页数。
	PageDone int
	// PagesTotal 计划页数（0 表示未知）。
	PagesTotal int
	// CountExists 接口返回的该市场证券总数（pagination.total）。
	CountExists int
	// TotalAvailable 本轮实际可采集数量。
	TotalAvailable int
	// RowsReceived 收到条目数。
	RowsReceived int64
	// RowsInsert / RowsUpdate 落库统计。
	RowsInsert int64
	RowsUpdate int64
	// Finished 是否为本轮最终回写（false 为逐页进度）。
	Finished bool
	// Error 失败原因（成功为空）。
	Error string
}

// StockListConfigStore 配置读取与进度回写能力（由调用方实现）。
type StockListConfigStore interface {
	// LoadStockListConfigs 读取启用中的采集配置（按采集顺序返回）。
	LoadStockListConfigs(ctx context.Context) ([]StockListCollectConfig, error)
	// SaveStockListProgress 回写采集进度/统计（含 count_exists、total_available）。
	SaveStockListProgress(ctx context.Context, cfg StockListCollectConfig, progress StockListProgress) error
}

// StockListCollectResult 单个市场的采集结果（接口返回与前端展示用）。
type StockListCollectResult struct {
	InstrumentSymbol string `json:"instrument_symbol"`
	MarketType       string `json:"market_type"`
	Pages            int    `json:"pages"`
	RowsReceived     int64  `json:"rows_receive"`
	RowsInsert       int64  `json:"insert"`
	RowsUpdate       int64  `json:"update"`
	CountExists      int    `json:"count_exists"`
	TotalAvailable   int    `json:"total_available"`
	ElapsedMs        int64  `json:"elapsed_ms"`
	Error            string `json:"error,omitempty"`
}

// StockListCollector 列表采集器（无状态，可并发调用不同配置）。
type StockListCollector struct {
	configs StockListConfigStore
	fetcher StockListFetcher
	sink    StockListSink
	logger  *log.Logger
	hook    StockListCollectHook
}

// NewStockListCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewStockListCollector(configs StockListConfigStore, fetcher StockListFetcher, sink StockListSink, logger *log.Logger) *StockListCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &StockListCollector{configs: configs, fetcher: fetcher, sink: sink, logger: logger}
}

// WithHook 设置过程事件回调（链式，便于调用方一次性装配）；传 nil 表示清除。
func (c *StockListCollector) WithHook(hook StockListCollectHook) *StockListCollector {
	c.hook = hook
	return c
}

// emit 触发过程事件（未设置回调时为空操作）。
func (c *StockListCollector) emit(event StockListCollectEvent) {
	if c.hook == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	c.hook(event)
}

// Collect 按配置逐市场采集；返回每个市场的结果。
//
// 返回的 error 是「首个失败市场」的错误（全部成功为 nil）。ContinueOnError
// 为 true 时即使某市场失败也会继续后续市场，此时 results 里可看到各自成败。
func (c *StockListCollector) Collect(ctx context.Context, opts StockListCollectOptions) ([]StockListCollectResult, error) {
	opts = opts.Normalize()

	all, err := c.configs.LoadStockListConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取采集配置失败: %w", err)
	}

	configs := make([]StockListCollectConfig, 0, len(all))
	for _, cfg := range all {
		if opts.FilterInstrumentSymbol != "" && cfg.InstrumentSymbol != opts.FilterInstrumentSymbol {
			continue
		}
		if opts.FilterMarketType != "" && cfg.MarketType != opts.FilterMarketType {
			continue
		}
		if opts.OverrideCountPlan != nil {
			plan := *opts.OverrideCountPlan
			cfg.CountPlan = &plan
		}
		configs = append(configs, cfg)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("没有匹配的采集配置（品种=%q 市场=%q）", opts.FilterInstrumentSymbol, opts.FilterMarketType)
	}

	results := make([]StockListCollectResult, 0, len(configs))
	var firstErr error
	for _, cfg := range configs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result := c.CollectOne(ctx, cfg, opts)
		results = append(results, result)
		if result.Error != "" && firstErr == nil {
			firstErr = errors.New(result.Error)
			if !opts.ContinueOnError {
				return results, firstErr
			}
		}
	}
	return results, firstErr
}

// CollectOne 采集单个配置（一个市场的一种品种）。
//
// 流程：第 1 页拿 pagination（total / pageCount）确定计划页数 → 逐页拉取并落库
// → 每页回写进度（供页面看进度、供断点续采）→ 结束时回写汇总。
func (c *StockListCollector) CollectOne(ctx context.Context, cfg StockListCollectConfig, opts StockListCollectOptions) StockListCollectResult {
	opts = opts.Normalize()
	started := time.Now()
	result := StockListCollectResult{InstrumentSymbol: cfg.InstrumentSymbol, MarketType: cfg.MarketType}
	progress := StockListProgress{}

	// finish 统一收尾：回写最终进度并填结果。
	finish := func(err error) StockListCollectResult {
		progress.PageDone = result.Pages
		progress.RowsReceived = result.RowsReceived
		progress.RowsInsert = result.RowsInsert
		progress.RowsUpdate = result.RowsUpdate
		progress.CountExists = result.CountExists
		progress.TotalAvailable = result.TotalAvailable
		progress.Finished = true
		if err != nil {
			progress.Error = err.Error()
			result.Error = err.Error()
		}
		result.ElapsedMs = time.Since(started).Milliseconds()
		if saveErr := c.configs.SaveStockListProgress(ctx, cfg, progress); saveErr != nil {
			c.logger.Printf("[cdeu][stocklist] 回写进度失败 cfg=%s err=%v", cfg, saveErr)
		}
		message := fmt.Sprintf("采集结束：%d 页、收到 %d 条、新增 %d、覆盖 %d",
			result.Pages, result.RowsReceived, result.RowsInsert, result.RowsUpdate)
		if err != nil {
			message = "采集失败：" + err.Error()
		}
		c.emit(StockListCollectEvent{
			Stage:   "done",
			Config:  cfg,
			Page:    result.Pages,
			Rows:    int(result.RowsReceived),
			Insert:  result.RowsInsert,
			Update:  result.RowsUpdate,
			Message: message,
		})
		return result
	}

	total, pagesNeeded := 0, 0
	for page := opts.PageStart; ; page++ {
		order := page - opts.PageStart + 1 // 第几页（用于页数上限与间隔判断）

		if order > 1 {
			if err := stockListSleep(ctx, opts.Interval); err != nil {
				return finish(err)
			}
		}
		if opts.MaxPages > 0 && order > opts.MaxPages {
			break
		}
		if pagesNeeded > 0 && order > pagesNeeded {
			break
		}

		pageData, err := c.fetchPage(ctx, cfg, page, opts)
		if err != nil {
			return finish(fmt.Errorf("第 %d 页拉取失败: %w", order, err))
		}

		items := pageData.Items
		if order == 1 {
			// 首页确定采集范围：total 随行情变化，每轮以本次观测为准。
			total = pageData.Pagination.Total
			pageCount := pageData.Pagination.PageCount
			result.CountExists = total
			result.TotalAvailable = stockListTargetCount(cfg.CountPlan, total)
			pagesNeeded = stockListPagesNeeded(result.TotalAvailable, opts.PageSize, pageCount)
			progress.PagesTotal = pagesNeeded
			c.logger.Printf("[cdeu][stocklist] 开始采集 cfg=%s total=%d pageCount=%d plan=%d pages=%d",
				cfg, total, pageCount, result.TotalAvailable, pagesNeeded)
			c.emit(StockListCollectEvent{
				Stage:  "begin",
				Config: cfg,
				Message: fmt.Sprintf("开始采集：市场总数 %d，本轮计划 %d，按每页 %d 共 %d 页",
					total, result.TotalAvailable, opts.PageSize, pagesNeeded),
			})
		}

		// 最后一页按计划数量截断，避免多采。
		if remain := result.TotalAvailable - int(result.RowsReceived); remain >= 0 && remain < len(items) {
			items = items[:remain]
		}
		var savedRows []StockListSavedRow
		if len(items) > 0 {
			var err error
			savedRows, err = c.sink.SaveStockListPage(ctx, cfg, order, items)
			if err != nil {
				return finish(fmt.Errorf("第 %d 页落库失败: %w", order, err))
			}
			inserts, updates := 0, 0
			for _, row := range savedRows {
				if row.Inserted {
					inserts++
				} else {
					updates++
				}
			}
			result.RowsReceived += int64(len(savedRows))
			result.RowsInsert += int64(inserts)
			result.RowsUpdate += int64(updates)
		}
		result.Pages = order

		detail := stockListFormatDetail(savedRows)
		pageMessage := fmt.Sprintf("第 %d/%d 页：%d 条（累计 新增 %d / 覆盖 %d）",
			order, pagesNeeded, len(savedRows), result.RowsInsert, result.RowsUpdate)
		if detail != "" {
			pageMessage += "\n" + detail
			c.logger.Printf("[cdeu][stocklist] cfg=%s 第 %d/%d 页 %d 条\n%s",
				cfg, order, pagesNeeded, len(savedRows), detail)
		}
		c.emit(StockListCollectEvent{
			Stage:   "page",
			Config:  cfg,
			Page:    order,
			Rows:    len(savedRows),
			Insert:  result.RowsInsert,
			Update:  result.RowsUpdate,
			Message: pageMessage,
		})

		// 逐页回写进度：页面能实时看到进度，进程中断后也能定位到已完成页。
		progress.PageDone = order
		progress.RowsReceived = result.RowsReceived
		progress.RowsInsert = result.RowsInsert
		progress.RowsUpdate = result.RowsUpdate
		progress.CountExists = result.CountExists
		progress.TotalAvailable = result.TotalAvailable
		if err := c.configs.SaveStockListProgress(ctx, cfg, progress); err != nil {
			c.logger.Printf("[cdeu][stocklist] 回写进度失败 cfg=%s page=%d err=%v", cfg, order, err)
		}

		if len(pageData.Items) < opts.PageSize {
			break // 服务端已无更多数据
		}
		if pagesNeeded > 0 && order >= pagesNeeded {
			break
		}
	}
	return finish(nil)
}

// fetchPage 拉取一页，按 opts 做超时与重试。
func (c *StockListCollector) fetchPage(ctx context.Context, cfg StockListCollectConfig, page int, opts StockListCollectOptions) (quoteapiv2.StockListPage, error) {
	req := quoteapiv2.GetStockListRequest{
		MarketType: cfg.MarketType,
		PlateType:  opts.PlateType,
		RankType:   opts.RankType,
		Page:       strconv.Itoa(page),
		PageSize:   strconv.Itoa(opts.PageSize),
	}
	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * opts.RetryBackoff
			c.logger.Printf("[cdeu][stocklist] 重试 cfg=%s page=%d attempt=%d/%d wait=%s err=%v",
				cfg, page, attempt, opts.MaxRetries, wait, lastErr)
			c.emit(StockListCollectEvent{
				Stage:  "retry",
				Config: cfg,
				Page:   page,
				Message: fmt.Sprintf("第 %d 页重试 %d/%d（等待 %s）：%v",
					page, attempt, opts.MaxRetries, wait, lastErr),
			})
			if err := stockListSleep(ctx, wait); err != nil {
				return quoteapiv2.StockListPage{}, err
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, opts.RequestTimeout)
		pageData, err := c.fetcher.GetStockList(reqCtx, req)
		cancel()
		if err == nil {
			return pageData, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return quoteapiv2.StockListPage{}, ctx.Err()
		}
	}
	return quoteapiv2.StockListPage{}, lastErr
}

// stockListTargetCount 本轮实际采集数量 = min(计划数量, 接口总数)；计划为空取总数。
func stockListTargetCount(plan *int, total int) int {
	if plan == nil || *plan <= 0 || *plan > total {
		return total
	}
	return *plan
}

// stockListPagesNeeded 计划页数 = ceil(计划数量 / 每页条数)，不超过接口总页数。
func stockListPagesNeeded(target, pageSize, pageCount int) int {
	if pageSize <= 0 || target <= 0 {
		return 0
	}
	pages := (target + pageSize - 1) / pageSize
	if pageCount > 0 && pages > pageCount {
		pages = pageCount
	}
	return pages
}

// stockListSleep 可被 ctx 取消的等待。
func stockListSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// StockListCollectRequest 采集接口请求体（全部可选：缺省用配置表 + 默认控制参数）。
//
// 采集范围（哪个市场/品种、采多少）与采集控制参数（间隔、重试、超时、页数上限）
// 都在这里动态传入，新增控制项时可继续加指针字段或用 Extra 透传。
type StockListCollectRequest struct {
	InstrumentSymbol string            `json:"instrument_symbol,omitempty"`
	MarketType       string            `json:"market_type,omitempty"`
	CountPlan        *int              `json:"count_plan,omitempty"`
	PlateType        string            `json:"plate_type,omitempty"`
	RankType         string            `json:"rank_type,omitempty"`
	PageSize         int               `json:"page_size,omitempty"`
	PageStart        *int              `json:"page_start,omitempty"`
	IntervalMs       int               `json:"interval_ms,omitempty"`
	MaxRetries       *int              `json:"max_retries,omitempty"`
	RetryBackoffMs   int               `json:"retry_backoff_ms,omitempty"`
	RequestTimeoutMs int               `json:"request_timeout_ms,omitempty"`
	MaxPages         int               `json:"max_pages,omitempty"`
	ContinueOnError  *bool             `json:"continue_on_error,omitempty"`
	Extra            map[string]string `json:"extra,omitempty"`
}

// Options 把请求体转成采集控制参数。
func (r StockListCollectRequest) Options() StockListCollectOptions {
	opts := StockListCollectOptions{
		PlateType:              r.PlateType,
		RankType:               r.RankType,
		PageSize:               r.PageSize,
		Interval:               time.Duration(r.IntervalMs) * time.Millisecond,
		RetryBackoff:           time.Duration(r.RetryBackoffMs) * time.Millisecond,
		RequestTimeout:         time.Duration(r.RequestTimeoutMs) * time.Millisecond,
		MaxPages:               r.MaxPages,
		ContinueOnError:        true,
		FilterInstrumentSymbol: r.InstrumentSymbol,
		FilterMarketType:       r.MarketType,
		OverrideCountPlan:      r.CountPlan,
		Extra:                  r.Extra,
	}
	if r.PageStart != nil {
		opts.PageStart = *r.PageStart
	}
	if r.MaxRetries != nil {
		opts.MaxRetries = *r.MaxRetries
	}
	if r.ContinueOnError != nil {
		opts.ContinueOnError = *r.ContinueOnError
	}
	return opts
}

// MarshalJSON 供日志打印（保持字段名一致）。
func (r StockListCollectResult) MarshalJSON() ([]byte, error) {
	type alias StockListCollectResult
	return json.Marshal(alias(r))
}

// stockListFormatDetail 把一页的落库行排成「每行 N 个、各列对齐」的文本块。
//
// 每格形如 `[I] 00005 HSBC HOLDINGS`：I=新增、U=更新；
// 宽度按显示列计算（CJK 字符占 2 列），超出截断、不足补空格，保证多行之间严格对齐。
func stockListFormatDetail(rows []StockListSavedRow) string {
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, (len(rows)+stockListDetailPerLine-1)/stockListDetailPerLine)
	for start := 0; start < len(rows); start += stockListDetailPerLine {
		end := start + stockListDetailPerLine
		if end > len(rows) {
			end = len(rows)
		}
		cells := make([]string, 0, stockListDetailPerLine)
		for _, row := range rows[start:end] {
			cells = append(cells, stockListDetailCell(row))
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n")
}

// stockListDetailCell 生成一个固定宽度的格子（含新增/更新标记）。
func stockListDetailCell(row StockListSavedRow) string {
	mark := "[U]"
	if row.Inserted {
		mark = "[I]"
	}
	body := strings.TrimSpace(strings.TrimSpace(row.StockCode) + " " + strings.TrimSpace(row.Name))
	if body == "" {
		body = strconv.FormatInt(row.StockID, 10)
	}
	cell := stockListTruncateDisplay(mark+" "+body, stockListDetailCellWidth)
	return cell + strings.Repeat(" ", stockListDetailCellWidth-stockListDisplayWidth(cell))
}

// stockListDisplayWidth 字符串的终端显示宽度（CJK 全角字符按 2 列计）。
func stockListDisplayWidth(text string) int {
	width := 0
	for _, r := range text {
		if stockListWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// stockListTruncateDisplay 按显示宽度截断（不拆分多字节字符）。
func stockListTruncateDisplay(text string, limit int) string {
	if limit <= 0 || stockListDisplayWidth(text) <= limit {
		return text
	}
	width := 0
	runes := make([]rune, 0, len(text))
	for _, r := range text {
		step := 1
		if stockListWideRune(r) {
			step = 2
		}
		if width+step > limit {
			break
		}
		width += step
		runes = append(runes, r)
	}
	return string(runes)
}

// stockListWideRune 判断是否为双宽（CJK）字符。
func stockListWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f, // 韩文字母
		r == 0x2329, r == 0x232a,
		r >= 0x2e80 && r <= 0xa4cf && r != 0x303f, // CJK 部首/汉字/假名等
		r >= 0xac00 && r <= 0xd7a3,                // 韩文音节
		r >= 0xf900 && r <= 0xfaff,                // CJK 兼容汉字
		r >= 0xfe30 && r <= 0xfe6f,                // CJK 兼容形式
		r >= 0xff00 && r <= 0xff60,                // 全角 ASCII
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x20000 && r <= 0x3fffd: // 扩展区汉字
		return true
	}
	return false
}
