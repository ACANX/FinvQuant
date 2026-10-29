package job

import (
	"context"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	dedup.RegisterJobExecutor(dedup.JobCalcDailyExportBudget, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "BUDGET", 0, "计算当日导出预算")
		options := svc.Options()
		today := options.Today()
		baseline, err := svc.Store().IngestStatsBaseline(ctx, today.AddDate(0, 0, -30), today)
		if err != nil {
			return err
		}
		pending, err := svc.Store().PendingRows(ctx)
		if err != nil {
			return err
		}
		pressure, err := svc.Store().SLAPressure(ctx, today, 3)
		if err != nil {
			return err
		}
		budget := dedup.ComputeBudget(today, baseline, pending, pressure)
		if err := svc.Store().UpsertBudget(ctx, budget); err != nil {
			return err
		}
		result.Details = budget
		log.Printf("[cdeu] %s 完成 biz_date=%s base_rows=%.0f p75_rows=%.0f backlog_days=%.2f sla_pressure=%.2f factor=%.2f budget_rows=%d hard_cap_rows=%d",
			dedup.JobCalcDailyExportBudget, today.Format("2006-01-02"), baseline.BaseRows, baseline.P75Rows,
			budget.BacklogRatio, budget.SLAPressure, budget.Factor, budget.BudgetRows, budget.HardCapRows)
		return nil
	})
}
