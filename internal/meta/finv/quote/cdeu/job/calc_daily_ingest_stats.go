package job

import (
	"context"
	"log"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

func init() {
	cdeu.RegisterJobExecutor(cdeu.JobCalcDailyIngestStats, func(ctx context.Context, svc *cdeu.Service, result *cdeu.Result) error {
		svc.ReportJobProgress(ctx, "STATS", 0, "聚合近 30 天入口流量")
		now := svc.Options().Now()
		from := svc.Options().Today().AddDate(0, 0, -30)
		updated, err := svc.Store().CalcIngestStats(ctx, from, now.Add(time.Minute), svc.Options().TimeZone)
		if err != nil {
			return err
		}
		result.Details = map[string]any{
			"stat_days":   updated,
			"window_from": from.Format(time.RFC3339),
			"window_to":   now.Format(time.RFC3339),
			"time_zone":   svc.Options().TimeZone,
		}
		log.Printf("[cdeu] %s 完成 stat_days=%d", cdeu.JobCalcDailyIngestStats, updated)
		return nil
	})
}
