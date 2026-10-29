package job

import (
	"context"
	"log"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

const watermarkMaturity = "REFRESH_QUOTE_MATURITY"

func init() {
	dedup.RegisterJobExecutor(dedup.JobRefreshQuoteMaturity, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "MATURITY", 0, "增量刷新成熟度")
		options := svc.Options()
		now := options.Now()
		since, found, err := svc.Store().Watermark(ctx, watermarkMaturity)
		if err != nil {
			return err
		}
		if !found || since.After(now) {
			since = now.AddDate(0, -3, 0)
		}
		rows, err := svc.Store().RefreshMaturityIncremental(ctx, since, now, options.MaturityThreshold)
		if err != nil {
			return err
		}
		discovered, err := svc.Store().DiscoverUsc(ctx, since, now)
		if err != nil {
			return err
		}
		var calibrated int64
		if !found || now.Day() == 1 {
			svc.ReportJobProgress(ctx, "MATURITY", 60, "全量校准最近 3 个月")
			dateFrom := dedup.FirstDayOfMonth(now.AddDate(0, -2, 0))
			calibrated, err = svc.Store().RecalibrateMaturity(ctx, dateFrom, options.MaturityThreshold)
			if err != nil {
				return err
			}
		}
		if err := svc.Store().UpsertWatermark(ctx, watermarkMaturity, now); err != nil {
			return err
		}
		result.ProcessedRows = rows + calibrated
		result.Details = map[string]any{
			"incremental_rows": rows,
			"calibrated_rows":  calibrated,
			"discovered_usc":   discovered,
			"watermark_from":   since.Format(time.RFC3339),
			"watermark_to":     now.Format(time.RFC3339),
			"threshold":        options.MaturityThreshold,
		}
		log.Printf("[cdeu] %s 完成 incremental_rows=%d calibrated_rows=%d discovered_usc=%d",
			dedup.JobRefreshQuoteMaturity, rows, calibrated, discovered)
		return nil
	})
}
