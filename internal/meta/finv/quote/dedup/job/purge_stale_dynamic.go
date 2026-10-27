package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	// PURGE_STALE_DYNAMIC —— 归档复核阶段：只清理历史动态 JOB 实例（需求 10717）。
	//
	// 保留天数取系统配置 20008（默认 30，越界回退默认）。
	// 只删「所属 workflow_run 已终态 + 实例自身已终态」的记录，运行中的实例永不删除。
	dedup.RegisterJobExecutor(dedup.JobPurgeStaleDynamic, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		retentionDays := svc.DynamicJobRetentionDays(ctx)
		svc.ReportJobProgress(ctx, "RETENTION", 0,
			fmt.Sprintf("清理历史动态 JOB 实例（保留 %d 天）", retentionDays))

		deleted, err := svc.Store().PurgeStaleDynamicJobRuns(ctx, retentionDays, 5000)
		if err != nil {
			return err
		}

		result.Details = map[string]any{
			"retention_days": retentionDays,
			"deleted":        deleted,
		}
		log.Printf("[cdeu] %s 完成 retention_days=%d deleted=%d",
			dedup.JobPurgeStaleDynamic, retentionDays, deleted)
		return nil
	})
}
