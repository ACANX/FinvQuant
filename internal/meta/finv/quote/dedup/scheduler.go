package dedup

import (
	"context"
	"log"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// Scheduler 进程内调度器：按 JobRuntimes 的每日时刻或固定间隔触发 Job。
//
// 设计取舍：不引入外部调度组件（sche 表为后续阶段），由服务进程内单个 goroutine
// 按固定 tick 计算到期，Job 自身通过数据库行锁 + 租约保证多实例下不会重复处理同一任务。
type Scheduler struct {
	service  *Service
	runtimes []JobRuntime
	lastRun  map[string]time.Time
	lastDay  map[string]string
	tick     time.Duration
	// gateLogged 记录「被拓扑门禁拦下」的最近日志时间，避免每 30 秒刷屏（需求 10829）。
	gateLogged map[string]time.Time
}

// NewScheduler 创建调度器；间隔类 Job 从启动时刻开始计时，避免启动瞬间集中触发。
func NewScheduler(service *Service) *Scheduler {
	runtimes := JobRuntimes()
	scheduler := &Scheduler{
		service:    service,
		runtimes:   runtimes,
		lastRun:    map[string]time.Time{},
		lastDay:    map[string]string{},
		tick:       30 * time.Second,
		gateLogged: map[string]time.Time{},
	}
	now := time.Now()
	for _, runtime := range runtimes {
		if runtime.Interval > 0 {
			scheduler.lastRun[runtime.Kind] = now
		}
	}
	return scheduler
}

// Run 启动调度循环，直到 ctx 取消。
func (s *Scheduler) Run(ctx context.Context) {
	// 调度计划以 CDEU 定义表为准；读取失败时保留 NewScheduler 的默认计划。
	s.runtimes = s.service.CDEUJobRuntimes(ctx)
	start, end, source, _ := s.service.Window(ctx)
	mode, modeErr := s.service.CDEUMode(ctx)
	switch {
	case modeErr != nil:
		log.Printf("[cdeu] 读取系统配置 20005（CDEU 运行模式）失败，按 Manual（不自动跑）处理: %v", modeErr)
	case mode == syscfg.CDEUModeManual:
		log.Printf("[cdeu] CDEU 运行模式 = Manual（默认）：调度器不自动触发任何 Job，只能通过 Job/Run 手动执行")
	default:
		log.Printf("[cdeu] CDEU 运行模式 = Auto：按定时任务时间自动运行")
	}
	log.Printf("[cdeu] 调度器启动，共 %d 个 Job（时区 %s，导出窗口 %02d:%02d:%02d-%02d:%02d:%02d，来源 %s）",
		len(s.runtimes), s.service.options.TimeZone,
		start/3600, start/60%60, start%60, end/3600, end/60%60, end%60, source)
	for _, runtime := range s.runtimes {
		log.Printf("[cdeu] 调度 %s：%s", runtime.Kind, runtime.Schedule)
	}

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[cdeu] 调度器退出")
			return
		case now := <-ticker.C:
			s.fire(ctx, now)
		}
	}
}

// fire 触发所有到期 Job（每个 Job 独立 goroutine，同类 Job 由 Service 互斥）。
func (s *Scheduler) fire(ctx context.Context, now time.Time) {
	local := now.In(s.service.options.Location)
	day := local.Format("2006-01-02")

	// 20005 CDEU 工作流运行模式：Manual（默认）时调度器不触发任何 Job；
	// 只有 Auto 才按定时任务时间自动跑。手动触发（Job/Run）不经过本函数，不受此开关限制。
	//
	// 设计缘由：Job 领取路径（Store.ClaimJob）只按 status + 到期时间领取，
	// CDEUTrigger* 仅写入运行日志，无法区分手动/调度；因此必须在调度入口处拦截，
	// 否则只要处于低峰窗口且 Job 到期，运维就无法阻止程序自动跑。
	if !s.service.CDEUScheduleEnabled(ctx) {
		return
	}

	for _, runtime := range s.runtimes {
		if !s.due(runtime, local, day) {
			continue
		}
		// 导出 Job 只在低峰窗口内触发；窗口外保持「到期」状态，进入窗口后立即执行。
		if runtime.Windowed && !s.service.InWindow(ctx, local) {
			continue
		}
		// 拓扑顺序门禁（需求 10829）：前序阶段尚未开始 / 已失败时本轮不触发。
		// 注意：**不能**在这里写 lastRun/lastDay —— 写了就会把 JOB 标记为「今天已跑」，
		// 前序阶段完成后当天再也不会重试。保持到期状态，由后续 tick 继续判定。
		if blocked, reason, gateErr := s.service.CDEUJobGate(ctx, runtime.Kind); gateErr != nil {
			log.Printf("[cdeu] 读取阶段门禁失败 job=%s err=%v（本轮按放行处理）", runtime.Kind, gateErr)
		} else if blocked {
			s.noteGateBlocked(runtime.Kind, reason, local)
			continue
		}
		s.lastRun[runtime.Kind] = now
		if runtime.DailyAt != "" {
			s.lastDay[runtime.Kind] = day
		}
		go func(kind string) {
			if _, err := s.service.RunWithTrigger(ctx, kind, CDEUTriggerSchedule); err != nil {
				log.Printf("[cdeu] Job %s 执行失败: %v", kind, err)
			}
		}(runtime.Kind)
	}
}

// gateLogInterval 同一 Job 被门禁拦下的日志最小间隔（避免 30 秒 tick 反复打印）。
const gateLogInterval = 5 * time.Minute

// noteGateBlocked 记录「被前序阶段拦下」的日志（同一 Job 5 分钟内只打一次）。
func (s *Scheduler) noteGateBlocked(kind, reason string, now time.Time) {
	if last, ok := s.gateLogged[kind]; ok && now.Sub(last) < gateLogInterval {
		return
	}
	s.gateLogged[kind] = now
	log.Printf("[cdeu] 调度 %s 暂不执行：%s（保持到期状态，前序完成后自动继续）", kind, reason)
}

// due 判断 Job 是否到期。
func (s *Scheduler) due(runtime JobRuntime, local time.Time, day string) bool {
	if runtime.Interval > 0 {
		last, ok := s.lastRun[runtime.Kind]
		if !ok {
			return true
		}
		return local.Sub(last) >= time.Duration(runtime.Interval)*time.Second
	}
	if s.lastDay[runtime.Kind] == day {
		return false
	}
	// DailyAt 为 HH:MM 或 HH:MM:SS，统一换算成自 0 点起的秒数。
	at, err := ParseClock(runtime.DailyAt)
	if err != nil {
		at = 0
	}
	second := local.Hour()*3600 + local.Minute()*60 + local.Second()
	return second >= at
}
