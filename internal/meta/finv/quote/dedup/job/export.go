package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	dedup.RegisterJobExecutor(dedup.JobRunDueExportJobs, RunDueExportJobs)
}

// RunDueExportJobs 执行批量 BASE/INCR 导出。单证券、单月、单类型的动态 JOB 复用该入口。
//
// 手动 / 调度的差别（ACANX 2026-10-27）：
//   - 调度触发（CDEUTriggerSchedule）：受低峰窗口与日预算双重约束，用来保护在线业务；
//   - 手动触发（CDEUTriggerManual）：由运维显式发起，**不受低峰窗口与日预算限制**
//     （历史全量重导不可能塞进"按日增量算出来的预算"里）；消耗仍记账，便于审计与
//     Auto 模式的预算基线。
func RunDueExportJobs(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
	options := svc.Options()
	manual := result.IsManualTrigger()
	if !manual && !svc.InWindow(ctx, options.Now()) {
		result.Message = "当前不在导出低峰窗口内，跳过"
		return nil
	}

	batchBase, batchIncr, err := svc.Store().GenerateExportBatch(ctx, options.Today(), options.CurrentMonth())
	if err != nil {
		return fmt.Errorf("证券导出批次清单获取失败: %w", err)
	}
	svc.ReportJobProgress(ctx, "BATCH", 0, fmt.Sprintf("批次清单 BASE=%d INCR=%d", batchBase, batchIncr))
	svc.AdvanceJobCDEUStage(ctx, "META")

	metadataRows, err := svc.Store().RefreshSecurityMetadata(ctx)
	if err != nil {
		return fmt.Errorf("证券元数据获取失败: %w", err)
	}
	svc.ReportJobProgress(ctx, "META", 0, fmt.Sprintf("证券元数据获取完成，补齐 %d 条", metadataRows))
	// RefreshSecurityMetadata 只管 finv_security.flag_enable='1' 的派生列；禁用的证券、
	// finv_security 未收录的品种（富途板块/主连/外汇对）以及 day_expect_rows 与
	// 优先级/SLA 默认值，统一由 BackfillUscDefaults 补全（只补空，不覆盖人工值）。
	backfill, err := svc.Store().BackfillUscDefaults(ctx, options.MaturityThreshold)
	if err != nil {
		return fmt.Errorf("证券元数据与默认值补全失败: %w", err)
	}
	reportUscBackfill(ctx, svc, "META", backfill)
	svc.AdvanceJobCDEUStage(ctx, "LOAD")

	budget, err := svc.EnsureDailyBudget(ctx, options.Today())
	if err != nil {
		return err
	}

	batchNo := options.Today().Format("20060102")
	created, err := svc.Store().InsertCandidateJobs(ctx, batchNo, options.CurrentMonth(), options.FetchLimit)
	if err != nil {
		return err
	}
	dynamicCreated, err := svc.Store().GenerateDynamicJobs(ctx, batchNo)
	if err != nil {
		return fmt.Errorf("生成单证券导出 JOB 失败: %w", err)
	}
	budgetNote := fmt.Sprintf("预算剩余 %d 行", budget.RemainingRows())
	if manual {
		budgetNote = fmt.Sprintf("预算剩余 %d 行（手动触发：不限制）", budget.RemainingRows())
	}
	svc.ReportJobProgress(ctx, "RUNNING", 0,
		fmt.Sprintf("候选 %d 个，动态 JOB %d 个，本轮最多处理 %d 个，%s",
			created, dynamicCreated, options.MaxJobsPerRun, budgetNote))
	// 需求 10832：批次/候选/动态任一为空时，把「卡在哪一步、为什么」打进日志。
	// 原先只有上面这一行，候选 0 时运维无从判断是元数据不全、未成熟、还是已导出过。
	svc.DiagnoseEmptyExportBatch(ctx, batchBase, batchIncr, created, dynamicCreated, options.CurrentMonth())

	var processed, paused, failed int
	for processed < options.MaxJobsPerRun {
		if err := ctx.Err(); err != nil {
			break
		}
		// 手动触发不受窗口与预算限制：整轮跑完，除非显式取消。
		if !manual && !svc.InWindow(ctx, options.Now()) {
			break
		}
		if !manual && budget.RemainingRows() <= 0 {
			break
		}
		job, claimErr := svc.Store().ClaimJob(ctx, options.WorkerID, options.Lease)
		if claimErr != nil {
			return claimErr
		}
		if job == nil {
			break
		}
		svc.ReportJobProgress(ctx, "RUNNING", float64(processed)/float64(options.MaxJobsPerRun)*100,
			fmt.Sprintf("已处理 %d/%d 个证券月，当前 job_id=%d usc=%s biz_month=%d",
				processed, options.MaxJobsPerRun, job.JobID, job.Usc, job.BizMonth))

		// 运行中立即回写 run_log_id：否则前端「执行日志」按钮（v-if="job.run_log_id"）
		// 在 JOB 跑的过程中不渲染，用户看不到实时进度。
		// 原实现传 0 → COALESCE(NULLIF(0,0), run_log_id) 保持 NULL，缺陷即此。
		_ = svc.Store().MarkDynamicJobStatus(ctx, job.Usc, job.BizMonth, dedup.CDEUStatusRunning, "",
			dedup.RunLogIDFromContext(ctx))
		outcome := svc.ProcessExportJob(ctx, job)
		if outcome.Rows > 0 {
			if err := svc.Store().AddBudgetConsumed(ctx, options.Today(), outcome.Rows, outcome.Bytes); err != nil {
				return err
			}
			budget.ConsumedRows += outcome.Rows
			budget.ConsumedBytes += outcome.Bytes
			result.ProcessedRows += outcome.Rows
			result.ProcessedBytes += outcome.Bytes
		}
		if outcome.Err != nil {
			_ = svc.Store().MarkDynamicJobStatus(ctx, job.Usc, job.BizMonth, dedup.CDEUStatusFailed, outcome.Err.Error(), outcome.RunLogID)
			_ = svc.Store().FailDynamicJobSteps(ctx, job.Usc, job.BizMonth, outcome.Err.Error())
		} else if outcome.Paused {
			_ = svc.Store().MarkDynamicJobStatus(ctx, job.Usc, job.BizMonth, dedup.CDEUStatusPaused, "", outcome.RunLogID)
			_ = svc.Store().MarkDynamicJobStepStatus(ctx, job.Usc, job.BizMonth, "STEP_LOAD", dedup.CDEUStatusPaused, "")
		} else {
			_ = svc.Store().MarkDynamicJobStatus(ctx, job.Usc, job.BizMonth, dedup.CDEUStatusSuccess, "", outcome.RunLogID)
			_ = svc.Store().CompleteDynamicJobSteps(ctx, job.Usc, job.BizMonth)
		}
		processed++
		if outcome.Paused {
			paused++
		}
		if outcome.Err != nil {
			failed++
		}
	}

	svc.AdvanceJobCDEUStage(ctx, "REGISTER")
	svc.AdvanceJobCDEUStage(ctx, "CLEANUP")
	result.Details = map[string]any{
		"candidates":      created,
		"dynamic_created": dynamicCreated,
		"processed":       processed,
		"paused":          paused,
		"failed":          failed,
		"budget_rows":     budget.BudgetRows,
		"consumed_rows":   budget.ConsumedRows,
	}
	log.Printf("[cdeu] %s 完成 candidates=%d processed=%d paused=%d failed=%d budget_rows=%d consumed_rows=%d",
		dedup.JobRunDueExportJobs, created, processed, paused, failed, budget.BudgetRows, budget.ConsumedRows)
	return nil
}
