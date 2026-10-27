package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	"github.com/acanx/finvquant/internal/syscfg"
)

func init() {
	// PURGE_HISTORY_RUNS —— 归档复核阶段：按 20009/20010 策略清理历史流程运行数据（需求 10723）。
	//
	// 目标是把 cdeu_* 运行表按 workflow_run_id 整体释放，控制库体积。
	// 策略：
	//   Off   不清理；
	//   Days  删除超过 20010 天前的 workflow_run（默认 30 天）；
	//   Count 只保留最近 20010 个 workflow_run（默认 30 个）。
	// 永久保留 run_id 最大的那一个，且只处理终态 run。
	dedup.RegisterJobExecutor(dedup.JobPurgeHistoryRuns, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		policy, keep := svc.HistoryPurgePolicy(ctx)
		if policy == syscfg.HistoryPurgeOff {
			svc.ReportJobProgress(ctx, "RETENTION", 100, "系统配置 20009=Off：不清理历史运行数据")
			result.Details = map[string]any{
				"policy":  policy,
				"keep":    keep,
				"deleted": map[string]int64{},
			}
			log.Printf("[cdeu] %s 跳过：20009=Off", dedup.JobPurgeHistoryRuns)
			return nil
		}

		svc.ReportJobProgress(ctx, "RETENTION", 0,
			fmt.Sprintf("清理历史运行数据（策略 %s，阈值 %d）", policy, keep))
		stats, err := svc.Store().PurgeHistoryRuns(ctx, policy, keep, 200)
		if err != nil {
			return err
		}

		var total int64
		for _, count := range stats {
			total += count
		}
		result.Details = map[string]any{
			"policy":  policy,
			"keep":    keep,
			"deleted": stats,
			"total":   total,
		}
		log.Printf("[cdeu] %s 完成 policy=%s keep=%d runs=%d job_runs=%d dynamic=%d logs=%d total=%d",
			dedup.JobPurgeHistoryRuns, policy, keep, stats["runs"], stats["job_runs"],
			stats["dynamic_jobs"], stats["run_log_lines"], total)
		return nil
	})
}
