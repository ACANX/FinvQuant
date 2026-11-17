package cdeu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件实现「采集运行管理」：把一次列表采集放到后台执行，并把逐页日志与进度
// 保存在内存里，供网页「行情快照手动同步」轮询展示。
//
// 说明：采集逻辑本身在 quote_list_stock_snapshot_sync.go（可迁移单文件）；
// 这里只负责运行状态与日志的持有，不外发任何存储写入。

const (
	// StockListRunRunning 运行中。
	StockListRunRunning = "RUNNING"
	// StockListRunSuccess 全部市场成功。
	StockListRunSuccess = "SUCCESS"
	// StockListRunPartial 部分市场失败（其余成功）。
	StockListRunPartial = "PARTIAL"
	// StockListRunFailed 启动即失败或全部市场失败。
	StockListRunFailed = "FAILED"

	// stockListRunLogLimit 单次运行保留的日志行数上限（超出丢弃最旧的）。
	stockListRunLogLimit = 500
	// stockListRunKeep 内存中保留的最近运行数（超出丢弃最旧的）。
	stockListRunKeep = 10
)

// StockListRunLog 一行采集日志（前端日志面板直接展示）。
type StockListRunLog struct {
	// Time 日志时间（HH:mm:ss）。
	Time string `json:"time"`
	// Level 级别：INFO / WARN / ERROR。
	Level string `json:"level"`
	// Market 该行所属市场（如 stock/1）；任务级日志为空。
	Market string `json:"market"`
	// Page 相关页码，0 表示与具体页无关。
	Page int `json:"page"`
	// Message 可读消息。
	Message string `json:"message"`
}

// StockListRun 一次采集运行的快照（对外 wire 结构）。
type StockListRun struct {
	RunID      string                   `json:"run_id"`
	Status     string                   `json:"status"`
	StartedAt  time.Time                `json:"started_at"`
	FinishedAt *time.Time               `json:"finished_at,omitempty"`
	ElapsedMs  int64                    `json:"elapsed_ms"`
	Params     map[string]any           `json:"params"`
	Results    []StockListCollectResult `json:"results"`
	Logs       []StockListRunLog        `json:"logs"`
	Error      string                   `json:"error,omitempty"`
}

// StockListRunner 采集运行的持有者（内存，进程重启即清空）。
type StockListRunner struct {
	mu     sync.Mutex
	runs   map[string]*StockListRun
	order  []string
	active string
}

// newStockListRunner 创建运行持有者。
func newStockListRunner() *StockListRunner {
	return &StockListRunner{runs: make(map[string]*StockListRun, stockListRunKeep)}
}

// begin 登记一次新运行；已有运行中的任务时返回错误（同一时间只允许一个采集任务）。
func (r *StockListRunner) begin(params map[string]any) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active != "" {
		if current, ok := r.runs[r.active]; ok && current.Status == StockListRunRunning {
			return "", fmt.Errorf("已有采集任务在运行（run_id=%s），请等待结束或稍后再试", current.RunID)
		}
	}

	runID := stockListRunID()
	run := &StockListRun{
		RunID:     runID,
		Status:    StockListRunRunning,
		StartedAt: time.Now(),
		Params:    params,
		Results:   []StockListCollectResult{},
		Logs:      make([]StockListRunLog, 0, 64),
	}
	r.runs[runID] = run
	r.order = append(r.order, runID)
	r.active = runID

	// 只保留最近 stockListRunKeep 次运行
	for len(r.order) > stockListRunKeep {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.runs, oldest)
	}
	return runID, nil
}

// appendLog 追加一行日志（超过上限丢弃最旧的）。
func (r *StockListRunner) appendLog(runID string, entry StockListRunLog) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[runID]
	if !ok {
		return
	}
	if entry.Time == "" {
		entry.Time = time.Now().Format("15:04:05")
	}
	if entry.Level == "" {
		entry.Level = "INFO"
	}
	if len(run.Logs) >= stockListRunLogLimit {
		run.Logs = run.Logs[len(run.Logs)-stockListRunLogLimit+1:]
	}
	run.Logs = append(run.Logs, entry)
}

// finish 收尾：写状态、结果与错误。
func (r *StockListRunner) finish(runID string, results []StockListCollectResult, runErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[runID]
	if !ok {
		return
	}
	now := time.Now()
	run.FinishedAt = &now
	run.ElapsedMs = now.Sub(run.StartedAt).Milliseconds()
	if results == nil {
		results = []StockListCollectResult{}
	}
	run.Results = results

	failed := 0
	for _, result := range results {
		if result.Error != "" {
			failed++
		}
	}
	switch {
	case runErr != nil && len(results) == 0:
		run.Status = StockListRunFailed
		run.Error = runErr.Error()
	case failed > 0 && failed == len(results):
		run.Status = StockListRunFailed
		if runErr != nil {
			run.Error = runErr.Error()
		}
	case failed > 0:
		run.Status = StockListRunPartial
		if runErr != nil {
			run.Error = runErr.Error()
		}
	default:
		run.Status = StockListRunSuccess
	}
	if r.active == runID {
		r.active = ""
	}
}

// snapshot 返回运行快照的副本（避免调用方直接改内部状态）。
func (r *StockListRunner) snapshot(runID string) (StockListRun, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[runID]
	if !ok {
		return StockListRun{}, false
	}
	copied := *run
	copied.Results = append([]StockListCollectResult(nil), run.Results...)
	copied.Logs = append([]StockListRunLog(nil), run.Logs...)
	return copied, true
}

// stockListRunID 生成运行编号：时间戳 + 4 位随机十六进制。
func stockListRunID() string {
	var buffer [2]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("run-%s-%s", time.Now().Format("20060102T150405"), hex.EncodeToString(buffer[:]))
}

// StartStockListRun 异步启动一次采集，立即返回运行编号；过程日志通过 StockListRun 查询。
func (s *Service) StartStockListRun(opts StockListCollectOptions) (StockListRun, error) {
	normalized := opts.Normalize()
	if s.stockListRunner == nil {
		s.stockListRunner = newStockListRunner()
	}

	// 需求 11262：BDFS（百度股市通）走成分股采集，与 moomoo 列表采集完全独立，
	// 不读配置表、不分市场，落库到 cdeu.finv_quote_secu_realtime（provider='BDFS'）。
	// 已登记来源见 BdfsSources（bsestock = 板块 GN2309，etf = 国证ETF 指数 399380）。
	if src, ok := BdfsSourceBySymbol(normalized.FilterInstrumentSymbol); ok {
		return s.startBdfsSecuRealtimeRun(normalized, src)
	}

	// 需求 11294：沪深京指数（cnindex）走中信 XetApp 全量指数采集（固定来源，无配置行），
	// 结果落 cdeu.finv_quote_secu_realtime（provider='ZXET'）。
	if normalized.FilterInstrumentSymbol == XetIndexSymbol {
		return s.startXetIndexRun(normalized)
	}

	// 需求 11274：期货列表采集（moomoo get-futures-list）与股票列表采集共用同一运行持有者
	// 与轮询端点，按 instrument_symbol=future 分支；结果落 cdeu.finv_quote_realtime_future。
	if normalized.FilterInstrumentSymbol == FuturesInstrumentSymbol {
		return s.startFuturesListRun(normalized)
	}

	params := map[string]any{
		"instrument_symbol":  normalized.FilterInstrumentSymbol,
		"market_type":        normalized.FilterMarketType,
		"count_plan":         normalized.OverrideCountPlan,
		"page_size":          normalized.PageSize,
		"page_start":         normalized.PageStart,
		"interval_ms":        normalized.Interval.Milliseconds(),
		"max_retries":        normalized.MaxRetries,
		"retry_backoff_ms":   normalized.RetryBackoff.Milliseconds(),
		"request_timeout_ms": normalized.RequestTimeout.Milliseconds(),
		"max_pages":          normalized.MaxPages,
		"continue_on_error":  normalized.ContinueOnError,
	}
	runID, err := s.stockListRunner.begin(params)
	if err != nil {
		return StockListRun{}, err
	}

	s.stockListRunner.appendLog(runID, StockListRunLog{
		Level: "INFO",
		Message: fmt.Sprintf("任务已启动：每页 %d 条、页间间隔 %s、重试 %d 次",
			normalized.PageSize, normalized.Interval, normalized.MaxRetries),
	})

	go func() {
		collector := NewStockListCollector(s.store, s.stockListFetcher, s.store, nil).
			WithEtfFetcher(s.etfListFetcher).
			WithHook(func(event StockListCollectEvent) {
				s.stockListRunner.appendLog(runID, StockListCollectEventToLog(event))
			})
		results, collectErr := collector.Collect(context.Background(), normalized)
		// 需求 11288：本链路写的是 cdeu.finv_quote_secu_realtime，采集结束后自动补一次
		// 人民币计价指标（新入库的行也能立刻带上 rmb_*，不必再手动点按钮）。
		s.syncRmbAfterCollect(context.Background(), runID, "证券列表采集")
		s.stockListRunner.finish(runID, results, collectErr)
	}()

	run, _ := s.stockListRunner.snapshot(runID)
	return run, nil
}

// startFuturesListRun 启动一次期货列表采集（需求 11274）。
//
// 与股票列表采集共用 StockListRunner（运行状态 + 日志 + 轮询接口），
// 因此前端「行情快照同步」页无需新增接口：同一个 /Run 接口按 instrument_symbol 分支。
// 采集范围来自 cdeu.finv_quote_cdeu_future_sync_config，结果落 cdeu.finv_quote_realtime_future。
func (s *Service) startFuturesListRun(opts StockListCollectOptions) (StockListRun, error) {
	if s.stockListRunner == nil {
		s.stockListRunner = newStockListRunner()
	}
	futuresOpts := FutureListCollectOptions{
		PageSize:          opts.PageSize,
		PageStart:         opts.PageStart,
		Interval:          opts.Interval,
		MaxRetries:        opts.MaxRetries,
		RetryBackoff:      opts.RetryBackoff,
		RequestTimeout:    opts.RequestTimeout,
		MaxPages:          opts.MaxPages,
		ContinueOnError:   opts.ContinueOnError,
		FilterFuturesType: opts.FilterFuturesType,
		FilterMarketType:  opts.FilterMarketType,
		OverrideCountPlan: opts.OverrideCountPlan,
		MarkOffline:       true,
		Extra:             opts.Extra,
	}.Normalize()

	params := map[string]any{
		"instrument_symbol":  FuturesInstrumentSymbol,
		"futures_type":       futuresOpts.FilterFuturesType,
		"market_type":        futuresOpts.FilterMarketType,
		"count_plan":         futuresOpts.OverrideCountPlan,
		"page_size":          futuresOpts.PageSize,
		"page_start":         futuresOpts.PageStart,
		"interval_ms":        futuresOpts.Interval.Milliseconds(),
		"max_retries":        futuresOpts.MaxRetries,
		"retry_backoff_ms":   futuresOpts.RetryBackoff.Milliseconds(),
		"request_timeout_ms": futuresOpts.RequestTimeout.Milliseconds(),
		"max_pages":          futuresOpts.MaxPages,
		"continue_on_error":  futuresOpts.ContinueOnError,
		"target_table":       tableFutureList,
	}
	runID, err := s.stockListRunner.begin(params)
	if err != nil {
		return StockListRun{}, err
	}

	s.stockListRunner.appendLog(runID, StockListRunLog{
		Level: "INFO",
		Message: fmt.Sprintf("任务已启动：期货列表采集，每页 %d 条、页间间隔 %s、重试 %d 次、分类=%s",
			futuresOpts.PageSize, futuresOpts.Interval, futuresOpts.MaxRetries,
			futureListFilterLabel(futuresOpts.FilterFuturesType)),
	})

	go func() {
		fetcher := s.futureListFetcher
		if fetcher == nil {
			fetcher = quoteapiv2.New(quoteapiv2.Config{})
		}
		collector := NewFutureListCollector(s.store, fetcher, s.store, nil).WithHook(
			func(event StockListCollectEvent) {
				s.stockListRunner.appendLog(runID, StockListCollectEventToLog(event))
			})
		results, collectErr := collector.Collect(context.Background(), futuresOpts)
		s.stockListRunner.finish(runID, toStockListResults(results), collectErr)
	}()

	run, _ := s.stockListRunner.snapshot(runID)
	return run, nil
}

// toStockListResults 把期货结果转成运行快照用的统一结果类型。
//
// 运行快照的 results 是 []StockListCollectResult；期货分类维度（futures_type /
// futures_type_name / rows_offline）在 wire 上额外用 futures_* 键透出，
// 前端期货 Tab 按这些键展示「分类」，股票分支不受影响。
func toStockListResults(results []FutureListCollectResult) []StockListCollectResult {
	if len(results) == 0 {
		return []StockListCollectResult{}
	}
	out := make([]StockListCollectResult, 0, len(results))
	for _, item := range results {
		out = append(out, StockListCollectResult{
			InstrumentSymbol: item.InstrumentSymbol,
			MarketType:       item.MarketType,
			FuturesType:      item.FuturesType,
			FuturesTypeName:  item.FuturesTypeName,
			Pages:            item.Pages,
			RowsReceived:     item.RowsReceived,
			RowsInsert:       item.RowsInsert,
			RowsUpdate:       item.RowsUpdate,
			CountExists:      item.CountExists,
			TotalAvailable:   item.TotalAvailable,
			ElapsedMs:        item.ElapsedMs,
			Error:            item.Error,
		})
	}
	return out
}

// futureListFilterLabel 分类过滤条件的日志文案。
func futureListFilterLabel(filter string) string {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" || trimmed == FuturesTypeAll {
		return "全部启用分类"
	}
	return "分类 " + trimmed
}

// FuturesConfigs 读取期货列表采集配置（网页「行情快照同步」期货 Tab 展示用）。
func (s *Service) FuturesConfigs(ctx context.Context) ([]FutureListCollectConfig, error) {
	return s.store.LoadFuturesConfigs(ctx)
}

// syncRmbAfterCollect 采集结束后自动补全人民币计价指标（需求 11288）。
//
// 只在「本轮确实写过 cdeu.finv_quote_secu_realtime」的链路调用：
// moomoo 列表采集（stock / etf）、BDFS 成分股采集（bsestock / cnetf）与
// 沪深京指数采集（cnindex，需求 11294）；
// 期货采集写的是 cdeu.finv_quote_realtime_future，与 rmb 无关，不调用。
//
// 幂等：SyncRMBMetrics 只在值发生变化时更新（SQL 里有 IS DISTINCT FROM 条件），
// 所以每轮采集都跑不会产生额外写入；失败只写 WARN 日志，不改采集本身的结果状态
// （采集成功就该是成功，rmb 补全是附加动作）。
func (s *Service) syncRmbAfterCollect(ctx context.Context, runID, label string) {
	started := time.Now()
	result, err := s.store.SyncRMBMetrics(ctx, RMBMetricsStepAll)
	if s.stockListRunner == nil {
		return
	}
	level, message := rmbSyncLogMessage(label, result, err, time.Since(started))
	s.stockListRunner.appendLog(runID, StockListRunLog{Level: level, Message: message})
}

// rmbSyncLogMessage 把一次人民币指标补全的结果转成一行日志（纯函数，便于单测）。
func rmbSyncLogMessage(label string, result RMBMetricsSyncResult, err error, elapsed time.Duration) (level, message string) {
	if err != nil {
		return "WARN", fmt.Sprintf("人民币计价指标自动补全失败（%s）：%v", label, err)
	}
	level = "INFO"
	message = fmt.Sprintf("人民币计价指标已自动补全（%s）：汇率 %d 行、金额 %d 行、耗时 %d ms",
		label, result.RateUpdate, result.AmountUpdate, elapsed.Milliseconds())
	if len(result.MissingCurrencies) > 0 {
		level = "WARN"
		message += fmt.Sprintf("；缺汇率币种：%s", strings.Join(result.MissingCurrencies, "、"))
	}
	return level, message
}

// StockListRunStatus 查询某次运行的快照；runID 为空时返回最近一次运行。
func (s *Service) StockListRunStatus(runID string) (StockListRun, bool) {
	if s.stockListRunner == nil {
		return StockListRun{}, false
	}
	return s.stockListRunner.snapshot(runID)
}

// LatestStockListRun 返回最近一次运行的编号（无运行时返回空串）。
func (s *Service) LatestStockListRun() string {
	if s.stockListRunner == nil {
		return ""
	}
	return s.stockListRunner.latest()
}

// latest 返回最近一次运行的编号。
func (r *StockListRunner) latest() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.order) == 0 {
		return ""
	}
	return r.order[len(r.order)-1]
}

// StockListConfigs 读取采集配置（网页「行情快照手动同步」展示用）。
func (s *Service) StockListConfigs(ctx context.Context) ([]StockListCollectConfig, error) {
	return s.store.LoadStockListConfigs(ctx)
}

// StockListCollectEventToLog 把采集事件转成一行可读日志。
func StockListCollectEventToLog(event StockListCollectEvent) StockListRunLog {
	level := "INFO"
	if event.Stage == "retry" {
		level = "WARN"
	}
	if event.Stage == "done" && strings.HasPrefix(event.Message, "采集失败") {
		level = "ERROR"
	}
	market := event.Config.InstrumentSymbol + "/" + event.Config.MarketType
	entry := StockListRunLog{
		Level:   level,
		Market:  market,
		Page:    event.Page,
		Message: event.Message,
	}
	if !event.Time.IsZero() {
		entry.Time = event.Time.Format("15:04:05")
	}
	return entry
}

// startXetIndexRun 启动一次沪深京指数采集（需求 11294）。
//
// 与 BDFS / 期货分支同位置：共用 StockListRunner（运行状态 + 日志 + 轮询接口），
// 固定来源（无配置行），结果写 cdeu.finv_quote_secu_realtime（provider='ZXET'）。
func (s *Service) startXetIndexRun(opts StockListCollectOptions) (StockListRun, error) {
	if s.stockListRunner == nil {
		s.stockListRunner = newStockListRunner()
	}

	params := map[string]any{
		"instrument_symbol":  XetIndexSymbol,
		"provider":           XetIndexProvider,
		"market_type":        XetIndexMarketType,
		"request_timeout_ms": opts.RequestTimeout.Milliseconds(),
		"target_table":       tableQuoteSecuRealtime,
	}
	runID, err := s.stockListRunner.begin(params)
	if err != nil {
		return StockListRun{}, err
	}

	s.stockListRunner.appendLog(runID, StockListRunLog{
		Level:   "INFO",
		Message: fmt.Sprintf("任务已启动：沪深京指数（XetApp，provider=%s），上游 4 池 + 深市 1200 + 北交所 1400 合并去重", XetIndexProvider),
	})

	go func() {
		fetcher := s.xetIndexFetcher
		if fetcher == nil {
			fetcher = NewXetIndexFetcher()
		}
		// 指数是 6 次串行请求，沿用列表采集的 20s 单页超时不够，至少给 XetIndexDefaultTimeout。
		timeout := opts.RequestTimeout
		if timeout < XetIndexDefaultTimeout {
			timeout = XetIndexDefaultTimeout
		}
		collector := NewXetIndexCollector(fetcher, s.store, s.store, nil).WithHook(
			func(event StockListCollectEvent) {
				s.stockListRunner.appendLog(runID, StockListCollectEventToLog(event))
			})
		result, collectErr := collector.Collect(context.Background(), XetIndexCollectOptions{Timeout: timeout})
		// 需求 11288：本链路写 cdeu.finv_quote_secu_realtime，采集后自动补人民币计价指标。
		s.syncRmbAfterCollect(context.Background(), runID, "沪深京指数")
		s.stockListRunner.finish(runID, []StockListCollectResult{result}, collectErr)
	}()

	run, _ := s.stockListRunner.snapshot(runID)
	return run, nil
}

// startBdfsSecuRealtimeRun 启动一次 BDFS 成分股采集（需求 11262）。
//
// 与 moomoo 列表采集共用 StockListRunner（运行状态 + 日志 + 轮询接口），
// 因此前端「行情快照同步」页无需新增接口：同一条 /Run 接口按 instrument_symbol 分支。
// src 决定数据来源（板块 / 指数）与写入的 instrument_type。
func (s *Service) startBdfsSecuRealtimeRun(opts StockListCollectOptions, src BdfsSource) (StockListRun, error) {
	if s.stockListRunner == nil {
		s.stockListRunner = newStockListRunner()
	}

	params := map[string]any{
		"instrument_symbol":  src.Symbol,
		"provider":           BdfsProvider,
		"finance_type":       src.FinanceType,
		"source_code":        src.Code,
		"page_size":          opts.PageSize,
		"request_timeout_ms": opts.RequestTimeout.Milliseconds(),
		"target_table":       tableQuoteSecuRealtime,
	}
	runID, err := s.stockListRunner.begin(params)
	if err != nil {
		return StockListRun{}, err
	}

	s.stockListRunner.appendLog(runID, StockListRunLog{
		Level:   "INFO",
		Message: fmt.Sprintf("任务已启动：BDFS %s（provider=%s），每页 %d 条", src.Label, BdfsProvider, opts.PageSize),
	})

	go func() {
		fetcher := s.bdfsConstituentsFetcher
		if fetcher == nil {
			fetcher = NewFinscopeConstituentsFetcher()
		}
		collector := NewBdfsSecuRealtimeCollector(fetcher, s.store, s.store, nil).WithHook(
			func(event StockListCollectEvent) {
				s.stockListRunner.appendLog(runID, StockListCollectEventToLog(event))
			})
		result, collectErr := collector.Collect(context.Background(), BdfsCollectOptions{
			Source:   src,
			PageSize: opts.PageSize,
			Timeout:  opts.RequestTimeout,
		})
		// 需求 11288：BDFS 也写 cdeu.finv_quote_secu_realtime —— 采集后自动补一次人民币计价指标。
		// 历史上正是这条链路把 rmb_* 写成 NULL（需求 11287 已改成「有值才覆盖」），
		// 这里再兜一层：新入库的行也要立刻有 rmb 值。
		s.syncRmbAfterCollect(context.Background(), runID, src.Label)
		s.stockListRunner.finish(runID, []StockListCollectResult{result}, collectErr)
	}()

	run, _ := s.stockListRunner.snapshot(runID)
	return run, nil
}

// SetBdfsConstituentsFetcher 注入 BDFS 成分股拉取实现（单测用；传 nil 恢复默认 finscope 客户端）。
func (s *Service) SetBdfsConstituentsFetcher(fetcher BdfsConstituentsFetcher) {
	s.bdfsConstituentsFetcher = fetcher
}
