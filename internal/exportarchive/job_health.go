package exportarchive

import (
	"context"
	"log"
	"os"
	"time"
)

// healthVerifyLimit 健康检查单轮 MVsv 结构抽检的文件数。
const healthVerifyLimit = 20

// runCheckExportJobHealth 汇总指标、抽检 MVsv 结构并输出告警（M-06 / M-07 / 5.2 / 5.3）。
func (s *Service) runCheckExportJobHealth(ctx context.Context, result *Result) error {
	metrics, err := s.buildMetrics(ctx)
	if err != nil {
		return err
	}

	// MVsv 结构抽检：第 16 行文件头、第 17 行空行、# Field 行、# Count 与实际行数。
	files, err := s.store.RecentFilesForVerify(ctx, healthVerifyLimit)
	if err != nil {
		return err
	}
	var anomalies int64
	for _, file := range files {
		if _, statErr := os.Stat(file.FilePath); statErr != nil {
			continue
		}
		if verifyErr := VerifyMVsv(file.FilePath, file.Compression, file.RowCount); verifyErr != nil {
			anomalies++
			log.Printf("[exportarchive] ALERT MVsv 结构异常 file_id=%d file=%s err=%v",
				file.FileID, file.FileName, verifyErr)
		}
	}
	metrics.MvsvAnomalies = anomalies
	metrics.Alerts = BuildAlerts(*metrics, s.options)
	for _, alert := range metrics.Alerts {
		log.Printf("[exportarchive] ALERT code=%s level=%s message=%s", alert.Code, alert.Level, alert.Message)
	}

	result.ProcessedRows = metrics.PendingRows
	result.Details = metrics
	log.Printf("[exportarchive] %s 完成 pending_rows=%d backlog_days=%.2f budget_factor=%.2f staging_rows=%d alerts=%d",
		JobCheckExportJobHealth, metrics.PendingRows, metrics.BacklogDays,
		metrics.BudgetFactor, metrics.StagingRows, len(metrics.Alerts))
	return nil
}

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
