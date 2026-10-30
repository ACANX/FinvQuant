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
//
// 过程日志（写入 cdeu.finv_quote_cdeu_job_run_log_line，stage=UPLOAD）逐关键节点一行：
// 上传开始（含 file_id/usc/业务月/类型/尝试次数）→ 本地文件与登记值 → 上传目标
// （网站/组织/仓库/分支/目标路径）→ 本地校验结果 → 远端 HEAD 去重结果 →
// PUT 字节数与耗时 → 远端校验结果 → 成功/失败汇总（含起止时间与总耗时）。
func (s *Service) uploadFile(ctx context.Context, file *File, result *Result) error {
	started := s.options.Now()
	targetPath := s.archiveTargetPath(ctx, file)

	// logStep 同时刷新「最近明细」并追加一条详细日志行（percent<0 表示不确定进度）。
	logStep := func(format string, args ...any) {
		s.reportProgress(ctx, "UPLOAD", -1, fmt.Sprintf(format, args...))
	}

	logStep("上传开始 file_id=%d usc=%s biz_month=%d type=%s attempt=%d 开始时间=%s",
		file.FileID, file.Usc, file.BizMonth, file.FileType, file.UploadAttempt,
		started.Format(time.RFC3339))

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
		// 失败项必须出现在本次运行的「执行日志」里（ERROR 级别），而不是只留在动态 JOB
		// 的异常字段与进程 stdout —— 否则页面只能看到笼统的「JOB 完成 · SUCCESS」。
		diagnostic := fmt.Sprintf(
			"上传失败 file_id=%d usc=%s 文件=%s 本地路径=%s 目标路径=%s 目标=%s attempt=%d 原因=%s",
			file.FileID, file.Usc, file.TargetFileName, file.FilePath, targetPath,
			describeUploadTarget(finishCtx, s.uploader, targetPath), file.UploadAttempt, cause)
		if exhausted {
			log.Printf("[cdeu] ALERT UPLOAD_RETRY_EXHAUSTED file_id=%d usc=%s biz_month=%d type=%s attempt=%d",
				file.FileID, file.Usc, file.BizMonth, file.FileType, file.UploadAttempt)
			diagnostic += fmt.Sprintf("；重试已耗尽（下次尝试时间=%s）", nextRetry.Format(time.RFC3339))
		} else {
			diagnostic += fmt.Sprintf("；下次重试=%s", nextRetry.Format(time.RFC3339))
		}
		s.reportRunLog(finishCtx, "ERROR", "UPLOAD", diagnostic)
		log.Printf("[cdeu] %s", diagnostic)
		return cause
	}

	// G-09：上传前校验本地文件存在、大小、checksum 与登记一致。
	logStep("本地文件=%s 登记大小=%d 登记 sha256=%s", file.FilePath, file.FileSize, file.Checksum)
	checksum, size, err := FileSHA256(file.FilePath)
	if err != nil {
		return markFailed(fmt.Errorf("本地文件不可读: %w", err))
	}
	if size != file.FileSize || !strings.EqualFold(checksum, file.Checksum) {
		return markFailed(fmt.Errorf("本地文件与登记不一致（size %d/%d checksum %s/%s）",
			size, file.FileSize, checksum, file.Checksum))
	}
	logStep("本地校验通过 实际大小=%d 实际 sha256=%s（与登记一致）", size, checksum)

	// 上传目标：网站 / 组织（用户）/ 仓库 / 分支 / 目标路径（敏感信息不入日志）。
	logStep("上传目标=%s", describeUploadTarget(ctx, s.uploader, targetPath))

	// 4.5：上传前 HEAD 目标路径。	//
	// 注意「远端校验值」的口径：dir/http 返回 SHA256，可与本地 checksum 直接比较；
	// github 返回 git blob sha1（GitHub 不提供 SHA256），与本地 SHA256 不可比，
	// 此时不在此处判定「内容是否相同」，而是交给 uploader.Put 按自身口径幂等处理。
	checksumAlgorithm := remoteChecksumAlgorithm(s.uploader)
	comparable := checksumAlgorithm == checksumAlgorithmSHA256
	exists, remoteSize, remoteChecksum, statErr := s.uploader.Stat(ctx, targetPath)
	if statErr != nil {
		// 部分网关不支持 HEAD，失败不阻断上传，交由 PUT + Verify 兜底。
		log.Printf("[cdeu] 远端 HEAD 失败 file_id=%d err=%v", file.FileID, statErr)
		logStep("远端 HEAD 失败（不阻断，交由 PUT+Verify 兜底）target=%s 错误=%v", targetPath, statErr)
	} else if exists && remoteSize == size && comparable && strings.EqualFold(remoteChecksum, checksum) {
		logStep("远端已存在同内容文件，跳过上传（幂等）远端大小=%d 远端校验值=%s", remoteSize, remoteChecksum)
		return s.finishUploadSuccess(ctx, file, targetPath, 0, started, result)
	} else if exists && !comparable {
		logStep("远端已存在 远端大小=%d 远端校验值=%s（口径=%s，与本地 SHA256 不可比），交由上传实现按自身口径判定是否覆盖",
			remoteSize, remoteChecksum, checksumAlgorithm)
	} else if exists {
		logStep("远端已存在但大小或内容不一致，将覆盖 远端大小=%d 远端校验值=%s 本地大小=%d 本地 sha256=%s",
			remoteSize, remoteChecksum, size, checksum)
	} else {
		logStep("远端不存在该文件，开始上传 目标路径=%s", targetPath)
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
				s.reportProgress(ctx, "UPLOAD", -1, fmt.Sprintf("上传中 file_id=%d %s 已用时 %s",
					file.FileID, file.TargetFileName, time.Since(started).Round(time.Second)))
			}
		}
	}()
	putStarted := time.Now()
	uploaded, err := s.uploader.Put(ctx, file.FilePath, targetPath)
	if err != nil {
		return markFailed(fmt.Errorf("PUT 失败: %w", err))
	}
	logStep("PUT 完成 字节=%d 耗时=%s（开始 %s / 结束 %s）",
		uploaded, time.Since(putStarted).Round(time.Millisecond),
		putStarted.Format(time.RFC3339), time.Now().Format(time.RFC3339))

	// G-11 / M-03：远端 checksum 校验通过后才标记成功。
	if err := s.uploader.Verify(ctx, targetPath, size, checksum); err != nil {
		return markFailed(fmt.Errorf("远端校验失败: %w", err))
	}
	logStep("远端校验通过 大小=%d sha256=%s", size, checksum)
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
	targetDescription := describeUploadTarget(ctx, s.uploader, targetPath)
	s.reportProgress(ctx, "UPLOAD", -1, fmt.Sprintf(
		"上传成功 file_id=%d usc=%s type=%s 字节=%d 总耗时=%s 结束时间=%s 目标=%s",
		file.FileID, file.Usc, file.FileType, uploaded,
		time.Since(started).Round(time.Millisecond), time.Now().Format(time.RFC3339), targetDescription))
	log.Printf("[cdeu] 上传成功 file_id=%d usc=%s biz_month=%d type=%s target=%s bytes=%d",
		file.FileID, file.Usc, file.BizMonth, file.FileType, targetPath, uploaded)
	return nil
}
