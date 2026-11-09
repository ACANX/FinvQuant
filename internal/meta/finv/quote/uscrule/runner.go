package uscrule

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/acanx/finvquant/internal/meta/base/cron"
)

// 本文件实现 USC 映射同步的异步运行与每日调度：
//
//   - 异步运行（需求 11072）：维护页点「应用」后立刻拿到 run_id，前端轮询 RunStatus 看进度与日志；
//   - 每日调度：独立 cron（读 sys.sys_config 20015，默认 0 30 3 * * *），不挂 cdeu workflow；
//     自动与手动共用同一个 Runner，页面看到的是同一份运行状态。
//
// 状态只存内存（进程重启即清空），同一时刻只允许一个运行。

const (
	RunStatusRunning = "RUNNING"
	RunStatusDone    = "DONE"
	RunStatusFailed  = "FAILED"

	RunLogLevelInfo  = "INFO"
	RunLogLevelError = "ERROR"
)

// maxRunLogs 单次运行保留的日志上限。
const maxRunLogs = 500

// runTimeout 单次应用的整体超时（7 万行 + 归档，给足 30 分钟）。
const runTimeout = 30 * time.Minute

// Trigger 触发来源。
type Trigger string

const (
	TriggerManual Trigger = "manual"
	TriggerCron   Trigger = "cron"
)

// RunLog 一条实时日志。
type RunLog struct {
	Seq   int    `json:"seq"`
	Time  string `json:"time"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// Run 一次运行的快照。
type Run struct {
	RunID       string   `json:"run_id"`
	Status      string   `json:"status"`
	Trigger     string   `json:"trigger"`
	Env         string   `json:"env"`
	Operator    string   `json:"operator"`
	RuleVersion string   `json:"rule_version"`
	Added       int64    `json:"added"`
	Changed     int64    `json:"changed"`
	Removed     int64    `json:"removed"`
	Reused      int64    `json:"reused"`
	Unchanged   int64    `json:"unchanged"`
	Escalated   int64    `json:"escalated"`
	Archived    int64    `json:"archived"`
	KeyUpdated  int64    `json:"key_updated"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at,omitempty"`
	ElapsedMs   int64    `json:"elapsed_ms"`
	Error       string   `json:"error,omitempty"`
	Logs        []RunLog `json:"logs"`
}

// RunRequest 一次运行的输入。
type RunRequest struct {
	Env          string
	PreviewToken string
	Operator     string
	Reason       string
	Trigger      Trigger
}

// Runner 异步运行持有者（并发安全）。
type Runner struct {
	service *Service
	now     func() time.Time

	mu      sync.Mutex
	current *runState
	lastID  string
}

type runState struct {
	run     Run
	started time.Time
	logs    []RunLog
	seq     int
}

// NewRunner 创建运行器。
func NewRunner(service *Service) *Runner {
	return &Runner{service: service, now: time.Now}
}

// Start 启动一次异步应用；已有运行在执行时返回错误（不排队、不覆盖）。
func (r *Runner) Start(req RunRequest) (Run, error) {
	r.mu.Lock()
	if r.current != nil && r.current.run.Status == RunStatusRunning {
		runID := r.current.run.RunID
		r.mu.Unlock()
		return Run{}, fmt.Errorf("已有 USC 映射同步任务在运行（run_id=%s），请等待完成", runID)
	}
	env := normalizeEnv(req.Env)
	runID := fmt.Sprintf("usc-%s", r.now().Format("20060102-150405.000000"))
	state := &runState{
		started: r.now(),
		run: Run{
			RunID:     runID,
			Status:    RunStatusRunning,
			Trigger:   string(req.Trigger),
			Env:       env,
			Operator:  nonEmpty(req.Operator, "finv-scheduler"),
			StartedAt: r.now().Format(time.RFC3339),
		},
	}
	r.current = state
	r.lastID = runID
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf("同步任务已启动（env=%s，触发=%s）", env, req.Trigger))
	snapshot := r.snapshotLocked(state)
	r.mu.Unlock()

	go r.execute(state, req, env)
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

// execute 在后台执行应用。
func (r *Runner) execute(state *runState, req RunRequest, env string) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	r.mu.Lock()
	r.appendLogLocked(state, RunLogLevelInfo, "开始计算候选与差异（归档复用 → 退市 → 写结果表 → 回写键表）")
	r.mu.Unlock()

	result, err := r.service.Apply(ctx, env, req.PreviewToken, req.Operator, req.Reason)

	r.mu.Lock()
	defer r.mu.Unlock()
	state.run.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	state.run.FinishedAt = r.now().Format(time.RFC3339)
	if err != nil {
		state.run.Status = RunStatusFailed
		state.run.Error = err.Error()
		r.appendLogLocked(state, RunLogLevelError, "同步失败："+err.Error())
		log.Printf("[usc] run=%s failed: %v", state.run.RunID, err)
		return
	}
	state.run.Status = RunStatusDone
	state.run.RuleVersion = result.RuleVersion
	state.run.Added = result.Added
	state.run.Changed = result.Changed
	state.run.Removed = result.Removed
	state.run.Reused = result.Reused
	state.run.Unchanged = result.Unchanged
	state.run.Escalated = result.Escalated
	state.run.Archived = result.Archived
	state.run.KeyUpdated = result.KeyUpdated
	r.appendLogLocked(state, RunLogLevelInfo, fmt.Sprintf(
		"同步完成：新增 %d，更新 %d，退市 %d，复用 %d，未变 %d，升级 %d，归档 %d 行，回写键表 %d 行，耗时 %d ms",
		result.Added, result.Changed, result.Removed, result.Reused, result.Unchanged,
		result.Escalated, result.Archived, result.KeyUpdated, result.ElapsedMs))
	log.Printf("[usc] run=%s done env=%s added=%d changed=%d removed=%d reused=%d escalated=%d archived=%d key_updated=%d elapsed_ms=%d",
		state.run.RunID, env, result.Added, result.Changed, result.Removed, result.Reused,
		result.Escalated, result.Archived, result.KeyUpdated, result.ElapsedMs)
}

// StartCron 启动每日调度（阻塞直到 ctx 取消）：每 30 秒检查系统配置 20015 的 cron 是否命中。
//
// 与手动触发共用同一个 Runner，因此自动运行同样出现在页面状态与日志里。
func (r *Runner) StartCron(ctx context.Context, env string) {
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
		expr, err := r.service.CronExpr(ctx)
		if err != nil {
			log.Printf("[usc] 读取同步 cron 失败: %v", err)
			return schedule
		}
		if expr == exprCache && schedule != nil {
			return schedule
		}
		parsed := ParseSchedule(expr)
		if parsed == nil {
			if expr != "" {
				log.Printf("[usc] 同步 cron 非法（%q），已关闭定时同步", expr)
			}
			exprCache = expr
			schedule = nil
			return nil
		}
		exprCache = expr
		schedule = parsed
		log.Printf("[usc] 每日同步 cron 已加载: %q（env=%s）", expr, env)
		return schedule
	}

	log.Printf("[usc] USC 映射同步服务已启动（cron 配置 %d）", ConfigCronNo)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[usc] USC 映射同步服务停止")
			return
		case now := <-ticker.C:
			current := getSchedule(now)
			if current == nil || !current.Matches(now) {
				continue
			}
			if _, err := r.Start(RunRequest{Env: env, Operator: "finv-scheduler", Reason: "每日定时同步", Trigger: TriggerCron}); err != nil {
				log.Printf("[usc] 定时同步被跳过: %v", err)
			}
		}
	}
}

// appendLogLocked 追加日志（调用方须持锁）。
func (r *Runner) appendLogLocked(state *runState, level, text string) {
	state.seq++
	state.logs = append(state.logs, RunLog{
		Seq:   state.seq,
		Time:  r.now().Format(time.RFC3339),
		Level: level,
		Text:  text,
	})
	if len(state.logs) > maxRunLogs {
		state.logs = state.logs[len(state.logs)-maxRunLogs:]
	}
}

// snapshotLocked 复制快照（调用方须持锁）。
func (r *Runner) snapshotLocked(state *runState) Run {
	snapshot := state.run
	snapshot.Logs = make([]RunLog, len(state.logs))
	copy(snapshot.Logs, state.logs)
	if snapshot.Status == RunStatusRunning {
		snapshot.ElapsedMs = r.now().Sub(state.started).Milliseconds()
	}
	return snapshot
}
