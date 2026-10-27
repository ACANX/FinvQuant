package job

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

const patchBatchLimit = 100

func init() {
	dedup.RegisterJobExecutor(dedup.JobScanPostSealPatches, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		// 手动触发（CDEU 20005=Manual 下的 Job/Run）不受日预算限制：
		// 手动模式语义是「人决定导多少」，增量补丁与全量导出必须行为一致，
		// 否则全量可直接导完而增量被卡在 budget_rows×PatchBudgetRatio 行/轮。
		manual := result.IsManualTrigger()
		allowed := int64(math.MaxInt64)
		budgetRows := int64(0)
		if !manual {
			budget, budgetErr := svc.EnsureDailyBudget(ctx, svc.Options().Today())
			if budgetErr != nil {
				return budgetErr
			}
			budgetRows = budget.BudgetRows
			allowed = int64(float64(budget.BudgetRows) * svc.Options().PatchBudgetRatio)
			if allowed <= 0 {
				allowed = 1
			}
		}
		batchNo := svc.Options().Today().Format("20060102")
		candidates, err := svc.Store().PatchCandidates(ctx, batchNo, patchBatchLimit)
		if err == nil && len(candidates) == 0 {
			candidates, err = svc.Store().PatchCandidates(ctx, "", patchBatchLimit)
		}
		if err != nil {
			return err
		}

		var rows, rewritten, patched int64
		for index, candidate := range candidates {
			if ctx.Err() != nil {
				break
			}
			if rows >= allowed {
				break
			}
			svc.ReportJobProgress(ctx, "PATCH", float64(index)/float64(len(candidates))*100,
				fmt.Sprintf("已处理 %d/%d 个证券月，job_id=%d usc=%s biz_month=%d",
					index, len(candidates), candidate.JobID, candidate.Usc, candidate.BizMonth))

			candidateRunID, startErr := svc.Store().StartRun(ctx, dedup.JobScanPostSealPatches,
				&candidate.JobID, candidate.Usc, &candidate.BizMonth)
			if startErr != nil {
				log.Printf("[cdeu] 登记补丁运行日志失败 job_id=%d usc=%s biz_month=%d err=%v",
					candidate.JobID, candidate.Usc, candidate.BizMonth, startErr)
			}
			candidateCtx := ctx
			if candidateRunID > 0 {
				candidateCtx = dedup.WithRunID(ctx, candidateRunID)
			}
			outcome, patchErr := svc.ProcessPatch(candidateCtx, candidate, allowed-rows)
			if patchErr != nil {
				_ = svc.Store().MarkDynamicJobStatus(ctx, candidate.Usc, candidate.BizMonth, dedup.CDEUStatusFailed, patchErr.Error(), candidateRunID)
				if candidateRunID > 0 {
					_ = svc.Store().FinishRun(context.WithoutCancel(ctx), candidateRunID, dedup.RunStatusFailed, 0, 0, patchErr.Error())
				}
				log.Printf("[cdeu] 补丁处理失败 job_id=%d usc=%s biz_month=%d err=%v",
					candidate.JobID, candidate.Usc, candidate.BizMonth, patchErr)
				continue
			}
			if outcome.Rows == 0 && outcome.Level == "" {
				svc.ReportJobProgress(candidateCtx, "PATCH", 100, "无增量数据，跳过补丁文件生成")
				if candidateRunID > 0 {
					_ = svc.Store().FinishRun(context.WithoutCancel(ctx), candidateRunID, dedup.RunStatusSuccess, 0, 0, "无增量数据")
				}
			} else {
				_ = svc.Store().MarkDynamicJobStatus(ctx, candidate.Usc, candidate.BizMonth, dedup.CDEUStatusSuccess, "", candidateRunID)
				if candidateRunID > 0 {
					_ = svc.Store().FinishRun(context.WithoutCancel(ctx), candidateRunID, dedup.RunStatusSuccess, outcome.Rows, 0, "")
				}
			}
			rows += outcome.Rows
			switch outcome.Level {
			case dedup.PatchLevelL1Hot:
				rewritten++
			case dedup.PatchLevelL2Warm, dedup.PatchLevelL3Cold:
				if outcome.Rows > 0 {
					patched++
				}
			}
		}

		result.ProcessedRows = rows
		result.Details = map[string]any{
			"candidates":        len(candidates),
			"rewritten_base":    rewritten,
			"patches":           patched,
			"patch_rows":        rows,
			"patch_budget_rows": allowed,
			"manual_trigger":    manual,
			"budget_rows":       budgetRows,
		}
		log.Printf("[cdeu] %s 完成 candidates=%d rewritten_base=%d patches=%d patch_rows=%d patch_budget_rows=%d manual=%v budget_rows=%d",
			dedup.JobScanPostSealPatches, len(candidates), rewritten, patched, rows, allowed, manual, budgetRows)
		return nil
	})
}
