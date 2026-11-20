package job

import (
	"context"
	"log"
	"os"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

const healthVerifyLimit = 20

func init() {
	cdeu.RegisterJobExecutor(cdeu.JobCheckExportJobHealth, func(ctx context.Context, svc *cdeu.Service, result *cdeu.Result) error {
		svc.ReportJobProgress(ctx, "HEALTH", 0, "汇总指标与告警")
		metrics, err := svc.BuildArchiveMetrics(ctx)
		if err != nil {
			return err
		}
		svc.ReportJobProgress(ctx, "HEALTH", 40, "抽检 MVsv 结构")
		files, err := svc.Store().RecentFilesForVerify(ctx, healthVerifyLimit)
		if err != nil {
			return err
		}
		var anomalies int64
		for _, file := range files {
			if _, statErr := os.Stat(file.FilePath); statErr != nil {
				continue
			}
			if verifyErr := cdeu.VerifyMVsv(file.FilePath, file.Compression, file.RowCount); verifyErr != nil {
				anomalies++
				log.Printf("[cdeu] ALERT MVsv 结构异常 file_id=%d file=%s err=%v",
					file.FileID, file.FileName, verifyErr)
			}
		}
		metrics.MvsvAnomalies = anomalies
		metrics.Alerts = cdeu.BuildAlerts(*metrics, svc.Options())
		for _, alert := range metrics.Alerts {
			log.Printf("[cdeu] ALERT code=%s level=%s message=%s", alert.Code, alert.Level, alert.Message)
		}
		result.ProcessedRows = metrics.PendingRows
		result.Details = metrics
		log.Printf("[cdeu] %s 完成 pending_rows=%d backlog_days=%.2f budget_factor=%.2f staging_rows=%d alerts=%d",
			cdeu.JobCheckExportJobHealth, metrics.PendingRows, metrics.BacklogDays,
			metrics.BudgetFactor, metrics.StagingRows, len(metrics.Alerts))
		return nil
	})
}
