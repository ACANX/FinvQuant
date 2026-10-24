package exportarchive

import (
	"context"
	"log"
	"os"
	"time"
)

// 维护类 Job 的批量上限（避免单轮执行时间过长）。
const (
	stagingPurgeLimit   = 50000
	localCleanupLimit   = 500
	mainPurgeBatchLimit = 50000
)

// runCleanExpiredExportArtifacts 清理超期中转残留、过期本地文件与主表旧数据（E-08 / L-01~L-04）。
func (s *Service) runCleanExpiredExportArtifacts(ctx context.Context, result *Result) error {
	// 1) 中转表残留：删除不属于任何活跃任务的数据，空表 TRUNCATE。
	stagingDeleted, err := s.store.PurgeResidualStaging(ctx, stagingPurgeLimit)
	if err != nil {
		return err
	}
	truncated, err := s.store.TruncateStagingIfEmpty(ctx)
	if err != nil {
		log.Printf("[exportarchive] TRUNCATE 中转表失败 err=%v", err)
	}

	// 2) 本地文件：上传成功 + 远端校验 + 网盘归档 + 无待处理补丁 + 保留期到达。
	localDeleted := int64(0)
	files, err := s.store.LocalCleanupCandidates(ctx, s.options.LocalRetention, localCleanupLimit)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			break
		}
		if removeErr := os.Remove(file.FilePath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("[exportarchive] 删除本地文件失败 file_id=%d path=%s err=%v", file.FileID, file.FilePath, removeErr)
			continue
		}
		if err := s.store.MarkLocalDeleted(ctx, file.FileID); err != nil {
			log.Printf("[exportarchive] 标记本地清理失败 file_id=%d err=%v", file.FileID, err)
			continue
		}
		localDeleted++
	}

	// 3) 主表旧数据：默认关闭（FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED），需全链路成功确认。
	mainDeleted := int64(0)
	purgedMonths := make([]int, 0, 4)
	if s.options.PurgeMainEnabled {
		beforeMonth := addMonths(s.options.CurrentMonth(), -s.options.PurgeMainMonths)
		months, err := s.store.PurgeMainCandidates(ctx, beforeMonth)
		if err != nil {
			return err
		}
		for _, month := range months {
			for {
				if err := ctx.Err(); err != nil {
					break
				}
				deleted, err := s.store.PurgeMainMonth(ctx, month, mainPurgeBatchLimit)
				if err != nil {
					return err
				}
				mainDeleted += deleted
				if deleted < mainPurgeBatchLimit {
					break
				}
			}
			purgedMonths = append(purgedMonths, month)
		}
	}

	result.ProcessedRows = stagingDeleted + mainDeleted
	result.Details = map[string]any{
		"staging_deleted":    stagingDeleted,
		"staging_truncated":  truncated,
		"local_deleted":      localDeleted,
		"main_deleted":       mainDeleted,
		"main_purge_months":  purgedMonths,
		"main_purge_enabled": s.options.PurgeMainEnabled,
	}
	log.Printf("[exportarchive] %s 完成 staging_deleted=%d truncated=%v local_deleted=%d main_deleted=%d main_purge_enabled=%v",
		JobCleanExpiredExportArtifacts, stagingDeleted, truncated, localDeleted, mainDeleted, s.options.PurgeMainEnabled)
	return nil
}

// runRecoverTimeoutExportJobs 回收心跳/租约超时的导出任务与上传记录（M-09）。
func (s *Service) runRecoverTimeoutExportJobs(ctx context.Context, result *Result) error {
	jobs, err := s.store.RecoverTimeoutJobs(ctx)
	if err != nil {
		return err
	}
	// 上传超时阈值：租约的 4 倍（且不少于 10 分钟）。
	timeout := 4 * s.options.Lease
	if timeout < 10*time.Minute {
		timeout = 10 * time.Minute
	}
	uploads, err := s.store.RecoverTimeoutUploads(ctx, timeout)
	if err != nil {
		return err
	}

	result.ProcessedRows = jobs
	result.Details = map[string]any{
		"recovered_jobs":    jobs,
		"recovered_uploads": uploads,
		"upload_timeout":    timeout.String(),
	}
	log.Printf("[exportarchive] %s 完成 recovered_jobs=%d recovered_uploads=%d upload_timeout=%s",
		JobRecoverTimeoutExportJobs, jobs, uploads, timeout)
	return nil
}
