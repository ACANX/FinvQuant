package uscbackfill

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// 本文件实现「上游主键参数回填」的异步运行（需求 11054/11056，交互与 Tab 1/2 一致）：
// 前端点「立即回填」拿到 run_id，随后每 800ms 轮询读取逐批实时日志与进度。
// 一次运行分两个阶段：① meta 表 usc（按 stock_id）→ ② key 表 usc + stock_id（按 ftmm_symbol）。
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

// maxRunLogs 单个运行保留的日志上限（按批一行；24,348 + 10,910 个键 / 500 ≈ 71 批，余量充足）。
const maxRunLogs = 2000

// runTimeout 单次异步运行的整体超时（逐行 PATCH，并发 8，实测两阶段约 3 分钟）。
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
	RunID string `json:"run_id"`
	Kind  string `json:"kind"` // 固定 uscbackfill（前端据此区分三个 Tab）
	// Status 运行状态：RUNNING / DONE / FAILED。
	Status string `json:"status"`
	// Phase 当前阶段：meta（表 ①）/ key（表 ②）。
	Phase string `json:"phase"`

	// MetaTable / MetaColumn / MetaKey / LocalTable 阶段 ① 的表与关联键。
	MetaTable  string `json:"meta_table"`
	MetaColumn string `json:"meta_column"`
	MetaKey    string `json:"meta_key"`
	LocalTable string `json:"local_table"`
	// RowsTotal / LocalRows 阶段 ① 待处理 stock_id 总数 / 本地来源行数。
	RowsTotal int64 `json:"rows_total"`
	LocalRows int64 `json:"local_rows"`
	// Fill / Overwrite / Unchanged / Ambiguous / NoTarget 阶段 ① 累计计数。
	Fill      int `json:"fill"`
	Overwrite int `json:"overwrite"`
	Unchanged int `json:"unchanged"`
	Ambiguous int `json:"ambiguous"`
	NoTarget  int `json:"no_target"`
	Batches   int `json:"batches"`

	// KeyTable / KeyColumns / KeyKey 阶段 ② 的表与关联键。
	KeyTable   string `json:"key_table"`
	KeyColumns string `json:"key_columns"`
	KeyKey     string `json:"key_key"`
	// KeyLocalRows / KeyResolvable / KeyUnresolved 阶段 ② 本地行数 / 可回填 / 无依据。
	KeyLocalRows  int64 `json:"key_local_rows"`
	KeyResolvable int64 `json:"key_resolvable"`
	KeyUnresolved int64 `json:"key_unresolved"`
	// KeyFill / KeyOverwrite / KeyUnchanged / KeyAmbiguous / KeyNoTarget 阶段 ② 累计计数。
	KeyFill      int `json:"key_fill"`
	KeyOverwrite int `json:"key_overwrite"`
	KeyUnchanged int `json:"key_unchanged"`
	KeyAmbiguous int `json:"key_ambiguous"`
	KeyNoTarget  int `json:"key_no_target"`
	KeyBatches   int `json:"key_batches"`

	// BatchSize 每批处理的键值个数。
	BatchSize int `json:"batch_size"`
	// RowsDone 两个阶段累计已处理键值数；Progress 为 0~100。
	RowsDone  int     `json:"rows_done"`
	Progress  float64 `json:"progress"`
	StartedAt string  `json:"started_at"`
	// FinishedAt 结束时间；ElapsedMs 总耗时。
	FinishedAt string   `json:"finished_at,omitempty"`
	ElapsedMs  int64    `json:"elapsed_ms"`
	Error      string   `json:"error,omitempty"`
	Logs       []RunLog `json:"logs"`
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

// Start 启动一次异步回填；已有运行在执行时返回错误（不排队、不覆盖）。
func (r *Runner) Start(req Request) (Run, error) {
	r.mu.Lock()
	if r.current != nil && r.current.run.Status == RunStatusRunning {
		runID := r.current.run.RunID
		r.mu.Unlock()
		return Run{}, fmt.Errorf("已有回填任务在运行（run_id=%s），请等待完成", runID)
	}
	runID := fmt.Sprintf("sk-ub-%s", r.now().Format("20060102-150405.000000"))
	state := &runnerState{
		started: r.now(),
		run: Run{
			RunID:      runID,
			Kind:       "uscbackfill",
			Status:     RunStatusRunning,
			Phase:      PhaseMeta,
			MetaTable:  MetaTable,
			MetaColumn: MetaUSCColumn,
			MetaKey:    MetaKeyColumn,
			LocalTable: LocalMetaTable,
			KeyTable:   KeyTable,
			KeyColumns: KeyUSCColumn + " + " + KeyStockIDColumn,
			KeyKey:     KeyKeyColumn,
			BatchSize:  NormalizeBatchSize(req.BatchSize),
			StartedAt:  r.now().Format(time.RFC3339),
		},
	}
	r.current = state
	r.lastID = runID
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"回填任务已启动：① %s.%s（按 %s，来源 %s）→ ② %s（%s，按 %s，来源 %s）",
		MetaTable, MetaUSCColumn, MetaKeyColumn, LocalMetaTable,
		KeyTable, KeyUSCColumn+" + "+KeyStockIDColumn, KeyKeyColumn, LocalKeyTable), 0, 0)
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

// execute 在后台跑回填，把进度与结果写进日志。
func (r *Runner) execute(state *runnerState, req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	result, err := r.service.SyncWithProgress(ctx, req, func(progress Progress) {
		r.mu.Lock()
		defer r.mu.Unlock()

		state.run.Phase = progress.Phase
		if progress.RowsTotal > 0 {
			state.run.RowsTotal = progress.RowsTotal
		}
		state.run.RowsDone = progress.RowsDone
		state.run.Progress = progressPercent(progress.RowsDone, progress.RowsTotal)
		state.run.ElapsedMs = r.now().Sub(state.started).Milliseconds()

		if progress.Phase == PhaseKey {
			// 阶段 ② 的累计值里同时带着阶段 ① 的结果，一并刷新（前端只读一份快照）。
			deltaFill := progress.KeyFill - state.run.KeyFill
			deltaOverwrite := progress.KeyOverwrite - state.run.KeyOverwrite
			deltaUnchanged := progress.KeyUnchanged - state.run.KeyUnchanged
			deltaAmbiguous := progress.KeyAmbiguous - state.run.KeyAmbiguous
			deltaNoTarget := progress.KeyNoTarget - state.run.KeyNoTarget

			state.run.KeyFill = progress.KeyFill
			state.run.KeyOverwrite = progress.KeyOverwrite
			state.run.KeyUnchanged = progress.KeyUnchanged
			state.run.KeyAmbiguous = progress.KeyAmbiguous
			state.run.KeyNoTarget = progress.KeyNoTarget
			state.run.KeyBatches = progress.Batch

			r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
				"键表第 %d 批：offset=%d，处理 %d 个 %s，写入 %d（填入 %d / 覆盖 %d），未变 %d，歧义 %d，无对应 %d（本批 %d ms）",
				progress.Batch, progress.Offset, progress.Fetched, KeyKeyColumn,
				deltaFill+deltaOverwrite, deltaFill, deltaOverwrite, deltaUnchanged,
				deltaAmbiguous, deltaNoTarget, progress.ElapsedMs),
				progress.Batch, progress.Offset)
			return
		}

		deltaFill := progress.Fill - state.run.Fill
		deltaOverwrite := progress.Overwrite - state.run.Overwrite
		deltaUnchanged := progress.Unchanged - state.run.Unchanged
		deltaAmbiguous := progress.Ambiguous - state.run.Ambiguous
		deltaNoTarget := progress.NoTarget - state.run.NoTarget

		state.run.Fill = progress.Fill
		state.run.Overwrite = progress.Overwrite
		state.run.Unchanged = progress.Unchanged
		state.run.Ambiguous = progress.Ambiguous
		state.run.NoTarget = progress.NoTarget
		state.run.Batches = progress.Batch

		r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
			"第 %d 批：offset=%d，处理 %d 个 %s，写入 %d（填入 %d / 覆盖 %d），未变 %d，歧义 %d，无对应 %d（本批 %d ms）",
			progress.Batch, progress.Offset, progress.Fetched, MetaKeyColumn,
			deltaFill+deltaOverwrite, deltaFill, deltaOverwrite, deltaUnchanged,
			deltaAmbiguous, deltaNoTarget, progress.ElapsedMs),
			progress.Batch, progress.Offset)
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	state.run.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	if err != nil {
		state.run.Status = RunStatusFailed
		state.run.Error = err.Error()
		state.run.FinishedAt = r.now().Format(time.RFC3339)
		r.appendLogLocked(state, RunLogLevelError, "回填失败："+err.Error(), 0, 0)
		log.Printf("[cdeu][uscbackfill] run=%s failed: %v", state.run.RunID, err)
		return
	}
	state.run.Status = RunStatusDone
	state.run.Phase = PhaseKey
	state.run.RowsTotal = result.RowsTotal
	state.run.LocalRows = result.LocalRows
	state.run.Fill = result.Fill
	state.run.Overwrite = result.Overwrite
	state.run.Unchanged = result.Unchanged
	state.run.Ambiguous = result.Ambiguous
	state.run.NoTarget = result.NoTarget
	state.run.Batches = result.Batches
	state.run.KeyLocalRows = result.KeyLocalRows
	state.run.KeyResolvable = result.KeyResolvable
	state.run.KeyUnresolved = result.KeyUnresolved
	state.run.KeyFill = result.KeyFill
	state.run.KeyOverwrite = result.KeyOverwrite
	state.run.KeyUnchanged = result.KeyUnchanged
	state.run.KeyAmbiguous = result.KeyAmbiguous
	state.run.KeyNoTarget = result.KeyNoTarget
	state.run.KeyBatches = result.KeyBatches
	state.run.RowsDone = int(result.RowsTotal + result.KeyResolvable)
	state.run.Progress = 100
	state.run.FinishedAt = r.now().Format(time.RFC3339)
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"回填完成：① %s 共 %d 行（%d 个 %s），%d 批，写入 %d（填入 %d / 覆盖 %d），未变 %d，歧义 %d，无对应 %d；"+
			"② %s 共 %d 行，可回填 %d（无依据 %d），%d 批，写入 %d（填入 %d / 覆盖 %d），未变 %d，歧义 %d，无对应 %d；耗时 %d ms",
		LocalMetaTable, result.LocalRows, result.RowsTotal, MetaKeyColumn, result.Batches,
		result.Fill+result.Overwrite, result.Fill, result.Overwrite, result.Unchanged,
		result.Ambiguous, result.NoTarget,
		result.KeyTable, result.KeyLocalRows, result.KeyResolvable, result.KeyUnresolved,
		result.KeyBatches, result.KeyFill+result.KeyOverwrite, result.KeyFill, result.KeyOverwrite,
		result.KeyUnchanged, result.KeyAmbiguous, result.KeyNoTarget, result.ElapsedMs), 0, 0)
	log.Printf("[cdeu][uscbackfill] run=%s done meta_fill=%d meta_overwrite=%d meta_unchanged=%d key_fill=%d key_overwrite=%d key_unchanged=%d elapsed_ms=%d",
		state.run.RunID, result.Fill, result.Overwrite, result.Unchanged,
		result.KeyFill, result.KeyOverwrite, result.KeyUnchanged, result.ElapsedMs)
}

// progressPercent 按已处理键值数 / 总数算进度（总数未知时为 0）。
func progressPercent(done int, total int64) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(done) / float64(total) * 100
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
