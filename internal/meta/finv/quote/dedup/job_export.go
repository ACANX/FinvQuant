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

	joblog "github.com/acanx/finvquant/internal/meta/finv/quote/dedup/jobmeta"
)

// JobOutcome 单任务执行结果。
type JobOutcome struct {
	Rows     int64
	Bytes    int64
	Paused   bool
	Err      error
	RunLogID int64
}

// runRunDueExportJobs 低峰窗口内生成候选并执行「LOAD → EXPORT → CLEANUP」（F-01~F-08）。
// dynamicJobLabels 读取动态导出 JOB 的编码/名称，用于详细日志定位。
func (s *Service) dynamicJobLabels(ctx context.Context, usc string, bizMonth int) (string, string) {
	var code, name string
	_ = s.store.Pool().QueryRow(ctx, `
SELECT job_code, name
  FROM cdeu.finv_quote_cdeu_job_dynamic
 WHERE usc = $1 AND biz_month = $2 AND job_category = 'EXPORT'
 ORDER BY dynamic_job_id DESC
 LIMIT 1`, usc, bizMonth).Scan(&code, &name)
	if code == "" {
		code = fmt.Sprintf("JOB_EXPORT_%s_%d", usc, bizMonth)
	}
	if name == "" {
		name = code
	}
	return code, name
}

// processJob 执行单个证券月任务：LOAD（可暂停续跑）→ EXPORT → CLEANUP。
func (s *Service) processJob(ctx context.Context, job *Job) JobOutcome {
	runID, err := s.store.StartRun(ctx, JobRunDueExportJobs, &job.JobID, job.Usc, &job.BizMonth)
	if err != nil {
		return JobOutcome{Err: err}
	}
	return s.processJobWithRunID(ctx, job, runID)
}

// processJobWithRunID 在已登记的运行日志上执行单个证券月任务。
func (s *Service) processJobWithRunID(ctx context.Context, job *Job, runID int64) JobOutcome {
	outcome := JobOutcome{RunLogID: runID}
	jobCode, jobName := s.dynamicJobLabels(ctx, job.Usc, job.BizMonth)
	jobCtxForLog := withRunID(ctx, runID)
	finishRun := func(status, errText string) {
		if finishErr := s.store.FinishRun(context.WithoutCancel(ctx), runID, status,
			outcome.Rows, outcome.Bytes, errText); finishErr != nil {
			log.Printf("[cdeu] 更新运行日志失败 job_id=%d err=%v", job.JobID, finishErr)
		}
	}
	// 本任务自己的运行日志是前端观察「当前证券月跑到哪一阶段」的主依据。
	jobCtx := jobCtxForLog
	loadStep := joblog.StepContext{JobCode: jobCode, JobName: jobName, StepCode: "STEP_LOAD",
		StepName: "装载", Usc: job.Usc, BizMonth: job.BizMonth}
	s.reportRunProgress(jobCtx, runID, "LOAD", 0, joblog.StepStart(loadStep))
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_LOAD", CDEUStatusRunning, "")
	s.advanceCDEUStage(jobCtx, "LOAD")

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
		log.Printf("[cdeu] 游标校验不一致 job_id=%d usc=%s biz_month=%d staging=%d loaded=%d，重置游标重装",
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
	paused, err := s.loadJob(jobCtx, job, &outcome)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}
	if paused {
		outcome.Paused = true
		if err := s.store.RequeueJob(context.WithoutCancel(ctx), job.JobID, JobStatusPaused, "", nil); err != nil {
			log.Printf("[cdeu] 暂停任务失败 job_id=%d err=%v", job.JobID, err)
		}
		finishRun(RunStatusPaused, "预算或窗口耗尽，已保存游标")
		log.Printf("[cdeu] 任务暂停 job_id=%d usc=%s biz_month=%d loaded_rows=%d", job.JobID, job.Usc, job.BizMonth, job.LoadedRows)
		return outcome
	}

	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_LOAD", CDEUStatusSuccess, "")
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_EXPORT", CDEUStatusRunning, "")
	s.reportRunProgress(jobCtx, runID, "LOAD", 100,
		joblog.StepEnd(loadStep, fmt.Sprintf("已装载 %d 行", job.LoadedRows)))

	// EXPORT + CLEANUP
	s.advanceCDEUStage(jobCtx, "EXPORT")
	exportStep := joblog.StepContext{JobCode: jobCode, JobName: jobName, StepCode: "STEP_EXPORT",
		StepName: "文件生成", Usc: job.Usc, BizMonth: job.BizMonth}
	s.reportRunProgress(jobCtx, runID, "EXPORT", 60, joblog.StepStart(exportStep))
	exportedRows, err := s.exportAndCleanup(jobCtx, job, info, &outcome)
	if err != nil {
		s.failJob(ctx, job, &outcome, err)
		finishRun(RunStatusFailed, err.Error())
		return outcome
	}

	finishRun(RunStatusSuccess, "")
	log.Printf("[cdeu] 任务完成 job_id=%d usc=%s biz_month=%d rows=%d bytes=%d",
		job.JobID, job.Usc, job.BizMonth, exportedRows, outcome.Bytes)
	return outcome
}

// dynamicExportTarget 一个已抢占、已登记运行日志的动态导出 JOB。
type dynamicExportTarget struct {
	dynamicJobID  int64
	processID     int64
	workflowRunID int64
	usc           string
	bizMonth      int
	job           *Job
	runID         int64
}

// RunDynamicExportJob 异步执行一个单证券动态导出 JOB，并立即返回运行日志 ID。
//
// autoContinue=true 时，完成后先续跑同 Process 中排在当前 JOB 之后的动态导出 JOB，
// 再继续执行导出阶段之后的静态 JOB。
func (s *Service) RunDynamicExportJob(ctx context.Context, dynamicJobID int64, autoContinue bool) (int64, error) {
	target, err := s.startDynamicExportJob(ctx, dynamicJobID)
	if err != nil {
		return 0, err
	}
	go func() {
		paused, _ := s.executeDynamicExportJob(context.Background(), target)
		if autoContinue {
			if !paused {
				s.runSubsequentDynamicExportJobs(context.Background(), target)
			}
			s.runSubsequentJobs(context.Background(), JobRunDueExportJobs)
		}
	}()
	return target.runID, nil
}

// startDynamicExportJob 校验并抢占动态 JOB，登记运行日志后返回执行目标。
func (s *Service) startDynamicExportJob(ctx context.Context, dynamicJobID int64) (dynamicExportTarget, error) {
	var target dynamicExportTarget
	var exportJobID int64
	var category, status string
	target.dynamicJobID = dynamicJobID
	if err := s.store.Pool().QueryRow(ctx, `
SELECT COALESCE(export_job_id, 0), process_id, usc, biz_month, job_category, status,
       COALESCE(workflow_run_id, 0)
  FROM cdeu.finv_quote_cdeu_job_dynamic
 WHERE dynamic_job_id = $1`, dynamicJobID).Scan(&exportJobID, &target.processID, &target.usc,
		&target.bizMonth, &category, &status, &target.workflowRunID); err != nil {
		return target, fmt.Errorf("动态 JOB 不存在: %w", err)
	}
	if category != "EXPORT" {
		return target, fmt.Errorf("当前只支持手动执行导出动态 JOB")
	}
	if exportJobID <= 0 {
		return target, fmt.Errorf("动态 JOB 未关联导出任务")
	}
	if !isDynamicManualStatus(status) {
		return target, fmt.Errorf("只有待执行 / 失败 / 暂停 / 跳过的动态 JOB 可以手动执行")
	}
	job, err := s.store.ClaimJobByID(ctx, exportJobID, s.options.WorkerID, s.options.Lease)
	if err != nil {
		return target, err
	}
	if job == nil {
		return target, fmt.Errorf("导出任务正在执行或已完成，无法重复执行")
	}
	runID, err := s.store.StartRun(ctx, JobRunDueExportJobs, &job.JobID, job.Usc, &job.BizMonth)
	if err != nil {
		_ = s.store.RequeueJob(context.WithoutCancel(ctx), job.JobID, JobStatusWaiting, "登记运行日志失败", nil)
		return target, fmt.Errorf("登记运行日志失败: %w", err)
	}
	target.job = job
	target.runID = runID
	_ = s.store.MarkDynamicJobStatus(ctx, target.usc, target.bizMonth, CDEUStatusRunning, "", runID)
	return target, nil
}

// executeDynamicExportJob 执行已抢占的动态 JOB，并同步动态 JOB / Step 状态。
func (s *Service) executeDynamicExportJob(ctx context.Context, target dynamicExportTarget) (bool, error) {
	outcome := s.processJobWithRunID(ctx, target.job, target.runID)
	if outcome.Err != nil {
		_ = s.store.MarkDynamicJobStatus(ctx, target.usc, target.bizMonth, CDEUStatusFailed, outcome.Err.Error(), outcome.RunLogID)
		_ = s.store.FailDynamicJobSteps(ctx, target.usc, target.bizMonth, outcome.Err.Error())
		return false, outcome.Err
	}
	if outcome.Paused {
		_ = s.store.MarkDynamicJobStatus(ctx, target.usc, target.bizMonth, CDEUStatusPaused, "", outcome.RunLogID)
		_ = s.store.MarkDynamicJobStepStatus(ctx, target.usc, target.bizMonth, "STEP_LOAD", CDEUStatusPaused, "")
		return true, nil
	}
	_ = s.store.MarkDynamicJobStatus(ctx, target.usc, target.bizMonth, CDEUStatusSuccess, "", outcome.RunLogID)
	_ = s.store.CompleteDynamicJobSteps(ctx, target.usc, target.bizMonth)
	return false, nil
}

// runSubsequentDynamicExportJobs 续跑同一 Process 中排在当前动态 JOB 之后的待执行动态导出 JOB。
func (s *Service) runSubsequentDynamicExportJobs(ctx context.Context, from dynamicExportTarget) {
	rows, err := s.store.Pool().Query(ctx, `
SELECT dynamic_job_id
  FROM cdeu.finv_quote_cdeu_job_dynamic
 WHERE process_id = $1 AND job_category = 'EXPORT'
   AND status IN ('PENDING','FAILED','PAUSED','SKIPPED')
   AND dynamic_job_id > $2
   AND workflow_run_id IS NOT DISTINCT FROM NULLIF($3::bigint, 0)
 ORDER BY dynamic_job_id`, from.processID, from.dynamicJobID, from.workflowRunID)
	if err != nil {
		log.Printf("[cdeu] 读取后续动态 JOB 失败 process_id=%d err=%v", from.processID, err)
		return
	}
	ids := make([]int64, 0, 16)
	for rows.Next() {
		var dynamicJobID int64
		if scanErr := rows.Scan(&dynamicJobID); scanErr != nil {
			rows.Close()
			log.Printf("[cdeu] 读取后续动态 JOB 失败 process_id=%d err=%v", from.processID, scanErr)
			return
		}
		ids = append(ids, dynamicJobID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Printf("[cdeu] 读取后续动态 JOB 失败 process_id=%d err=%v", from.processID, err)
		return
	}
	for _, dynamicJobID := range ids {
		if ctx.Err() != nil {
			return
		}
		target, startErr := s.startDynamicExportJob(ctx, dynamicJobID)
		if startErr != nil {
			log.Printf("[cdeu] 续跑动态 JOB 启动失败 dynamic_job_id=%d err=%v", dynamicJobID, startErr)
			continue
		}
		log.Printf("[cdeu] 手动续跑动态 JOB dynamic_job_id=%d usc=%s biz_month=%d run_id=%d",
			dynamicJobID, target.usc, target.bizMonth, target.runID)
		paused, runErr := s.executeDynamicExportJob(ctx, target)
		if runErr != nil {
			log.Printf("[cdeu] 续跑动态 JOB 失败 dynamic_job_id=%d err=%v", dynamicJobID, runErr)
		}
		if paused {
			return
		}
	}
}

func isDynamicManualStatus(status string) bool {
	switch status {
	case CDEUStatusPending, CDEUStatusFailed, CDEUStatusPaused, CDEUStatusSkipped:
		return true
	default:
		return false
	}
}

// loadJob 阶段一：按 date 逐日、按 ts keyset 分页把主表数据装入中转表（F-02 / F-03）。
//
// 预算只在「是否启动新任务」时判断（H-05），已启动任务允许在窗口内做完，避免半途暂停。
func (s *Service) loadJob(ctx context.Context, job *Job, outcome *JobOutcome) (bool, error) {
	_, last := MonthFirstLastDay(job.BizMonth)
	firstDate := job.BizMonth*100 + 1
	lastDate := job.BizMonth*100 + last.Day()

	cursorDate := firstDate
	cursorTS := int64(0)
	// 游标必须落在本月合法区间内才采纳：
	//   历史缺陷：load_cursor_date 为 0（非 NULL）时被当成有效游标，
	//   nextDate(0) 计算 time.Date(0,0,0) 得到负日期并逐次下探，
	//   最终 CountMainDate 绑定 int4 参数时溢出（-2147539797 < int4 min）。
	//   因此只要游标不在 [firstDate, lastDate] 内，一律从当月 1 日重新开始。
	if job.LoadCursorDate != nil && *job.LoadCursorDate >= firstDate && *job.LoadCursorDate <= lastDate {
		cursorDate = *job.LoadCursorDate
		if job.LoadCursorTS != nil && *job.LoadCursorTS >= 0 {
			cursorTS = *job.LoadCursorTS
		}
	}
	loaded := job.LoadedRows

	// 进度停滞保护（需求 10736）：
	// 原缺陷：游标异常时外层 for 会空转，而每轮 UpdateLoadProgress 都刷新
	// gmt_heartbeat / locked_until，导致 RECOVER_TIMEOUT_EXPORT_JOBS 的
	// 「locked_until < now()」永不成立——**循环活着但什么也没干，却永远不超时**。
	//
	// 两道闸：
	//   ① nextDate 必须严格递增，否则游标已经不可能覆盖本月，立即报错；
	//   ② 总轮数上限取 62（正常一个月最多 31 天，留一倍余量），超出即视为异常。
	const maxLoadDays = 62
	dayCount := 0
	for date := cursorDate; date <= lastDate; date = nextDate(date) {
		dayCount++
		if dayCount > maxLoadDays {
			return false, fmt.Errorf("装载游标异常：已循环 %d 轮仍未覆盖 %d（当前 date=%d），疑似游标不前进",
				dayCount, lastDate, date)
		}
		next := nextDate(date)
		if next <= date {
			return false, fmt.Errorf("装载游标不前进：date=%d nextDate=%d（lastDate=%d）", date, next, lastDate)
		}
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
func (s *Service) loadDate(ctx context.Context, job *Job, date int, cursorTS *int64, loaded *int64, outcome *JobOutcome) (bool, error) {
	baseline, err := s.store.CountMainDate(ctx, job.Usc, date)
	if err != nil {
		return false, err
	}

	var stagingCount, mainCount int64
	// 最多尝试两次：第一次装载后若中转行数少于主表（装载期间有新数据到达），重跑一次该日。
	// 手动触发不检查窗口：整月分页装完为止（窗口只约束调度触发）。
	manual := IsManualTriggerFromContext(ctx)
	for attempt := 0; attempt < 2; attempt++ {
		for {
			if !manual && !s.InWindow(ctx, s.options.Now()) {
				return true, nil
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}

			prevTS := *cursorTS
			page, err := s.store.LoadPage(ctx, job.Usc, date, *cursorTS, s.options.PageSize)
			if err != nil {
				return false, err
			}
			*loaded += page.InsertedRows
			outcome.Rows += page.InsertedRows
			*cursorTS = page.MaxTS
			// keyset 分页必须严格前进（需求 10736）：
			// 若 LoadPage 返回的 MaxTS 未推进而页仍满，会在此处无限循环，
			// 且每轮 UpdateLoadProgress 刷心跳、永远躲过超时回收。
			if page.PageRows > 0 && page.MaxTS <= prevTS {
				return false, fmt.Errorf("装载分页游标不前进：job_id=%d usc=%s date=%d prevTS=%d maxTS=%d rows=%d",
					job.JobID, job.Usc, date, prevTS, page.MaxTS, page.PageRows)
			}
			if err := s.store.UpdateLoadProgress(ctx, job.JobID, date, *cursorTS, *loaded,
				JobStatusLoading, s.options.Lease); err != nil {
				return false, err
			}
			s.reportProgress(ctx, "LOAD", -1,
				fmt.Sprintf("job_id=%d date=%d 已装载 %d 行", job.JobID, date, *loaded))
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
		log.Printf("[cdeu] 交易日行数不足，重装 job_id=%d usc=%s date=%d staging=%d main=%d",
			job.JobID, job.Usc, date, stagingCount, mainCount)
		*cursorTS = 0
	}

	if stagingCount < mainCount {
		return false, fmt.Errorf("交易日 %d 装载行数不足（中转 %d < 主表 %d）", date, stagingCount, mainCount)
	}
	if baseline != mainCount {
		log.Printf("[cdeu] 装载期间主表行数变化 job_id=%d usc=%s date=%d baseline=%d after=%d",
			job.JobID, job.Usc, date, baseline, mainCount)
	}
	return false, nil
}

// exportAndCleanup 阶段二、三：中转表 → MVsv 文件 → 登记 → 清理中转数据（F-04~F-08 / E-06）。
func (s *Service) exportAndCleanup(ctx context.Context, job *Job, info *UscInfo, outcome *JobOutcome) (int64, error) {
	jobCode, jobName := s.dynamicJobLabels(ctx, job.Usc, job.BizMonth)
	stepCtx := func(code, name string) joblog.StepContext {
		return joblog.StepContext{JobCode: jobCode, JobName: jobName, StepCode: code, StepName: name,
			Usc: job.Usc, BizMonth: job.BizMonth, StagingTable: tableStaging}
	}
	from, to := MonthRange(job.BizMonth)
	rowCount, err := s.store.CountStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if rowCount != job.LoadedRows {
		log.Printf("[cdeu] 导出前行数校验不一致 job_id=%d loaded_rows=%d staging=%d", job.JobID, job.LoadedRows, rowCount)
	}

	meta := info.Meta(job.BizMonth)
	folder := s.jobFolder(meta)
	basePath := filepath.Join(folder, meta.BaseFileName())
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	absoluteBasePath, _ := filepath.Abs(basePath)
	version := job.BaseVersion + 1
	sealDate := job.SealDate
	exportStep := stepCtx("STEP_EXPORT", "文件生成")

	s.reportProgress(ctx, "EXPORT", -1,
		fmt.Sprintf("STEP %s 写入开始 · job_code=%s job_name=%s step_code=STEP_EXPORT step_name=文件生成 · path=%s 预期 %d 行",
			exportStep.StepCode, jobCode, jobName, absoluteBasePath, rowCount))
	write, err := WriteMVsv(basePath, meta, s.options.Compression, rowCount,
		s.stagingSource(ctx, job.Usc, from, to))
	if err != nil {
		return 0, err
	}
	if err := VerifyMVsv(basePath, s.options.Compression, rowCount); err != nil {
		return 0, fmt.Errorf("MVsv 结构校验失败: %w", err)
	}
	s.reportProgress(ctx, "EXPORT", -1,
		joblog.ExportWritten(exportStep, absoluteBasePath, write.RowCount, write.Size, write.Checksum))
	s.reportProgress(ctx, "EXPORT", 100,
		joblog.StepEnd(exportStep, fmt.Sprintf("已生成 %d 行", write.RowCount)))
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_EXPORT", CDEUStatusSuccess, "")
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_REGISTER", CDEUStatusRunning, "")
	registerStep := stepCtx("STEP_REGISTER", "文件登记")
	s.reportProgress(ctx, "EXPORT", -1, joblog.StepStart(registerStep))

	target := s.options.ArchiveTargetPath(meta.Region, meta.Market, meta.Usc, meta.BaseFileName())
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
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_REGISTER", CDEUStatusSuccess, "")
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_CLEANUP", CDEUStatusRunning, "")
	s.reportProgress(ctx, "EXPORT", -1, joblog.StepEnd(registerStep, "BASE 文件与 Manifest 已登记"))

	// 阶段三：CLEANUP
	cleanupStep := stepCtx("STEP_CLEANUP", "中转清理")
	s.reportProgress(ctx, "CLEANUP", 90,
		joblog.CleanupStart(cleanupStep, job.Usc, job.BizMonth))
	deleted, err := s.store.CleanupStaging(ctx, job.Usc, from, to)
	if err != nil {
		return 0, err
	}
	if truncated, err := s.store.TruncateStagingIfEmpty(ctx); err != nil {
		log.Printf("[cdeu] TRUNCATE 中转表失败 err=%v", err)
	} else if truncated {
		log.Printf("[cdeu] 中转表已清空，执行 TRUNCATE")
	}

	if err := s.store.MarkJobExported(ctx, job.JobID, rowCount, version); err != nil {
		return 0, err
	}
	_ = s.store.MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_CLEANUP", CDEUStatusSuccess, "")
	s.reportProgress(ctx, "CLEANUP", 100,
		joblog.StepEnd(cleanupStep, fmt.Sprintf("已删除中转表 %s 中 %d 行", tableStaging, deleted)))
	job.LoadedRows = rowCount
	job.BaseVersion = version
	job.Status = JobStatusExported

	outcome.Bytes += write.Size
	log.Printf("[cdeu] 导出完成 job_id=%d file=%s rows=%d size=%d checksum=%s cleanup_rows=%d",
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
		written := 0
		for rows.Next() {
			cols, err := scanColumns(rows)
			if err != nil {
				return err
			}
			if err := yield(cols); err != nil {
				return err
			}
			written++
			// 写文件可能持续较久：按行数周期性刷新心跳，避免被误判为「卡住」。
			if written%20000 == 0 {
				s.reportProgress(ctx, "EXPORT", -1, fmt.Sprintf("已写入 %d 行", written))
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
	target := s.options.ArchiveTargetPath(meta.Region, meta.Market, meta.Usc, meta.ManifestFileName())
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
func (s *Service) failJob(ctx context.Context, job *Job, outcome *JobOutcome, cause error) {
	outcome.Err = cause
	nextRun := s.options.Now().Add(backoffFor(job.Attempt))
	if err := s.store.RequeueJob(context.WithoutCancel(ctx), job.JobID, JobStatusFailed,
		cause.Error(), &nextRun); err != nil {
		log.Printf("[cdeu] 记录任务失败状态失败 job_id=%d err=%v", job.JobID, err)
	}
	log.Printf("[cdeu] 任务失败 job_id=%d usc=%s biz_month=%d attempt=%d next_run=%s err=%v",
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
	// 防御：非法的年月日（如 0 或负数游标）不能进入 time.Date——它会把月/日归一化成
	// 负数日期并逐次放大。原值返回，交由调用方的区间校验兜底。
	if year < 1900 || month < 1 || month > 12 || day < 1 || day > 31 {
		return date
	}
	next := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	return next.Year()*10000 + int(next.Month())*100 + next.Day()
}
