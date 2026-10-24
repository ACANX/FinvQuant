package dedup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// uploadBatchLimit 单轮上传 Job 最多处理的文件数（避免长时间占用调度协程）。
const uploadBatchLimit = 200

// runUploadPendingExportFiles 扫描 finv_dedup_export_file 待上传记录并上传独享存储库（G-08~G-14）。
func (s *Service) runUploadPendingExportFiles(ctx context.Context, result *Result) error {
	if s.uploader.Mode() == UploadModeNone {
		result.Message = "未启用上传（FINV_QUOTE_DEDUB_UPLOAD_MODE=none），跳过"
		return nil
	}

	var uploaded, failed int
	for index := 0; index < uploadBatchLimit; index++ {
		if err := ctx.Err(); err != nil {
			break
		}
		file, err := s.store.PickUploadFile(ctx)
		if err != nil {
			return err
		}
		if file == nil {
			break
		}
		if err := s.uploadFile(ctx, file, result); err != nil {
			failed++
			continue
		}
		uploaded++
	}

	result.Details = map[string]any{
		"mode":     s.uploader.Mode(),
		"uploaded": uploaded,
		"failed":   failed,
	}
	log.Printf("[exportarchive] %s 完成 mode=%s uploaded=%d failed=%d",
		JobUploadPendingExportFiles, s.uploader.Mode(), uploaded, failed)
	return nil
}

// uploadFile 上传单个文件：本地校验 → 远端去重 → PUT → 远端校验 → 落定状态与审计日志。
func (s *Service) uploadFile(ctx context.Context, file *File, result *Result) error {
	started := s.options.Now()
	targetPath := s.options.TargetPath(file.TargetFileName)

	markFailed := func(cause error) error {
		nextRetry := s.options.Now().Add(backoffFor(file.UploadAttempt))
		exhausted := file.UploadAttempt >= s.options.MaxAttempts
		if exhausted {
			nextRetry = s.options.Now().Add(24 * time.Hour)
		}
		finishCtx := context.WithoutCancel(ctx)
		if err := s.store.FinishUpload(finishCtx, file.FileID, UploadStatusFailed, false, &targetPath, &nextRetry); err != nil {
			log.Printf("[exportarchive] 记录上传失败状态失败 file_id=%d err=%v", file.FileID, err)
		}
		if err := s.store.InsertUploadLog(finishCtx, file.FileID, file.UploadAttempt, targetPath,
			started, "FAILED", 0, cause.Error()); err != nil {
			log.Printf("[exportarchive] 写上传审计日志失败 file_id=%d err=%v", file.FileID, err)
		}
		if exhausted {
			log.Printf("[exportarchive] ALERT UPLOAD_RETRY_EXHAUSTED file_id=%d usc=%s biz_month=%d type=%s attempt=%d",
				file.FileID, file.Usc, file.BizMonth, file.FileType, file.UploadAttempt)
		}
		return cause
	}

	// G-09：上传前校验本地文件存在、大小、checksum 与登记一致。
	checksum, size, err := FileSHA256(file.FilePath)
	if err != nil {
		return markFailed(fmt.Errorf("本地文件不可读: %w", err))
	}
	if size != file.FileSize || !strings.EqualFold(checksum, file.Checksum) {
		return markFailed(fmt.Errorf("本地文件与登记不一致（size %d/%d checksum %s/%s）",
			size, file.FileSize, checksum, file.Checksum))
	}

	// 4.5：上传前 HEAD 目标路径，已存在且 checksum 一致则直接标成功（幂等重传）。
	exists, remoteSize, remoteChecksum, statErr := s.uploader.Stat(ctx, targetPath)
	if statErr != nil {
		// 部分网关不支持 HEAD，失败不阻断上传，交由 PUT + Verify 兜底。
		log.Printf("[exportarchive] 远端 HEAD 失败 file_id=%d err=%v", file.FileID, statErr)
	} else if exists && remoteSize == size && strings.EqualFold(remoteChecksum, checksum) {
		return s.finishUploadSuccess(ctx, file, targetPath, 0, started, result)
	}

	// G-10：以 target_file_name（默认等于 file_name，.mvsv / .json）上传。
	uploaded, err := s.uploader.Put(ctx, file.FilePath, targetPath)
	if err != nil {
		return markFailed(err)
	}

	// G-11 / M-03：远端 checksum 校验通过后才标记成功。
	if err := s.uploader.Verify(ctx, targetPath, size, checksum); err != nil {
		return markFailed(err)
	}
	return s.finishUploadSuccess(ctx, file, targetPath, uploaded, started, result)
}

func (s *Service) finishUploadSuccess(ctx context.Context, file *File, targetPath string, uploaded int64, started time.Time, result *Result) error {
	if err := s.store.FinishUpload(ctx, file.FileID, UploadStatusUploaded, true, &targetPath, nil); err != nil {
		return err
	}
	if err := s.store.InsertUploadLog(ctx, file.FileID, file.UploadAttempt, targetPath,
		started, "SUCCESS", uploaded, ""); err != nil {
		log.Printf("[exportarchive] 写上传审计日志失败 file_id=%d err=%v", file.FileID, err)
	}
	if uploaded <= 0 {
		uploaded = file.FileSize
	}
	result.ProcessedBytes += uploaded
	log.Printf("[exportarchive] 上传成功 file_id=%d usc=%s biz_month=%d type=%s target=%s bytes=%d",
		file.FileID, file.Usc, file.BizMonth, file.FileType, targetPath, uploaded)
	return nil
}
