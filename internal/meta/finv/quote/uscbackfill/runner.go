package uscbackfill

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// 本文件实现「本地 usc 回推上游」的异步运行（需求 11084，交互与 Tab 1/2 一致）：
// 前端点「立即回推」拿到 run_id，随后每 800ms 轮询读取逐批实时日志与进度。
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

// maxRunLogs 单个运行保留的日志上限（按批一行；73,212 / 500 ≈ 147 批，余量充足）。
const maxRunLogs = 2000

// runTimeout 单次异步运行的整体超时（逐行 PATCH，并发 8）。
const runTimeout = 90 * time.Minute

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

	// KeyTable / KeyColumn / KeyKey 上游被回推的表、列与关联键。
	KeyTable  string `json:"key_table"`
	KeyColumn string `json:"key_column"`
	KeyKey    string `json:"key_key"`
	// LocalTable 本地来源表。
	LocalTable string `json:"local_table"`
	// LocalRows / RowsTotal / LocalEmptyUSC 本地总行数 / 待处理行数 / usc 为空跳过行数。
	LocalRows     int64 `json:"local_rows"`
	RowsTotal     int64 `json:"rows_total"`
	LocalEmptyUSC int64 `json:"local_empty_usc"`
	// Fill / Overwrite / Keep / Missing 累计计数。
	Fill      int `json:"fill"`
	Overwrite int `json:"overwrite"`
	Keep      int `json:"keep"`
	Missing   int `json:"missing"`
	Batches   int `json:"batches"`

	// BatchSize 每批处理的 ftmm_symbol 个数。
	BatchSize int `json:"batch_size"`
	// Fields 本次回推写的列（需求 11163）：{usc, priority}，默认都 true。
	Fields FieldSelection `json:"fields"`
	// RowsDone 累计已处理键值数；Progress 为 0~100。
	RowsDone   int      `json:"rows_done"`
	Progress   float64  `json:"progress"`
	StartAt    string   `json:"start_at"`
	FinishAt   string   `json:"finish_at,omitempty"`
	DurationMs int64    `json:"duration_ms"`
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

// Start 启动一次异步回推；已有运行在执行时返回错误（不排队、不覆盖）。
func (r *Runner) Start(req Request) (Run, error) {
	// 字段选择先归一：两个都不勾直接拒绝，不占用运行位
	fields, err := NormalizeFields(req.Fields)
	if err != nil {
		return Run{}, err
	}
	normalized := req
	normalized.Fields = &fields

	r.mu.Lock()
	if r.current != nil && r.current.run.Status == RunStatusRunning {
		runID := r.current.run.RunID
		r.mu.Unlock()
		return Run{}, fmt.Errorf("已有回推任务在运行（run_id=%s），请等待完成", runID)
	}
	runID := fmt.Sprintf("sk-ub-%s", r.now().Format("20060102-150405.000000"))
	state := &runnerState{
		started: r.now(),
		run: Run{
			RunID:      runID,
			Kind:       "uscbackfill",
			Status:     RunStatusRunning,
			KeyTable:   KeyTable,
			KeyColumn:  KeyUSCColumn,
			KeyKey:     KeyKeyColumn,
			LocalTable: LocalKeyTable,
			BatchSize:  NormalizeBatchSize(req.BatchSize),
			Fields:     fields,
			StartAt:    r.now().Format(time.RFC3339),
		},
	}
	r.current = state
	r.lastID = runID
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"回推任务已启动：%s ← %s（按 %s 关联，本次写 %s 列；未勾选的列不动）",
		KeyTable, LocalKeyTable, KeyKeyColumn, fields.Text()), 0, 0)
	snapshot := r.snapshotLocked(state)
	r.mu.Unlock()

	go r.execute(state, normalized)
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

// execute 在后台跑回推，把进度与结果写进日志。
func (r *Runner) execute(state *runnerState, req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	result, err := r.service.SyncWithProgress(ctx, req, func(progress Progress) {
		r.mu.Lock()
		defer r.mu.Unlock()

		if progress.RowsTotal > 0 {
			state.run.RowsTotal = progress.RowsTotal
		}
		state.run.RowsDone = progress.RowsDone
		state.run.Progress = progressPercent(progress.RowsDone, progress.RowsTotal)
		state.run.DurationMs = r.now().Sub(state.started).Milliseconds()

		deltaFill := progress.Fill - state.run.Fill
		deltaOverwrite := progress.Overwrite - state.run.Overwrite
		deltaKeep := progress.Keep - state.run.Keep
		deltaMissing := progress.Missing - state.run.Missing

		state.run.Fill = progress.Fill
		state.run.Overwrite = progress.Overwrite
		state.run.Keep = progress.Keep
		state.run.Missing = progress.Missing
		state.run.Batches = progress.Batch

		r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
			"第 %d 批：offset=%d，处理 %d 个 %s，写入 %d（填入 %d / 覆盖 %d），未变 %d，无对应 %d（本批 %d ms）",
			progress.Batch, progress.Offset, progress.Fetched, KeyKeyColumn,
			deltaFill+deltaOverwrite, deltaFill, deltaOverwrite, deltaKeep,
			deltaMissing, progress.DurationMs),
			progress.Batch, progress.Offset)
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	state.run.DurationMs = r.now().Sub(state.started).Milliseconds()
	if err != nil {
		state.run.Status = RunStatusFailed
		state.run.Error = err.Error()
		state.run.FinishAt = r.now().Format(time.RFC3339)
		r.appendLogLocked(state, RunLogLevelError, "回推失败："+err.Error(), 0, 0)
		log.Printf("[cdeu][uscbackfill] run=%s failed: %v", state.run.RunID, err)
		return
	}
	state.run.Status = RunStatusDone
	state.run.LocalRows = result.LocalRows
	state.run.RowsTotal = result.RowsTotal
	state.run.LocalEmptyUSC = result.SkipEmpty
	state.run.Fill = result.Fill
	state.run.Overwrite = result.Overwrite
	state.run.Keep = result.Keep
	state.run.Missing = result.Missing
	state.run.Batches = result.Batches
	state.run.RowsDone = int(result.RowsTotal)
	state.run.Progress = 100
	state.run.FinishAt = r.now().Format(time.RFC3339)
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"回推完成：字段 %s，本地 %s 共 %d 行（usc 有值 %d，为空跳过 %d），%d 批，"+
			"写入 %d（填入 %d / 覆盖 %d），未变 %d，上游无对应 %d，新增 %d（未插入 %d）；耗时 %d ms",
		result.Fields.Text(), LocalKeyTable, result.LocalRows, result.RowsTotal, result.SkipEmpty,
		result.Batches, result.Fill+result.Overwrite, result.Fill, result.Overwrite, result.Keep,
		result.Missing, result.Insert, result.InsertSkip, result.DurationMs), 0, 0)
	log.Printf("[cdeu][uscbackfill] run=%s done fill=%d overwrite=%d keep=%d missing=%d duration_ms=%d",
		state.run.RunID, result.Fill, result.Overwrite, result.Keep, result.Missing, result.DurationMs)
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
		snapshot.DurationMs = r.now().Sub(state.started).Milliseconds()
	}
	return snapshot
}
