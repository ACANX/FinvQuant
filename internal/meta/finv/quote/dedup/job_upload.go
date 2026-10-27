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
// uploadFile 上传单个文件：本地校验 → 远端去重 → PUT → 远端校验 → 落定状态与审计日志。
func (s *Service) uploadFile(ctx context.Context, file *File, result *Result) error {
	started := s.options.Now()
	targetPath := s.archiveTargetPath(ctx, file)

	markFailed := func(cause error) error {
		nextRetry := s.options.Now().Add(backoffFor(file.UploadAttempt))
		exhausted := file.UploadAttempt >= s.options.MaxAttempts
		if exhausted {
			nextRetry = s.options.Now().Add(24 * time.Hour)
		}
		finishCtx := context.WithoutCancel(ctx)
		if err := s.store.FinishUpload(finishCtx, file.FileID, UploadStatusFailed, false, &targetPath, &nextRetry); err != nil {
			log.Printf("[cdeu] 记录上传失败状态失败 file_id=%d err=%v", file.FileID, err)
		}
		if err := s.store.InsertUploadLog(finishCtx, file.FileID, file.UploadAttempt, targetPath,
			started, "FAILED", 0, cause.Error()); err != nil {
			log.Printf("[cdeu] 写上传审计日志失败 file_id=%d err=%v", file.FileID, err)
		}
		if exhausted {
			log.Printf("[cdeu] ALERT UPLOAD_RETRY_EXHAUSTED file_id=%d usc=%s biz_month=%d type=%s attempt=%d",
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
		log.Printf("[cdeu] 远端 HEAD 失败 file_id=%d err=%v", file.FileID, statErr)
	} else if exists && remoteSize == size && strings.EqualFold(remoteChecksum, checksum) {
		return s.finishUploadSuccess(ctx, file, targetPath, 0, started, result)
	}

	// G-10：以 target_file_name（默认等于 file_name，.mvsv / .json）上传。
	s.reportProgress(ctx, "UPLOAD", -1, fmt.Sprintf("上传 file_id=%d %s", file.FileID, file.TargetFileName))
	// 大文件 PUT 可能持续较久：另起心跳协程周期性刷新，避免被误判为「卡住」。
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ticker.C:
				s.reportProgress(ctx, "UPLOAD", -1, fmt.Sprintf("上传中 file_id=%d %s", file.FileID, file.TargetFileName))
			}
		}
	}()
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

// archiveTargetPath 计算上传文件的远端相对路径（不含 sys_config 20004 的 pre_path_key）。
//
// 分层目录 {region}_{market}/{usc}/ 的 region/market 不在 finv_dedup_export_file
// 里，需从证券主数据（cdeu.finv_quote_cdeu_usc）读取：
//   - 取到且非空 → 使用分层路径（默认布局）；
//   - 取不到（历史/手工登记数据缺失）→ 退回扁平路径并打警告，不因元数据
//     缺失阻塞上传（文件名本身已含 region/market，可事后补救）。
func (s *Service) archiveTargetPath(ctx context.Context, file *File) string {
	if s.options.UploadLayout != UploadLayoutBySecurity {
		return s.options.TargetPath(file.TargetFileName)
	}
	info, err := s.store.UscInfo(ctx, file.Usc)
	if err != nil {
		log.Printf("[cdeu] 读取证券元数据失败，本次按扁平路径上传 file_id=%d usc=%s err=%v",
			file.FileID, file.Usc, err)
		return s.options.TargetPath(file.TargetFileName)
	}
	if info == nil || info.Region == nil || info.Market == nil ||
		strings.TrimSpace(*info.Region) == "" || strings.TrimSpace(*info.Market) == "" {
		log.Printf("[cdeu] 证券元数据缺 region/market，本次按扁平路径上传 file_id=%d usc=%s",
			file.FileID, file.Usc)
		return s.options.TargetPath(file.TargetFileName)
	}
	return s.options.ArchiveTargetPath(*info.Region, *info.Market, file.Usc, file.TargetFileName)
}

func (s *Service) finishUploadSuccess(ctx context.Context, file *File, targetPath string, uploaded int64, started time.Time, result *Result) error {
	if err := s.store.FinishUpload(ctx, file.FileID, UploadStatusUploaded, true, &targetPath, nil); err != nil {
		return err
	}
	if err := s.store.InsertUploadLog(ctx, file.FileID, file.UploadAttempt, targetPath,
		started, "SUCCESS", uploaded, ""); err != nil {
		log.Printf("[cdeu] 写上传审计日志失败 file_id=%d err=%v", file.FileID, err)
	}
	if uploaded <= 0 {
		uploaded = file.FileSize
	}
	result.ProcessedBytes += uploaded
	log.Printf("[cdeu] 上传成功 file_id=%d usc=%s biz_month=%d type=%s target=%s bytes=%d",
		file.FileID, file.Usc, file.BizMonth, file.FileType, targetPath, uploaded)
	return nil
}
