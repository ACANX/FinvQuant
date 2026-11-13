package cdeu

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// 本文件实现 USC 补丁执行器（需求 10931，界面要求见 10932 的日志展示约定）。
//
// 与 brs.Runner 同一套模式：内存状态 + 增量日志 + 手动/定时两种触发，
// 前端按 sinceSeq 轮询，只取新增日志。这样「执行」不再是同步阻塞请求，
// 页面可以看到逐批日志与实时统计。

const (
	// uscPatchStatusIdle 尚未执行过。
	uscPatchStatusIdle = "idle"
	// uscPatchStatusRunning 正在执行。
	uscPatchStatusRunning = "running"
	// uscPatchStatusSucceeded 最近一次执行成功。
	uscPatchStatusSucceeded = "succeeded"
	// uscPatchStatusFailed 最近一次执行失败。
	uscPatchStatusFailed = "failed"
	// uscPatchMaxLogEntries 单轮运行最多保留的日志条数（超出丢弃最旧的，seq 不回退）。
	uscPatchMaxLogEntries = 1000
	// uscPatchDefaultBatch 单批扫描行数上限。
	uscPatchDefaultBatch = 5000
)

// UscPatchLogEntry 一条补丁执行日志。
type UscPatchLogEntry struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// UscPatchRunState 补丁执行状态快照。
type UscPatchRunState struct {
	RunID        string             `json:"run_id"`
	Trigger      string             `json:"trigger"` // manual / scheduled
	Status       string             `json:"status"`
	StartedAt    *time.Time         `json:"started_at"`
	FinishedAt   *time.Time         `json:"finished_at"`
	ElapsedMs    int64              `json:"elapsed_ms"`
	RuleCount    int64              `json:"rule_count"`
	Batches      int64              `json:"batches"`
	ScannedRows  int64              `json:"scanned_rows"`
	MovedRows    int64              `json:"moved_rows"`
	ConflictRows int64              `json:"conflict_rows"`
	DeletedRows  int64              `json:"deleted_rows"`
	CurrentBatch string             `json:"current_batch"`
	Error        string             `json:"error"`
	Logs         []UscPatchLogEntry `json:"logs"`
	LastSeq      int64              `json:"last_seq"`
}

// UscPatchBusyError 已有补丁任务在运行。
type UscPatchBusyError struct {
	RunningTrigger string
	RunningSince   *time.Time
}

// Error 实现 error 接口。
func (e *UscPatchBusyError) Error() string {
	since := ""
	if e.RunningSince != nil {
		since = "，开始于 " + e.RunningSince.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf("已有补丁任务在运行（%s%s），请等待其结束", e.RunningTrigger, since)
}

// UscPatchRunner 补丁执行器：内存状态 + 日志，支持手动与定时两种触发。
type UscPatchRunner struct {
	store *Store

	mu      sync.Mutex
	state   UscPatchRunState
	seq     int64
	baseCtx context.Context
}

// NewUscPatchRunner 创建补丁执行器。
func NewUscPatchRunner(store *Store) *UscPatchRunner {
	return &UscPatchRunner{
		store: store,
		state: UscPatchRunState{Trigger: "manual", Status: uscPatchStatusIdle, Logs: []UscPatchLogEntry{}},
	}
}

// Start 注入运行生命周期上下文（服务级，不能用 HTTP 请求的 ctx）。
func (r *UscPatchRunner) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseCtx = ctx
}

// TriggerManual 手动触发一轮补丁（异步执行）。batchSize<=0 时用默认 5000 行/批。
func (r *UscPatchRunner) TriggerManual(batchSize int) (UscPatchRunState, error) {
	return r.trigger("manual", batchSize)
}

// TriggerScheduled 定时触发一轮补丁（异步执行）。
func (r *UscPatchRunner) TriggerScheduled() (UscPatchRunState, error) {
	return r.trigger("scheduled", uscPatchDefaultBatch)
}

// Busy 是否有运行中的补丁任务。
func (r *UscPatchRunner) Busy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.Status == uscPatchStatusRunning
}

// trigger 领取运行权并异步执行；已有运行中时返回 *UscPatchBusyError。
func (r *UscPatchRunner) trigger(triggerType string, batchSize int) (UscPatchRunState, error) {
	r.mu.Lock()
	if r.state.Status == uscPatchStatusRunning {
		busy := &UscPatchBusyError{RunningTrigger: r.state.Trigger, RunningSince: r.state.StartedAt}
		r.mu.Unlock()
		return UscPatchRunState{}, busy
	}
	now := time.Now()
	r.state = UscPatchRunState{
		RunID:     now.Format("20060102-150405"),
		Trigger:   triggerType,
		Status:    uscPatchStatusRunning,
		StartedAt: &now,
		Logs:      make([]UscPatchLogEntry, 0, 64),
	}
	snapshot := r.snapshotLocked(0)
	runCtx := r.baseCtx
	if runCtx == nil {
		runCtx = context.Background()
	}
	r.mu.Unlock()

	if batchSize <= 0 {
		batchSize = uscPatchDefaultBatch
	}
	go r.run(runCtx, triggerType, batchSize)
	return snapshot, nil
}

// run 执行一轮补丁并维护状态与日志。
func (r *UscPatchRunner) run(ctx context.Context, triggerType string, batchSize int) {
	r.appendLog("info", fmt.Sprintf("开始执行过时 USC 补丁（run_id=%s，触发=%s）", r.runID(), triggerType))

	started := time.Now()
	stats, err := r.store.PatchOutdatedUsc(ctx, batchSize, func(s UscPatchStats) {
		r.mu.Lock()
		r.state.RuleCount = s.RuleCount
		r.state.Batches = s.Batches
		r.state.ScannedRows = s.ScannedRows
		r.state.MovedRows = s.MovedRows
		r.state.ConflictRows = s.ConflictRows
		r.state.DeletedRows = s.DeletedRows
		r.state.CurrentBatch = formatTsRange(s.LastBatchFrom, s.LastBatchTo)
		r.mu.Unlock()
		r.appendLog("info", fmt.Sprintf(
			"第 %d 批 %s：本批扫描 %d 行；累计迁移 %d 行、冲突 %d 行、删除 %d 行；本批耗时 %d ms",
			s.Batches, formatTsRange(s.LastBatchFrom, s.LastBatchTo), s.LastBatchRows,
			s.MovedRows, s.ConflictRows, s.DeletedRows, s.LastBatchMs))
	})

	r.mu.Lock()
	finished := time.Now()
	r.state.FinishedAt = &finished
	r.state.ElapsedMs = finished.Sub(started).Milliseconds()
	r.state.CurrentBatch = ""
	r.state.RuleCount = stats.RuleCount
	r.state.Batches = stats.Batches
	r.state.ScannedRows = stats.ScannedRows
	r.state.MovedRows = stats.MovedRows
	r.state.ConflictRows = stats.ConflictRows
	r.state.DeletedRows = stats.DeletedRows
	if err != nil {
		r.state.Status = uscPatchStatusFailed
		r.state.Error = err.Error()
	} else {
		r.state.Status = uscPatchStatusSucceeded
	}
	r.mu.Unlock()

	if stats.RuleCount == 0 {
		r.appendLog("warn", "没有生效的补丁规则，本轮未做任何改动")
	}
	if err != nil {
		r.appendLog("error", fmt.Sprintf("运行结束（失败）：%v", err))
		return
	}
	if stats.MaturityMoved > 0 || stats.MaturityDeleted > 0 {
		r.appendLog("info", fmt.Sprintf("成熟度记录迁移 %d 行、删除旧 usc 记录 %d 行",
			stats.MaturityMoved, stats.MaturityDeleted))
	}
	if stats.RegisteredRows > 0 {
		r.appendLog("info", fmt.Sprintf("补齐目标 usc 登记 %d 行", stats.RegisteredRows))
	}
	r.appendLog("info", fmt.Sprintf(
		"运行结束（成功）：%d 批，扫描 %d 行，迁移 %d 行，冲突 %d 行，删除 %d 行，耗时 %d ms",
		stats.Batches, stats.ScannedRows, stats.MovedRows, stats.ConflictRows,
		stats.DeletedRows, finished.Sub(started).Milliseconds()))
}

// runID 读取当前运行 ID。
func (r *UscPatchRunner) runID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.RunID
}

// appendLog 追加一条日志（超过上限丢弃最旧的）。
func (r *UscPatchRunner) appendLog(level, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	r.state.Logs = append(r.state.Logs, UscPatchLogEntry{Seq: r.seq, Time: time.Now(), Level: level, Message: message})
	if len(r.state.Logs) > uscPatchMaxLogEntries {
		r.state.Logs = r.state.Logs[len(r.state.Logs)-uscPatchMaxLogEntries:]
	}
	r.state.LastSeq = r.seq
}

// Status 返回状态快照；sinceSeq > 0 时只返回该序号之后的日志。
func (r *UscPatchRunner) Status(sinceSeq int64) UscPatchRunState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(sinceSeq)
}

// snapshotLocked 在持锁前提下构造快照。
func (r *UscPatchRunner) snapshotLocked(sinceSeq int64) UscPatchRunState {
	out := r.state
	logs := make([]UscPatchLogEntry, 0, len(r.state.Logs))
	for _, entry := range r.state.Logs {
		if entry.Seq > sinceSeq {
			logs = append(logs, entry)
		}
	}
	out.Logs = logs
	if out.Status == uscPatchStatusRunning && out.StartedAt != nil {
		out.ElapsedMs = time.Since(*out.StartedAt).Milliseconds()
	}
	return out
}

// formatTsRange 把秒级时间戳区间格式化成可读时间。
func formatTsRange(from, to int64) string {
	if from == 0 && to == 0 {
		return "-"
	}
	return time.Unix(from, 0).Format("2006-01-02 15:04") + " ~ " + time.Unix(to, 0).Format("2006-01-02 15:04")
}
