package dedup

import (
	"context"
	"time"
)

// 本文件为独立 job 子包提供受控执行适配层，避免把 Service 的私有字段暴露出去。

func (s *Service) ReportJobProgress(ctx context.Context, stage string, percent float64, detail string) {
	s.reportProgress(ctx, stage, percent, detail)
}

func (s *Service) AdvanceJobCDEUStage(ctx context.Context, stage string) {
	s.advanceCDEUStage(ctx, stage)
}

func (s *Service) EnsureDailyBudget(ctx context.Context, today time.Time) (*Budget, error) {
	return s.ensureBudget(ctx, today)
}

func (s *Service) ProcessExportJob(ctx context.Context, job *Job) JobOutcome {
	return s.processJob(ctx, job)
}

func (s *Service) ProcessPatch(ctx context.Context, candidate PatchCandidate, rowsLeft int64) (PatchOutcome, error) {
	return s.processPatchCandidate(ctx, candidate, rowsLeft)
}

func (s *Service) FinishUpload(ctx context.Context, file *File, targetPath string, uploaded int64,
	started time.Time, result *Result) error {
	return s.finishUploadSuccess(ctx, file, targetPath, uploaded, started, result)
}

func (s *Service) UploadOneFile(ctx context.Context, file *File, result *Result) error {
	return s.uploadFile(ctx, file, result)
}

func (s *Service) BuildArchiveMetrics(ctx context.Context) (*Metrics, error) {
	return s.buildMetrics(ctx)
}

func (s *Service) DynamicJobLabels(ctx context.Context, usc string, bizMonth int) (string, string) {
	return s.dynamicJobLabels(ctx, usc, bizMonth)
}

func StagingTableName() string { return tableStaging }

// FirstDayOfMonth 返回 t 所在月份首日的 yyyyMMdd。
func FirstDayOfMonth(t time.Time) int { return firstDayOfMonth(t) }

// MaxSealedBizMonth 推导当前可收口的最大业务月份。
func MaxSealedBizMonth(t time.Time) int { return maxSealedBizMonth(t) }

// AddMonths 对 yyyyMM 进行月份加减。
func AddMonths(bizMonth, delta int) int { return addMonths(bizMonth, delta) }

// RunLogIDFromContext 返回当前 Job 运行日志 ID。
func RunLogIDFromContext(ctx context.Context) int64 { return runIDFromContext(ctx) }

// WithRunID 为 Job 子运行写入运行日志 ID。
func WithRunID(ctx context.Context, runID int64) context.Context { return withRunID(ctx, runID) }
