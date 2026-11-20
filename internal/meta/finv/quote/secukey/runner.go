package secukey

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// 本文件实现「证券参数同步」的异步运行（需求 11046）：
// 前端点「立即同步」后立刻拿到 run_id，随后轮询 RunStatus 读取**逐批实时日志**与进度，
// 无需等整次同步结束就能看到推进情况（同步执行的老接口仍保留，见 Service.Sync）。
//
// 与 cdeu 的 StockListRunner 同一套路：状态只存内存（进程重启即清空），
// 同一时刻只允许一个运行，日志条数有上限（防止异常循环把内存吃光）。

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

// maxRunLogs 单个运行保留的日志上限（按批一行 + 起止行，7 万行/1000 批 ≈ 74 行，上限给足余量）。
const maxRunLogs = 2000

// runTimeout 单次异步运行的整体超时（百万行量级也够用）。
const runTimeout = 60 * time.Minute

// RunLog 一条实时日志（前端日志区按 seq 顺序追加）。
type RunLog struct {
	Seq    int    `json:"seq"`
	Time   string `json:"time"` // RFC3339（本地时区）
	Level  string `json:"level"`
	Text   string `json:"text"`
	Batch  int    `json:"batch,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// Run 一次异步运行的快照（RunStatus 接口直接返回它）。
type Run struct {
	RunID       string   `json:"run_id"`
	Status      string   `json:"status"` // RUNNING / DONE / FAILED
	SourceTable string   `json:"source_table"`
	BatchSize   int      `json:"batch_size"`
	RowsTotal   int64    `json:"rows_total"`
	RowsReceive int      `json:"rows_receive"`
	Insert      int      `json:"insert"`
	Update      int      `json:"update"`
	Unchanged   int      `json:"unchanged"`
	Skipped     int      `json:"skipped"`
	StatusReset int      `json:"status_reset"`
	Batches     int      `json:"batches"`
	Progress    float64  `json:"progress"` // 0~100（按已拉取行数 / 源表总行数）
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at,omitempty"`
	ElapsedMs   int64    `json:"elapsed_ms"`
	Error       string   `json:"error,omitempty"`
	Logs        []RunLog `json:"logs"`
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
//
// 返回启动瞬间的快照（Status=RUNNING、含首条日志），前端据此先渲染日志区再开始轮询。
func (r *Runner) Start(req SyncRequest) (Run, error) {
	r.mu.Lock()
	if r.current != nil && r.current.run.Status == RunStatusRunning {
		runID := r.current.run.RunID
		r.mu.Unlock()
		return Run{}, fmt.Errorf("已有同步任务在运行（run_id=%s），请等待完成", runID)
	}
	runID := fmt.Sprintf("sk-%s", r.now().Format("20060102-150405.000000"))
	state := &runnerState{
		started: r.now(),
		run: Run{
			RunID:     runID,
			Status:    RunStatusRunning,
			BatchSize: NormalizeBatchSize(req.BatchSize),
			StartedAt: r.now().Format(time.RFC3339),
		},
	}
	r.current = state
	r.lastID = runID
	r.appendLogLocked(state, RunLogLevelInfo, "同步任务已启动", 0, 0)
	snapshot := r.snapshotLocked(state)
	r.mu.Unlock()

	go r.execute(state, req)
	return snapshot, nil
}

// Status 按 runID 读取快照；runID 为空时返回最近一次运行的快照。
func (r *Runner) Status(runID string) (Run, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return Run{}, false
	}
	if runID != "" && runID != r.current.run.RunID {
		// 只保留最近一次运行：请求旧 run_id 时不返回别的运行（避免张冠李戴）。
		return Run{}, false
	}
	return r.snapshotLocked(r.current), true
}

// LatestRunID 最近一次运行的 ID（无运行时为空）。
func (r *Runner) LatestRunID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastID
}

// execute 在后台跑同步，把进度与结果写进日志。
func (r *Runner) execute(state *runnerState, req SyncRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	result, err := r.service.SyncWithProgress(ctx, req, func(progress Progress) {
		r.mu.Lock()
		defer r.mu.Unlock()
		// 进度里的计数是累计值；本批增量靠与上一次累计值相减得到（日志更直观）。
		deltaInsert := progress.Insert - state.run.Insert
		deltaUpdate := progress.Update - state.run.Update
		deltaUnchanged := progress.Unchanged - state.run.Unchanged
		deltaSkipped := progress.Skipped - state.run.Skipped

		if progress.RowsTotal > 0 {
			state.run.RowsTotal = progress.RowsTotal
		}
		state.run.RowsReceive = progress.RowsReceive
		state.run.Insert = progress.Insert
		state.run.Update = progress.Update
		state.run.Unchanged = progress.Unchanged
		state.run.Skipped = progress.Skipped
		state.run.StatusReset = progress.StatusReset
		state.run.Batches = progress.Batch
		state.run.Progress = progressPercent(progress.RowsReceive, state.run.RowsTotal)

		r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
			"第 %d 批：offset=%d，拉取 %d，新增 %d，更新 %d，未变 %d，跳过 %d（本批 %d ms）",
			progress.Batch, progress.Offset, progress.Fetched,
			deltaInsert, deltaUpdate, deltaUnchanged, deltaSkipped, progress.ElapsedMs),
			progress.Batch, progress.Offset)
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	state.run.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	if err != nil {
		state.run.Status = RunStatusFailed
		state.run.Error = err.Error()
		state.run.FinishedAt = r.now().Format(time.RFC3339)
		r.appendLogLocked(state, RunLogLevelError, "同步失败："+err.Error(), 0, 0)
		log.Printf("[cdeu][secukey] run=%s failed: %v", state.run.RunID, err)
		return
	}
	state.run.Status = RunStatusDone
	state.run.SourceTable = result.SourceTable
	state.run.RowsTotal = result.RowsTotal
	state.run.RowsReceive = result.RowsReceive
	state.run.Insert = result.Insert
	state.run.Update = result.Update
	state.run.Unchanged = result.Unchanged
	state.run.Skipped = result.Skipped
	state.run.StatusReset = result.StatusReset
	state.run.Batches = result.Batches
	state.run.Progress = 100
	state.run.FinishedAt = r.now().Format(time.RFC3339)
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"同步完成：源表 %s 共 %d 行，%d 批，新增 %d，更新 %d，未变 %d，跳过 %d，耗时 %d ms",
		result.SourceTable, result.RowsTotal, result.Batches, result.Insert, result.Update,
		result.Unchanged, result.Skipped, result.ElapsedMs), 0, 0)
	log.Printf("[cdeu][secukey] run=%s done source=%s total=%d insert=%d update=%d unchanged=%d elapsed_ms=%d",
		state.run.RunID, result.SourceTable, result.RowsTotal, result.Insert, result.Update,
		result.Unchanged, result.ElapsedMs)
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

// snapshotLocked 复制一份快照（调用方必须持锁）：logs 用副本，避免并发读写同一底层数组。
func (r *Runner) snapshotLocked(state *runnerState) Run {
	snapshot := state.run
	snapshot.Logs = make([]RunLog, len(state.logs))
	copy(snapshot.Logs, state.logs)
	if snapshot.Status == RunStatusRunning {
		snapshot.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	}
	return snapshot
}
