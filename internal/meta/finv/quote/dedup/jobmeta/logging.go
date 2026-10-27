// Package job 提供归档 JOB 的可观测日志文本构造，不依赖 dedup 主包，避免循环引用。
package jobmeta

import "fmt"

// StepContext 描述一次 Step 日志所需的 JOB / Step 上下文。
type StepContext struct {
	JobCode      string
	JobName      string
	StepCode     string
	StepName     string
	Usc          string
	BizMonth     int
	StagingTable string
}

// StepStart 返回 Step 开始日志。
func StepStart(ctx StepContext) string {
	return fmt.Sprintf("STEP %s 开始 · job_code=%s job_name=%s step_code=%s step_name=%s · usc=%s biz_month=%d",
		ctx.StepCode, ctx.JobCode, ctx.JobName, ctx.StepCode, ctx.StepName, ctx.Usc, ctx.BizMonth)
}

// StepEnd 返回 Step 结束日志。
func StepEnd(ctx StepContext, detail string) string {
	message := fmt.Sprintf("STEP %s 结束 · job_code=%s job_name=%s step_code=%s step_name=%s · usc=%s biz_month=%d",
		ctx.StepCode, ctx.JobCode, ctx.JobName, ctx.StepCode, ctx.StepName, ctx.Usc, ctx.BizMonth)
	if detail != "" {
		message += " · " + detail
	}
	return message
}

// ExportWritten 返回 MVsv 写入完成日志，明确记录实际绝对路径。
func ExportWritten(ctx StepContext, absolutePath string, rows int64, size int64, checksum string) string {
	return fmt.Sprintf("STEP %s 写入完成 · job_code=%s job_name=%s step_code=%s step_name=%s · path=%s rows=%d size=%d checksum=%s",
		ctx.StepCode, ctx.JobCode, ctx.JobName, ctx.StepCode, ctx.StepName,
		absolutePath, rows, size, checksum)
}

// CleanupStart 返回清理中转表开始日志。
func CleanupStart(ctx StepContext, usc string, bizMonth int) string {
	return fmt.Sprintf("STEP %s 开始 · job_code=%s job_name=%s step_code=%s step_name=%s · 中转表=%s usc=%s biz_month=%d",
		ctx.StepCode, ctx.JobCode, ctx.JobName, ctx.StepCode, ctx.StepName,
		ctx.StagingTable, usc, bizMonth)
}
