package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

// 明细日志的条数上限：可导出项全量打印（通常不会太多），
// 排除项按原因各打印前若干条，避免一次刷出上千行日志。
const (
	explainExportableLogLimit = 200
	explainExcludedLogLimit   = 10
	explainDetailsItemLimit   = 300
)

func init() {
	// EXPLAIN_EXPORT_DECISION —— 决策阶段：逐条解释导出判定结果（需求 10832）。
	//
	// 背景：此前决策阶段只有 EVALUATE_EXPORT_CONDITION 给出「各原因的数量」，
	// 导出 JOB 在执行时最多打一行「候选 0 个」。一旦某个证券月没有被导出，
	// 运维无法从日志判断是元数据缺失、未成熟、已导出，还是成熟度表压根没有记录，
	// 只能手工逐表比对，排查成本极高。
	//
	// 本 JOB 对成熟度表里**每一个** (usc, biz_month) 给出：
	//   - 结论（可导出 / 排除）；
	//   - 原因码与具体描述（缺哪个字段、差哪几个交易日、已导出的 job_id 与文件名）；
	//   - 可导出项的预计行数、目标文件名、本地落盘路径、远端相对路径；
	//   - 另外扫描「行情表有数据但成熟度表无记录」的月份——这类月份在判定侧完全隐形。
	dedup.RegisterJobExecutor(dedup.JobExplainExportDecision, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		now := svc.Options().Now()
		currentMonth := now.Year()*100 + int(now.Month())

		svc.ReportJobProgress(ctx, "DECISION", 0, "逐条评估导出条件")
		report, err := svc.ExplainExportDecision(ctx, currentMonth, true)
		if err != nil {
			return err
		}

		svc.ReportJobProgress(ctx, "DECISION", 60,
			fmt.Sprintf("共 %d 个证券月：可导出 %d，排除 %d，成熟度缺失 %d",
				report.Total, report.Exportable, report.Excluded, len(report.MaturityGaps)))

		// ---- 逐条打印：可导出项（含文件名与路径）----
		exportable := 0
		for _, item := range report.Items {
			if item.Verdict != dedup.ExportVerdictExportable {
				continue
			}
			if exportable < explainExportableLogLimit {
				log.Printf("[cdeu] 导出决策 可导出 %s %d 优先级=%d 交易日=%d 应有行=%d 实际行=%d 文件=%s 本地=%s 远端=%s",
					item.Usc, item.BizMonth, item.PriorityRank, item.Days,
					item.ExpectedRows, item.ActualRows, item.FileName, item.LocalPath, item.RemotePath)
			}
			exportable++
		}
		if exportable > explainExportableLogLimit {
			log.Printf("[cdeu] 导出决策 可导出共 %d 条，日志仅打印前 %d 条，完整清单见运行日志明细",
				exportable, explainExportableLogLimit)
		}

		// ---- 逐条打印：排除项（按原因分桶，各打前 N 条）----
		excludedByReason := map[string]int{}
		loggedByReason := map[string]int{}
		for _, item := range report.Items {
			if item.Verdict != dedup.ExportVerdictExcluded {
				continue
			}
			excludedByReason[item.ReasonCode]++
			if loggedByReason[item.ReasonCode] >= explainExcludedLogLimit {
				continue
			}
			loggedByReason[item.ReasonCode]++
			log.Printf("[cdeu] 导出决策 不可导出 %s %d 原因=%s 说明=%s",
				item.Usc, item.BizMonth, item.ReasonCode, item.ReasonDetail)
		}
		for reason, count := range excludedByReason {
			if count > explainExcludedLogLimit {
				log.Printf("[cdeu] 导出决策 原因 %s 共 %d 条，日志仅打印前 %d 条", reason, count, explainExcludedLogLimit)
			}
		}

		// ---- 成熟度缺失：判定侧完全看不到的月份 ----
		for index, gap := range report.MaturityGaps {
			if index >= explainExcludedLogLimit {
				log.Printf("[cdeu] 导出决策 成熟度缺失共 %d 条，日志仅打印前 %d 条", len(report.MaturityGaps), explainExcludedLogLimit)
				break
			}
			log.Printf("[cdeu] 导出决策 成熟度缺失 %s %d 说明=quote 行情表已有该月数据，但成熟度表无记录"+
				"（去重/成熟度刷新未产出），导出判定看不到该月，需先排查 REFRESH_QUOTE_MATURITY",
				gap.Usc, gap.BizMonth)
		}

		log.Printf("[cdeu] %s 完成 当月=%d 总数=%d 可导出=%d 排除=%d 原因分布=%v 成熟度缺失=%d",
			dedup.JobExplainExportDecision, report.CurrentMonth, report.Total,
			report.Exportable, report.Excluded, report.ByReason, len(report.MaturityGaps))

		details := map[string]any{
			"current_month": report.CurrentMonth,
			"total":         report.Total,
			"exportable":    report.Exportable,
			"excluded":      report.Excluded,
			"by_reason":     report.ByReason,
			"gap_scan_from": report.GapScanFrom,
			"maturity_gaps": report.MaturityGaps,
		}
		items := report.Items
		if len(items) > explainDetailsItemLimit {
			details["items_truncated"] = len(items)
			items = items[:explainDetailsItemLimit]
		}
		details["items"] = items
		result.Details = details
		return nil
	})
}
