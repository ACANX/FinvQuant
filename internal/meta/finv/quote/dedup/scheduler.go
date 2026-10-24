package dedup

import (
	"context"
	"log"
	"time"
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
}

// NewScheduler 创建调度器；间隔类 Job 从启动时刻开始计时，避免启动瞬间集中触发。
func NewScheduler(service *Service) *Scheduler {
	runtimes := JobRuntimes()
	scheduler := &Scheduler{
		service:  service,
		runtimes: runtimes,
		lastRun:  map[string]time.Time{},
		lastDay:  map[string]string{},
		tick:     30 * time.Second,
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
	log.Printf("[exportarchive] 调度器启动，共 %d 个 Job（时区 %s，导出窗口 %02d:%02d-%02d:%02d）",
		len(s.runtimes), s.service.options.TimeZone,
		s.service.options.WindowStart/60, s.service.options.WindowStart%60,
		s.service.options.WindowEnd/60, s.service.options.WindowEnd%60)
	for _, runtime := range s.runtimes {
		log.Printf("[exportarchive] 调度 %s：%s", runtime.Kind, runtime.Schedule)
	}

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[exportarchive] 调度器退出")
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

	for _, runtime := range s.runtimes {
		if !s.due(runtime, local, day) {
			continue
		}
		// 导出 Job 只在低峰窗口内触发；窗口外保持「到期」状态，进入窗口后立即执行。
		if runtime.Windowed && !s.service.options.InWindow(local) {
			continue
		}
		s.lastRun[runtime.Kind] = now
		if runtime.DailyAt != "" {
			s.lastDay[runtime.Kind] = day
		}
		go func(kind string) {
			if _, err := s.service.Run(ctx, kind); err != nil {
				log.Printf("[exportarchive] Job %s 执行失败: %v", kind, err)
			}
		}(runtime.Kind)
	}
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
	at := parseClockMinutes(runtime.DailyAt, 0)
	minute := local.Hour()*60 + local.Minute()
	return minute >= at
}
