package exportarchive

import (
	"context"
	"log"
	"math"
	"time"
)

// fallbackBudgetRows 入口统计尚未就绪（首次上线 / 统计缺口）时的兜底日预算条数。
const fallbackBudgetRows int64 = 200000

// ComputeBudget 计算每日导出预算（方案稿 4.2 节）。
//
//	base_rows   = 近 30 天日均入口条数
//	p75_rows    = 近 30 天入口条数 P75
//	backlog     = 未完成证券月 pending_rows / base_rows
//	sla_pressure = 临近 seal/freeze 的任务占比
//	factor      = clamp(1.20 + 0.20*sla_pressure + 0.05*min(backlog, 5), 1.20, 2.00)
//	budget_rows = min(p75_rows, base_rows * factor)
//	hard_cap    = base_rows * 2.0
//
// 周末不降低 factor（公式不含周内差异）；base_rows 缺失时按 fallbackBudgetRows 兜底。
func ComputeBudget(bizDate time.Time, baseline IngestBaseline, pendingRows int64, slaPressure float64) Budget {
	effectiveBase := baseline.BaseRows
	if effectiveBase <= 0 {
		effectiveBase = float64(fallbackBudgetRows)
	}
	if slaPressure < 0 {
		slaPressure = 0
	}
	if slaPressure > 1 {
		slaPressure = 1
	}
	backlog := float64(pendingRows) / effectiveBase
	factor := 1.20 + 0.20*slaPressure + 0.05*math.Min(backlog, 5)
	factor = clampFloat(factor, 1.20, 2.00)

	target := effectiveBase * factor
	if baseline.P75Rows > 0 && baseline.P75Rows < target {
		target = baseline.P75Rows
	}
	if target < 1 {
		target = 1
	}

	baseBytes := baseline.BaseBytes
	if baseBytes <= 0 {
		baseBytes = effectiveBase * 128 // 单行约 128 字节的粗估，仅用于日志与指标
	}

	return Budget{
		BizDate:      bizDate,
		BaseRows:     int64(math.Round(baseline.BaseRows)),
		BaseBytes:    int64(math.Round(baseline.BaseBytes)),
		P75Rows:      int64(math.Round(baseline.P75Rows)),
		BacklogRatio: backlog,
		SLAPressure:  slaPressure,
		Factor:       factor,
		BudgetRows:   int64(math.Round(target)),
		BudgetBytes:  int64(math.Round(baseBytes * factor)),
		HardCapRows:  int64(math.Round(effectiveBase * 2.0)),
		Status:       "CALCULATED",
	}
}

// ensureBudget 读取当日预算；缺失时按当前统计即时计算一份（避免首次上线无预算可用）。
func (s *Service) ensureBudget(ctx context.Context, today time.Time) (*Budget, error) {
	budget, err := s.store.GetBudget(ctx, today)
	if err != nil {
		return nil, err
	}
	if budget != nil {
		return budget, nil
	}

	baseline, err := s.store.IngestStatsBaseline(ctx, today.AddDate(0, 0, -30), today)
	if err != nil {
		return nil, err
	}
	pending, err := s.store.PendingRows(ctx)
	if err != nil {
		return nil, err
	}
	pressure, err := s.store.SLAPressure(ctx, today, 3)
	if err != nil {
		return nil, err
	}
	computed := ComputeBudget(today, baseline, pending, pressure)
	if err := s.store.UpsertBudget(ctx, computed); err != nil {
		return nil, err
	}
	return &computed, nil
}

// runCalcDailyIngestStats 每日统计入口流量（H-01）。
func (s *Service) runCalcDailyIngestStats(ctx context.Context, result *Result) error {
	now := s.options.Now()
	from := s.options.Today().AddDate(0, 0, -30)
	updated, err := s.store.CalcIngestStats(ctx, from, now.Add(time.Minute), s.options.TimeZone)
	if err != nil {
		return err
	}
	result.Details = map[string]any{
		"stat_days":   updated,
		"window_from": from.Format(time.RFC3339),
		"window_to":   now.Format(time.RFC3339),
		"time_zone":   s.options.TimeZone,
	}
	log.Printf("[exportarchive] %s 完成 stat_days=%d", JobCalcDailyIngestStats, updated)
	return nil
}

// runCalcDailyExportBudget 每日计算导出预算（H-02 / H-03 / H-07）。
func (s *Service) runCalcDailyExportBudget(ctx context.Context, result *Result) error {
	today := s.options.Today()
	baseline, err := s.store.IngestStatsBaseline(ctx, today.AddDate(0, 0, -30), today)
	if err != nil {
		return err
	}
	pending, err := s.store.PendingRows(ctx)
	if err != nil {
		return err
	}
	pressure, err := s.store.SLAPressure(ctx, today, 3)
	if err != nil {
		return err
	}
	budget := ComputeBudget(today, baseline, pending, pressure)
	if err := s.store.UpsertBudget(ctx, budget); err != nil {
		return err
	}
	result.Details = budget
	log.Printf("[exportarchive] %s 完成 biz_date=%s base_rows=%.0f p75_rows=%.0f backlog_days=%.2f sla_pressure=%.2f factor=%.2f budget_rows=%d hard_cap_rows=%d",
		JobCalcDailyExportBudget, today.Format("2006-01-02"), baseline.BaseRows, baseline.P75Rows,
		budget.BacklogRatio, budget.SLAPressure, budget.Factor, budget.BudgetRows, budget.HardCapRows)
	return nil
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
