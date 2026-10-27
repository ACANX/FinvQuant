package dedup

import (
	"context"
	"time"
)

// healthVerifyLimit 健康检查单轮 MVsv 结构抽检的文件数。
const healthVerifyLimit = 20

// runCheckExportJobHealth 汇总指标、抽检 MVsv 结构并输出告警（M-06 / M-07 / 5.2 / 5.3）。
// buildMetrics 汇总归档链路指标。
func (s *Service) buildMetrics(ctx context.Context) (*Metrics, error) {
	metrics := &Metrics{
		GeneratedAt:  s.options.Now().UTC().Format(time.RFC3339),
		JobStatus:    map[string]int64{},
		UploadStatus: map[string]int64{},
		Alerts:       []Alert{},
	}

	jobStatus, err := s.store.JobStatusCounts(ctx)
	if err != nil {
		return nil, err
	}
	metrics.JobStatus = jobStatus
	batchCounts, err := s.store.JobStatusCountsByMonth(ctx, s.options.CurrentMonth())
	if err != nil {
		return nil, err
	}
	metrics.JobProgress = SummarizeJobProgress(batchCounts)
	patchCounts, err := s.store.PatchStatusCountsByMonth(ctx, s.options.CurrentMonth())
	if err != nil {
		return nil, err
	}
	metrics.JobProgress = mergePatchProgress(metrics.JobProgress, patchCounts)

	uploadStatus, err := s.store.UploadStatusCounts(ctx)
	if err != nil {
		return nil, err
	}
	metrics.UploadStatus = uploadStatus

	if metrics.PendingRows, err = s.store.PendingRows(ctx); err != nil {
		return nil, err
	}
	if metrics.StagingRows, metrics.StagingUscs, err = s.store.StagingSummary(ctx); err != nil {
		return nil, err
	}

	today := s.options.Today()
	if budget, budgetErr := s.store.GetBudget(ctx, today); budgetErr != nil {
		return nil, budgetErr
	} else if budget != nil {
		metrics.BudgetFactor = budget.Factor
		metrics.BudgetRows = budget.BudgetRows
		metrics.BudgetConsumedRows = budget.ConsumedRows
		metrics.BacklogDays = budget.BacklogRatio
		if budget.BudgetRows > 0 {
			metrics.BudgetUsageRatio = float64(budget.ConsumedRows) / float64(budget.BudgetRows)
		}
	} else if baseline, baseErr := s.store.IngestStatsBaseline(ctx, today.AddDate(0, 0, -30), today); baseErr != nil {
		return nil, baseErr
	} else if baseline.BaseRows > 0 {
		metrics.BacklogDays = float64(metrics.PendingRows) / baseline.BaseRows
	}

	minMonth := addMonths(s.options.CurrentMonth(), -6)
	if metrics.FrozenMissingRows, metrics.FrozenExpectedRows, err = s.store.FrozenMissing(ctx, minMonth); err != nil {
		return nil, err
	}
	if metrics.FrozenExpectedRows > 0 {
		metrics.FrozenMissingRatio = float64(metrics.FrozenMissingRows) / float64(metrics.FrozenExpectedRows)
	}
	if metrics.HeartbeatTimeouts, err = s.store.HeartbeatTimeoutCount(ctx); err != nil {
		return nil, err
	}
	if metrics.UploadRetryExhausted, err = s.store.UploadRetryExhaustedCount(ctx, s.options.MaxAttempts); err != nil {
		return nil, err
	}
	if metrics.PatchLast7Days, metrics.HighFrequencyPatchGroups, err = s.store.PatchStats(ctx); err != nil {
		return nil, err
	}
	if metrics.LocalFilesPendingCleanup, err = s.store.LocalFilesPendingCleanup(ctx); err != nil {
		return nil, err
	}
	return metrics, nil
}

// SummarizeJobProgress 把任务状态分布汇总为批次进度（成功 / 失败 / 进行中 / 待执行 / 暂停 / 冻结）。
func SummarizeJobProgress(counts map[string]int64) JobProgress {
	progress := JobProgress{}
	for status, count := range counts {
		progress.Total += count
		switch status {
		case JobStatusExported:
			progress.Success += count
		case JobStatusFailed:
			progress.Failed += count
		case JobStatusLoading:
			progress.Running += count
			progress.Loading += count
		case JobStatusLoaded:
			progress.Running += count
			progress.Loaded += count
		case JobStatusExporting:
			progress.Running += count
			progress.Exporting += count
		case JobStatusWaiting:
			progress.Waiting += count
		case JobStatusPaused:
			progress.Paused += count
		case JobStatusFrozen:
			progress.Frozen += count
		}
	}
	return progress
}

// mergePatchProgress 把补丁任务状态合并进批次进度（成功 / 失败 / 运行中 / 待处理）。
func mergePatchProgress(progress JobProgress, counts map[string]int64) JobProgress {
	for status, count := range counts {
		progress.PatchTotal += count
		switch status {
		case PatchStatusDone:
			progress.PatchSuccess += count
		case PatchStatusFailed:
			progress.PatchFailed += count
		case PatchStatusRunning:
			progress.PatchRunning += count
		case PatchStatusPending:
			progress.PatchPending += count
		}
	}
	return progress
}

// BuildAlerts 按方案稿 5.3 的阈值生成告警项。
func BuildAlerts(metrics Metrics, options Options) []Alert {
	alerts := make([]Alert, 0, 8)
	appendAlert := func(code, level, message string) {
		alerts = append(alerts, Alert{Code: code, Level: level, Message: message})
	}
	if metrics.BacklogDays > 7 {
		appendAlert("BACKLOG_TOO_HIGH", "WARN", "积压天数超过 7 天")
	}
	if metrics.BudgetFactor > 1.8 {
		appendAlert("BUDGET_FACTOR_HIGH", "WARN", "预算因子超过 1.8，导出压力偏高")
	}
	if metrics.HeartbeatTimeouts > 0 {
		appendAlert("HEARTBEAT_TIMEOUT", "WARN", "存在心跳/租约超时的运行中任务")
	}
	if metrics.UploadRetryExhausted > 0 {
		appendAlert("UPLOAD_RETRY_EXHAUSTED", "ERROR", "存在上传重试耗尽的文件")
	}
	if metrics.HighFrequencyPatchGroups > 0 {
		appendAlert("PATCH_FREQUENCY_HIGH", "WARN", "同一证券月 7 天内补丁超过 3 次")
	}
	if metrics.StagingRows > 0 {
		appendAlert("STAGING_RESIDUAL", "INFO", "中转表存在残留数据，请确认是否有活跃任务")
	}
	if metrics.FrozenMissingRatio > 0.05 {
		appendAlert("FROZEN_MISSING_HIGH", "WARN", "FROZEN 证券月缺失比例超过 5%")
	}
	if metrics.MvsvAnomalies > 0 {
		appendAlert("MVSV_STRUCTURE_INVALID", "ERROR", "抽检发现 MVsv 文件头/空行/Count 异常")
	}
	_ = options
	return alerts
}
