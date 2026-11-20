package indicator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// 本文件实现「FT证券指标数据同步」的异步运行（需求 11050，交互与 Tab 1 一致）：
// 前端点「立即同步」拿到 run_id，随后轮询读取逐批实时日志与进度。
// 状态只存内存（进程重启即清空）；同一时刻只允许一个运行；日志上限 2000 行。

// 运行状态取值。
const (
	RunStatusRunning = "RUNNING"
	RunStatusDone    = "DONE"
	RunStatusFailed  = "FAILED"
)

// 日志级别。
const (
	RunLogLevelInfo  = "INFO"
	RunLogLevelError = "ERROR"
)

// maxRunLogs 单个运行保留的日志上限（按批一行）。
const maxRunLogs = 2000

// runTimeout 单次异步运行的整体超时。
const runTimeout = 60 * time.Minute

// RunLog 一条实时日志。
type RunLog struct {
	Seq    int    `json:"seq"`
	Time   string `json:"time"`
	Level  string `json:"level"`
	Text   string `json:"text"`
	Batch  int    `json:"batch,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Run 一次异步运行的快照。
type Run struct {
	RunID        string   `json:"run_id"`
	Kind         string   `json:"kind"` // 固定 indicator（前端据此区分两个 Tab）
	Status       string   `json:"status"`
	SourceTable  string   `json:"source_table"`
	BatchSize    int      `json:"batch_size"`
	RowsTotal    int64    `json:"rows_total"`
	RowsReceive  int      `json:"rows_receive"`
	IndInsert    int      `json:"ind_insert"`
	IndUpdate    int      `json:"ind_update"`
	IndUnchanged int      `json:"ind_unchanged"`
	QcInsert     int      `json:"qc_insert"`
	QcUpdate     int      `json:"qc_update"`
	QcSkip       int      `json:"qc_skip"`
	Batches      int      `json:"batches"`
	Progress     float64  `json:"progress"`
	StartedAt    string   `json:"started_at"`
	FinishedAt   string   `json:"finished_at,omitempty"`
	ElapsedMs    int64    `json:"elapsed_ms"`
	Error        string   `json:"error,omitempty"`
	Logs         []RunLog `json:"logs"`
}

// runnerState Runner 的内部可变状态。
type runnerState struct {
	run     Run
	started time.Time
	logs    []RunLog
	seq     int
}

// Runner 异步运行持有者（并发安全）。
type Runner struct {
	service *Service
	now     func() time.Time

	mu      sync.Mutex
	current *runnerState
	lastID  string
}

// NewRunner 创建异步运行器。
func NewRunner(service *Service) *Runner {
	return &Runner{service: service, now: time.Now}
}

// Start 启动一次异步同步；已有运行在执行时返回错误（不排队、不覆盖）。
func (r *Runner) Start(req Request) (Run, error) {
	r.mu.Lock()
	if r.current != nil && r.current.run.Status == RunStatusRunning {
		runID := r.current.run.RunID
		r.mu.Unlock()
		return Run{}, fmt.Errorf("已有指标同步任务在运行（run_id=%s），请等待完成", runID)
	}
	runID := fmt.Sprintf("sk-ind-%s", r.now().Format("20060102-150405.000000"))
	state := &runnerState{
		started: r.now(),
		run: Run{
			RunID:       runID,
			Kind:        "indicator",
			Status:      RunStatusRunning,
			SourceTable: SourceTable,
			BatchSize:   NormalizeBatchSize(req.BatchSize),
			StartedAt:   r.now().Format(time.RFC3339),
		},
	}
	r.current = state
	r.lastID = runID
	r.appendLogLocked(state, RunLogLevelInfo, "指标同步任务已启动", 0, 0)
	snapshot := r.snapshotLocked(state)
	r.mu.Unlock()

	go r.execute(state, req)
	return snapshot, nil
}

// Status 按 runID 读取快照；runID 为空时返回最近一次运行。
func (r *Runner) Status(runID string) (Run, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return Run{}, false
	}
	if runID != "" && runID != r.current.run.RunID {
		return Run{}, false
	}
	return r.snapshotLocked(r.current), true
}

// execute 后台跑同步，把进度与结果写进日志。
func (r *Runner) execute(state *runnerState, req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	result, err := r.service.SyncWithProgress(ctx, req, func(progress Progress) {
		r.mu.Lock()
		defer r.mu.Unlock()
		// 进度里是累计值；本批差值靠与上一次累计值相减（日志更直观）。
		deltaIndInsert := progress.IndInsert - state.run.IndInsert
		deltaIndUpdate := progress.IndUpdate - state.run.IndUpdate
		deltaIndUnchanged := progress.IndUnchanged - state.run.IndUnchanged
		deltaQcInsert := progress.QcInsert - state.run.QcInsert
		deltaQcUpdate := progress.QcUpdate - state.run.QcUpdate
		deltaQcSkip := progress.QcSkip - state.run.QcSkip

		if progress.RowsTotal > 0 {
			state.run.RowsTotal = progress.RowsTotal
		}
		state.run.RowsReceive = progress.RowsReceive
		state.run.IndInsert = progress.IndInsert
		state.run.IndUpdate = progress.IndUpdate
		state.run.IndUnchanged = progress.IndUnchanged
		state.run.QcInsert = progress.QcInsert
		state.run.QcUpdate = progress.QcUpdate
		state.run.QcSkip = progress.QcSkip
		state.run.Batches = progress.Batch
		state.run.Progress = progressPercent(progress.RowsReceive, state.run.RowsTotal)

		r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
			"第 %d 批：offset=%d，拉取 %d，指标 新增 %d / 更新 %d / 未变 %d，采集属性 新增 %d / 更新 %d / 跳过 %d（本批 %d ms）",
			progress.Batch, progress.Offset, progress.Fetched,
			deltaIndInsert, deltaIndUpdate, deltaIndUnchanged,
			deltaQcInsert, deltaQcUpdate, deltaQcSkip, progress.ElapsedMs),
			progress.Batch, progress.Offset)
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	state.run.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	if err != nil {
		state.run.Status = RunStatusFailed
		state.run.Error = err.Error()
		state.run.FinishedAt = r.now().Format(time.RFC3339)
		r.appendLogLocked(state, RunLogLevelError, "指标同步失败："+err.Error(), 0, 0)
		log.Printf("[cdeu][indicator] run=%s failed: %v", state.run.RunID, err)
		return
	}
	state.run.Status = RunStatusDone
	state.run.RowsTotal = result.RowsTotal
	state.run.RowsReceive = result.RowsReceive
	state.run.IndInsert = result.IndInsert
	state.run.IndUpdate = result.IndUpdate
	state.run.IndUnchanged = result.IndUnchanged
	state.run.QcInsert = result.QcInsert
	state.run.QcUpdate = result.QcUpdate
	state.run.QcSkip = result.QcSkip
	state.run.Batches = result.Batches
	state.run.Progress = 100
	state.run.FinishedAt = r.now().Format(time.RFC3339)
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"指标同步完成：源表 %s 共 %d 行，%d 批；指标表 新增 %d / 更新 %d / 未变 %d；采集属性表 新增 %d / 更新 %d / 跳过 %d；耗时 %d ms",
		SourceTable, result.RowsTotal, result.Batches,
		result.IndInsert, result.IndUpdate, result.IndUnchanged,
		result.QcInsert, result.QcUpdate, result.QcSkip, result.ElapsedMs), 0, 0)
	log.Printf("[cdeu][indicator] run=%s done source=%s total=%d ind_insert=%d ind_update=%d qc_insert=%d qc_update=%d elapsed_ms=%d",
		state.run.RunID, SourceTable, result.RowsTotal, result.IndInsert, result.IndUpdate,
		result.QcInsert, result.QcUpdate, result.ElapsedMs)
}

// progressPercent 按已拉取行数 / 源表总行数算进度（总行数未知时为 0）。
func progressPercent(received int, total int64) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(received) / float64(total) * 100
	if value > 100 {
		return 100
	}
	return value
}

// appendLogLocked 追加一条日志（调用方必须持锁）；超过上限时保留最近的 maxRunLogs 条。
func (r *Runner) appendLogLocked(state *runnerState, level, text string, batch, offset int) {
	state.seq++
	state.logs = append(state.logs, RunLog{
		Seq:    state.seq,
		Time:   r.now().Format(time.RFC3339),
		Level:  level,
		Text:   text,
		Batch:  batch,
		Offset: offset,
	})
	if len(state.logs) > maxRunLogs {
		state.logs = state.logs[len(state.logs)-maxRunLogs:]
	}
}

// snapshotLocked 复制一份快照（调用方必须持锁）。
func (r *Runner) snapshotLocked(state *runnerState) Run {
	snapshot := state.run
	snapshot.Logs = make([]RunLog, len(state.logs))
	copy(snapshot.Logs, state.logs)
	if snapshot.Status == RunStatusRunning {
		snapshot.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	}
	return snapshot
}
