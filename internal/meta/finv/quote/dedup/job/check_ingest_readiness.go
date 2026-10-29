package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	// CHECK_INGEST_READINESS —— 采集阶段：quote 侧数据新鲜度体检（需求 10741）。
	//
	// 采集本身由外部程序负责，本地没有写入路径；本 JOB 做只读体检，
	// 把「最新交易日 / 滞后天数 / 总行数 / 证券数」写入运行明细，
	// 让「历史行情数据采集」这个流程有了可执行的观测点与执行日志。
	dedup.RegisterJobExecutor(dedup.JobCheckIngestReadiness, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "INGEST", 0, "读取 quote 侧数据新鲜度")
		snapshot, err := svc.Store().QuoteIngestSnapshot(ctx, svc.Options().Now())
		if err != nil {
			return err
		}

		svc.ReportJobProgress(ctx, "INGEST", 60,
			fmt.Sprintf("最新交易日 %d，滞后 %d 天，共 %d 行 / %d 个证券",
				snapshot.LatestDate, snapshot.LagDays, snapshot.TotalRows, snapshot.DistinctUsc))

		// 滞后超过 2 天视为采集可能停摆（周末之外没有新增交易日即异常）。
		stale := snapshot.LagDays > 2
		if stale {
			log.Printf("[cdeu] ALERT 采集滞后 latest_date=%d lag_days=%d rows=%d",
				snapshot.LatestDate, snapshot.LagDays, snapshot.TotalRows)
		}

		result.Details = map[string]any{
			"latest_date":      snapshot.LatestDate,
			"latest_create":    snapshot.LatestCreate,
			"total_rows":       snapshot.TotalRows,
			"distinct_usc":     snapshot.DistinctUsc,
			"lag_days":         snapshot.LagDays,
			"current_month":    snapshot.CurrentMonth,
			"has_current_data": snapshot.HasCurrentData,
			"stale":            stale,
		}
		log.Printf("[cdeu] %s 完成 latest_date=%d lag_days=%d rows=%d usc=%d stale=%v",
			dedup.JobCheckIngestReadiness, snapshot.LatestDate, snapshot.LagDays,
			snapshot.TotalRows, snapshot.DistinctUsc, stale)
		return nil
	})
}
