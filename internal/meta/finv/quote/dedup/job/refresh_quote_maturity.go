package job

import (
	"context"
	"fmt"
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
		// 自动发现只建「证券存在」这个事实，归档必需的默认值（模板 day_expect_rows /
		// 优先级 100 / 5 / 7）在这里统一补齐，避免新证券因缺期望条数永远停在 PARTIAL。
		backfill, err := svc.Store().BackfillUscDefaults(ctx)
		if err != nil {
			return fmt.Errorf("证券默认值补全失败: %w", err)
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
			"incremental_rows":  rows,
			"calibrated_rows":   calibrated,
			"discovered_usc":    discovered,
			"backfill_expect":   backfill.DayExpectRows,
			"backfill_priority": backfill.Priority,
			"watermark_from":    since.Format(time.RFC3339),
			"watermark_to":      now.Format(time.RFC3339),
			"threshold":         options.MaturityThreshold,
		}
		log.Printf("[cdeu] %s 完成 incremental_rows=%d calibrated_rows=%d discovered_usc=%d backfill_expect=%d backfill_priority=%d",
			dedup.JobRefreshQuoteMaturity, rows, calibrated, discovered, backfill.DayExpectRows, backfill.Priority)
		return nil
	})
}
