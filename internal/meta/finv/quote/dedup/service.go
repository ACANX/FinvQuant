package dedup

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Result 一次 Job 执行的结果（同时写入 finv_dedup_job_run_log_export_file）。
type Result struct {
	JobKind        string `json:"job_kind"`
	Status         string `json:"status"`
	ProcessedRows  int64  `json:"processed_rows"`
	ProcessedBytes int64  `json:"processed_bytes"`
	Message        string `json:"message,omitempty"`
	Details        any    `json:"details,omitempty"`
}

// Service 归档链路服务：实现 10 个 Job 的编排，并对外暴露状态查询与人工重试。
type Service struct {
	store    *Store
	uploader Uploader
	options  Options

	mu      sync.Mutex
	running map[string]bool
}

// NewService 创建归档服务。
func NewService(pool *pgxpool.Pool, options Options) (*Service, error) {
	uploader, err := NewUploader(options)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:    NewStore(pool),
		uploader: uploader,
		options:  options,
		running:  map[string]bool{},
	}, nil
}

// Options 返回运行参数。
func (s *Service) Options() Options { return s.options }

// Store 返回数据访问层。
func (s *Service) Store() *Store { return s.store }

// Uploader 返回上传适配器。
func (s *Service) Uploader() Uploader { return s.uploader }

// JobRuntime 描述单个 Job 的调度方式（供 scheduler 与运维接口使用）。
type JobRuntime struct {
	Kind     string `json:"job_kind"`
	DailyAt  string `json:"daily_at,omitempty"`
	Interval int    `json:"interval_seconds,omitempty"`
	Windowed bool   `json:"windowed"`
	Schedule string `json:"schedule"`
}

// JobRuntimes 返回全部 Job 的调度配置（方案稿 2.2 节）。
func JobRuntimes() []JobRuntime {
	return []JobRuntime{
		{Kind: JobCalcDailyIngestStats, DailyAt: "00:05", Schedule: "每日 00:05"},
		{Kind: JobCalcDailyExportBudget, DailyAt: "00:05", Schedule: "每日 00:05"},
		{Kind: JobRefreshQuoteMaturity, DailyAt: "00:10", Schedule: "每日 00:10"},
		{Kind: JobMarkSealedJobsFrozen, DailyAt: "00:20", Schedule: "每日 00:20"},
		{Kind: JobRunDueExportJobs, Interval: 60, Windowed: true, Schedule: "低峰窗口内每 60 秒一轮"},
		{Kind: JobUploadPendingExportFiles, Interval: 300, Schedule: "全天每 5 分钟"},
		{Kind: JobScanPostSealPatches, DailyAt: "03:00", Schedule: "每日 03:00"},
		{Kind: JobCleanExpiredExportArtifacts, DailyAt: "06:00", Schedule: "每日 06:00"},
		{Kind: JobRecoverTimeoutExportJobs, Interval: 300, Schedule: "每 5 分钟"},
		{Kind: JobCheckExportJobHealth, Interval: 900, Schedule: "每 15 分钟"},
	}
}

// Run 执行一个 Job：进程内同类 Job 互斥，执行前后写运行日志。
func (s *Service) Run(ctx context.Context, kind string) (Result, error) {
	if !s.begin(kind) {
		return Result{}, fmt.Errorf("Job %s 正在执行中，跳过本次触发", kind)
	}
	defer s.end(kind)

	runID, err := s.store.StartRun(ctx, kind, nil, "", nil)
	if err != nil {
		return Result{}, fmt.Errorf("登记运行日志失败: %w", err)
	}

	result := Result{JobKind: kind}
	runErr := s.dispatch(ctx, kind, &result)
	status := RunStatusSuccess
	errText := ""
	if runErr != nil {
		status = RunStatusFailed
		errText = runErr.Error()
	}
	result.Status = status

	finishCtx := context.WithoutCancel(ctx)
	if finishErr := s.store.FinishRun(finishCtx, runID, status, result.ProcessedRows, result.ProcessedBytes,
		joinMessage(errText, result.Message)); finishErr != nil && runErr == nil {
		runErr = fmt.Errorf("更新运行日志失败: %w", finishErr)
	}
	return result, runErr
}

func (s *Service) dispatch(ctx context.Context, kind string, result *Result) error {
	switch kind {
	case JobCalcDailyIngestStats:
		return s.runCalcDailyIngestStats(ctx, result)
	case JobCalcDailyExportBudget:
		return s.runCalcDailyExportBudget(ctx, result)
	case JobRefreshQuoteMaturity:
		return s.runRefreshQuoteMaturity(ctx, result)
	case JobMarkSealedJobsFrozen:
		return s.runMarkSealedJobsFrozen(ctx, result)
	case JobRunDueExportJobs:
		return s.runRunDueExportJobs(ctx, result)
	case JobUploadPendingExportFiles:
		return s.runUploadPendingExportFiles(ctx, result)
	case JobScanPostSealPatches:
		return s.runScanPostSealPatches(ctx, result)
	case JobCleanExpiredExportArtifacts:
		return s.runCleanExpiredExportArtifacts(ctx, result)
	case JobRecoverTimeoutExportJobs:
		return s.runRecoverTimeoutExportJobs(ctx, result)
	case JobCheckExportJobHealth:
		return s.runCheckExportJobHealth(ctx, result)
	default:
		return fmt.Errorf("未知 Job: %s", kind)
	}
}

func (s *Service) begin(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[kind] {
		return false
	}
	s.running[kind] = true
	return true
}

func (s *Service) end(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, kind)
}

// Metrics 返回当前健康指标与告警（供运维接口与监控采集使用）。
func (s *Service) Metrics(ctx context.Context) (*Metrics, error) {
	metrics, err := s.buildMetrics(ctx)
	if err != nil {
		return nil, err
	}
	metrics.Alerts = BuildAlerts(*metrics, s.options)
	return metrics, nil
}

// Trigger 异步触发一个 Job（供运维接口使用）；Job 标识非法时返回错误。
func (s *Service) Trigger(kind string) error {
	for _, runtime := range JobRuntimes() {
		if runtime.Kind != kind {
			continue
		}
		go func() {
			if _, err := s.Run(context.Background(), kind); err != nil {
				log.Printf("[exportarchive] 手动触发 %s 失败: %v", kind, err)
			}
		}()
		return nil
	}
	return fmt.Errorf("未知 Job: %s", kind)
}

func joinMessage(parts ...string) string {
	message := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if message != "" {
			message += " | "
		}
		message += part
	}
	return message
}
