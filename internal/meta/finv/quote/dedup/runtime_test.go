package dedup

import (
	"testing"
	"time"
)

// TestBuildJobRuntimeStates 校验运行快照的合并逻辑：
// 进程内运行状态优先、运行日志提供阶段 / 进度、心跳超时判定为「疑似卡住」。
func TestBuildJobRuntimeStates(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	runtimes := []JobRuntime{
		{Kind: JobRunDueExportJobs, Schedule: "低峰窗口内每 60 秒一轮", Interval: 60, Windowed: true},
		{Kind: JobCheckExportJobHealth, Schedule: "每 15 分钟", Interval: 900},
		{Kind: JobRefreshQuoteMaturity, Schedule: "每日 00:05", DailyAt: "00:05"},
	}

	successAt := now.Add(-2 * time.Hour)
	failureAt := now.Add(-30 * time.Minute)
	aggregates := map[string]JobRunAggregate{
		JobRunDueExportJobs: {
			JobKind: JobRunDueExportJobs, Success: 3, Failed: 1, Paused: 2, Total: 6,
			LastSuccessAt: &successAt, LastFailureAt: &failureAt,
		},
	}

	stage := "LOAD"
	percent := -1.0
	detail := "job_id=9 usc=US.AAPL biz_month=202604"
	started := now.Add(-2 * time.Minute)
	heartbeat := now.Add(-5 * time.Second)
	finished := now.Add(-10 * time.Minute)
	recent := map[string][]RunLogRow{
		// RUN_DUE_EXPORT_JOBS：本进程正在执行，含阶段 / 进度 / 心跳
		JobRunDueExportJobs: {
			{
				RunID: 30, JobKind: JobRunDueExportJobs, GmtStart: started,
				Stage: &stage, ProgressPercent: &percent, ProgressDetail: &detail, GmtHeartbeat: &heartbeat,
			},
			{
				RunID: 29, JobKind: JobRunDueExportJobs, GmtStart: finished, GmtFinish: &finished,
				Status: stringPtr(RunStatusSuccess), Stage: stringPtr("DONE"),
			},
		},
		// CHECK_EXPORT_JOB_HEALTH：无进程内状态且心跳超时 → 疑似卡住
		JobCheckExportJobHealth: {
			{
				RunID: 40, JobKind: JobCheckExportJobHealth, GmtStart: now.Add(-10 * time.Minute),
				GmtHeartbeat: timePtr(now.Add(-5 * time.Minute)),
			},
		},
		// REFRESH_QUOTE_MATURITY：无运行中记录，只有最近一次成功
		JobRefreshQuoteMaturity: {
			{
				RunID: 50, JobKind: JobRefreshQuoteMaturity, GmtStart: successAt,
				GmtFinish: &successAt, Status: stringPtr(RunStatusSuccess),
			},
		},
	}
	inProcess := map[string]time.Time{JobRunDueExportJobs: started}

	states := buildJobRuntimeStates(now, runtimes, aggregates, recent, inProcess, 2*time.Minute)
	if len(states) != 3 {
		t.Fatalf("快照数量应为 3，实际 %d", len(states))
	}

	running := states[0]
	if !running.InProcess || running.Stale {
		t.Fatalf("运行中 Job 应 in_process=true 且 stale=false，实际 %+v", running)
	}
	if running.ElapsedSec != 120 {
		t.Fatalf("已运行秒数应为 120，实际 %d", running.ElapsedSec)
	}
	if running.Running == nil || running.Running.RunID != 30 {
		t.Fatalf("应取未结束记录 run_id=30，实际 %+v", running.Running)
	}
	if running.Running.Stage == nil || *running.Running.Stage != "LOAD" {
		t.Fatalf("运行中阶段应为 LOAD，实际 %+v", running.Running.Stage)
	}
	if running.Running.ProgressDetail == nil || *running.Running.ProgressDetail != detail {
		t.Fatalf("进度明细不符，实际 %+v", running.Running.ProgressDetail)
	}
	if running.HeartbeatAgeSec == nil || *running.HeartbeatAgeSec != 5 {
		t.Fatalf("心跳间隔应为 5 秒，实际 %+v", running.HeartbeatAgeSec)
	}
	if running.Last == nil || running.Last.RunID != 29 {
		t.Fatalf("最近一次执行应为 run_id=29，实际 %+v", running.Last)
	}
	if running.Success24h != 3 || running.Failed24h != 1 || running.Paused24h != 2 || running.Total24h != 6 {
		t.Fatalf("24 小时统计不符: %+v", running)
	}
	if running.LastSuccessAt == nil || !running.LastSuccessAt.Equal(successAt) {
		t.Fatalf("最近成功时间不符: %+v", running.LastSuccessAt)
	}

	stale := states[1]
	if stale.InProcess || stale.Running == nil || !stale.Stale {
		t.Fatalf("心跳超时的残留记录应标记 stale=true，实际 %+v", stale)
	}
	if stale.HeartbeatAgeSec == nil || *stale.HeartbeatAgeSec != 300 {
		t.Fatalf("心跳间隔应为 300 秒，实际 %+v", stale.HeartbeatAgeSec)
	}

	idle := states[2]
	if idle.Running != nil || idle.InProcess || idle.Stale {
		t.Fatalf("空闲 Job 不应有运行中记录，实际 %+v", idle)
	}
	if idle.Last == nil || idle.Last.RunID != 50 {
		t.Fatalf("空闲 Job 应展示最近一次执行，实际 %+v", idle.Last)
	}
}

// TestBuildJobRuntimeStatesFreshOtherWorker 校验多实例场景：本进程未执行但心跳新鲜时不判为卡住。
func TestBuildJobRuntimeStatesFreshOtherWorker(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	runtimes := []JobRuntime{{Kind: JobUploadPendingExportFiles, Schedule: "全天每 5 分钟", Interval: 300}}
	started := now.Add(-45 * time.Second)
	heartbeat := now.Add(-10 * time.Second)
	recent := map[string][]RunLogRow{
		JobUploadPendingExportFiles: {
			{RunID: 7, JobKind: JobUploadPendingExportFiles, GmtStart: started, GmtHeartbeat: &heartbeat},
		},
	}

	states := buildJobRuntimeStates(now, runtimes, nil, recent, nil, 2*time.Minute)
	if len(states) != 1 {
		t.Fatalf("快照数量应为 1，实际 %d", len(states))
	}
	state := states[0]
	if state.InProcess {
		t.Fatalf("本进程未执行时 in_process 应为 false")
	}
	if state.Stale {
		t.Fatalf("心跳新鲜时不应标记 stale，实际 %+v", state)
	}
	if state.ElapsedSec != 45 {
		t.Fatalf("已运行秒数应为 45，实际 %d", state.ElapsedSec)
	}
}

func stringPtr(value string) *string { return &value }

func timePtr(value time.Time) *time.Time { return &value }
