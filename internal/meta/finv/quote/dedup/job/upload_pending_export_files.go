package job

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

const uploadBatchLimit = 200

// manifestSkipReason MANIFEST 不参与上传的审计说明。
const manifestSkipReason = "MANIFEST 按业务约定不上传（仅上传 .mvsv 数据文件）"

func init() {
	dedup.RegisterJobExecutor(dedup.JobUploadPendingExportFiles, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		// MANIFEST（xx_..._Manifest.json）只留本地校验，不推送远端仓库：
		// 统一标记为 SKIPPED 终态，与上传模式无关——避免它们卡在待上传队列里
		// 阻塞本地清理判定与监控统计（历史记录中已上传过的保持 UPLOADED）。
		skippedManifests, err := svc.Store().SkipManifestUploads(ctx)
		if err != nil {
			return fmt.Errorf("标记 MANIFEST 为跳过失败: %w", err)
		}
		if len(skippedManifests) > 0 {
			started := time.Now()
			for _, item := range skippedManifests {
				if logErr := svc.Store().InsertUploadLog(ctx, item.FileID, item.Attempt, item.TargetName,
					started, dedup.UploadStatusSkipped, 0, manifestSkipReason); logErr != nil {
					log.Printf("[cdeu] 写 MANIFEST 跳过审计日志失败 file_id=%d err=%v", item.FileID, logErr)
				}
				runLogID := dedup.RunLogIDFromContext(ctx)
				_ = svc.Store().MarkDynamicUploadJobStatus(ctx, item.FileID, dedup.CDEUStatusSkipped, manifestSkipReason, runLogID)
				_ = svc.Store().MarkDynamicUploadJobStepStatus(ctx, item.FileID, dedup.CDEUStatusSkipped, manifestSkipReason)
			}
			log.Printf("[cdeu] %s 跳过 %d 个 MANIFEST（按约定仅上传 .mvsv）",
				dedup.JobUploadPendingExportFiles, len(skippedManifests))
		}

		if svc.Uploader().Mode() == dedup.UploadModeNone {
			result.Message = "未启用上传（FINV_QUOTE_DEDUB_UPLOAD_MODE=none），跳过"
			return nil
		}
		if _, err := svc.Store().GenerateUploadDynamicJobs(ctx); err != nil {
			return fmt.Errorf("生成上传动态 JOB 失败: %w", err)
		}

		var uploaded, failed int
		for index := 0; index < uploadBatchLimit; index++ {
			if err := ctx.Err(); err != nil {
				break
			}
			file, err := svc.Store().PickUploadFile(ctx)
			if err != nil {
				return err
			}
			if file == nil {
				break
			}
			svc.ReportJobProgress(ctx, "UPLOAD", float64(index)/float64(uploadBatchLimit)*100,
				fmt.Sprintf("已上传 %d 个 / 失败 %d 个，当前 file_id=%d %s", uploaded, failed, file.FileID, file.FileName))
			runLogID := dedup.RunLogIDFromContext(ctx)
			_ = svc.Store().MarkDynamicUploadJobStatus(ctx, file.FileID, dedup.CDEUStatusRunning, "", runLogID)
			_ = svc.Store().MarkDynamicUploadJobStepStatus(ctx, file.FileID, dedup.CDEUStatusRunning, "")
			if err := svc.UploadOneFile(ctx, file, result); err != nil {
				_ = svc.Store().MarkDynamicUploadJobStatus(ctx, file.FileID, dedup.CDEUStatusFailed, err.Error(), runLogID)
				_ = svc.Store().MarkDynamicUploadJobStepStatus(ctx, file.FileID, dedup.CDEUStatusFailed, err.Error())
				failed++
				continue
			}
			_ = svc.Store().MarkDynamicUploadJobStatus(ctx, file.FileID, dedup.CDEUStatusSuccess, "", runLogID)
			_ = svc.Store().MarkDynamicUploadJobStepStatus(ctx, file.FileID, dedup.CDEUStatusSuccess, "")
			uploaded++
		}

		// 主表数据释放已迁至 PHASE_REVIEW 的 PURGE_EXPORTED_MONTH（需求 10716/10717），
		// 上传 JOB 只负责把文件送到远端，不再碰数据生命周期。
		result.Details = map[string]any{
			"mode":              svc.Uploader().Mode(),
			"uploaded":          uploaded,
			"failed":            failed,
			"manifests_skipped": len(skippedManifests),
		}
		log.Printf("[cdeu] %s 完成 mode=%s uploaded=%d failed=%d manifests_skipped=%d",
			dedup.JobUploadPendingExportFiles, svc.Uploader().Mode(), uploaded, failed,
			len(skippedManifests))
		return nil
	})
}
