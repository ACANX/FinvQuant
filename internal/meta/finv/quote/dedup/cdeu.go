package dedup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 本文件实现 CDEU（Collect → Dedup → Export → Upload）结构化运行层：
//   - 定义与运行实例登记在 cdeu.finv_quote_cdeu_* 表；
//   - 现有 10 个 Job 仍由原调度器触发，执行时双写 CDEU 运行表；
//   - RUN_DUE_EXPORT_JOBS 的 LOAD / EXPORT / REGISTER / CLEANUP 四个阶段登记为 4 条 JOB 运行；
//   - 每个 JOB 运行下登记串行 Step 运行；
//   - 运行结束后按完成条件逐级评估 Process / Phase / Workflow。
//
// 新模型：Workflow → Phase → Process → JOB → Step。
// 设计依据：Docs/Refactor/2610-007-CDEUProcessModelDesign.md。

const (
	// CDEUWorkflowDaily 当前唯一启用的端到端工作流。
	CDEUWorkflowDaily = "CDEU_DAILY"

	// CDEU 触发类型（cdeu_workflow_run.trigger_type）。
	CDEUTriggerSchedule = "SCHEDULE"
	CDEUTriggerManual   = "MANUAL"

	// CDEU 运行状态（与表 CHECK 约束一致）。
	CDEUStatusPending   = "PENDING"
	CDEUStatusReady     = "READY"
	CDEUStatusRunning   = "RUNNING"
	CDEUStatusSuccess   = "SUCCESS"
	CDEUStatusPartial   = "PARTIAL"
	CDEUStatusFailed    = "FAILED"
	CDEUStatusPaused    = "PAUSED"
	CDEUStatusCancelled = "CANCELLED"
	CDEUStatusSkipped   = "SKIPPED"

	// 作业运行触发方式（cdeu_job_run.trigger_type）。
	CDEUJobTriggerProcess     = "PROCESS"
	CDEUJobTriggerManualStage = "MANUAL_STAGE"
)

// ErrCDEUWorkflowActive 当天已有活动 CDEU 工作流，禁止重复启动。
var ErrCDEUWorkflowActive = errors.New("当天已有运行中的 CDEU 工作流，请等待完成后再启动")

// CDEUCompletionRule 完成条件 JSON（cdeu_*.completion_rule）。
type CDEUCompletionRule struct {
	Mode      string   `json:"mode"`
	Items     []string `json:"items,omitempty"`
	MinCount  *int     `json:"min_count,omitempty"`
	MinRatio  *float64 `json:"min_ratio,omitempty"`
	CountMode string   `json:"count_mode,omitempty"`
}

// CDEUJobStepDefinition JOB 内串行原子 Step 定义。
type CDEUJobStepDefinition struct {
	StepID            int64  `json:"step_id"`
	StepCode          string `json:"step_code"`
	Name              string `json:"name"`
	SeqNo             int    `json:"seq_no"`
	Critical          bool   `json:"critical"`
	AffectsCompletion bool   `json:"affects_completion"`
	TimeoutSeconds    *int   `json:"timeout_seconds,omitempty"`
}

// CDEUJobDefinition JOB 定义的扁平视图（含所属工作流 / 阶段 / Process / Step）。
type CDEUJobDefinition struct {
	WorkflowID        int64                   `json:"workflow_id"`
	WorkflowCode      string                  `json:"workflow_code"`
	WorkflowVersion   int                     `json:"workflow_version"`
	PhaseID           int64                   `json:"phase_id"`
	PhaseCode         string                  `json:"phase_code"`
	ProcessID         int64                   `json:"process_id"`
	ProcessCode       string                  `json:"process_code"`
	JobID             int64                   `json:"job_id"`
	JobCode           string                  `json:"job_code"`
	JobKind           string                  `json:"job_kind"`
	ExecutionStage    string                  `json:"execution_stage,omitempty"`
	SeqNo             int                     `json:"seq_no"`
	Critical          bool                    `json:"critical"`
	AffectsCompletion bool                    `json:"affects_completion"`
	ManualRerun       bool                    `json:"manual_rerun_enabled"`
	Steps             []CDEUJobStepDefinition `json:"steps"`
}

// CDEURunState 工作流运行实例的当前状态。
type CDEURunState struct {
	RunID      int64
	WorkflowID int64
	Status     string
	ReadOnly   bool // 工作流已终态：后续 JOB 只登记运行，不再改写历史阶段/Process 状态。
}

// cdeuJobStepRun JOB 内 Step 运行实例。
type cdeuJobStepRun struct {
	StepRunID int64
	StepCode  string
	Status    string
}

// cdeuRunKey 标识一次 JOB 运行实例。
//
// 自 V200044 起 cdeu_job_run 不再使用自增主键 job_run_id，改由
// (workflow_run_id, process_id, job_id) 三元组唯一确定一次运行实例，
// 因此所有定位 JOB 运行的入口统一用该结构体传参。
type cdeuRunKey struct {
	WorkflowRunID int64
	ProcessID     int64
	JobID         int64
}

// String 便于日志输出运行键。
func (k cdeuRunKey) String() string {
	return fmt.Sprintf("run=%d/process=%d/job=%d", k.WorkflowRunID, k.ProcessID, k.JobID)
}

// cdeuStageRun 一次执行中某个 JOB 及其中 Step 的运行实例。
type cdeuStageRun struct {
	Key     cdeuRunKey
	JobCode string
	Stage   string
	Status  string
	Steps   []cdeuJobStepRun
}

// CDEUExecution 一次 Job 执行的 CDEU 上下文（同一次 Service.Run 内共享）。
type CDEUExecution struct {
	mu         sync.Mutex
	RunID      int64
	WorkflowID int64
	PhaseID    int64
	PhaseRunID int64
	ProcessID  int64
	RunLogID   int64
	ReadOnly   bool
	Current    int
	Stages     []cdeuStageRun
}

type cdeuExecContextKey struct{}

func withCDEUExecution(ctx context.Context, execution *CDEUExecution) context.Context {
	return context.WithValue(ctx, cdeuExecContextKey{}, execution)
}

func cdeuExecutionFromContext(ctx context.Context) *CDEUExecution {
	execution, _ := ctx.Value(cdeuExecContextKey{}).(*CDEUExecution)
	return execution
}

// setRunLogID 关联旧运行日志（finv_dedup_job_run_log_export_file）。
func (e *CDEUExecution) setRunLogID(runID int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.RunLogID = runID
	e.mu.Unlock()
}

// beginCDEU 在 JOB 执行前登记 CDEU 运行树。
//
// 返回 nil 表示该 JOB 没有 CDEU 定义或登记失败；调用方应继续执行业务逻辑，
// 不能因为 CDEU 观测层异常而阻断原有归档链路。
func (s *Service) beginCDEU(ctx context.Context, kind, triggerType string) (*CDEUExecution, error) {
	defs, err := s.store.CDEUJobDefinitions(ctx, kind)
	if err != nil {
		return nil, err
	}
	if len(defs) == 0 {
		return nil, nil
	}

	run, err := s.store.EnsureCDEUWorkflowRun(ctx, defs[0], s.options.Today(), triggerType)
	if err != nil {
		return nil, err
	}

	execution := &CDEUExecution{
		RunID:      run.RunID,
		WorkflowID: run.WorkflowID,
		ReadOnly:   run.ReadOnly,
		Stages:     make([]cdeuStageRun, 0, len(defs)),
	}
	if run.ReadOnly {
		// 工作流已结束（典型场景：上传 / 回收 / 健康检查等 CONTINUOUS JOB，或终态后的重跑）。
		// 先补齐运行树：历史已完成运行可能缺少后新增的 Phase/Process（如批次清单 Process）。
		if seedErr := s.store.SeedCDEUProcessRuns(ctx, run.RunID, defs); seedErr != nil {
			return nil, seedErr
		}
		// 只更新每个 JOB 最近一次运行，避免 cdeu_job_run 按周期无限增长。
		phaseRunID, findErr := s.store.CDEUPhaseRunID(ctx, run.RunID, defs[0].PhaseID)
		if findErr != nil {
			return nil, findErr
		}
		execution.PhaseRunID = phaseRunID
		for _, definition := range defs {
			key := cdeuRunKey{WorkflowRunID: run.RunID, ProcessID: definition.ProcessID, JobID: definition.JobID}
			if insertErr := s.store.UpsertReadOnlyCDEUJobRun(ctx, key, nil); insertErr != nil {
				return nil, insertErr
			}
			steps, stepErr := s.store.ResetCDEUJobStepRuns(ctx, key, definition.Steps)
			if stepErr != nil {
				return nil, stepErr
			}
			stage := cdeuStageRun{
				Key:     key,
				JobCode: definition.JobCode,
				Stage:   definition.ExecutionStage,
				Status:  CDEUStatusRunning,
				Steps:   steps,
			}
			if len(stage.Steps) > 0 {
				if startErr := s.store.StartCDEUJobStepRun(ctx, stage.Steps[0].StepRunID); startErr != nil {
					return nil, startErr
				}
				stage.Steps[0].Status = CDEUStatusRunning
			}
			execution.Stages = append(execution.Stages, stage)
		}
		return execution, nil
	}

	if err := s.store.SeedCDEUProcessRuns(ctx, run.RunID, defs); err != nil {
		return nil, err
	}
	definition := defs[0]
	phaseRunID, err := s.store.CDEUPhaseRunID(ctx, run.RunID, definition.PhaseID)
	if err != nil {
		return nil, err
	}
	execution.PhaseID = definition.PhaseID
	execution.PhaseRunID = phaseRunID
	execution.ProcessID = definition.ProcessID

	for index, def := range defs {
		status := CDEUStatusPending
		attempt := 0
		if index == 0 {
			status = CDEUStatusRunning
			attempt = 1
		}
		key := cdeuRunKey{WorkflowRunID: run.RunID, ProcessID: def.ProcessID, JobID: def.JobID}
		if insertErr := s.store.InsertCDEUJobRun(ctx, key, def,
			CDEUJobTriggerProcess, status, attempt, nil); insertErr != nil {
			return nil, insertErr
		}
		steps, stepErr := s.store.InsertCDEUJobStepRuns(ctx, key, def.Steps)
		if stepErr != nil {
			return nil, stepErr
		}
		stage := cdeuStageRun{
			Key:     key,
			JobCode: def.JobCode,
			Stage:   def.ExecutionStage,
			Status:  status,
			Steps:   steps,
		}
		if status == CDEUStatusRunning && len(stage.Steps) > 0 {
			if startErr := s.store.StartCDEUJobStepRun(ctx, stage.Steps[0].StepRunID); startErr != nil {
				return nil, startErr
			}
			stage.Steps[0].Status = CDEUStatusRunning
		}
		execution.Stages = append(execution.Stages, stage)
	}
	if err := s.store.TouchCDEUHierarchy(ctx, run.RunID, definition.PhaseID, phaseRunID,
		definition.ProcessID); err != nil {
		return nil, err
	}
	return execution, nil
}

// advanceCDEUStage 推进内嵌执行阶段（仅 RUN_DUE_EXPORT_JOBS 的 4 个登记 JOB 生效）。
//
// 只允许向前推进；重复上报较早阶段（例如逐任务上报 LOAD/EXPORT）会被忽略。
func (s *Service) advanceCDEUStage(ctx context.Context, stage string) {
	execution := cdeuExecutionFromContext(ctx)
	if execution == nil || execution.ReadOnly || stage == "" {
		return
	}
	execution.mu.Lock()
	defer execution.mu.Unlock()

	next := -1
	for index := range execution.Stages {
		if execution.Stages[index].Stage == stage {
			next = index
			break
		}
	}
	if next < 0 || next <= execution.Current {
		return
	}

	current := &execution.Stages[execution.Current]
	if current.Status == CDEUStatusRunning {
		if err := s.store.FinishCDEUJobRun(context.WithoutCancel(ctx), current.Key,
			CDEUStatusSuccess, 0, 0, execution.RunLogID, ""); err != nil {
			log.Printf("[cdeu] CDEU JOB 完成失败 key=%s err=%v", current.Key, err)
			return
		}
		s.finishCDEUJobSteps(ctx, current, CDEUStatusSuccess, 0, 0, "")
		current.Status = CDEUStatusSuccess
	}
	nextRun := &execution.Stages[next]
	if err := s.store.StartCDEUJobRun(ctx, nextRun.Key, execution.RunLogID); err != nil {
		log.Printf("[cdeu] CDEU JOB 启动失败 key=%s err=%v", nextRun.Key, err)
		return
	}
	nextRun.Status = CDEUStatusRunning
	if len(nextRun.Steps) > 0 {
		if err := s.store.StartCDEUJobStepRun(ctx, nextRun.Steps[0].StepRunID); err != nil {
			log.Printf("[cdeu] CDEU Step 启动失败 step_run_id=%d err=%v", nextRun.Steps[0].StepRunID, err)
		} else {
			nextRun.Steps[0].Status = CDEUStatusRunning
		}
	}
	execution.Current = next
	if err := s.store.TouchCDEUHierarchy(ctx, execution.RunID, execution.PhaseID,
		execution.PhaseRunID, execution.ProcessID); err != nil {
		log.Printf("[cdeu] CDEU 阶段状态更新失败 run_id=%d err=%v", execution.RunID, err)
	}
}

func (s *Service) finishCDEUJobSteps(ctx context.Context, stage *cdeuStageRun,
	status string, rows, bytes int64, errText string) {
	for index := range stage.Steps {
		step := &stage.Steps[index]
		if step.Status != CDEUStatusPending && step.Status != CDEUStatusRunning {
			continue
		}
		if err := s.store.FinishCDEUJobStepRun(ctx, step.StepRunID, status, rows, bytes, errText); err != nil {
			log.Printf("[cdeu] CDEU Step 收口失败 step_run_id=%d err=%v", step.StepRunID, err)
			continue
		}
		step.Status = status
	}
}

// finishCDEU 结束 CDEU JOB 运行并逐级评估完成条件。
func (s *Service) finishCDEU(ctx context.Context, execution *CDEUExecution, status string,
	rows, bytes int64, errText string) {
	if execution == nil {
		return
	}
	execution.mu.Lock()
	current := &execution.Stages[execution.Current]
	jobStatus := cdeuJobStatus(status)
	if execution.ReadOnly {
		for index := range execution.Stages {
			stageRun := &execution.Stages[index]
			stageRows, stageBytes := int64(0), int64(0)
			if index == execution.Current {
				stageRows, stageBytes = rows, bytes
			}
			if err := s.store.FinishCDEUJobRun(ctx, stageRun.Key, jobStatus, stageRows, stageBytes,
				execution.RunLogID, errText); err != nil {
				log.Printf("[cdeu] CDEU JOB 运行收口失败 key=%s err=%v", stageRun.Key, err)
			}
			s.finishCDEUJobSteps(ctx, stageRun, jobStatus, stageRows, stageBytes, errText)
			stageRun.Status = jobStatus
		}
		execution.mu.Unlock()
		if evalErr := s.store.EvaluateCDEURunProcesses(ctx, execution.RunID); evalErr != nil {
			log.Printf("[cdeu] CDEU 只读运行 Process 完成判定失败 run_id=%d err=%v", execution.RunID, evalErr)
		}
		return
	}

	if err := s.store.FinishCDEUJobRun(ctx, current.Key, jobStatus, rows, bytes,
		execution.RunLogID, errText); err != nil {
		log.Printf("[cdeu] CDEU JOB 运行收口失败 key=%s err=%v", current.Key, err)
	}
	s.finishCDEUJobSteps(ctx, current, jobStatus, rows, bytes, errText)
	current.Status = jobStatus

	for index := range execution.Stages {
		if index == execution.Current {
			continue
		}
		stageRun := &execution.Stages[index]
		if stageRun.Status != CDEUStatusPending {
			continue
		}
		// 整体成功时，未被逐任务触发的阶段（例如本轮没有导出候选，直接跳到 REGISTER/CLEANUP）
		// 按「空阶段成功」收口，避免遗留 PENDING 导致 Process 永远 RUNNING。
		stageStatus := CDEUStatusSkipped
		stageMessage := "前序阶段未完成"
		if jobStatus == CDEUStatusSuccess {
			stageStatus = CDEUStatusSuccess
			stageMessage = ""
		}
		if err := s.store.FinishCDEUJobRun(context.WithoutCancel(ctx), stageRun.Key,
			stageStatus, 0, 0, execution.RunLogID, stageMessage); err != nil {
			log.Printf("[cdeu] CDEU 收口未执行 JOB 失败 key=%s err=%v", stageRun.Key, err)
		}
		s.finishCDEUJobSteps(context.WithoutCancel(ctx), stageRun, stageStatus, 0, 0, stageMessage)
		stageRun.Status = stageStatus
	}
	execution.mu.Unlock()

	if err := s.store.EvaluateCDEURunProcesses(ctx, execution.RunID); err != nil {
		log.Printf("[cdeu] CDEU Process 完成判定失败 run_id=%d err=%v", execution.RunID, err)
	}
	if err := s.store.EvaluateCDEUWorkflow(ctx, execution.RunID); err != nil {
		log.Printf("[cdeu] CDEU Workflow 完成判定失败 run_id=%d err=%v", execution.RunID, err)
	}
}

// cdeuJobStatus 把旧运行日志状态映射为 CDEU 状态。
func cdeuJobStatus(status string) string {
	switch status {
	case RunStatusSuccess:
		return CDEUStatusSuccess
	case RunStatusPaused:
		return CDEUStatusPaused
	default:
		return CDEUStatusFailed
	}
}

// StartCDEUWorkflow 手动启动一次完整 CDEU 工作流。
//
// 创建独立的 MANUAL 运行实例后，在后台按 Phase/Process/JOB 顺序依次触发所有直接 JOB；
// 导出阶段只触发一次 RUN_DUE_EXPORT_JOBS（内部仍走 LOAD → EXPORT → REGISTER → CLEANUP）。
// TerminateCDEUWorkflow 强制终止指定（或最近一次运行中的）CDEU 工作流。
//
// runID <= 0 时自动取最近一次 PENDING/READY/RUNNING 的运行。
func (s *Service) TerminateCDEUWorkflow(ctx context.Context, runID int64) (CDEUTerminateResult, error) {
	if runID <= 0 {
		latest, err := s.store.LatestActiveCDEURunID(ctx)
		if err != nil {
			return CDEUTerminateResult{}, err
		}
		if latest <= 0 {
			return CDEUTerminateResult{}, ErrCDEUWorkflowActive
		}
		runID = latest
	}
	return s.store.TerminateCDEURun(ctx, runID)
}

func (s *Service) StartCDEUWorkflow(ctx context.Context, operator string) (int64, error) {
	definitions, err := s.store.CDEUWorkflowJobDefinitions(ctx, CDEUWorkflowDaily)
	if err != nil {
		return 0, err
	}
	if len(definitions) == 0 {
		return 0, fmt.Errorf("CDEU 未登记 %s 工作流定义", CDEUWorkflowDaily)
	}
	run, err := s.store.CreateCDEUManualRun(ctx, definitions[0], s.options.Today(), operator)
	if err != nil {
		return 0, err
	}

	// 按定义顺序去重 job_kind；EMBEDDED 的 4 个阶段只保留一次父 JOB。
	kinds := make([]string, 0, 10)
	seen := map[string]bool{}
	for _, definition := range definitions {
		if seen[definition.JobKind] {
			continue
		}
		seen[definition.JobKind] = true
		kinds = append(kinds, definition.JobKind)
	}

	go func() {
		for _, kind := range kinds {
			if _, runErr := s.RunWithTrigger(context.Background(), kind, CDEUTriggerManual); runErr != nil {
				log.Printf("[cdeu] CDEU 手动工作流 JOB %s 执行失败: %v", kind, runErr)
			}
		}
	}()
	return run.RunID, nil
}

// CDEUOverview 返回 CDEU 定义树、指定运行（runID=0 取最近一次）与历史运行。
// CDEUOverview 查询 CDEU 工作流总览。
//
// 除定义树与运行状态外，额外带上系统配置 20005 的运行模式（Manual / Auto），
// 供前端页头直接展示「运行模式：手动 / 自动」，避免额外请求 Status 接口。
func (s *Service) CDEUOverview(ctx context.Context, runID int64, historyLimit int) (*CDEUOverview, error) {
	overview, err := s.store.CDEUOverview(ctx, runID, historyLimit)
	if err != nil {
		return nil, err
	}
	mode, modeErr := s.CDEUMode(ctx)
	if modeErr != nil || mode == "" {
		mode = syscfg.CDEUModeManual
	}
	overview.CDEUMode = mode
	overview.CDEUScheduleEnabled = mode == syscfg.CDEUModeAuto
	return overview, nil
}

// RunLogDetail 查询单次 Job 运行的汇总与详细日志行。
func (s *Service) RunLogDetail(ctx context.Context, runID int64) (*RunLogDetail, error) {
	return s.store.RunLogDetail(ctx, runID)
}

// CDEUJobRuntimes 用 CDEU 定义覆盖进程内调度计划（schedule_kind/value / windowed）。
//
// 调度器启动时调用；数据库不可用或某 JOB 未登记时保留 JobRuntimes() 的默认值，
// 避免 CDEU 定义表异常导致原有调度停摆。RUN_DUE_EXPORT_JOBS 为 4 个内嵌阶段，
// schedule_kind=NONE，继续使用父 JOB 的窗口内固定间隔配置。
func (s *Service) CDEUJobRuntimes(ctx context.Context) []JobRuntime {
	runtimes := JobRuntimes()
	for index := range runtimes {
		runtime := &runtimes[index]
		var scheduleKind, scheduleValue string
		var windowed bool
		err := s.store.pool.QueryRow(ctx, `
SELECT j.schedule_kind, COALESCE(j.schedule_value, ''),
       COALESCE((j.job_params->>'windowed')::boolean, false)
  FROM cdeu.finv_quote_cdeu_job j
  JOIN cdeu.finv_quote_cdeu_process pr ON pr.process_id = j.process_id
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = pr.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE j.job_kind = $1 AND j.enabled AND pr.enabled AND ph.enabled AND w.enabled
   AND j.schedule_kind <> 'NONE'
   AND w.version = (
       SELECT max(w2.version) FROM cdeu.finv_quote_cdeu_workflow w2
        WHERE w2.workflow_code = w.workflow_code AND w2.enabled
   )
 ORDER BY j.job_id LIMIT 1`, runtime.Kind).Scan(&scheduleKind, &scheduleValue, &windowed)
		if err != nil {
			continue
		}
		switch scheduleKind {
		case "DAILY":
			runtime.DailyAt = scheduleValue
			runtime.Interval = 0
			runtime.Schedule = "每日 " + scheduleValue
		case "INTERVAL":
			var seconds int
			if _, parseErr := fmt.Sscanf(scheduleValue, "%d", &seconds); parseErr == nil && seconds > 0 {
				runtime.Interval = seconds
				runtime.DailyAt = ""
				runtime.Schedule = fmt.Sprintf("每 %d 秒", seconds)
			}
		}
		runtime.Windowed = runtime.Windowed || windowed
	}
	return runtimes
}

// CDEUDiagnostic 返回 CDEU 定义与当前工作流运行摘要，供启动日志与联调使用。
func (s *Service) CDEUDiagnostic(ctx context.Context) (string, error) {
	defs, err := s.store.CDEUJobDefinitions(ctx, JobRunDueExportJobs)
	if err != nil {
		return "", err
	}
	if len(defs) == 0 {
		return "CDEU 未登记 CDEU_DAILY", nil
	}
	run, err := s.store.EnsureCDEUWorkflowRun(ctx, defs[0], s.options.Today(), CDEUTriggerSchedule)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CDEU_DAILY run_id=%d status=%s read_only=%t", run.RunID, run.Status, run.ReadOnly), nil
}

// marshalCDEUCompletion 生成完成判定快照。
func marshalCDEUCompletion(status string, completed, total, failed int) []byte {
	payload := map[string]any{
		"status":       status,
		"completed":    completed,
		"total":        total,
		"failed":       failed,
		"evaluated_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	return data
}
