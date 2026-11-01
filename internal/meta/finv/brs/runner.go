// BRS 备份运行器：把「定时调度」与「手动触发」收敛为同一条执行链，
// 并把运行进度与日志保存在内存中，供前端实时展示。
//
// 设计要点：
//   - 自动（cron 命中 39001）与手动（页面按钮）走同一个 run()，行为一致；
//   - 同一时刻只允许一个运行，重复触发返回 BusyError；
//   - 运行状态只存内存（服务重启即清空），备份产物本身仍落磁盘/对象存储；
//   - 日志按 seq 递增，前端可用 sinceSeq 增量拉取。
package brs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/base/cron"
)

// Trigger 备份触发来源。
type Trigger string

// 触发来源取值。
const (
	TriggerAuto   Trigger = "auto"   // 39001 cron 定时触发
	TriggerManual Trigger = "manual" // 前端手动触发
)

// 运行状态取值。
const (
	StatusIdle      = "idle"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// maxLogEntries 单个运行最多保留的日志条数（超出丢弃最旧的，seq 不回退）。
const maxLogEntries = 1000

// maxSchedulePreview 调度配置返回的未来执行时间条数。
const maxSchedulePreview = 5

// LogEntry 一条运行日志。
type LogEntry struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"` // info / warn / error
	Message string    `json:"message"`
}

// RunState 备份运行状态快照。
type RunState struct {
	RunID        string     `json:"run_id"`
	Trigger      Trigger    `json:"trigger"`
	Status       string     `json:"status"` // idle / running / succeeded / failed
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ElapsedMs    int64      `json:"elapsed_ms"`
	Total        int        `json:"total"`
	Done         int        `json:"done"`
	Failed       int        `json:"failed"`
	CurrentTable string     `json:"current_table"`
	BackupDir    string     `json:"backup_dir"`
	// 磁盘水位（备份目录所在文件系统）
	DiskPath         string     `json:"disk_path"`
	DiskFreeBytes    uint64     `json:"disk_free_bytes"`
	DiskTotalBytes   uint64     `json:"disk_total_bytes"`
	DiskMinFreeBytes uint64     `json:"disk_min_free_bytes"`
	DiskLow          bool       `json:"disk_low"`
	DiskError        string     `json:"disk_error"`
	Error            string     `json:"error"`
	Logs             []LogEntry `json:"logs"`
	LastSeq          int64      `json:"last_seq"`
}

// BusyError 表示已有备份任务在运行。
type BusyError struct {
	RunningTrigger Trigger
	RunningSince   *time.Time
}

// Error 实现 error 接口。
func (e *BusyError) Error() string {
	if e.RunningSince != nil {
		return fmt.Sprintf("已有备份任务在运行（%s，开始于 %s），请等待其结束",
			e.RunningTrigger, e.RunningSince.Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("已有备份任务在运行（%s），请等待其结束", e.RunningTrigger)
}

// Runner 备份运行器（自动调度 + 手动触发 + 运行状态）。
type Runner struct {
	pool *pgxpool.Pool
	opts Options

	mu    sync.Mutex
	state RunState
	seq   int64
	// baseCtx 是运行的生命周期上下文（由 Start 注入服务级 ctx）。
	// 手动触发**不能**用 HTTP 请求的 ctx：请求结束即被取消，备份会被中断。
	baseCtx context.Context
}

// NewRunner 创建备份运行器。
func NewRunner(pool *pgxpool.Pool, opts Options) *Runner {
	if strings.TrimSpace(opts.BackupDir) == "" {
		opts.BackupDir = defaultBackupDir
	}
	// 启动即取一次磁盘水位：页面在尚未执行过备份时也能看到剩余空间。
	disk := checkDisk(opts.BackupDir, opts.DiskMinFreeGB)
	if disk.Low {
		log.Printf("[brs] %s", lowDiskMessage(disk))
	}
	return &Runner{
		pool:    pool,
		opts:    opts,
		baseCtx: context.Background(),
		state: RunState{
			Status:           StatusIdle,
			BackupDir:        opts.BackupDir,
			DiskPath:         disk.Path,
			DiskFreeBytes:    disk.AvailableBytes,
			DiskTotalBytes:   disk.TotalBytes,
			DiskMinFreeBytes: disk.MinFreeBytes,
			DiskLow:          disk.Low,
			DiskError:        disk.Error,
			Logs:             []LogEntry{},
		},
	}
}

// Start 启动自动调度（阻塞直到 ctx 取消）：每秒检查 39001 cron 是否命中。
//
// 与手动触发共用 run()，因此自动任务同样会出现在页面的运行状态与日志里。
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	r.baseCtx = ctx
	r.mu.Unlock()

	log.Printf("[brs] 备份服务已启动（目录 %s，cron 配置 %d）", r.opts.BackupDir, CronConfigNo)

	var (
		mu        sync.Mutex
		lastRead  time.Time
		schedule  *cron.Schedule
		exprCache string
	)
	getSchedule := func(now time.Time) *cron.Schedule {
		mu.Lock()
		defer mu.Unlock()
		if now.Sub(lastRead) < time.Minute && schedule != nil {
			return schedule
		}
		lastRead = now
		expr, err := loadCronExpr(ctx, r.pool)
		if err != nil {
			log.Printf("[brs] 读取 %d cron 失败: %v", CronConfigNo, err)
			return schedule
		}
		if expr == exprCache && schedule != nil {
			return schedule
		}
		parsed, err := cron.Parse(expr)
		if err != nil {
			log.Printf("[brs] %d cron 非法（%q）: %v", CronConfigNo, expr, err)
			return schedule
		}
		exprCache = expr
		schedule = parsed
		log.Printf("[brs] 每日备份 cron 已加载: %q", expr)
		return schedule
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[brs] 备份服务停止")
			return
		case now := <-ticker.C:
			current := getSchedule(now)
			if current == nil || !current.Matches(now) {
				continue
			}
			if _, err := r.trigger(TriggerAuto); err != nil {
				log.Printf("[brs] 定时触发被跳过: %v", err)
			}
		}
	}
}

// TriggerManual 手动触发一轮备份（异步执行），返回本次运行的状态快照。
//
// 不接收调用方 ctx：备份生命周期跟随服务，避免 HTTP 请求结束后被取消。
func (r *Runner) TriggerManual() (RunState, error) {
	return r.trigger(TriggerManual)
}

// trigger 领取运行权并异步执行；已有运行中时返回 *BusyError。
func (r *Runner) trigger(t Trigger) (RunState, error) {
	r.mu.Lock()
	if r.state.Status == StatusRunning {
		busy := &BusyError{RunningTrigger: r.state.Trigger, RunningSince: r.state.StartedAt}
		r.mu.Unlock()
		return RunState{}, busy
	}
	now := time.Now()
	disk := checkDisk(r.opts.BackupDir, r.opts.DiskMinFreeGB)
	r.state = RunState{
		RunID:            fmt.Sprintf("%s-%s", now.Format("20060102-150405"), t),
		Trigger:          t,
		Status:           StatusRunning,
		StartedAt:        &now,
		BackupDir:        optsBackupDir(r.opts),
		DiskPath:         disk.Path,
		DiskFreeBytes:    disk.AvailableBytes,
		DiskTotalBytes:   disk.TotalBytes,
		DiskMinFreeBytes: disk.MinFreeBytes,
		DiskLow:          disk.Low,
		DiskError:        disk.Error,
		Logs:             make([]LogEntry, 0, 64),
	}
	snapshot := r.snapshotLocked(0)
	runCtx := r.baseCtx
	r.mu.Unlock()

	go r.run(runCtx, t)
	return snapshot, nil
}

// run 执行一轮备份并维护运行状态。
func (r *Runner) run(ctx context.Context, t Trigger) {
	hooks := Hooks{
		OnLog: func(level, message string) { r.appendLog(level, message) },
		OnProgress: func(index, done, total int, table Table) {
			r.mu.Lock()
			r.state.Total = total
			r.state.Done = done
			r.state.CurrentTable = table.SchemaName + "." + table.TableName
			r.mu.Unlock()
		},
	}

	// 磁盘水位：低于阈值时先告警（备份可能因空间不足而失败）。
	if disk := checkDisk(r.opts.BackupDir, r.opts.DiskMinFreeGB); disk.Low {
		r.appendLog("warn", lowDiskMessage(disk))
	}
	r.appendLog("info", fmt.Sprintf("开始执行 %s 备份（run_id=%s）", triggerLabel(t), r.runID()))
	err := RunOnceWithHooks(ctx, r.pool, r.opts, hooks)

	r.mu.Lock()
	finished := time.Now()
	r.state.FinishedAt = &finished
	if r.state.StartedAt != nil {
		r.state.ElapsedMs = finished.Sub(*r.state.StartedAt).Milliseconds()
	}
	r.state.CurrentTable = ""
	if err != nil {
		r.state.Status = StatusFailed
		r.state.Error = err.Error()
		r.state.Failed++
	} else {
		r.state.Status = StatusSucceeded
		r.state.Done = r.state.Total
	}
	// 结束再刷新一次磁盘水位（本轮写入会消耗空间）
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

// runID 读取当前运行 ID。
func (r *Runner) runID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.RunID
}

// appendLog 追加一条运行日志（超过上限丢弃最旧的）。
func (r *Runner) appendLog(level, message string) {
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
func (r *Runner) Status(sinceSeq int64) RunState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(sinceSeq)
}

// snapshotLocked 在持有锁的前提下构造快照。
func (r *Runner) snapshotLocked(sinceSeq int64) RunState {
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

// ScheduleConfig 39001 的调度配置视图。
type ScheduleConfig struct {
	CronExpr  string     `json:"cron_expr"`
	Remark    string     `json:"remark"`
	GmtModify *time.Time `json:"gmt_modify"`
	NextRuns  []string   `json:"next_runs"` // 未来 5 次执行时间（服务器本地时区）
}

// LoadSchedule 读取 39001 的 cron 配置。
func (r *Runner) LoadSchedule(ctx context.Context) (ScheduleConfig, error) {
	var expr, remark string
	var gmtModify *time.Time
	err := r.pool.QueryRow(ctx, `
SELECT COALESCE(str_value, ''), COALESCE(remark, ''), gmt_modify
  FROM sys.sys_config
 WHERE config_no = $1`, CronConfigNo).Scan(&expr, &remark, &gmtModify)
	if err != nil {
		return ScheduleConfig{}, fmt.Errorf("读取备份调度配置失败: %w", err)
	}
	cfg := ScheduleConfig{CronExpr: strings.TrimSpace(expr), Remark: remark, GmtModify: gmtModify}
	cfg.NextRuns = nextRuns(cfg.CronExpr, time.Now(), maxSchedulePreview)
	return cfg, nil
}

// SaveSchedule 保存 39001 的 cron 配置（校验 6 段表达式后落库）。
func (r *Runner) SaveSchedule(ctx context.Context, expr, operator string) (ScheduleConfig, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ScheduleConfig{}, inputErrorf("cron 表达式不能为空")
	}
	sched, err := cron.Parse(expr)
	if err != nil {
		return ScheduleConfig{}, inputErrorf("cron 表达式非法: %v", err)
	}
	tag, err := r.pool.Exec(ctx, `
UPDATE sys.sys_config
   SET str_value = $1,
       gmt_modify = CURRENT_TIMESTAMP(6),
       modify_by = $2
 WHERE config_no = $3`, expr, operator, CronConfigNo)
	if err != nil {
		return ScheduleConfig{}, fmt.Errorf("保存备份调度配置失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ScheduleConfig{}, inputErrorf("系统配置 %d 不存在，请先执行迁移 V300060", CronConfigNo)
	}
	cfg, err := r.LoadSchedule(ctx)
	if err != nil {
		return ScheduleConfig{}, err
	}
	// 用刚解析的表达式保证预览一致（LoadSchedule 内部也会重算）
	cfg.NextRuns = nextRunsFrom(sched, time.Now(), maxSchedulePreview)
	return cfg, nil
}

// nextRuns 解析表达式并给出未来若干次执行时间；表达式非法时返回空切片。
func nextRuns(expr string, from time.Time, n int) []string {
	sched, err := cron.Parse(expr)
	if err != nil {
		return []string{}
	}
	return nextRunsFrom(sched, from, n)
}

// nextRunsFrom 迭代计算未来 n 次执行时间。
func nextRunsFrom(sched *cron.Schedule, from time.Time, n int) []string {
	out := make([]string, 0, n)
	cursor := from
	for i := 0; i < n; i++ {
		next := sched.Next(cursor)
		if next.IsZero() {
			break
		}
		out = append(out, next.Format("2006-01-02 15:04:05"))
		cursor = next
	}
	return out
}

// optsBackupDir 返回生效的备份目录。
func optsBackupDir(opts Options) string {
	if strings.TrimSpace(opts.BackupDir) == "" {
		return defaultBackupDir
	}
	return opts.BackupDir
}

// triggerLabel 触发来源的中文名。
func triggerLabel(t Trigger) string {
	if t == TriggerManual {
		return "手动"
	}
	return "定时"
}
