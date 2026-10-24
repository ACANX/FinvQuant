package dedup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// jobOutcome 单任务执行结果。
type jobOutcome struct {
	Rows   int64
	Bytes  int64
	Paused bool
	Err    error
}

// runRunDueExportJobs 低峰窗口内生成候选并执行「LOAD → EXPORT → CLEANUP」（F-01~F-08）。
func (s *Service) runRunDueExportJobs(ctx context.Context, result *Result) error {
	if !s.options.InWindow(s.options.Now()) {
		result.Message = "当前不在导出低峰窗口内，跳过"
		return nil
	}

	budget, err := s.ensureBudget(ctx, s.options.Today())
	if err != nil {
		return err
	}

	created, err := s.store.InsertCandidateJobs(ctx, s.options.CurrentMonth(), s.options.FetchLimit)
	if err != nil {
		return err
	}

	var processed, paused, failed int
	for processed < s.options.MaxJobsPerRun {
		if err := ctx.Err(); err != nil {
			break
		}
		if !s.options.InWindow(s.options.Now()) {
			break
		}
		if budget.RemainingRows() <= 0 {
			break
		}

		job, claimErr := s.store.ClaimJob(ctx, s.options.WorkerID, s.options.Lease)
		if claimErr != nil {
			return claimErr
		}
		if job == nil {
			break
		}

		outcome := s.processJob(ctx, job)
		if outcome.Rows > 0 {
			if err := s.store.AddBudgetConsumed(ctx, s.options.Today(), outcome.Rows, outcome.Bytes); err != nil {
				return err
			}
			budget.ConsumedRows += outcome.Rows
			budget.ConsumedBytes += outcome.Bytes
			result.ProcessedRows += outcome.Rows
			result.ProcessedBytes += outcome.Bytes
		}
		processed++
		if outcome.Paused {
			paused++
		}
		if outcome.Err != nil {
			failed++
		}
	}

	status := "RUNNING"
	switch {
	case budget.ConsumedRows <= 0:
		status = "CALCULATED"
	case budget.ConsumedRows >= budget.BudgetRows:
		status = "DONE"
	}
	if err := s.store.SetBudgetStatus(ctx, s.options.Today(), status); err != nil {
		return err
	}

	result.Details = map[string]any{
		"candidates_created": created,
		"processed_jobs":     processed,
		"paused_jobs":        paused,
		"failed_jobs":        failed,
		"budget_rows":        budget.BudgetRows,
		"consumed_rows":      budget.ConsumedRows,
		"remaining_rows":     budget.RemainingRows(),
	}
	log.Printf("[exportarchive] %s 完成 candidates=%d processed=%d paused=%d failed=%d budget_rows=%d consumed_rows=%d",
		JobRunDueExportJobs, created, processed, paused, failed, budget.BudgetRows, budget.ConsumedRows)
	return nil
}

// processJob 执行单个证券月任务：LOAD（可暂停续跑）→ EXPORT → CLEANUP。
func (s *Service) processJob(ctx context.Context, job *Job) jobOutcome {
	outcome := jobOutcome{}
	runID, err := s.store.StartRun(ctx, JobRunDueExportJobs, &job.JobID, job.Usc, &job.BizMonth)
	if err != nil {
		outcome.Err = err
		return outcome
	}
	finishRun := func(status, errText string) {
		if finishErr := s.store.FinishRun(context.WithoutCancel(ctx), runID, status,
			outcome.Rows, outcome.Bytes, errText); finishErr != nil {
			log.Printf("[exportarchive] 更新运行日志失败 job_id=%d err=%v", job.JobID, finishErr)
		}
	}

	info, err := s.store.UscInfo(ctx, job.Usc)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}
	if !info.Complete() {
		err := fmt.Errorf("证券 %s 元数据不完整（region/market/provider/name/timezone/currency 必须齐备）", job.Usc)
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}

	// E-07：续跑前校验 loaded_rows 与中转表实际行数一致；不一致则重置游标全量重装（幂等）。
	from, to := MonthRange(job.BizMonth)
	stagingCount, err := s.store.CountStaging(ctx, job.Usc, from, to)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}
	if stagingCount != job.LoadedRows {
		log.Printf("[exportarchive] 游标校验不一致 job_id=%d usc=%s biz_month=%d staging=%d loaded=%d，重置游标重装",
			job.JobID, job.Usc, job.BizMonth, stagingCount, job.LoadedRows)
		if err := s.store.ResetLoadCursor(ctx, job.JobID); err != nil {
			s.failJob(ctx, job, &outcome, err)
			finishRun(RunStatusFailed, err.Error())
			return outcome
		}
		job.LoadCursorDate = nil
		job.LoadCursorTS = nil
		job.LoadedRows = 0
	}

	// LOAD
	paused, err := s.loadJob(ctx, job, &outcome)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}
	if paused {
		outcome.Paused = true
		if err := s.store.RequeueJob(context.WithoutCancel(ctx), job.JobID, JobStatusPaused, "", nil); err != nil {
			log.Printf("[exportarchive] 暂停任务失败 job_id=%d err=%v", job.JobID, err)
		}
		finishRun(RunStatusPaused, "预算或窗口耗尽，已保存游标")
		log.Printf("[exportarchive] 任务暂停 job_id=%d usc=%s biz_month=%d loaded_rows=%d", job.JobID, job.Usc, job.BizMonth, job.LoadedRows)
		return outcome
	}

	// EXPORT + CLEANUP
	exportedRows, err := s.exportAndCleanup(ctx, job, info, &outcome)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}

	finishRun(RunStatusSuccess, "")
	log.Printf("[exportarchive] 任务完成 job_id=%d usc=%s biz_month=%d rows=%d bytes=%d",
		job.JobID, job.Usc, job.BizMonth, exportedRows, outcome.Bytes)
	return outcome
}

// loadJob 阶段一：按 date 逐日、按 ts keyset 分页把主表数据装入中转表（F-02 / F-03）。
//
// 预算只在「是否启动新任务」时判断（H-05），已启动任务允许在窗口内做完，避免半途暂停。
func (s *Service) loadJob(ctx context.Context, job *Job, outcome *jobOutcome) (bool, error) {
	_, last := MonthFirstLastDay(job.BizMonth)
	firstDate := job.BizMonth*100 + 1
	lastDate := job.BizMonth*100 + last.Day()

	cursorDate := firstDate
	cursorTS := int64(0)
	if job.LoadCursorDate != nil {
		cursorDate = *job.LoadCursorDate
		if job.LoadCursorTS != nil {
			cursorTS = *job.LoadCursorTS
		}
	}
	loaded := job.LoadedRows

	for date := cursorDate; date <= lastDate; date = nextDate(date) {
		if date != cursorDate {
			cursorTS = 0
		}
		paused, err := s.loadDate(ctx, job, date, &cursorTS, &loaded, outcome)
		if err != nil {
			return false, err
		}
		if paused {
			return true, nil
		}
		if err := s.store.UpdateLoadProgress(ctx, job.JobID, date, 0, loaded, JobStatusLoading, s.options.Lease); err != nil {
			return false, err
		}
	}
	job.LoadedRows = loaded
	return false, nil
}

// loadDate 装载单个交易日：keyset 分页 + 每页一条语句（短事务）+ 行数基线校验（M-01）。
func (s *Service) loadDate(ctx context.Context, job *Job, date int, cursorTS *int64, loaded *int64, outcome *jobOutcome) (bool, error) {
	baseline, err := s.store.CountMainDate(ctx, job.Usc, date)
	if err != nil {
		return false, err
	}

	var stagingCount, mainCount int64
	// 最多尝试两次：第一次装载后若中转行数少于主表（装载期间有新数据到达），重跑一次该日。
	for attempt := 0; attempt < 2; attempt++ {
		for {
			if !s.options.InWindow(s.options.Now()) {
				return true, nil
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}

			page, err := s.store.LoadPage(ctx, job.Usc, date, *cursorTS, s.options.PageSize)
			if err != nil {
				return false, err
			}
			*loaded += page.InsertedRows
			outcome.Rows += page.InsertedRows
			*cursorTS = page.MaxTS
			if err := s.store.UpdateLoadProgress(ctx, job.JobID, date, *cursorTS, *loaded,
				JobStatusLoading, s.options.Lease); err != nil {
				return false, err
			}
			if page.PageRows == 0 || page.PageRows < int64(s.options.PageSize) {
				break
			}
		}

		if stagingCount, err = s.store.CountStagingDate(ctx, job.Usc, date); err != nil {
			return false, err
		}
		if mainCount, err = s.store.CountMainDate(ctx, job.Usc, date); err != nil {
			return false, err
		}
		if stagingCount >= mainCount {
			break
		}
		log.Printf("[exportarchive] 交易日行数不足，重装 job_id=%d usc=%s date=%d staging=%d main=%d",
			job.JobID, job.Usc, date, stagingCount, mainCount)
		*cursorTS = 0
	}

	if stagingCount < mainCount {
		return false, fmt.Errorf("交易日 %d 装载行数不足（中转 %d < 主表 %d）", date, stagingCount, mainCount)
	}
	if baseline != mainCount {
		log.Printf("[exportarchive] 装载期间主表行数变化 job_id=%d usc=%s date=%d baseline=%d after=%d",
			job.JobID, job.Usc, date, baseline, mainCount)
	}
	return false, nil
}

// exportAndCleanup 阶段二、三：中转表 → MVsv 文件 → 登记 → 清理中转数据（F-04~F-08 / E-06）。
func (s *Service) exportAndCleanup(ctx context.Context, job *Job, info *UscInfo, outcome *jobOutcome) (int64, error) {
	from, to := MonthRange(job.BizMonth)
	rowCount, err := s.store.CountStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if rowCount != job.LoadedRows {
		log.Printf("[exportarchive] 导出前行数校验不一致 job_id=%d loaded_rows=%d staging=%d", job.JobID, job.LoadedRows, rowCount)
	}

	meta := info.Meta(job.BizMonth)
	folder := s.jobFolder(meta)
	basePath := filepath.Join(folder, meta.BaseFileName())
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	version := job.BaseVersion + 1
	sealDate := job.SealDate

	write, err := WriteMVsv(basePath, meta, s.options.Compression, rowCount,
		s.stagingSource(ctx, job.Usc, from, to))
	if err != nil {
		return 0, err
	}
	if err := VerifyMVsv(basePath, s.options.Compression, rowCount); err != nil {
		return 0, fmt.Errorf("MVsv 结构校验失败: %w", err)
	}

	target := s.options.TargetPath(meta.BaseFileName())
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

	if _, err := s.rebuildManifest(ctx, job, meta, manifestPath); err != nil {
		return 0, err
	}

	// 阶段三：CLEANUP
	deleted, err := s.store.CleanupStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if truncated, err := s.store.TruncateStagingIfEmpty(ctx); err != nil {
		log.Printf("[exportarchive] TRUNCATE 中转表失败 err=%v", err)
	} else if truncated {
		log.Printf("[exportarchive] 中转表已清空，执行 TRUNCATE")
	}

	if err := s.store.MarkJobExported(ctx, job.JobID, rowCount, version); err != nil {
		return 0, err
	}
	job.LoadedRows = rowCount
	job.BaseVersion = version
	job.Status = JobStatusExported

	outcome.Bytes += write.Size
	log.Printf("[exportarchive] 导出完成 job_id=%d file=%s rows=%d size=%d checksum=%s cleanup_rows=%d",
		job.JobID, meta.BaseFileName(), write.RowCount, write.Size, write.Checksum, deleted)
	return rowCount, nil
}

// stagingSource 构造中转表数据行来源（按 ts 排序流式读取）。
func (s *Service) stagingSource(ctx context.Context, usc string, from, to int) RowSource {
	return func(yield func(cols [12]string) error) error {
		rows, err := s.store.QueryStagingRows(ctx, usc, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			cols, err := scanColumns(rows)
			if err != nil {
				return err
			}
			if err := yield(cols); err != nil {
				return err
			}
		}
		return rows.Err()
	}
}

// mainSource 构造主表数据行来源（L1_HOT 重写 base / 生成补丁）。
func (s *Service) mainSource(ctx context.Context, usc string, from, to int, since *time.Time) RowSource {
	return func(yield func(cols [12]string) error) error {
		rows, err := s.store.QueryMainRows(ctx, usc, from, to, since)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			cols, err := scanColumns(rows)
			if err != nil {
				return err
			}
			if err := yield(cols); err != nil {
				return err
			}
		}
		return rows.Err()
	}
}

// scanColumns 把一行（ts|date|time|9 个数值文本）拼成 MVsv 12 列。
func scanColumns(rows pgx.Rows) ([12]string, error) {
	var ts int64
	var date, clock *int
	var open, high, low, close, volume, turnover *string
	var changePrice, changeRatio, lastClose *string

	if err := rows.Scan(&ts, &date, &clock, &open, &high, &low, &close,
		&volume, &turnover, &changePrice, &changeRatio, &lastClose); err != nil {
		return [12]string{}, err
	}
	return [12]string{
		strconv.FormatInt(ts, 10),
		formatIntPtr(date),
		formatIntPtr(clock),
		trimNumeric(open),
		trimNumeric(high),
		trimNumeric(low),
		trimNumeric(close),
		trimNumeric(volume),
		trimNumeric(turnover),
		trimNumeric(changePrice),
		trimNumeric(changeRatio),
		trimNumeric(lastClose),
	}, nil
}

func formatIntPtr(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

// rebuildManifest 按当前生效的 BASE + PATCH 重新生成 Manifest 并登记（K-08）。
func (s *Service) rebuildManifest(ctx context.Context, job *Job, meta FileMeta, manifestPath string) (WriteResult, error) {
	files, err := s.store.BaseAndPatchFiles(ctx, job.JobID)
	if err != nil {
		return WriteResult{}, err
	}
	var base *File
	patches := make([]ManifestFile, 0, len(files))
	for index := range files {
		file := files[index]
		if file.FileType == FileTypeBase {
			base = &files[index]
			continue
		}
		patches = append(patches, ManifestFile{
			FileName: file.FileName,
			RowCount: file.RowCount,
			FileSize: file.FileSize,
			Checksum: file.Checksum,
			Version:  file.Version,
			Reason:   PatchReasonLateArrival,
		})
	}
	if base == nil {
		return WriteResult{}, errors.New("缺少生效的 BASE 文件，无法生成 Manifest")
	}

	manifest := BuildManifest(meta, job.SealDate, s.options.Compression, ManifestFile{
		FileName: base.FileName,
		RowCount: base.RowCount,
		FileSize: base.FileSize,
		Checksum: base.Checksum,
		Version:  base.Version,
	}, patches, s.options.Now())

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return WriteResult{}, fmt.Errorf("序列化 Manifest 失败: %w", err)
	}
	write, err := WriteFileAtomic(manifestPath, append(data, '\n'))
	if err != nil {
		return WriteResult{}, err
	}

	version, err := s.store.NextManifestVersion(ctx, job.Usc, job.BizMonth)
	if err != nil {
		return WriteResult{}, err
	}
	target := s.options.TargetPath(meta.ManifestFileName())
	sealDate := job.SealDate
	if _, err := s.store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            job.Usc,
		BizMonth:       job.BizMonth,
		FileType:       FileTypeManifest,
		Version:        version,
		FileName:       meta.ManifestFileName(),
		FilePath:       write.Path,
		TargetFileName: meta.ManifestFileName(),
		UploadTarget:   &target,
		Format:         MvsvFormat,
		Compression:    CompressionNone,
		RowCount:       0,
		FileSize:       write.Size,
		Checksum:       write.Checksum,
		SealDate:       &sealDate,
		ExportStatus:   ExportStatusExported,
		UploadStatus:   UploadStatusPending,
	}); err != nil {
		return WriteResult{}, err
	}
	return write, nil
}

// jobFolder 返回证券月的本地导出目录：{OutputDir}/{usc}/{yyyyMM}/。
func (s *Service) jobFolder(meta FileMeta) string {
	return filepath.Join(s.options.OutputDir, sanitizeFilePart(meta.Usc), strconv.Itoa(meta.BizMonth))
}

// failJob 失败退避：指数退避 + 上限 1 小时，保留 last_error 供排查与告警。
func (s *Service) failJob(ctx context.Context, job *Job, outcome *jobOutcome, cause error) {
	outcome.Err = cause
	nextRun := s.options.Now().Add(backoffFor(job.Attempt))
	if err := s.store.RequeueJob(context.WithoutCancel(ctx), job.JobID, JobStatusFailed,
		cause.Error(), &nextRun); err != nil {
		log.Printf("[exportarchive] 记录任务失败状态失败 job_id=%d err=%v", job.JobID, err)
	}
	log.Printf("[exportarchive] 任务失败 job_id=%d usc=%s biz_month=%d attempt=%d next_run=%s err=%v",
		job.JobID, job.Usc, job.BizMonth, job.Attempt, nextRun.Format(time.RFC3339), cause)
}

// backoffFor 指数退避：1、2、4、8 … 分钟，上限 60 分钟。
func backoffFor(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	minutes := math.Pow(2, float64(attempt-1))
	if minutes > 60 {
		minutes = 60
	}
	return time.Duration(minutes * float64(time.Minute))
}

// nextDate 返回 yyyyMMdd 的下一天（自动处理月末与闰年）。
func nextDate(date int) int {
	year, month, day := date/10000, date/100%100, date%100
	next := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	return next.Year()*10000 + int(next.Month())*100 + next.Day()
}
