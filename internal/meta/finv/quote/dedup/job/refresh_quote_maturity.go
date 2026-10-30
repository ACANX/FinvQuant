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
		// 自动发现只建「证券存在」这个事实：元数据（region/market/category/name/timezone/currency）
		// 与归档必需默认值（模板 day_expect_rows / 优先级 100/5/7）在这里统一补齐。
		backfill, err := svc.Store().BackfillUscDefaults(ctx, options.MaturityThreshold)
		if err != nil {
			return fmt.Errorf("证券默认值与元数据补全失败: %w", err)
		}
		reportUscBackfill(ctx, svc, "MATURITY", backfill)
		missingSample, sampleErr := svc.Store().UscMissingSample(ctx, 20)
		if sampleErr != nil {
			log.Printf("[cdeu] 读取未补全证券样本失败: %v", sampleErr)
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
			"backfill_metadata": backfill.MetadataRows,
			"backfill_expect":   backfill.DayExpectRows,
			"backfill_maturity": backfill.MaturityRows,
			"backfill_priority": backfill.Priority,
			"missing_fields":    backfill.Missing,
			"missing_sample":    missingSample,
			"watermark_from":    since.Format(time.RFC3339),
			"watermark_to":      now.Format(time.RFC3339),
			"threshold":         options.MaturityThreshold,
		}
		log.Printf("[cdeu] %s 完成 incremental_rows=%d calibrated_rows=%d discovered_usc=%d backfill_metadata=%d backfill_expect=%d backfill_priority=%d",
			dedup.JobRefreshQuoteMaturity, rows, calibrated, discovered,
			backfill.MetadataRows, backfill.DayExpectRows, backfill.Priority)
		return nil
	})
}

// reportUscBackfill 把证券元数据 / 默认值补全结果写进 JOB 进度短信（stage 由调用方指定），
// 并在日志里输出「仍缺字段」的统计（缺什么、多少行），便于运维定位需要人工维护的证券。
func reportUscBackfill(ctx context.Context, svc *dedup.Service, stage string, stat dedup.UscBackfillStat) {
	if stat.MetadataRows > 0 || stat.DerivedRows > 0 || stat.DayExpectRows > 0 ||
		stat.MaturityRows > 0 || stat.Priority > 0 {
		svc.ReportJobProgress(ctx, stage, 0, fmt.Sprintf(
			"证券补全：元数据 %d 行、按区域/前缀推导 %d 行、day_expect_rows %d 行、重算成熟度 %d 行、优先级/SLA %d 行",
			stat.MetadataRows, stat.DerivedRows, stat.DayExpectRows, stat.MaturityRows, stat.Priority))
	}
	m := stat.Missing
	log.Printf("[cdeu] 证券补全结果：元数据 %d、推导 %d、day_expect_rows %d、重算成熟度 %d、优先级 %d；仍缺 provider=%d category=%d region=%d market=%d name=%d timezone=%d currency=%d day_expect_rows=%d",
		stat.MetadataRows, stat.DerivedRows, stat.DayExpectRows, stat.MaturityRows, stat.Priority,
		m.Provider, m.Category, m.Region, m.Market, m.Name, m.Timezone, m.Currency, m.DayExpectRows)
}
