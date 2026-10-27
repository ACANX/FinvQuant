package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	// EVALUATE_EXPORT_CONDITION —— 决策阶段：只读评估导出条件（需求 10741）。
	//
	// 原实现里「导出条件判定」是一个没有任何 JOB 的占位流程，真正的筛选逻辑内嵌在
	// PHASE_EXPORT 的 JOB_FETCH_EXPORT_BATCH 中，导致决策阶段既看不到判定结果、
	// 也没有执行日志。本 JOB 用完全相同的口径做一次只读评估，输出：
	//   - 满足 BASE 导出条件的证券月数（candidates）；
	//   - 各排除原因的数量（元数据不全 / 已退市 / 当月 / 未成熟 / 已导出）；
	//   - 已导出且可生成增量补丁的月份数（incr_candidates）；
	//   - 前 10 个候选样本，便于人工核对。
	// 只读，不写批次表；真正的批次清单仍由 JOB_FETCH_EXPORT_BATCH 生成。
	dedup.RegisterJobExecutor(dedup.JobEvaluateExportCondition, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		now := svc.Options().Now()
		currentMonth := now.Year()*100 + int(now.Month())

		svc.ReportJobProgress(ctx, "DECISION", 0, "评估 BASE 导出条件")
		report, err := svc.Store().EvaluateExportConditions(ctx, currentMonth)
		if err != nil {
			return err
		}

		svc.ReportJobProgress(ctx, "DECISION", 70,
			fmt.Sprintf("候选 %d 个证券月（当月 %d，未成熟 %d，已导出 %d）",
				report.CandidateMonths, report.CurrentMonthSkips,
				report.ImmatureMonths, report.AlreadyExported))

		result.Details = map[string]any{
			"current_month":       report.CurrentMonth,
			"candidate_months":    report.CandidateMonths,
			"already_exported":    report.AlreadyExported,
			"immature_months":     report.ImmatureMonths,
			"incomplete_usc":      report.IncompleteUsc,
			"delisted_months":     report.DelistedMonths,
			"current_month_skips": report.CurrentMonthSkips,
			"incr_candidates":     report.IncrCandidates,
			"sample_candidates":   report.SampleCandidates,
		}
		log.Printf("[cdeu] %s 完成 current_month=%d candidates=%d exported=%d immature=%d incomplete_usc=%d delisted=%d current_skip=%d incr=%d",
			dedup.JobEvaluateExportCondition, report.CurrentMonth, report.CandidateMonths,
			report.AlreadyExported, report.ImmatureMonths, report.IncompleteUsc,
			report.DelistedMonths, report.CurrentMonthSkips, report.IncrCandidates)
		return nil
	})
}
