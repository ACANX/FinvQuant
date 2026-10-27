package dedup

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"
)

// patchBatchLimit 单轮补丁扫描最多检查的证券月数。
const patchBatchLimit = 100

// runScanPostSealPatches 扫描收口后到达的数据，按 L1_HOT / L2_WARM / L3_COLD 分级补丁（K-01~K-11）。
// PatchOutcome 单个证券月补丁处理结果。
type PatchOutcome struct {
	Rows  int64
	Level string
}

// processPatchCandidate 处理单个证券月：脏数据为 0 则只推进水位，否则重写 base 或生成补丁。
func (s *Service) processPatchCandidate(ctx context.Context, candidate PatchCandidate, rowsLeft int64) (PatchOutcome, error) {
	outcome := PatchOutcome{}
	base, err := s.store.LatestBaseFile(ctx, candidate.JobID)
	if err != nil || base == nil {
		return outcome, err
	}

	watermarkName := patchWatermarkName(candidate.Usc, candidate.BizMonth)
	since, found, err := s.store.Watermark(ctx, watermarkName)
	if err != nil {
		return outcome, err
	}
	if !found {
		since = base.GmtCreate
	}
	scanUntil := s.options.Now()

	from, to := MonthRange(candidate.BizMonth)
	delta, err := s.store.CountMainDelta(ctx, candidate.Usc, from, to, since)
	if err != nil {
		return outcome, err
	}
	if delta == 0 {
		return outcome, s.store.UpsertWatermark(ctx, watermarkName, scanUntil)
	}

	info, err := s.store.UscInfo(ctx, candidate.Usc)
	if err != nil {
		return outcome, err
	}
	if !info.Complete() {
		return outcome, fmt.Errorf("证券 %s 元数据不完整，跳过补丁", candidate.Usc)
	}
	// 只有确认 delta > 0 后才创建 INCR 动态 JOB，避免零增量任务出现在 JOB 清单。
	if err := s.store.EnsureDynamicExportJob(ctx, "PROCESS_EXPORT_FILE_MON_INCR",
		candidate.Usc, candidate.BizMonth, "INCR", candidate.JobID); err != nil {
		return outcome, err
	}
	_ = s.store.MarkDynamicJobStatus(ctx, candidate.Usc, candidate.BizMonth, CDEUStatusRunning, "", runIDFromContext(ctx))
	meta := info.Meta(candidate.BizMonth)
	job := &Job{
		JobID:       candidate.JobID,
		Usc:         candidate.Usc,
		BizMonth:    candidate.BizMonth,
		SealDate:    candidate.SealDate,
		BaseVersion: candidate.BaseVersion,
		PatchCount:  candidate.PatchCount,
	}

	// 20007=0（保留主表数据）：重复导出**直接重写同名 BASE 覆盖**，不生成 INCR。
	//
	// 语义与 20007=1 互斥：
	//   - 20007=0 → 始终 rewriteBase（主表数据还在，可整月重写）；
	//   - 20007=1 → 保持原有分级逻辑（BASE 未上传成功时 rewriteBase；已上传则生成 INCR，
	//     因为此时主表该月数据可能已在上传成功后被删除，无法再整月重写）。
	//
	// 注意：BASE 文件名不含版本号，重写即同名覆盖，不存在「版本号递增」；
	// 只有 INCR 文件名（_Incr_N.mvsv）的 N 会随 patch_count 递增。
	forceRewriteBase := !s.PurgeAfterExportEnabled(ctx)
	baseNotUploaded := base.UploadStatus == UploadStatusPending || base.UploadStatus == UploadStatusFailed ||
		base.UploadStatus == UploadStatusUploading
	if forceRewriteBase && !baseNotUploaded {
		log.Printf("[cdeu] 20007=0 覆盖 BASE（不生成 INCR）job_id=%d usc=%s biz_month=%d 原上传状态=%s",
			candidate.JobID, candidate.Usc, candidate.BizMonth, base.UploadStatus)
	}

	// L1_HOT：base 尚未上传成功，或 20007=0 强制覆盖 → 重写 base，原子覆盖同名文件。
	if baseNotUploaded || forceRewriteBase {
		outcome.Level = PatchLevelL1Hot
		if delta > rowsLeft {
			log.Printf("[cdeu] L1 重写 base 数据量较大 job_id=%d usc=%s delta=%d 预算剩余=%d，仍按完整数据执行",
				job.JobID, job.Usc, delta, rowsLeft)
		}
		written, err := s.rewriteBase(ctx, job, base, meta)
		if err != nil {
			return outcome, err
		}
		outcome.Rows = written
		task := &PatchTask{
			JobID:        job.JobID,
			Usc:          job.Usc,
			BizMonth:     job.BizMonth,
			PatchLevel:   PatchLevelL1Hot,
			PatchVersion: 0,
			SourceRows:   written,
			PatchRows:    written,
			Reason:       PatchReasonLateArrival,
			ArrivalStart: since,
			ArrivalEnd:   scanUntil,
			Status:       PatchStatusDone,
		}
		if _, err := s.store.InsertPatchTask(ctx, task); err != nil {
			return outcome, err
		}
	} else {
		// L2_WARM / L3_COLD：生成增量补丁（..._Incr_{n}.mvsv）并更新 Manifest。
		outcome.Level = PatchLevelL2Warm
		if base.NetworkStatus == NetworkStatusArchived || base.CleanupStatus == CleanupStatusLocalDeleted {
			outcome.Level = PatchLevelL3Cold
		}
		// 数据量大的补丁也必须完整导出（走中转表 / 分页装载）；超出小预算只记录告警，不截断。
		if delta > rowsLeft {
			log.Printf("[cdeu] 补丁数据量较大 job_id=%d usc=%s biz_month=%d delta=%d 预算剩余=%d，仍按完整补丁执行",
				job.JobID, job.Usc, job.BizMonth, delta, rowsLeft)
		}
		written, err := s.writePatch(ctx, job, base, meta, since, delta, outcome.Level, scanUntil)
		if err != nil {
			return outcome, err
		}
		outcome.Rows = written
	}

	if err := s.store.UpsertWatermark(ctx, watermarkName, scanUntil); err != nil {
		return outcome, err
	}
	log.Printf("[cdeu] 补丁完成 job_id=%d usc=%s biz_month=%d level=%s rows=%d",
		job.JobID, job.Usc, job.BizMonth, outcome.Level, outcome.Rows)
	return outcome, nil
}

// stagePatchRows 将补丁数据分页装入中转表（与正常导出共用 export.finv_quote_secu_kline_min）。
//
// since 为 nil 表示整月（L1 重写 base），否则只装载 gmt_create/gmt_update 晚于水位的增量。
// 装载前会清理该证券月残留，避免上次失败数据混入；写文件后由 cleanupPatchStaging 统一清理。
func (s *Service) stagePatchRows(ctx context.Context, job *Job, from, to int, since *time.Time, expected int64) (int64, error) {
	if _, err := s.store.CleanupStaging(ctx, job.Usc, from, to); err != nil {
		return 0, err
	}
	var loaded int64
	for date := from; date <= to; date = nextDate(date) {
		cursor := int64(0)
		for {
			page, err := s.store.LoadPatchPage(ctx, job.Usc, date, cursor, since, s.options.PageSize)
			if err != nil {
				return loaded, err
			}
			loaded += page.InsertedRows
			cursor = page.MaxTS
			percent := -1.0
			if expected > 0 {
				percent = float64(loaded) / float64(expected) * 100
				if percent > 100 {
					percent = 100
				}
			}
			s.reportProgress(ctx, "PATCH", percent,
				fmt.Sprintf("补丁装载 %s date=%d 已装载 %d/%d 行", job.Usc, date, loaded, expected))
			if page.PageRows == 0 || page.PageRows < int64(s.options.PageSize) {
				break
			}
		}
	}
	return loaded, nil
}

// cleanupPatchStaging 清理补丁占用的中转数据（成功/失败均调用，避免残留）。
func (s *Service) cleanupPatchStaging(ctx context.Context, job *Job, from, to int) {
	cleanupCtx := context.WithoutCancel(ctx)
	if _, err := s.store.CleanupStaging(cleanupCtx, job.Usc, from, to); err != nil {
		log.Printf("[cdeu] 清理补丁中转数据失败 usc=%s biz_month=%d err=%v", job.Usc, job.BizMonth, err)
		return
	}
	if truncated, err := s.store.TruncateStagingIfEmpty(cleanupCtx); err != nil {
		log.Printf("[cdeu] TRUNCATE 中转表失败 err=%v", err)
	} else if truncated {
		log.Printf("[cdeu] 补丁完成后中转表已清空，执行 TRUNCATE")
	}
}

// rewriteBase L1_HOT：以主表整月数据重写 BASE（base_version+1），旧记录标记 SUPERSEDED。
func (s *Service) rewriteBase(ctx context.Context, job *Job, oldBase *File, meta FileMeta) (int64, error) {
	from, to := MonthRange(job.BizMonth)
	rowCount, err := s.store.CountMainMonth(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if rowCount <= 0 {
		return 0, fmt.Errorf("主表 %s %d 月无数据，无法重写 base", job.Usc, job.BizMonth)
	}
	if _, err := s.stagePatchRows(ctx, job, from, to, nil, rowCount); err != nil {
		s.cleanupPatchStaging(ctx, job, from, to)
		return 0, err
	}
	defer s.cleanupPatchStaging(ctx, job, from, to)

	actual, err := s.store.CountStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if actual <= 0 {
		return 0, fmt.Errorf("补丁中转表为空（期望 %d 行）", rowCount)
	}

	folder := s.jobFolder(meta)
	basePath := filepath.Join(folder, meta.BaseFileName())
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	version := oldBase.Version + 1

	write, err := WriteMVsv(basePath, meta, s.options.Compression, actual,
		s.stagingSource(ctx, job.Usc, from, to))
	if err != nil {
		return 0, err
	}
	if err := VerifyMVsv(basePath, s.options.Compression, actual); err != nil {
		return 0, fmt.Errorf("MVsv 结构校验失败（L1_HOT 重写）: %w", err)
	}

	target := s.options.ArchiveTargetPath(meta.Region, meta.Market, meta.Usc, meta.BaseFileName())
	sealDate := job.SealDate
	if _, err := s.store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            job.Usc,
		BizMonth:       job.BizMonth,
		FileType:       FileTypeBase,
		Version:        version,
		FileName:       meta.BaseFileName(),
		FilePath:       write.Path,
		TargetFileName: meta.BaseFileName(),
		UploadTarget:   &target,
		Format:         MvsvFormat,
		Compression:    s.options.Compression,
		RowCount:       write.RowCount,
		FileSize:       write.Size,
		Checksum:       write.Checksum,
		ManifestPath:   &manifestPath,
		SealDate:       &sealDate,
		ExportStatus:   ExportStatusExported,
		UploadStatus:   UploadStatusPending,
	}); err != nil {
		return 0, err
	}
	if err := s.store.SupersedeFile(ctx, oldBase.FileID); err != nil {
		return 0, err
	}
	if _, err := s.rebuildManifest(ctx, job, meta, manifestPath); err != nil {
		return 0, err
	}
	job.BaseVersion = version
	if err := s.store.UpdateJobPatchState(ctx, job.JobID, version, job.PatchCount); err != nil {
		return 0, err
	}
	log.Printf("[cdeu] L1_HOT 重写 base（经中转表）job_id=%d usc=%s biz_month=%d version=%d rows=%d",
		job.JobID, job.Usc, job.BizMonth, version, write.RowCount)
	return write.RowCount, nil
}

// writePatch 生成增量补丁文件（..._Incr_{n}.mvsv）并登记 PATCH + MANIFEST。
//
// 与全量导出一致：先把增量装入中转表，再从中间表写补丁文件，最后清理中转数据。
func (s *Service) writePatch(ctx context.Context, job *Job, base *File, meta FileMeta, since time.Time, expectedRows int64, level string, scanUntil time.Time) (int64, error) {
	from, to := MonthRange(job.BizMonth)
	if _, err := s.stagePatchRows(ctx, job, from, to, &since, expectedRows); err != nil {
		s.cleanupPatchStaging(ctx, job, from, to)
		return 0, err
	}
	defer s.cleanupPatchStaging(ctx, job, from, to)

	actual, err := s.store.CountStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if actual <= 0 {
		return 0, fmt.Errorf("补丁中转表为空（期望 %d 行）", expectedRows)
	}

	version, err := s.store.NextPatchVersion(ctx, job.JobID)
	if err != nil {
		return 0, err
	}

	folder := s.jobFolder(meta)
	patchName := meta.PatchFileName(version)
	patchPath := filepath.Join(folder, patchName)
	manifestPath := filepath.Join(folder, meta.ManifestFileName())

	write, err := WriteMVsv(patchPath, meta, s.options.Compression, actual,
		s.stagingSource(ctx, job.Usc, from, to))
	if err != nil {
		return 0, err
	}
	if err := VerifyMVsv(patchPath, s.options.Compression, actual); err != nil {
		return 0, fmt.Errorf("MVsv 结构校验失败（Patch）: %w", err)
	}

	target := s.options.ArchiveTargetPath(meta.Region, meta.Market, meta.Usc, patchName)
	sealDate := job.SealDate
	fileID, err := s.store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            job.Usc,
		BizMonth:       job.BizMonth,
		FileType:       FileTypePatch,
		Version:        version,
		FileName:       patchName,
		FilePath:       write.Path,
		TargetFileName: patchName,
		UploadTarget:   &target,
		Format:         MvsvFormat,
		Compression:    s.options.Compression,
		RowCount:       write.RowCount,
		FileSize:       write.Size,
		Checksum:       write.Checksum,
		ManifestPath:   &manifestPath,
		SealDate:       &sealDate,
		ExportStatus:   ExportStatusExported,
		UploadStatus:   UploadStatusPending,
	})
	if err != nil {
		return 0, err
	}

	task := &PatchTask{
		JobID:        job.JobID,
		Usc:          job.Usc,
		BizMonth:     job.BizMonth,
		PatchLevel:   level,
		PatchVersion: version,
		SourceRows:   write.RowCount,
		PatchRows:    write.RowCount,
		Reason:       PatchReasonLateArrival,
		ArrivalStart: since,
		ArrivalEnd:   scanUntil,
		Status:       PatchStatusDone,
		PatchFileID:  &fileID,
	}
	patchID, err := s.store.InsertPatchTask(ctx, task)
	if err != nil {
		return 0, err
	}
	if _, err := s.rebuildManifest(ctx, job, meta, manifestPath); err != nil {
		_ = s.store.FinishPatchTask(ctx, patchID, PatchStatusFailed, write.RowCount, &fileID, err.Error())
		return 0, err
	}
	if err := s.store.UpdateJobPatchState(ctx, job.JobID, base.Version, version); err != nil {
		return 0, err
	}
	job.PatchCount = version
	log.Printf("[cdeu] 生成补丁（经中转表）job_id=%d usc=%s biz_month=%d level=%s n=%d rows=%d checksum=%s",
		job.JobID, job.Usc, job.BizMonth, level, version, write.RowCount, write.Checksum)
	return write.RowCount, nil
}

// patchWatermarkName 每个证券月独立的补丁扫描水位名。
func patchWatermarkName(usc string, bizMonth int) string {
	return fmt.Sprintf("POST_SEAL_PATCH_%s_%d", usc, bizMonth)
}
