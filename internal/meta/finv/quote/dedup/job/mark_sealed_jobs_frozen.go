package job

import (
	"context"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	dedup.RegisterJobExecutor(dedup.JobMarkSealedJobsFrozen, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "SEAL", 0, "收口冻结与差异报告生成")
		options := svc.Options()
		today := options.Today()
		maxMonth := dedup.MaxSealedBizMonth(today)
		sealed, err := svc.Store().SealMaturity(ctx, maxMonth)
		if err != nil {
			return err
		}
		frozenJobs, err := svc.Store().MarkJobsFrozen(ctx, today)
		if err != nil {
			return err
		}
		minMonth := dedup.AddMonths(today.Year()*100+int(today.Month()), -6)
		diff, err := svc.Store().FrozenDiffReport(ctx, minMonth, 200)
		if err != nil {
			return err
		}
		for _, item := range diff {
			log.Printf("[cdeu] FROZEN_DIFF usc=%s biz_month=%d expected_days=%d missing_days=%d expected_rows=%d actual_rows=%d missing_rows=%d missing_ratio=%.4f",
				item.Usc, item.BizMonth, item.ExpectedDays, item.MissingDays,
				item.ExpectedRows, item.ActualRows, item.MissingRows, item.MissingRatio)
		}
		result.ProcessedRows = sealed
		result.Details = map[string]any{
			"sealed_maturity_rows": sealed,
			"frozen_jobs":          frozenJobs,
			"sealed_biz_month":     maxMonth,
			"diff_report":          diff,
		}
		log.Printf("[cdeu] %s 完成 sealed_maturity_rows=%d frozen_jobs=%d sealed_biz_month=%d diff_rows=%d",
			dedup.JobMarkSealedJobsFrozen, sealed, frozenJobs, maxMonth, len(diff))
		return nil
	})
}
