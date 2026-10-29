package job

import (
	"context"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	// CLEAN_EXPORT_STORAGE —— 归档复核阶段：只管中转表，不碰本地文件与数据生命周期。
	//
	// 需求 10744 起职责再次收窄：
	//   · 本地 .mvsv / Manifest 的清理迁到 PROCESS_PURGE 的 JOB_PURGE_LOCAL_FILES
	//     （由 20011 控制，判据是「该月全部文件已交付且超过 24 小时」）；
	//   · 本 JOB 只保留 export schema 中转表的无主残留清理，空了就 TRUNCATE。
	// 主表数据由 PURGE_EXPORTED_MONTH 负责，历史动态实例由 PURGE_STALE_DYNAMIC 负责。
	dedup.RegisterJobExecutor(dedup.JobCleanExportStorage, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "STORAGE", 0, "清理中转表残留")
		stagingDeleted, err := svc.Store().PurgeResidualStaging(ctx, stagingPurgeLimit)
		if err != nil {
			return err
		}
		truncated, err := svc.Store().TruncateStagingIfEmpty(ctx)
		if err != nil {
			// TRUNCATE 失败不影响已完成的残留清理。
			log.Printf("[cdeu] TRUNCATE 中转表失败 err=%v", err)
		}

		result.ProcessedRows = stagingDeleted
		result.Details = map[string]any{
			"staging_deleted":   stagingDeleted,
			"staging_truncated": truncated,
		}
		log.Printf("[cdeu] %s 完成 staging_deleted=%d truncated=%v",
			dedup.JobCleanExportStorage, stagingDeleted, truncated)
		return nil
	})
}
