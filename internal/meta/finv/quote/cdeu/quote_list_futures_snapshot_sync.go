package cdeu

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件实现「期货列表快照同步」（future list snapshot sync，需求 11273/11274）：
// 按「市场 + 期货分类」分页拉取 moomoo get-futures-list，
// 逐页交给 FutureListSink 落库，并把进度回写配置表。
//
// 迁移说明（刻意约束，与 quote_list_stock_snapshot_sync.go 同款）：
//   - 本文件只依赖标准库与 quoteapiv2，**不引用本项目其它文件的类型**；
//   - 落库（FutureListSink）与配置读写（FutureListConfigStore）都通过接口注入，
//     后续整体搬到 MetaExt 第三方包时，本文件连同三个接口一起搬即可；
//   - 过程事件复用 StockListCollectEvent / StockListCollectHook（见 stocklist_run.go），
//     这样运行日志面板、StockListCollectEventToLog 与轮询接口一行都不用改。
//
// 采集范围（哪个分类、采多少）来自配置表 cdeu.finv_quote_cdeu_future_sync_config；
// 控制参数（页大小、页间间隔、重试、超时、页数上限）运行时传入。

// FuturesInstrumentSymbol 「期货」在采集运行里的品种符号。
//
// 与「行情快照同步」页的期货 Tab、StockListCollectRequest.instrument_symbol 一致；
// Service.StartStockListRun 见到该值即走本文件的采集链路（与 BDFS 分支同位置）。
const FuturesInstrumentSymbol = "future"

// FuturesTypeAll 表示「全部期货分类」（请求参数与日志展示用）。
const FuturesTypeAll = "all"

// 采集控制参数的默认值（Options 零值字段按这些补齐）。
const (
	// FutureListDefaultPageSize 每页条数默认值（接口观测默认 40）。
	FutureListDefaultPageSize = 40
	// FutureListDefaultPageStart 页码起始值：从 0 开始（本接口 page 是 0-based）。
	FutureListDefaultPageStart = 0
	// FutureListDefaultMarketType 市场类型默认值：0 = 聚合口径。
	//
	// 依据（用户 2026-11-20 决策）：网页直接请求
	// `get-futures-list?marketType=0&futuresType=N&page=0&pageSize=40` 即返回该分类下
	// 全部市场的合约（每行的市场在 fields["19"] 里），因此默认 0；
	// 个别分类若要求具体市场，在配置表把 market_type 改成 1/2 等即可，无需改代码。
	FutureListDefaultMarketType = "0"
	// FutureListDefaultInterval 页间间隔默认值（避免触发服务端限流）。
	//
	// 与股票列表的 6s 不同：期货每个分类通常只有 1~3 页，8 个分类串行跑完仍在 1 分钟内。
	FutureListDefaultInterval = 2 * time.Second
	// FutureListDefaultMaxRetries 单页失败重试次数默认值。
	FutureListDefaultMaxRetries = 2
	// FutureListDefaultRetryBackoff 重试退避基数默认值（第 n 次重试等待 n×该值）。
	FutureListDefaultRetryBackoff = 2 * time.Second
	// FutureListDefaultRequestTimeout 单次请求超时默认值。
	FutureListDefaultRequestTimeout = 20 * time.Second
)

// 日志明细排版：每行 5 个合约、每格固定显示宽度（中文按 2 列计），便于对齐阅读。
//
// 格宽 20：`[I] ` 占 4 显示列，余 16 列足够放下最长的合约代码
// （观测最长 `NK225MCmain` 11 字符）；早先 13 列会把 `NK225Mmain` 截成 `NK225Mmai`。
const (
	futureListDetailPerLine   = 5
	futureListDetailCellWidth = 20
)

// FutureListCollectConfig 一份采集范围配置（对应配置表一行）。
type FutureListCollectConfig struct {
	// MarketType 市场类型编码（接口 marketType 原值，配置表可空则取默认 0）。
	MarketType string `json:"market_type"`
	// FuturesType 期货分类（接口 futuresType 原值 1~8）。
	FuturesType string `json:"futures_type"`
	// FuturesTypeName 分类中文名（可空；空则前端降级显示「分类 N」）。
	FuturesTypeName string `json:"futures_type_name"`
	// CountPlan 计划采集数量；nil 表示按接口返回的总数全量采集。
	CountPlan *int `json:"count_plan"`
	// CountExists 该分类当前存在的合约总数（配置表回写值，仅展示用）。
	CountExists int `json:"count_exists"`
	// TotalAvailable 最近一轮实际可采集数量（配置表回写值，仅展示用）。
	TotalAvailable int `json:"total_available"`
}

// String 便于日志输出。
func (c FutureListCollectConfig) String() string {
	plan := "全部"
	if c.CountPlan != nil {
		plan = strconv.Itoa(*c.CountPlan)
	}
	return fmt.Sprintf("%s/%s(计划 %s)", FuturesInstrumentSymbol, c.FuturesType, plan)
}

// DisplayName 分类显示名：有中文名用中文名，否则「分类 N」。
func (c FutureListCollectConfig) DisplayName() string {
	if name := strings.TrimSpace(c.FuturesTypeName); name != "" {
		return name
	}
	return "分类 " + c.FuturesType
}

// FutureListCollectOptions 采集控制参数（运行时动态传入）。
//
// 零值字段一律按 FutureListDefault* 常量补齐；新增控制项优先用 Extra 透传，
// 避免每次扩展都改结构体与调用方。
type FutureListCollectOptions struct {
	// PageSize 每页条数，0 取默认。
	PageSize int
	// PageStart 页码起始值（0-based，第一页即 page=0），负数取默认 0。
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
	// ContinueOnError 单个分类失败后是否继续采集后续分类。
	ContinueOnError bool
	// FilterFuturesType 只采集该分类（空或 FuturesTypeAll=全部）。
	FilterFuturesType string
	// FilterMarketType 只采集该市场（空=全部）。
	FilterMarketType string
	// OverrideCountPlan 覆盖配置里的计划数量（nil=用配置值）。
	OverrideCountPlan *int
	// MarkOffline 整轮成功后是否把「未再出现」的合约标记下架（默认 true）。
	MarkOffline bool
	// Extra 预留扩展参数（当前未消费，供后续新增控制项透传）。
	Extra map[string]string
}

// Normalize 补齐默认值，返回可直接使用的新副本（不修改调用方入参）。
func (o FutureListCollectOptions) Normalize() FutureListCollectOptions {
	if o.PageSize <= 0 {
		o.PageSize = FutureListDefaultPageSize
	}
	if o.PageStart < 0 {
		o.PageStart = FutureListDefaultPageStart
	}
	if o.Interval <= 0 {
		o.Interval = FutureListDefaultInterval
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = FutureListDefaultMaxRetries
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = 0
	}
	if o.RetryBackoff <= 0 {
		o.RetryBackoff = FutureListDefaultRetryBackoff
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = FutureListDefaultRequestTimeout
	}
	return o
}

// wantFuturesType 判断某个分类是否在本轮采集范围内。
func (o FutureListCollectOptions) wantFuturesType(futuresType string) bool {
	filter := strings.TrimSpace(o.FilterFuturesType)
	if filter == "" || filter == FuturesTypeAll {
		return true
	}
	return filter == futuresType
}

// FutureListFetcher 单页拉取能力（*quoteapiv2.Client 已实现；测试可注入假实现）。
type FutureListFetcher interface {
	GetFuturesList(ctx context.Context, req quoteapiv2.GetFuturesListRequest) (quoteapiv2.FuturesListPage, error)
}

// FutureListSavedRow 一行的落库结果（供日志展示「哪些合约、是新增还是更新」）。
type FutureListSavedRow struct {
	// StockID 合约编号（源 stockId，本表主键）。
	StockID int64
	// USC Finv 统一证券编码（可空：未解析到为空串）。
	USC string
	// ContractCode 合约代码（如 GCmain）。
	ContractCode string
	// Name 合约名称。
	Name string
	// Inserted true=新增（此前不存在），false=更新。
	Inserted bool
}

// FutureListSink 落库能力（由调用方实现，采集逻辑不关心存储细节）。
type FutureListSink interface {
	// SaveFuturesListPage 保存一页列表数据，按行返回新增/更新标记（空切片表示该页无可写行）。
	SaveFuturesListPage(ctx context.Context, cfg FutureListCollectConfig, page int, items []quoteapiv2.FuturesListItem) (rows []FutureListSavedRow, err error)
	// MarkFuturesOffline 把「最近一次出现早于 notSeenBeforeMs」的在售合约标记下架（flag_online='0'）。
	//
	// 只在**整轮全分类采集成功**后调用一次：主键是单列 stock_id，按分类逐个置下架
	// 会把其它分类仍在售的合约误标（需求 11273 的主键决策直接导致该约束）。
	MarkFuturesOffline(ctx context.Context, notSeenBeforeMs int64) (int64, error)
}

// FutureListProgress 采集进度与结果（回写配置表）。
type FutureListProgress struct {
	// PageDone 已完成页数。
	PageDone int
	// PagesTotal 计划页数（0 表示未知）。
	PagesTotal int
	// CountExists 接口返回的该分类合约总数（pagination.total）。
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

// FutureListConfigStore 配置读取与进度回写能力（由调用方实现）。
type FutureListConfigStore interface {
	// LoadFuturesConfigs 读取启用中的采集配置（按分类顺序返回）。
	LoadFuturesConfigs(ctx context.Context) ([]FutureListCollectConfig, error)
	// SaveFuturesProgress 回写采集进度/统计（含 count_exists、total_available）。
	SaveFuturesProgress(ctx context.Context, cfg FutureListCollectConfig, progress FutureListProgress) error
}

// FutureListCollectResult 单个分类的采集结果（接口返回与前端展示用）。
//
// 字段命名与 StockListCollectResult 对齐（前端表格复用同一套列），
// 另加 futures_type / futures_type_name 供期货 Tab 展示分类。
type FutureListCollectResult struct {
	InstrumentSymbol string `json:"instrument_symbol"`
	MarketType       string `json:"market_type"`
	FuturesType      string `json:"futures_type"`
	FuturesTypeName  string `json:"futures_type_name"`
	Pages            int    `json:"pages"`
	RowsReceived     int64  `json:"rows_receive"`
	RowsInsert       int64  `json:"insert"`
	RowsUpdate       int64  `json:"update"`
	CountExists      int    `json:"count_exists"`
	TotalAvailable   int    `json:"total_available"`
	ElapsedMs        int64  `json:"elapsed_ms"`
	Error            string `json:"error,omitempty"`
}

// FutureListCollector 期货列表采集器（无状态，可并发调用不同配置）。
type FutureListCollector struct {
	configs FutureListConfigStore
	fetcher FutureListFetcher
	sink    FutureListSink
	logger  *log.Logger
	hook    StockListCollectHook
}

// NewFutureListCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewFutureListCollector(configs FutureListConfigStore, fetcher FutureListFetcher, sink FutureListSink, logger *log.Logger) *FutureListCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &FutureListCollector{configs: configs, fetcher: fetcher, sink: sink, logger: logger}
}

// WithHook 设置过程事件回调（链式，便于调用方一次性装配）；传 nil 表示清除。
func (c *FutureListCollector) WithHook(hook StockListCollectHook) *FutureListCollector {
	c.hook = hook
	return c
}

// emit 触发过程事件（未设置回调时为空操作）。
//
// Config.MarketType 传「期货分类」（而不是请求的 marketType）：运行日志据此显示
// `(future/5)`，前端日志面板按 `future/` 前缀过滤即可拿到期货全部日志。
func (c *FutureListCollector) emit(cfg FutureListCollectConfig, stage string, page int, rows int,
	insert, update int64, message string) {
	if c.hook == nil {
		return
	}
	c.hook(StockListCollectEvent{
		Stage: stage,
		Config: StockListCollectConfig{
			InstrumentSymbol: FuturesInstrumentSymbol,
			MarketType:       cfg.FuturesType,
		},
		Page:    page,
		Rows:    rows,
		Insert:  insert,
		Update:  update,
		Message: message,
		Time:    time.Now(),
	})
}

// Collect 按配置逐分类采集；返回每个分类的结果。
//
// 返回的 error 是「首个失败分类」的错误（全部成功为 nil）。ContinueOnError
// 为 true 时即使某分类失败也会继续后续分类，此时 results 里可看到各自成败。
//
// 下架标记（MarkFuturesOffline）只在「本轮覆盖全部启用分类且全部成功」时执行：
// 结果表主键是单列 stock_id，分批置下架会把尚未采到的分类合约误标。
func (c *FutureListCollector) Collect(ctx context.Context, opts FutureListCollectOptions) ([]FutureListCollectResult, error) {
	opts = opts.Normalize()
	// 整轮水位：本轮开始时刻，用于判定「哪些合约本轮没再出现」。
	roundStartMs := time.Now().UnixMilli()

	all, err := c.configs.LoadFuturesConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取采集配置失败: %w", err)
	}

	configs := make([]FutureListCollectConfig, 0, len(all))
	for _, cfg := range all {
		if !opts.wantFuturesType(cfg.FuturesType) {
			continue
		}
		if opts.FilterMarketType != "" && cfg.MarketType != opts.FilterMarketType {
			continue
		}
		if strings.TrimSpace(cfg.MarketType) == "" {
			cfg.MarketType = FutureListDefaultMarketType
		}
		if opts.OverrideCountPlan != nil {
			plan := *opts.OverrideCountPlan
			cfg.CountPlan = &plan
		}
		configs = append(configs, cfg)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("没有匹配的采集配置（分类=%q 市场=%q）",
			opts.FilterFuturesType, opts.FilterMarketType)
	}

	results := make([]FutureListCollectResult, 0, len(configs))
	var firstErr error
	failed := 0
	for _, cfg := range configs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result := c.CollectOne(ctx, cfg, opts)
		results = append(results, result)
		if result.Error != "" {
			failed++
			if firstErr == nil {
				firstErr = errors.New(result.Error)
				if !opts.ContinueOnError {
					return results, firstErr
				}
			}
		}
	}

	// 下架标记：只有「本轮选中的就是全部启用分类」且无一失败时才动手。
	if opts.MarkOffline && failed == 0 && len(configs) == len(all) && ctx.Err() == nil {
		offline, offErr := c.sink.MarkFuturesOffline(ctx, roundStartMs)
		if offErr != nil {
			c.logger.Printf("[cdeu][futurelist] 标记下架失败 err=%v", offErr)
			c.emit(FutureListCollectConfig{MarketType: FutureListDefaultMarketType, FuturesType: FuturesTypeAll},
				"round", 0, 0, 0, 0, "标记下架失败："+offErr.Error())
		} else if offline > 0 {
			c.logger.Printf("[cdeu][futurelist] 标记下架 rows=%d", offline)
			c.emit(FutureListCollectConfig{MarketType: FutureListDefaultMarketType, FuturesType: FuturesTypeAll},
				"round", 0, int(offline), 0, 0,
				fmt.Sprintf("下架标记：%d 条合约本轮未再出现，已置 flag_online='0'", offline))
		}
	} else if opts.MarkOffline {
		c.logger.Printf("[cdeu][futurelist] 跳过下架标记（本轮 %d/%d 个分类、失败 %d 个）",
			len(configs), len(all), failed)
	}
	return results, firstErr
}

// CollectOne 采集单个配置（一个期货分类）。
//
// 流程：第 1 页拿 pagination（total / pageCount）确定计划页数 → 逐页拉取并落库
// → 每页回写进度（供页面看进度、供断点续采）→ 结束时回写汇总。
func (c *FutureListCollector) CollectOne(ctx context.Context, cfg FutureListCollectConfig, opts FutureListCollectOptions) FutureListCollectResult {
	opts = opts.Normalize()
	started := time.Now()
	label := cfg.DisplayName()
	result := FutureListCollectResult{
		InstrumentSymbol: FuturesInstrumentSymbol,
		MarketType:       cfg.MarketType,
		FuturesType:      cfg.FuturesType,
		FuturesTypeName:  cfg.FuturesTypeName,
	}
	progress := FutureListProgress{}

	// finish 统一收尾：回写最终进度并填结果。
	finish := func(err error) FutureListCollectResult {
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
		if saveErr := c.configs.SaveFuturesProgress(ctx, cfg, progress); saveErr != nil {
			c.logger.Printf("[cdeu][futurelist] 回写进度失败 cfg=%s err=%v", cfg, saveErr)
		}
		message := fmt.Sprintf("%s 采集结束：%d 页、收到 %d 条、新增 %d、覆盖 %d",
			label, result.Pages, result.RowsReceived, result.RowsInsert, result.RowsUpdate)
		if err != nil {
			message = label + " 采集失败：" + err.Error()
		}
		c.emit(cfg, "done", result.Pages, int(result.RowsReceived), result.RowsInsert, result.RowsUpdate, message)
		return result
	}

	pagesNeeded := 0
	for page := opts.PageStart; ; page++ {
		order := page - opts.PageStart + 1 // 第几页（用于页数上限与间隔判断）

		if order > 1 {
			if err := futureListSleep(ctx, opts.Interval); err != nil {
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
			total := pageData.Pagination.Total
			result.CountExists = total
			result.TotalAvailable = futureListTargetCount(cfg.CountPlan, total)
			pagesNeeded = futureListPagesNeeded(result.TotalAvailable, opts.PageSize, pageData.Pagination.PageCount)
			progress.PagesTotal = pagesNeeded
			c.logger.Printf("[cdeu][futurelist] 开始采集 cfg=%s market_type=%s total=%d pageCount=%d plan=%d pages=%d",
				cfg, cfg.MarketType, total, pageData.Pagination.PageCount, result.TotalAvailable, pagesNeeded)
			// 字段字典观测：first 采样打印 fieldId/fieldName/contentType/precision，
			// 用于确认 fields 的位置化口径（本仓库只收录 1/2/3/4/19，其余留档在 fields_json）。
			c.logger.Printf("[cdeu][futurelist] 字段字典采样 cfg=%s 样本=%s",
				cfg, futureListSampleFields(items))
			c.emit(cfg, "begin", 0, 0, 0, 0,
				fmt.Sprintf("%s 开始采集：分类合约总数 %d，本轮计划 %d，按每页 %d 共 %d 页（marketType=%s）",
					label, total, result.TotalAvailable, opts.PageSize, pagesNeeded, cfg.MarketType))
			// 口径漂移告警：total 与配置表记录的上次值差异过大时提示（同一接口按出口 IP /
			// 请求语言可能返回不同集合，差异过大基本就是口径变了）。
			if cfg.CountExists > 0 && total > 0 {
				ratio := float64(total) / float64(cfg.CountExists)
				if ratio < 0.5 || ratio > 2 {
					message := fmt.Sprintf("本轮 %s 总数 %d 与上次记录 %d 差异过大，可能被按 IP/语言返回了不同口径",
						label, total, cfg.CountExists)
					c.logger.Printf("[cdeu][futurelist] 警告 cfg=%s %s", cfg, message)
					c.emit(cfg, "warn", 0, 0, 0, 0, message)
				}
			}
		}

		// 最后一页按计划数量截断，避免多采。
		if remain := result.TotalAvailable - int(result.RowsReceived); remain >= 0 && remain < len(items) {
			items = items[:remain]
		}
		var savedRows []FutureListSavedRow
		if len(items) > 0 {
			var err error
			savedRows, err = c.sink.SaveFuturesListPage(ctx, cfg, order, items)
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

		detail := futureListFormatDetail(savedRows)
		pageMessage := fmt.Sprintf("%s 第 %d/%d 页：%d 条（累计 新增 %d / 覆盖 %d）",
			label, order, pagesNeeded, len(savedRows), result.RowsInsert, result.RowsUpdate)
		if detail != "" {
			pageMessage += "\n" + detail
			c.logger.Printf("[cdeu][futurelist] cfg=%s 第 %d/%d 页 %d 条\n%s",
				cfg, order, pagesNeeded, len(savedRows), detail)
		}
		c.emit(cfg, "page", order, len(savedRows), result.RowsInsert, result.RowsUpdate, pageMessage)

		// 逐页回写进度：页面能实时看到进度，进程中断后也能定位到已完成页。
		progress.PageDone = order
		progress.RowsReceived = result.RowsReceived
		progress.RowsInsert = result.RowsInsert
		progress.RowsUpdate = result.RowsUpdate
		progress.CountExists = result.CountExists
		progress.TotalAvailable = result.TotalAvailable
		if err := c.configs.SaveFuturesProgress(ctx, cfg, progress); err != nil {
			c.logger.Printf("[cdeu][futurelist] 回写进度失败 cfg=%s page=%d err=%v", cfg, order, err)
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
func (c *FutureListCollector) fetchPage(ctx context.Context, cfg FutureListCollectConfig, page int, opts FutureListCollectOptions) (quoteapiv2.FuturesListPage, error) {
	req := quoteapiv2.GetFuturesListRequest{
		MarketType:  cfg.MarketType,
		FuturesType: cfg.FuturesType,
		Page:        strconv.Itoa(page),
		PageSize:    strconv.Itoa(opts.PageSize),
		// `_` 毫秒时间戳：它是 quote-token 签名输入的一部分，缺失会让服务端返回降级数据
		// （与股票列表同款教训，见需求 10991），因此每页都带。
		TimestampMillis: strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt) * opts.RetryBackoff
			c.logger.Printf("[cdeu][futurelist] 重试 cfg=%s page=%d attempt=%d/%d wait=%s err=%v",
				cfg, page, attempt, opts.MaxRetries, wait, lastErr)
			c.emit(cfg, "retry", page, 0, 0, 0,
				fmt.Sprintf("%s 第 %d 页重试 %d/%d（等待 %s）：%v",
					cfg.DisplayName(), page, attempt, opts.MaxRetries, wait, lastErr))
			if err := futureListSleep(ctx, wait); err != nil {
				return quoteapiv2.FuturesListPage{}, err
			}
		}
		reqCtx, cancel := context.WithTimeout(ctx, opts.RequestTimeout)
		pageData, err := c.fetcher.GetFuturesList(reqCtx, req)
		cancel()
		if err == nil {
			return pageData, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return quoteapiv2.FuturesListPage{}, ctx.Err()
		}
	}
	return quoteapiv2.FuturesListPage{}, lastErr
}

// futureListTargetCount 本轮实际采集数量 = min(计划数量, 接口总数)；计划为空取总数。
func futureListTargetCount(plan *int, total int) int {
	if plan == nil || *plan <= 0 || *plan > total {
		return total
	}
	return *plan
}

// futureListPagesNeeded 计划页数 = ceil(计划数量 / 每页条数)，不超过接口总页数。
func futureListPagesNeeded(target, pageSize, pageCount int) int {
	if pageSize <= 0 || target <= 0 {
		return 0
	}
	pages := (target + pageSize - 1) / pageSize
	if pageCount > 0 && pages > pageCount {
		pages = pageCount
	}
	return pages
}

// futureListSleep 可被 ctx 取消的等待。
func futureListSleep(ctx context.Context, d time.Duration) error {
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

// futureListFormatDetail 把一页的落库行排成「每行 N 个、各列对齐」的文本块。
//
// 每格形如 `[I] GCmain`：I=新增、U=更新（只展示合约代码，不含名称）；
// 每行 futureListDetailPerLine 个，宽度按显示列计算（CJK 字符占 2 列），
// 超出 futureListDetailCellWidth 才截断、不足补空格，保证多行之间严格对齐。
func futureListFormatDetail(rows []FutureListSavedRow) string {
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, (len(rows)+futureListDetailPerLine-1)/futureListDetailPerLine)
	for start := 0; start < len(rows); start += futureListDetailPerLine {
		end := start + futureListDetailPerLine
		if end > len(rows) {
			end = len(rows)
		}
		cells := make([]string, 0, futureListDetailPerLine)
		for _, row := range rows[start:end] {
			cells = append(cells, futureListDetailCell(row))
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n")
}

// futureListDetailCell 生成一个固定宽度的格子（含新增/更新标记）。
func futureListDetailCell(row FutureListSavedRow) string {
	mark := "[U]"
	if row.Inserted {
		mark = "[I]"
	}
	// 只展示合约代码；代码缺失时退回 stock_id，便于人工定位是哪个合约。
	body := strings.TrimSpace(row.ContractCode)
	if body == "" {
		body = strconv.FormatInt(row.StockID, 10)
	}
	cell := futureListTruncateDisplay(mark+" "+body, futureListDetailCellWidth)
	return cell + strings.Repeat(" ", futureListDetailCellWidth-futureListDisplayWidth(cell))
}

// futureListDisplayWidth 字符串的终端显示宽度（CJK 全角字符按 2 列计）。
func futureListDisplayWidth(text string) int {
	width := 0
	for _, r := range text {
		if futureListWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// futureListTruncateDisplay 按显示宽度截断（不拆分多字节字符）。
func futureListTruncateDisplay(text string, limit int) string {
	if limit <= 0 || futureListDisplayWidth(text) <= limit {
		return text
	}
	width := 0
	runes := make([]rune, 0, len(text))
	for _, r := range text {
		step := 1
		if futureListWideRune(r) {
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

// futureListWideRune 判断是否为双宽（CJK）字符。
func futureListWideRune(r rune) bool {
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

// futureListSampleFields 拼出首条合约的「fieldId=fieldName/类型/精度」清单。
//
// 该日志是确认 fields 位置化字典的唯一现场证据：本仓库只把 1/2/3/4/19 提成列，
// 其余编号的含义靠这条日志 + 表内 field_list_json 留档确认后再补提列。
func futureListSampleFields(items []quoteapiv2.FuturesListItem) string {
	if len(items) == 0 {
		return "（空）"
	}
	item := items[0]
	parts := make([]string, 0, len(item.FieldList))
	for _, field := range item.FieldList {
		parts = append(parts, fmt.Sprintf("%d=%s/type%d/prec%d",
			field.FieldID, strings.TrimSpace(field.FieldName), field.ContentType, field.Precision))
	}
	extra := make([]string, 0, 4)
	for id := range item.Fields {
		known := false
		for _, field := range item.FieldList {
			if strconv.Itoa(field.FieldID) == id {
				known = true
				break
			}
		}
		if !known {
			extra = append(extra, id)
		}
	}
	text := fmt.Sprintf("stockId=%d %s", item.StockID, strings.Join(parts, " "))
	if len(extra) > 0 {
		// 键序按字典序输出：map 遍历顺序随机，日志要可对比就不能抖。
		sort.Strings(extra)
		text += fmt.Sprintf("（fields 独有键：%s）", strings.Join(extra, ","))
	}
	return text
}
