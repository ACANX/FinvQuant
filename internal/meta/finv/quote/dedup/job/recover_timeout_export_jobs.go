package job

import (
	"context"
	"log"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	dedup.RegisterJobExecutor(dedup.JobRecoverTimeoutExportJobs, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		svc.ReportJobProgress(ctx, "RECOVER", 0, "回收心跳 / 租约超时的任务与上传")
		jobs, err := svc.Store().RecoverTimeoutJobs(ctx)
		if err != nil {
			return err
		}
		timeout := 4 * svc.Options().Lease
		if timeout < 10*time.Minute {
			timeout = 10 * time.Minute
		}
		uploads, err := svc.Store().RecoverTimeoutUploads(ctx, timeout)
		if err != nil {
			return err
		}
		result.ProcessedRows = jobs
		result.Details = map[string]any{
			"recovered_jobs":    jobs,
			"recovered_uploads": uploads,
			"upload_timeout":    timeout.String(),
		}
		log.Printf("[cdeu] %s 完成 recovered_jobs=%d recovered_uploads=%d upload_timeout=%s",
			dedup.JobRecoverTimeoutExportJobs, jobs, uploads, timeout)
		return nil
	})
}
