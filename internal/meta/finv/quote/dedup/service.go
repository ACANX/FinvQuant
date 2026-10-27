package dedup

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	jobdef "github.com/acanx/finvquant/internal/meta/finv/quote/dedup/jobmeta"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// Result 一次 Job 执行的结果（同时写入 finv_dedup_job_run_log_export_file）。
type Result struct {
	JobKind        string `json:"job_kind"`
	Status         string `json:"status"`
	ProcessedRows  int64  `json:"processed_rows"`
	ProcessedBytes int64  `json:"processed_bytes"`
	Message        string `json:"message,omitempty"`
	Details        any    `json:"details,omitempty"`
	// Trigger 本次运行的触发来源（CDEUTriggerManual / CDEUTriggerSchedule）。
	// 供 Job 实现区分「手动触发」与「调度触发」：手动触发不受日预算与低峰窗口限制。
	Trigger string `json:"trigger_type,omitempty"`
}

// IsManualTrigger 本次运行是否为手动触发（管理员 Job/Run）。
func (r Result) IsManualTrigger() bool { return r.Trigger == CDEUTriggerManual }

// triggerContextKey 触发来源在 context 中的键（供 Job 实现读取，避免层层传参）。
type triggerContextKey struct{}

// withTrigger 把触发来源放入 context（CDEUTriggerManual / CDEUTriggerSchedule）。
func withTrigger(ctx context.Context, triggerType string) context.Context {
	return context.WithValue(ctx, triggerContextKey{}, triggerType)
}

// TriggerFromContext 读取 context 中的触发来源（不存在返回空串）。
func TriggerFromContext(ctx context.Context) string {
	triggerType, _ := ctx.Value(triggerContextKey{}).(string)
	return triggerType
}

// IsManualTriggerFromContext 本次运行是否为手动触发（管理员 Job/Run）。
//
// 手动触发不受日预算与低峰窗口限制：历史全量重导不可能塞进按日增量算出的预算里。
func IsManualTriggerFromContext(ctx context.Context) bool {
	return TriggerFromContext(ctx) == CDEUTriggerManual
}

// Service 归档链路服务：实现 10 个 Job 的编排，并对外暴露状态查询与人工重试。
type Service struct {
	store    *Store
	uploader Uploader
	options  Options
	cfg      *syscfg.Store // sys.sys_config 读取器（20002–20006）；nil 时按默认值兜底

	mu      sync.Mutex
	running map[string]time.Time

	// 低峰窗口统一从 sys.sys_config 20006 读取（syscfg.Store 自带 30s TTL 缓存），
	// 不再在内存里单独缓存，避免与数据库配置双写不一致。
}

// NewService 创建归档服务。
//
// 上传实现的选择优先级：sys.sys_config 20002='1' → GitHub Contents API；
// 否则回退环境变量派生的 none/dir/http（既有部署行为不变）。pool 为空时
// （未接库的单测场景）直接使用环境变量实现。
func NewService(pool *pgxpool.Pool, options Options) (*Service, error) {
	uploader, err := NewUploader(options)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		uploader = newDynamicUploader(syscfg.NewStore(pool), uploader, options.UploadGitHubAPIBase)
	}
	var cfgStore *syscfg.Store
	if pool != nil {
		cfgStore = syscfg.NewStore(pool)
	}
	return &Service{
		store:    NewStore(pool),
		uploader: uploader,
		options:  options,
		cfg:      cfgStore,
		running:  map[string]time.Time{},
	}, nil
}

// Options 返回运行参数。
//
// 注意：Options.WindowStart/WindowEnd 只是**兜底默认值**，当前生效窗口请用 Window(ctx)，
// 它读取 sys_config 20006（运维可在线修改，无需重启）。
func (s *Service) Options() Options { return s.options }

// Window 返回当前生效的低峰窗口（自 0 点起的秒数）、来源与快照时间。
//
// 来源取值：
//   - "sysconfig"：来自 sys.sys_config 20006 且格式合法；
//   - "default"  ：20006 缺失/非法或读取失败，已回退内置默认（syscfg.ExportWindowDefault）。
func (s *Service) Window(ctx context.Context) (start, end int, source string, updated time.Time) {
	if s.cfg == nil {
		return s.options.WindowStart, s.options.WindowEnd, "default", time.Time{}
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return s.options.WindowStart, s.options.WindowEnd, "default", time.Time{}
	}
	if values.ExportWindowErr != nil {
		// 缺失或非法：Values 里已填入默认窗口，这里标注来源为 default。
		return values.ExportWindow.Start, values.ExportWindow.End, "default", values.LoadedAt
	}
	return values.ExportWindow.Start, values.ExportWindow.End, "sysconfig", values.LoadedAt
}

// InWindow 判断给定时刻是否处于当前生效的低峰窗口（读取 20006）。
func (s *Service) InWindow(ctx context.Context, at time.Time) bool {
	start, end, _, _ := s.Window(ctx)
	return inWindowAt(at, s.options.Location, start, end)
}

// CDEUMode 返回系统配置 20005 的 CDEU 工作流运行模式（Manual / Auto）。
//
// 读配置失败、未配置或取值非法一律返回 Manual（安全侧：不自动跑归档），
// 并把原始错误回传给调用方用于日志诊断。窗口（EXPORT_WINDOW_*）控制「什么时段可以跑」，
// 本模式控制「是否允许自动跑」，两者独立：
//   - Manual（默认）：进程内调度器不触发任何 CDEU Job，只有手动 Job/Run 能执行；
//   - Auto：每天按定时任务设定的时间自动跑。
func (s *Service) CDEUMode(ctx context.Context) (string, error) {
	if s.cfg == nil {
		return syscfg.CDEUModeManual, nil
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return syscfg.CDEUModeManual, err
	}
	return syscfg.NormalizeCDEUMode(values.CDEUModeRaw), nil
}

// PurgeAfterExportEnabled 读取 20007：导出成功后是否删除 dedup 主表行情数据。
//
//   - true（默认）：远端上传成功后删除 cdeu.finv_quote_secu_kline_min 中该
//     (usc, biz_month) 的数据；此后表内再次插入历史数据按 INCR 增量导出（序号 N 递增）。
//   - false：保留主表数据，重复导出直接重写同名 BASE 覆盖，不生成 INCR。
//
// 读配置失败时按 true（与种子默认一致）处理。
func (s *Service) PurgeAfterExportEnabled(ctx context.Context) bool {
	if s.cfg == nil {
		return true
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return true
	}
	return values.PurgeAfterExport
}

// DynamicJobRetentionDays 返回历史运行动态 JOB 实例的保留天数（系统配置 20008）。
//
// 读配置失败或取值非法时回退默认 30 天，保证清理逻辑始终有界。
func (s *Service) DynamicJobRetentionDays(ctx context.Context) int {
	if s.cfg == nil {
		return syscfg.DynamicJobRetentionDefault
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return syscfg.DynamicJobRetentionDefault
	}
	return syscfg.NormalizeDynamicJobRetentionDays(values.DynamicJobRetentionDays)
}

// PurgeLocalExportFilesEnabled 归档复核阶段是否清理本地导出文件（系统配置 20011）。
//
// 读取失败时按 true（清理）兜底，与种子默认值一致。
func (s *Service) PurgeLocalExportFilesEnabled(ctx context.Context) bool {
	if s.cfg == nil {
		return true
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return true
	}
	return values.PurgeLocalExportFiles
}

// HistoryPurgePolicy 返回历史流程运行数据的清理策略与阈值（20009 / 20010）。
//
// 读取失败时按 Days + 默认 30 兜底，保证 PURGE_HISTORY_RUNS 有确定行为。
func (s *Service) HistoryPurgePolicy(ctx context.Context) (policy string, keep int) {
	if s.cfg == nil {
		return syscfg.HistoryPurgeDays, syscfg.HistoryPurgeKeepDefault
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return syscfg.HistoryPurgeDays, syscfg.HistoryPurgeKeepDefault
	}
	return syscfg.NormalizeHistoryPurgePolicy(values.HistoryPurgePolicy),
		syscfg.NormalizeHistoryPurgeKeep(values.HistoryPurgeKeep)
}

// CDEUScheduleEnabled 是否允许进程内调度器自动触发 CDEU Job。
// 只有 20005 明确为 Auto 时才返回 true。
func (s *Service) CDEUScheduleEnabled(ctx context.Context) bool {
	mode, _ := s.CDEUMode(ctx)
	return mode == syscfg.CDEUModeAuto
}

// SetWindow 在运行时修改低峰窗口：写入 sys.sys_config 20006（秒级）后立即生效。
//
// 写入成功后 syscfg.Store 会失效本地缓存，下一次读取即拿到新值，无需重启服务。
func (s *Service) SetWindow(ctx context.Context, start, end int) error {
	if start < 0 || start > 86399 || end < 0 || end > 86399 {
		return fmt.Errorf("窗口时间必须在 00:00:00 ~ 23:59:59 之间")
	}
	if s.cfg == nil {
		return fmt.Errorf("数据库配置不可用，无法保存低峰窗口")
	}
	value := syscfg.FormatExportWindow(syscfg.ExportWindow{Start: start, End: end})
	if err := s.cfg.WriteString(ctx, syscfg.KeyExportWindow, value); err != nil {
		return err
	}
	return nil
}

// Store 返回数据访问层。
func (s *Service) Store() *Store { return s.store }

// Uploader 返回上传适配器。
func (s *Service) Uploader() Uploader { return s.uploader }

// JobRuntime 描述单个 Job 的调度方式、中文名称与运维说明（供 scheduler 与运维接口使用）。
type JobRuntime struct {
	Kind     string `json:"job_kind"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	Flow     string `json:"flow"`
	Details  string `json:"details"`
	Cautions string `json:"cautions"`
	DailyAt  string `json:"daily_at,omitempty"`
	Interval int    `json:"interval_seconds,omitempty"`
	Windowed bool   `json:"windowed"`
	Schedule string `json:"schedule"`
}

// JobRuntimes 返回全部 Job 的调度配置与中文说明。
// 每个 JOB 的说明位于 internal/meta/finv/quote/dedup/job 下独立的文件中。
func JobRuntimes() []JobRuntime {
	defs := jobdef.All()
	runtimes := make([]JobRuntime, 0, len(defs))
	for _, def := range defs {
		runtimes = append(runtimes, JobRuntime{
			Kind: def.Kind, Name: def.Name, Summary: def.Summary, Flow: def.Flow,
			Details: def.Details, Cautions: def.Cautions, DailyAt: def.DailyAt,
			Interval: def.Interval, Windowed: def.Windowed, Schedule: def.Schedule,
		})
	}
	return runtimes
}

// Run 执行一个 Job（默认按定时触发登记 CDEU）。
func (s *Service) Run(ctx context.Context, kind string) (Result, error) {
	return s.RunWithTrigger(ctx, kind, CDEUTriggerSchedule)
}

// RunWithTrigger 执行一个 Job：进程内同类 Job 互斥，执行前后写旧运行日志与 CDEU 运行树。
//
// triggerType 取 SCHEDULE / MANUAL：手动触发在当天已有终态流程时会创建新的 MANUAL 运行。
func (s *Service) RunWithTrigger(ctx context.Context, kind, triggerType string) (Result, error) {
	if !s.begin(kind) {
		return Result{}, fmt.Errorf("Job %s 正在执行中，跳过本次触发", kind)
	}
	defer s.end(kind)

	runID, err := s.store.StartRun(ctx, kind, nil, "", nil)
	if err != nil {
		return Result{}, fmt.Errorf("登记运行日志失败: %w", err)
	}
	return s.runWithTriggerRunID(ctx, kind, triggerType, runID)
}

// runWithTriggerRunID 在已登记的运行日志上执行 Job（调用方负责 begin/end 互斥）。
func (s *Service) runWithTriggerRunID(ctx context.Context, kind, triggerType string, runID int64) (Result, error) {
	// CDEU 是结构化观测层：登记失败只记日志，不能阻断原有归档链路。
	cdeuExecution, cdeuErr := s.beginCDEU(ctx, kind, triggerType)
	if cdeuErr != nil {
		log.Printf("[cdeu] CDEU 登记失败 job=%s err=%v", kind, cdeuErr)
	}

	// 把 run_id 放入上下文，供各 Job 上报阶段进度与心跳；CDEU 上下文用于阶段推进。
	runCtx := withTrigger(withRunID(ctx, runID), triggerType)
	if cdeuExecution != nil {
		cdeuExecution.setRunLogID(runID)
		runCtx = withCDEUExecution(runCtx, cdeuExecution)
	}
	result := Result{JobKind: kind, Trigger: triggerType}
	runErr := s.dispatch(runCtx, kind, &result)
	status := RunStatusSuccess
	errText := ""
	if runErr != nil {
		status = RunStatusFailed
		errText = runErr.Error()
	}
	result.Status = status

	finishCtx := context.WithoutCancel(ctx)
	message := joinMessage(errText, result.Message)
	if finishErr := s.store.FinishRun(finishCtx, runID, status, result.ProcessedRows, result.ProcessedBytes,
		message); finishErr != nil && runErr == nil {
		runErr = fmt.Errorf("更新运行日志失败: %w", finishErr)
	}
	s.finishCDEU(finishCtx, cdeuExecution, status, result.ProcessedRows, result.ProcessedBytes, message)
	return result, runErr
}

func (s *Service) begin(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[kind]; ok {
		return false
	}
	s.running[kind] = time.Now()
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
	_, err := s.TriggerWithOptions(context.Background(), kind, false)
	return err
}

// TriggerWithOptions 手动触发 Job 并立即返回 run_log_id，前端可立刻打开执行日志。
//
// autoContinue=false 只执行当前 JOB；true 时按工作流定义顺序继续执行其后的 JOB。
func (s *Service) TriggerWithOptions(ctx context.Context, kind string, autoContinue bool) (int64, error) {
	known := false
	for _, runtime := range JobRuntimes() {
		if runtime.Kind == kind {
			known = true
			break
		}
	}
	if !known {
		return 0, fmt.Errorf("未知 Job: %s", kind)
	}
	if !s.begin(kind) {
		return 0, fmt.Errorf("Job %s 正在执行中，跳过本次触发", kind)
	}
	runID, err := s.store.StartRun(ctx, kind, nil, "", nil)
	if err != nil {
		s.end(kind)
		return 0, fmt.Errorf("登记运行日志失败: %w", err)
	}

	go func() {
		_, runErr := s.runWithTriggerRunID(context.Background(), kind, CDEUTriggerManual, runID)
		s.end(kind)
		if runErr != nil {
			log.Printf("[cdeu] 手动触发 %s 失败: %v", kind, runErr)
		}
		if autoContinue {
			s.runSubsequentJobs(context.Background(), kind)
		}
	}()
	return runID, nil
}

// runSubsequentJobs 按 CDEU 工作流顺序执行指定 JOB 之后的 JOB（同 kind 只执行一次）。
func (s *Service) runSubsequentJobs(ctx context.Context, fromKind string) {
	definitions, err := s.store.CDEUWorkflowJobDefinitions(ctx, CDEUWorkflowDaily)
	if err != nil {
		log.Printf("[cdeu] 自动续跑读取工作流定义失败 from=%s err=%v", fromKind, err)
		return
	}
	seen := map[string]bool{fromKind: true}
	for _, definition := range subsequentJobDefinitions(definitions, fromKind) {
		if ctx.Err() != nil {
			return
		}
		if seen[definition.JobKind] {
			continue
		}
		seen[definition.JobKind] = true
		log.Printf("[cdeu] 手动续跑 JOB kind=%s job_code=%s", definition.JobKind, definition.JobCode)
		if _, runErr := s.RunWithTrigger(ctx, definition.JobKind, CDEUTriggerManual); runErr != nil {
			log.Printf("[cdeu] 手动续跑 JOB %s 失败: %v", definition.JobKind, runErr)
		}
	}
}

// subsequentJobDefinitions 返回工作流中 fromKind 最后一次出现之后的 JOB 定义。
//
// 同一 job_kind 可能对应多个内嵌阶段（如 RUN_DUE_EXPORT_JOBS），续跑必须从最后一个
// 内嵌阶段之后开始，避免重复执行当前 JOB。
func subsequentJobDefinitions(definitions []CDEUJobDefinition, fromKind string) []CDEUJobDefinition {
	last := -1
	for index, definition := range definitions {
		if definition.JobKind == fromKind {
			last = index
		}
	}
	if last < 0 {
		return nil
	}
	return definitions[last+1:]
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
