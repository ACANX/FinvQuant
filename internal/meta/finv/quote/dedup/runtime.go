package dedup

import (
	"context"
	"log"
	"time"
)

// 本文件实现「Job 运行快照」与「执行阶段进度上报」：
//   - 快照：合并进程内正在执行的 Job 与运行日志表（finv_dedup_job_run_log_export_file），
//     给出每个 Job 是否在跑、当前阶段 / 进度 / 心跳、最近一次结果与 24 小时统计；
//   - 上报：Job 内部通过 reportProgress 更新阶段、进度与心跳，供前端区分「进行中 / 卡住 / 已完成」。

// runIDContextKey 在 context 中携带当前运行日志主键（供 Job 上报进度）。
type runIDContextKey struct{}

// withRunID 把 run_id 写入 context。
func withRunID(ctx context.Context, runID int64) context.Context {
	return context.WithValue(ctx, runIDContextKey{}, runID)
}

// runIDFromContext 读取 context 中的 run_id（不存在返回 0）。
func runIDFromContext(ctx context.Context) int64 {
	runID, _ := ctx.Value(runIDContextKey{}).(int64)
	return runID
}

// cdeuRunIDFromContext 读取 context 中的 CDEU 工作流运行 ID（不存在返回 0）。
func cdeuRunIDFromContext(ctx context.Context) int64 {
	execution := cdeuExecutionFromContext(ctx)
	if execution == nil {
		return 0
	}
	return execution.RunID
}

// reportProgress 更新当前运行的阶段 / 进度 / 心跳；写库失败只记日志，不影响主流程。
//
// percent 取 0~100；负值表示无法估算总量的不确定进度（前端显示为不确定进度条）。
func (s *Service) reportProgress(ctx context.Context, stage string, percent float64, detail string) {
	s.reportRunProgress(ctx, runIDFromContext(ctx), stage, percent, detail)
}

// reportRunProgress 更新指定运行记录的阶段 / 进度 / 心跳。
func (s *Service) reportRunProgress(ctx context.Context, runID int64, stage string, percent float64, detail string) {
	if runID <= 0 {
		return
	}
	if err := s.store.UpdateRunProgress(ctx, runID, stage, percent, detail); err != nil {
		log.Printf("[cdeu] 更新运行进度失败 run_id=%d stage=%s err=%v", runID, stage, err)
	}
}

// staleAfter 返回运行记录心跳超时阈值：取租约的 2 倍，且不低于 2 分钟。
func (s *Service) staleAfter() time.Duration {
	threshold := 2 * s.options.Lease
	if threshold < 2*time.Minute {
		threshold = 2 * time.Minute
	}
	return threshold
}

// StaleAfter 返回心跳超时阈值（供运维接口与前端展示「是否卡住」）。
func (s *Service) StaleAfter() time.Duration { return s.staleAfter() }

// JobRuntime 返回全部 Job 的运行快照（顺序与 JobRuntimes 一致）。
func (s *Service) JobRuntime(ctx context.Context) ([]JobRuntimeState, JobProgress, error) {
	runtimes := JobRuntimes()
	kinds := make([]string, 0, len(runtimes))
	for _, runtime := range runtimes {
		kinds = append(kinds, runtime.Kind)
	}

	now := time.Now()
	aggregates, err := s.store.JobRunAggregates(ctx, kinds, now.Add(-24*time.Hour))
	if err != nil {
		return nil, JobProgress{}, err
	}
	recent, err := s.store.ListRecentRunsByKind(ctx, kinds, 20)
	if err != nil {
		return nil, JobProgress{}, err
	}
	batchCounts, err := s.store.JobStatusCountsByMonth(ctx, s.options.CurrentMonth())
	if err != nil {
		return nil, JobProgress{}, err
	}
	patchCounts, err := s.store.PatchStatusCountsByMonth(ctx, s.options.CurrentMonth())
	if err != nil {
		return nil, JobProgress{}, err
	}

	// 复制进程内运行状态，避免持锁期间做数据库查询。
	inProcess := map[string]time.Time{}
	s.mu.Lock()
	for kind, started := range s.running {
		inProcess[kind] = started
	}
	s.mu.Unlock()

	staleAfter := s.staleAfter()
	states := buildJobRuntimeStates(now, runtimes, aggregates, recent, inProcess, staleAfter)
	progress := mergePatchProgress(SummarizeJobProgress(batchCounts), patchCounts)
	return states, progress, nil
}

// buildJobRuntimeStates 将数据库与进程内状态合并为每个 Job 的运行快照（纯函数，便于单测）。
func buildJobRuntimeStates(now time.Time, runtimes []JobRuntime, aggregates map[string]JobRunAggregate,
	recent map[string][]RunLogRow, inProcess map[string]time.Time, staleAfter time.Duration) []JobRuntimeState {
	states := make([]JobRuntimeState, 0, len(runtimes))
	for _, runtime := range runtimes {
		state := JobRuntimeState{
			JobKind:  runtime.Kind,
			Name:     runtime.Name,
			Summary:  runtime.Summary,
			Flow:     runtime.Flow,
			Details:  runtime.Details,
			Cautions: runtime.Cautions,
			Schedule: runtime.Schedule,
			DailyAt:  runtime.DailyAt,
			Interval: runtime.Interval,
			Windowed: runtime.Windowed,
		}
		if item, ok := aggregates[runtime.Kind]; ok {
			state.Success24h = item.Success
			state.Failed24h = item.Failed
			state.Paused24h = item.Paused
			state.Total24h = item.Total
			state.LastSuccessAt = item.LastSuccessAt
			state.LastFailureAt = item.LastFailureAt
		}

		// 最近记录按 gmt_start 倒序：第一条未结束记录为运行中，第一条已结束记录为最近一次。
		var runningRow *RunLogRow
		for _, row := range recent[runtime.Kind] {
			rowCopy := row
			if rowCopy.GmtFinish == nil {
				if runningRow == nil {
					runningRow = &rowCopy
				}
				continue
			}
			if state.Last == nil {
				state.Last = &rowCopy
			}
		}

		state.Running = runningRow
		if started, ok := inProcess[runtime.Kind]; ok {
			state.InProcess = true
			state.ElapsedSec = int64(now.Sub(started).Seconds())
		} else if runningRow != nil {
			// 非本进程执行：可能来自其他实例（多 worker）或进程重启后的残留记录，由心跳区分。
			state.ElapsedSec = int64(now.Sub(runningRow.GmtStart).Seconds())
		}

		if state.Running != nil {
			heartbeat := state.Running.GmtStart
			if state.Running.GmtHeartbeat != nil {
				heartbeat = *state.Running.GmtHeartbeat
			}
			age := int64(now.Sub(heartbeat).Seconds())
			if age < 0 {
				age = 0
			}
			state.HeartbeatAgeSec = &age
			state.Stale = time.Duration(age)*time.Second > staleAfter
		}

		states = append(states, state)
	}
	return states
}
