package cdeu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
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
		collector := NewStockListCollector(s.store, s.stockListFetcher, s.store, nil).WithHook(
			func(event StockListCollectEvent) {
				s.stockListRunner.appendLog(runID, StockListCollectEventToLog(event))
			})
		results, collectErr := collector.Collect(context.Background(), normalized)
		s.stockListRunner.finish(runID, results, collectErr)
	}()

	run, _ := s.stockListRunner.snapshot(runID)
	return run, nil
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
