// 大表分片备份运行器：手动触发一轮分片备份，并把状态与日志保存在内存中供页面实时展示。
//
// 与小表全量备份的 Runner 完全独立（各自的运行状态、互不阻塞）：
//
//	· 自动调度暂未开放（需求 10905 先只做手动，验收后再决定是否接入定时）；
//	· 状态只存内存（服务重启清空），备份产物本身仍落磁盘/对象存储；
//	· 日志按 seq 递增，前端可用 sinceSeq 增量拉取。
package brs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SliceRunState 分片备份运行状态快照。
type SliceRunState struct {
	RunID        string     `json:"run_id"`           // 本轮运行 ID（yyyyMMdd-HHmmss-manual）
	Trigger      Trigger    `json:"trigger"`          // 触发来源（当前只有 manual）
	Status       string     `json:"status"`           // 运行状态：idle / running / succeeded / failed
	StartedAt    *time.Time `json:"started_at"`       // 开始时间
	FinishedAt   *time.Time `json:"finished_at"`      // 结束时间
	ElapsedMs    int64      `json:"elapsed_ms"`       // 已耗时（毫秒）
	CurrentTable string     `json:"current_table"`    // 当前处理的表（db.schema.table）
	CurrentSlice string     `json:"current_slice"`    // 当前处理的分片键
	Total        int        `json:"total"`            // 本轮需处理的分片数（累计）
	Done         int        `json:"done"`             // 已完成分片数
	Failed       int        `json:"failed"`           // 失败分片数
	BackupDir    string     `json:"backup_dir"`       // 本地备份根目录
	BackupDirOK  bool       `json:"backup_dir_ok"`    // 备份目录是否已创建且可写
	BackupDirErr string     `json:"backup_dir_error"` // 目录不可用的原因（可用时为空）
	// 磁盘水位（备份目录所在文件系统）
	DiskPath         string     `json:"disk_path"`
	DiskFreeBytes    uint64     `json:"disk_free_bytes"`
	DiskTotalBytes   uint64     `json:"disk_total_bytes"`
	DiskMinFreeBytes uint64     `json:"disk_min_free_bytes"`
	DiskLow          bool       `json:"disk_low"`
	DiskError        string     `json:"disk_error"`
	Error            string     `json:"error"`    // 失败原因（成功时为空）
	Logs             []LogEntry `json:"logs"`     // 运行日志（增量拉取）
	LastSeq          int64      `json:"last_seq"` // 最大日志序号
}

// SliceRunner 大表分片备份运行器（仅手动触发）。
type SliceRunner struct {
	pool *pgxpool.Pool
	opts Options

	mu    sync.Mutex
	state SliceRunState
	seq   int64
}

// NewSliceRunner 创建分片备份运行器。
func NewSliceRunner(pool *pgxpool.Pool, opts Options) *SliceRunner {
	if opts.BackupDir == "" {
		opts.BackupDir = defaultBackupDir
	}
	// 启动即预检一次：目录不可写在页面/日志里立刻可见，不必等到首次手动触发。
	dirOK, dirErr := true, ""
	if _, err := EnsureSliceRoot(opts.BackupDir); err != nil {
		dirOK, dirErr = false, err.Error()
		log.Printf("[brs] 大表分片备份目录不可用: %v", err)
	}
	disk := checkDisk(opts.BackupDir, opts.DiskMinFreeGB)
	return &SliceRunner{
		pool: pool,
		opts: opts,
		state: SliceRunState{
			DiskPath:         disk.Path,
			DiskFreeBytes:    disk.AvailableBytes,
			DiskTotalBytes:   disk.TotalBytes,
			DiskMinFreeBytes: disk.MinFreeBytes,
			DiskLow:          disk.Low,
			DiskError:        disk.Error,
			Status:           StatusIdle,
			BackupDir:        opts.BackupDir,
			BackupDirOK:      dirOK,
			BackupDirErr:     dirErr,
			Logs:             []LogEntry{},
		},
	}
}

// TriggerManual 手动触发一轮分片备份（异步执行），返回本次运行的状态快照。
//
// 分片备份是长任务：生命周期跟随服务（context.Background），不用 HTTP 请求的 ctx。
func (r *SliceRunner) TriggerManual() (SliceRunState, error) {
	r.mu.Lock()
	if r.state.Status == StatusRunning {
		busy := &BusyError{RunningTrigger: r.state.Trigger, RunningSince: r.state.StartedAt}
		r.mu.Unlock()
		return SliceRunState{}, busy
	}
	now := time.Now()
	disk := checkDisk(r.opts.BackupDir, r.opts.DiskMinFreeGB)
	r.state = SliceRunState{
		RunID:            fmt.Sprintf("%s-%s", now.Format("20060102-150405"), TriggerManual),
		Trigger:          TriggerManual,
		Status:           StatusRunning,
		StartedAt:        &now,
		BackupDir:        r.opts.BackupDir,
		DiskPath:         disk.Path,
		DiskFreeBytes:    disk.AvailableBytes,
		DiskTotalBytes:   disk.TotalBytes,
		DiskMinFreeBytes: disk.MinFreeBytes,
		DiskLow:          disk.Low,
		DiskError:        disk.Error,
		Logs:             make([]LogEntry, 0, 64),
	}
	snapshot := r.snapshotLocked(0)
	r.mu.Unlock()

	go r.run()
	return snapshot, nil
}

// run 执行一轮分片备份并维护运行状态。
func (r *SliceRunner) run() {
	hooks := Hooks{
		OnLog: func(level, message string) { r.appendLog(level, message) },
		OnSliceProgress: func(rule SliceRule, slice Slice, done, total int) {
			r.mu.Lock()
			r.state.CurrentTable = rule.Table()
			r.state.CurrentSlice = slice.Key
			r.state.Done = done
			r.state.Total = total
			r.mu.Unlock()
		},
	}

	if _, err := EnsureSliceRoot(r.opts.BackupDir); err != nil {
		r.mu.Lock()
		r.state.BackupDirOK = false
		r.state.BackupDirErr = err.Error()
		r.mu.Unlock()
		r.appendLog("error", err.Error())
		r.mu.Lock()
		finished := time.Now()
		r.state.FinishedAt = &finished
		r.state.Status = StatusFailed
		r.state.Error = err.Error()
		r.state.LastSeq = r.seq
		r.mu.Unlock()
		return
	}
	r.mu.Lock()
	r.state.BackupDirOK = true
	r.state.BackupDirErr = ""
	r.mu.Unlock()

	// 磁盘水位：低于阈值时先告警。
	if disk := checkDisk(r.opts.BackupDir, r.opts.DiskMinFreeGB); disk.Low {
		r.appendLog("warn", lowDiskMessage(disk))
	}
	r.appendLog("info", fmt.Sprintf("开始执行大表分片备份（run_id=%s）", r.runID()))
	err := RunSliceOnceWithHooks(context.Background(), r.pool, r.opts, hooks)

	r.mu.Lock()
	finished := time.Now()
	r.state.FinishedAt = &finished
	if r.state.StartedAt != nil {
		r.state.ElapsedMs = finished.Sub(*r.state.StartedAt).Milliseconds()
	}
	r.state.CurrentTable = ""
	r.state.CurrentSlice = ""
	if err != nil {
		r.state.Status = StatusFailed
		r.state.Error = err.Error()
	} else {
		r.state.Status = StatusSucceeded
		// 成功时按已处理分片数收尾，避免进度条停在 done < total。
		r.state.Done = r.state.Total
	}
	disk := checkDisk(r.opts.BackupDir, r.opts.DiskMinFreeGB)
	r.state.DiskPath = disk.Path
	r.state.DiskFreeBytes = disk.AvailableBytes
	r.state.DiskTotalBytes = disk.TotalBytes
	r.state.DiskMinFreeBytes = disk.MinFreeBytes
	r.state.DiskLow = disk.Low
	r.state.DiskError = disk.Error
	r.state.LastSeq = r.seq
	r.mu.Unlock()

	if disk.Low {
		r.appendLog("warn", lowDiskMessage(disk))
	}
	if err != nil {
		r.appendLog("error", fmt.Sprintf("运行结束（失败）：%v", err))
	} else {
		r.appendLog("info", "运行结束（成功）")
	}
}

// Pool 返回运行器持有的数据库连接池（供分片计划查询复用）。
func (r *SliceRunner) Pool() *pgxpool.Pool { return r.pool }

// Options 返回运行器持有的备份参数（供分片计划查询复用）。
func (r *SliceRunner) Options() Options { return r.opts }

// runID 读取当前运行 ID。
func (r *SliceRunner) runID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.RunID
}

// appendLog 追加一条运行日志（超过上限丢弃最旧的）。
func (r *SliceRunner) appendLog(level, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	r.state.Logs = append(r.state.Logs, LogEntry{Seq: r.seq, Time: time.Now(), Level: level, Message: message})
	if len(r.state.Logs) > maxLogEntries {
		r.state.Logs = r.state.Logs[len(r.state.Logs)-maxLogEntries:]
	}
	r.state.LastSeq = r.seq
}

// Status 返回运行状态快照；sinceSeq > 0 时只返回该序号之后的日志。
func (r *SliceRunner) Status(sinceSeq int64) SliceRunState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(sinceSeq)
}

// snapshotLocked 在持有锁的前提下构造快照。
func (r *SliceRunner) snapshotLocked(sinceSeq int64) SliceRunState {
	out := r.state
	out.Logs = make([]LogEntry, 0, len(r.state.Logs))
	for _, e := range r.state.Logs {
		if sinceSeq > 0 && e.Seq <= sinceSeq {
			continue
		}
		out.Logs = append(out.Logs, e)
	}
	if out.StartedAt != nil && out.Status == StatusRunning {
		out.ElapsedMs = time.Since(*out.StartedAt).Milliseconds()
	}
	return out
}
