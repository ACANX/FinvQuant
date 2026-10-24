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
func (s *Service) runScanPostSealPatches(ctx context.Context, result *Result) error {
	budget, err := s.ensureBudget(ctx, s.options.Today())
	if err != nil {
		return err
	}
	allowed := int64(float64(budget.BudgetRows) * s.options.PatchBudgetRatio)
	if allowed <= 0 {
		allowed = 1
	}

	candidates, err := s.store.PatchCandidates(ctx, patchBatchLimit)
	if err != nil {
		return err
	}

	var rows, rewritten, patched int64
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		if rows >= allowed {
			break
		}
		outcome, patchErr := s.processPatchCandidate(ctx, candidate, allowed-rows)
		if patchErr != nil {
			log.Printf("[exportarchive] 补丁处理失败 job_id=%d usc=%s biz_month=%d err=%v",
				candidate.JobID, candidate.Usc, candidate.BizMonth, patchErr)
			continue
		}
		rows += outcome.Rows
		switch outcome.Level {
		case PatchLevelL1Hot:
			rewritten++
		case PatchLevelL2Warm, PatchLevelL3Cold:
			if outcome.Rows > 0 {
				patched++
			}
		}
	}

	result.ProcessedRows = rows
	result.Details = map[string]any{
		"candidates":        len(candidates),
		"rewritten_base":    rewritten,
		"patches":           patched,
		"patch_rows":        rows,
		"patch_budget_rows": allowed,
	}
	log.Printf("[exportarchive] %s 完成 candidates=%d rewritten_base=%d patches=%d patch_rows=%d patch_budget_rows=%d",
		JobScanPostSealPatches, len(candidates), rewritten, patched, rows, allowed)
	return nil
}

// patchOutcome 单个证券月补丁处理结果。
type patchOutcome struct {
	Rows  int64
	Level string
}

// processPatchCandidate 处理单个证券月：脏数据为 0 则只推进水位，否则重写 base 或生成补丁。
func (s *Service) processPatchCandidate(ctx context.Context, candidate PatchCandidate, rowsLeft int64) (patchOutcome, error) {
	outcome := patchOutcome{}
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
	meta := info.Meta(candidate.BizMonth)
	job := &Job{
		JobID:       candidate.JobID,
		Usc:         candidate.Usc,
		BizMonth:    candidate.BizMonth,
		SealDate:    candidate.SealDate,
		BaseVersion: candidate.BaseVersion,
		PatchCount:  candidate.PatchCount,
	}

	// L1_HOT：base 尚未上传成功 → 重写 base（base_version+1），原子覆盖同名文件。
	if base.UploadStatus == UploadStatusPending || base.UploadStatus == UploadStatusFailed ||
		base.UploadStatus == UploadStatusUploading {
		outcome.Level = PatchLevelL1Hot
		outcome.Rows = delta
		if err := s.rewriteBase(ctx, job, base, meta); err != nil {
			return outcome, err
		}
		task := &PatchTask{
			JobID:        job.JobID,
			Usc:          job.Usc,
			BizMonth:     job.BizMonth,
			PatchLevel:   PatchLevelL1Hot,
			PatchVersion: 0,
			SourceRows:   delta,
			PatchRows:    delta,
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
		// 补丁受独立小预算控制；超出本轮预算则留待下一轮（不推进水位）。
		if delta > rowsLeft {
			outcome.Level = ""
			return outcome, nil
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
	log.Printf("[exportarchive] 补丁完成 job_id=%d usc=%s biz_month=%d level=%s rows=%d",
		job.JobID, job.Usc, job.BizMonth, outcome.Level, outcome.Rows)
	return outcome, nil
}

// rewriteBase L1_HOT：以主表整月数据重写 BASE（base_version+1），旧记录标记 SUPERSEDED。
func (s *Service) rewriteBase(ctx context.Context, job *Job, oldBase *File, meta FileMeta) error {
	from, to := MonthRange(job.BizMonth)
	rowCount, err := s.store.CountMainMonth(ctx, job.Usc, from, to)
	if err != nil {
		return err
	}

	folder := s.jobFolder(meta)
	basePath := filepath.Join(folder, meta.BaseFileName())
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	version := oldBase.Version + 1

	write, err := WriteMVsv(basePath, meta, s.options.Compression, rowCount,
		s.mainSource(ctx, job.Usc, from, to, nil))
	if err != nil {
		return err
	}
	if err := VerifyMVsv(basePath, s.options.Compression, rowCount); err != nil {
		return fmt.Errorf("MVsv 结构校验失败（L1_HOT 重写）: %w", err)
	}

	target := s.options.TargetPath(meta.BaseFileName())
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
		return err
	}
	if err := s.store.SupersedeFile(ctx, oldBase.FileID); err != nil {
		return err
	}
	if _, err := s.rebuildManifest(ctx, job, meta, manifestPath); err != nil {
		return err
	}
	job.BaseVersion = version
	if err := s.store.UpdateJobPatchState(ctx, job.JobID, version, job.PatchCount); err != nil {
		return err
	}
	log.Printf("[exportarchive] L1_HOT 重写 base job_id=%d usc=%s biz_month=%d version=%d rows=%d",
		job.JobID, job.Usc, job.BizMonth, version, write.RowCount)
	return nil
}

// writePatch 生成增量补丁文件（..._Incr_{n}.mvsv）并登记 PATCH + MANIFEST。
func (s *Service) writePatch(ctx context.Context, job *Job, base *File, meta FileMeta, since time.Time, expectedRows int64, level string, scanUntil time.Time) (int64, error) {
	from, to := MonthRange(job.BizMonth)
	version, err := s.store.NextPatchVersion(ctx, job.JobID)
	if err != nil {
		return 0, err
	}

	folder := s.jobFolder(meta)
	patchName := meta.PatchFileName(version)
	patchPath := filepath.Join(folder, patchName)
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	sinceArg := since

	write, err := WriteMVsv(patchPath, meta, s.options.Compression, expectedRows,
		s.mainSource(ctx, job.Usc, from, to, &sinceArg))
	if err != nil {
		return 0, err
	}
	if err := VerifyMVsv(patchPath, s.options.Compression, expectedRows); err != nil {
		return 0, fmt.Errorf("MVsv 结构校验失败（Patch）: %w", err)
	}

	target := s.options.TargetPath(patchName)
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
		SourceRows:   expectedRows,
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
	log.Printf("[exportarchive] 生成补丁 job_id=%d usc=%s biz_month=%d level=%s n=%d rows=%d checksum=%s",
		job.JobID, job.Usc, job.BizMonth, level, version, write.RowCount, write.Checksum)
	return write.RowCount, nil
}

// patchWatermarkName 每个证券月独立的补丁扫描水位名。
func patchWatermarkName(usc string, bizMonth int) string {
	return fmt.Sprintf("POST_SEAL_PATCH_%s_%d", usc, bizMonth)
}
