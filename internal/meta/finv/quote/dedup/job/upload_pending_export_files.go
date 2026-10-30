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

		batchStarted := time.Now()
		svc.ReportJobProgress(ctx, "UPLOAD", 0, fmt.Sprintf(
			"开始扫描待上传文件 上传模式=%s 单轮上限=%d 个 开始时间=%s",
			svc.Uploader().Mode(), uploadBatchLimit, batchStarted.Format(time.RFC3339)))

		var uploaded, failed int
		var firstFailedID int64
		firstFailedErr := ""
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
				if failed == 0 {
					firstFailedID = file.FileID
					firstFailedErr = err.Error()
				}
				failed++
				continue
			}
			_ = svc.Store().MarkDynamicUploadJobStatus(ctx, file.FileID, dedup.CDEUStatusSuccess, "", runLogID)
			_ = svc.Store().MarkDynamicUploadJobStepStatus(ctx, file.FileID, dedup.CDEUStatusSuccess, "")
			uploaded++
		}

		// 主表数据释放已迁至 PHASE_REVIEW 的 PURGE_EXPORTED_MONTH（需求 10716/10717），
		// 上传 JOB 只负责把文件送到远端，不再碰数据生命周期。
		elapsed := time.Since(batchStarted).Round(time.Second)
		result.Details = map[string]any{
			"mode":              svc.Uploader().Mode(),
			"uploaded":          uploaded,
			"failed":            failed,
			"manifests_skipped": len(skippedManifests),
			"uploaded_bytes":    result.ProcessedBytes,
			"elapsed_seconds":   elapsed.Seconds(),
			"first_failed_id":   firstFailedID,
			"first_failed_err":  firstFailedErr,
		}
		// 有失败项时把摘要写进 result.Message：它会被 FinishRun 写入 run 的 error 字段
		// 与末条日志，使页面的「异常」列与「执行日志」一致，不再出现只有 SUCCESS 的情况。
		if failed > 0 {
			result.Message = fmt.Sprintf("本轮 %d 个文件上传失败（首个 file_id=%d：%s），失败项已按退避策略排入重试；详细原因见 ERROR 级日志行",
				failed, firstFailedID, firstFailedErr)
		}
		svc.ReportJobProgress(ctx, "UPLOAD", 100, fmt.Sprintf(
			"本轮上传完成 成功 %d 个 / 失败 %d 个 / 跳过 MANIFEST %d 个 共 %d 字节 结束时间=%s 耗时=%s",
			uploaded, failed, len(skippedManifests), result.ProcessedBytes,
			time.Now().Format(time.RFC3339), elapsed))
		log.Printf("[cdeu] %s 完成 mode=%s uploaded=%d failed=%d manifests_skipped=%d bytes=%d elapsed=%s first_failed=%d %s",
			dedup.JobUploadPendingExportFiles, svc.Uploader().Mode(), uploaded, failed,
			len(skippedManifests), result.ProcessedBytes, elapsed, firstFailedID, firstFailedErr)
		return nil
	})
}
