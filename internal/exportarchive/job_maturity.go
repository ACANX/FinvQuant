package exportarchive

import (
	"context"
	"log"
	"time"
)

// watermarkMaturity 成熟度增量扫描水位名。
const watermarkMaturity = "REFRESH_QUOTE_MATURITY"

// runRefreshQuoteMaturity 增量刷新成熟度并自动发现新 usc（J-01~J-05 / P1）。
func (s *Service) runRefreshQuoteMaturity(ctx context.Context, result *Result) error {
	now := s.options.Now()
	since, found, err := s.store.Watermark(ctx, watermarkMaturity)
	if err != nil {
		return err
	}
	if !found || since.After(now) {
		since = now.AddDate(0, -3, 0)
	}

	rows, err := s.store.RefreshMaturityIncremental(ctx, since, now, s.options.MaturityThreshold)
	if err != nil {
		return err
	}
	discovered, err := s.store.DiscoverUsc(ctx, since, now)
	if err != nil {
		return err
	}

	// 周期性全量校准：水位首次建立或每月 1 日，按最近 3 个月按日期重算（防水位漏行长期偏差）。
	var calibrated int64
	if !found || now.Day() == 1 {
		dateFrom := firstDayOfMonth(now.AddDate(0, -2, 0))
		calibrated, err = s.store.RecalibrateMaturity(ctx, dateFrom, s.options.MaturityThreshold)
		if err != nil {
			return err
		}
	}

	if err := s.store.UpsertWatermark(ctx, watermarkMaturity, now); err != nil {
		return err
	}

	result.ProcessedRows = rows + calibrated
	result.Details = map[string]any{
		"incremental_rows": rows,
		"calibrated_rows":  calibrated,
		"discovered_usc":   discovered,
		"watermark_from":   since.Format(time.RFC3339),
		"watermark_to":     now.Format(time.RFC3339),
		"threshold":        s.options.MaturityThreshold,
	}
	log.Printf("[exportarchive] %s 完成 incremental_rows=%d calibrated_rows=%d discovered_usc=%d",
		JobRefreshQuoteMaturity, rows, calibrated, discovered)
	return nil
}

// runMarkSealedJobsFrozen 收口冻结 + 差异报告（J-06 / J-07 / M-04）。
func (s *Service) runMarkSealedJobsFrozen(ctx context.Context, result *Result) error {
	today := s.options.Today()
	maxMonth := maxSealedBizMonth(today)

	sealed, err := s.store.SealMaturity(ctx, maxMonth)
	if err != nil {
		return err
	}
	frozenJobs, err := s.store.MarkJobsFrozen(ctx, today)
	if err != nil {
		return err
	}

	minMonth := addMonths(today.Year()*100+int(today.Month()), -6)
	diff, err := s.store.FrozenDiffReport(ctx, minMonth, 200)
	if err != nil {
		return err
	}
	for _, item := range diff {
		log.Printf("[exportarchive] FROZEN_DIFF usc=%s biz_month=%d expected_days=%d missing_days=%d expected_rows=%d actual_rows=%d missing_rows=%d missing_ratio=%.4f",
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
	log.Printf("[exportarchive] %s 完成 sealed_maturity_rows=%d frozen_jobs=%d sealed_biz_month=%d diff_rows=%d",
		JobMarkSealedJobsFrozen, sealed, frozenJobs, maxMonth, len(diff))
	return nil
}

// firstDayOfMonth 返回 t 所在月份首日的 yyyyMMdd。
func firstDayOfMonth(t time.Time) int {
	return t.Year()*10000 + int(t.Month())*100 + 1
}

// maxSealedBizMonth 返回已过收口日（seal_date <= today）的最大归档月份。
func maxSealedBizMonth(today time.Time) int {
	month := today.Year()*100 + int(today.Month())
	for i := 0; i < 24; i++ {
		if !SealDateOf(month).After(today) {
			return month
		}
		month = addMonths(month, -1)
	}
	return month
}

// addMonths 在 yyyyMM 上加减月份。
func addMonths(month int, delta int) int {
	year, mon := month/100, time.Month(month%100)
	shifted := time.Date(year, mon, 1, 0, 0, 0, 0, time.UTC).AddDate(0, delta, 0)
	return shifted.Year()*100 + int(shifted.Month())
}
