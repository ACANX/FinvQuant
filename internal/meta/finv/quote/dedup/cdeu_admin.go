package dedup

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// 本文件提供 CDEU 工作流控制台的只读聚合接口：
//   - 定义树：Workflow → Phase → Process → JOB → Step
//   - 当前/指定运行：PhaseRun / ProcessRun / JobRun / JobStepRun 状态叠加到定义树
//   - 最近运行历史
// 前端页面：/Meta/Finv/Quote/DeDup/Export/CDEU

// ErrCDEURunNotFound 指定 runId 不存在。
var ErrCDEURunNotFound = errors.New("CDEU 工作流运行实例不存在")

// CDEUOverview CDEU 工作流定义 + 指定运行树 + 历史运行。
type CDEUOverview struct {
	GeneratedAt         time.Time        `json:"generated_at"`
	CDEUMode            string           `json:"cdeu_mode"`
	CDEUScheduleEnabled bool             `json:"cdeu_schedule_enabled"`
	Workflow            CDEUWorkflowView `json:"workflow"`
	Run                 *CDEURunView     `json:"run"`
	Phases              []CDEUPhaseView  `json:"phases"`
	History             []CDEURunView    `json:"history"`
}

// CDEUWorkflowView 工作流定义视图。
type CDEUWorkflowView struct {
	WorkflowID       int64  `json:"workflow_id"`
	WorkflowCode     string `json:"workflow_code"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Version          int    `json:"version"`
	RunMode          string `json:"run_mode"`
	ScheduleTime     string `json:"schedule_time"`
	ScheduleTimezone string `json:"schedule_timezone"`
	ScheduleEnabled  bool   `json:"schedule_enabled"`
	CompletionRule   string `json:"completion_rule"`
}

// CDEURunView 工作流运行摘要。
type CDEURunView struct {
	RunID            int64      `json:"run_id"`
	WorkflowCode     string     `json:"workflow_code"`
	WorkflowVersion  int        `json:"workflow_version"`
	BizDate          string     `json:"biz_date"`
	TriggerType      string     `json:"trigger_type"`
	Status           string     `json:"status"`
	ScheduledAt      *time.Time `json:"scheduled_at"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	CompletionRule   *string    `json:"completion_rule_snapshot"`
	CompletionResult *string    `json:"completion_snapshot"`
	Operator         *string    `json:"operator"`
	Error            *string    `json:"error"`
	GmtCreate        time.Time  `json:"gmt_create"`
}

// CDEUPhaseView 阶段定义 + 阶段运行状态。
type CDEUPhaseView struct {
	PhaseID           int64             `json:"phase_id"`
	PhaseCode         string            `json:"phase_code"`
	Name              string            `json:"name"`
	SeqNo             int               `json:"seq_no"`
	Critical          bool              `json:"critical"`
	AffectsCompletion bool              `json:"affects_completion"`
	RunMode           string            `json:"run_mode"`
	BatchEnabled      bool              `json:"batch_enabled"`
	PhaseRunID        *int64            `json:"phase_run_id"`
	Status            string            `json:"status"`
	StartedAt         *time.Time        `json:"started_at"`
	FinishedAt        *time.Time        `json:"finished_at"`
	TotalCount        int               `json:"total_count"`
	SuccessCount      int               `json:"success_count"`
	FailedCount       int               `json:"failed_count"`
	Processes         []CDEUProcessView `json:"processes"`
}

// CDEUProcessView Process 定义 + 运行状态。
type CDEUProcessView struct {
	ProcessID         int64         `json:"process_id"`
	ProcessCode       string        `json:"process_code"`
	Name              string        `json:"name"`
	SeqNo             int           `json:"seq_no"`
	Critical          bool          `json:"critical"`
	AffectsCompletion bool          `json:"affects_completion"`
	RunMode           string        `json:"run_mode"`
	BatchEnabled      bool          `json:"batch_enabled"`
	BatchSource       *string       `json:"batch_source"`
	BatchKeyField     *string       `json:"batch_key_field"`
	BatchSize         *int          `json:"batch_size"`
	MaxParallel       int           `json:"max_parallel"`
	FailurePolicy     string        `json:"failure_policy"`
	Status            string        `json:"status"`
	StartedAt         *time.Time    `json:"started_at"`
	FinishedAt        *time.Time    `json:"finished_at"`
	BatchTotal        int           `json:"batch_total"`
	BatchSuccess      int           `json:"batch_success"`
	BatchFailed       int           `json:"batch_failed"`
	Error             *string       `json:"error"`
	Jobs              []CDEUJobView `json:"jobs"`
}

// CDEUJobView JOB 定义 + 最近一次 JOB 运行状态 + JOB 内 Step 状态。
type CDEUJobView struct {
	JobID             int64      `json:"job_id"`
	JobCode           string     `json:"job_code"`
	Name              string     `json:"name"`
	JobKind           string     `json:"job_kind"`
	ExecutionStage    *string    `json:"execution_stage"`
	SeqNo             int        `json:"seq_no"`
	Critical          bool       `json:"critical"`
	AffectsCompletion bool       `json:"affects_completion"`
	TriggerMode       string     `json:"trigger_mode"`
	ExecutionMode     string     `json:"execution_mode"`
	ManualRerun       bool       `json:"manual_rerun_enabled"`
	ScheduleKind      string     `json:"schedule_kind"`
	ScheduleValue     *string    `json:"schedule_value"`
	Status            string     `json:"status"`
	Attempt           int        `json:"attempt"`
	BatchKey          *string    `json:"batch_key"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	ProcessedRows     int64      `json:"processed_rows"`
	ProcessedBytes    int64      `json:"processed_bytes"`
	RunLogID          *int64     `json:"run_log_id"`
	Error             *string    `json:"error"`
	Instances         int        `json:"instances"`
	// FromCurrentRun 动态实例是否属于「当前展示的这次运行」。
	// 动态实例是跨运行累积的工作项（每个 usc+biz_month 只有一行，随每次运行更新状态），
	// 不属于当前运行但仍是有效在办项，需要展示并标记，否则最新一次运行没处理任何实例时整棵树为空。
	FromCurrentRun bool              `json:"from_current_run"`
	Steps          []CDEUJobStepView `json:"steps"`
	// 动态单证券 JOB 扩展字段；静态 JOB 均为空。
	DynamicJobID  *int64  `json:"dynamic_job_id,omitempty"`
	Usc           *string `json:"usc,omitempty"`
	BizMonth      *int    `json:"biz_month,omitempty"`
	ExportType    *string `json:"export_type,omitempty"`
	JobCategory   *string `json:"job_category,omitempty"`
	WorkflowRunID *int64  `json:"-"`
	FileID        *int64  `json:"file_id,omitempty"`
	FileType      *string `json:"file_type,omitempty"`
	FileVersion   *int    `json:"file_version,omitempty"`
	IsDynamic     bool    `json:"is_dynamic"`
}

// CDEUJobStepView JOB 内 Step 定义 + 最近一次 Step 运行状态。
type CDEUJobStepView struct {
	StepID            int64      `json:"step_id"`
	StepCode          string     `json:"step_code"`
	Name              string     `json:"name"`
	SeqNo             int        `json:"seq_no"`
	Critical          bool       `json:"critical"`
	AffectsCompletion bool       `json:"affects_completion"`
	StepRunID         *int64     `json:"step_run_id"`
	Status            string     `json:"status"`
	Attempt           int        `json:"attempt"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	ProcessedRows     int64      `json:"processed_rows"`
	ProcessedBytes    int64      `json:"processed_bytes"`
	Error             *string    `json:"error"`
}

// CDEUOverview 读取 CDEU 定义树、指定运行（runID=0 取最近一次）与历史运行。
func (s *Store) CDEUOverview(ctx context.Context, runID int64, historyLimit int) (*CDEUOverview, error) {
	if historyLimit <= 0 {
		historyLimit = 20
	}
	if historyLimit > 100 {
		historyLimit = 100
	}
	overview := &CDEUOverview{GeneratedAt: time.Now()}
	if err := s.pool.QueryRow(ctx, `
SELECT workflow_id, workflow_code, name, COALESCE(description, ''), version, run_mode,
       COALESCE(to_char(schedule_time, 'HH24:MI'), ''), schedule_timezone,
       schedule_enabled, completion_rule::text
  FROM cdeu.finv_quote_cdeu_workflow
 WHERE workflow_code = $1 AND enabled
 ORDER BY version DESC LIMIT 1`, CDEUWorkflowDaily).Scan(
		&overview.Workflow.WorkflowID, &overview.Workflow.WorkflowCode, &overview.Workflow.Name,
		&overview.Workflow.Description, &overview.Workflow.Version, &overview.Workflow.RunMode,
		&overview.Workflow.ScheduleTime, &overview.Workflow.ScheduleTimezone,
		&overview.Workflow.ScheduleEnabled, &overview.Workflow.CompletionRule); err != nil {
		return nil, err
	}
	workflowID := overview.Workflow.WorkflowID

	phases, err := s.cdeuDefinitions(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	overview.Phases = phases

	if runID > 0 {
		run, err := s.cdeuRunByID(ctx, runID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCDEURunNotFound
		}
		if err != nil {
			return nil, err
		}
		overview.Run = run
	} else {
		run, err := s.cdeuLatestRun(ctx, workflowID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		overview.Run = run
	}
	if overview.Run != nil {
		if err := s.attachCDEURun(ctx, overview); err != nil {
			return nil, err
		}
	}

	history, err := s.cdeuRunHistory(ctx, workflowID, historyLimit)
	if err != nil {
		return nil, err
	}
	overview.History = history
	return overview, nil
}

func (s *Store) cdeuDefinitions(ctx context.Context, workflowID int64) ([]CDEUPhaseView, error) {
	phases := make([]CDEUPhaseView, 0, 5)
	phaseIndex := map[int64]int{}
	rows, err := s.pool.Query(ctx, `
SELECT phase_id, phase_code, name, seq_no, critical, affects_completion, run_mode, batch_enabled
  FROM cdeu.finv_quote_cdeu_phase
 WHERE workflow_id = $1 AND enabled
 ORDER BY seq_no`, workflowID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var phase CDEUPhaseView
		if err := rows.Scan(&phase.PhaseID, &phase.PhaseCode, &phase.Name, &phase.SeqNo,
			&phase.Critical, &phase.AffectsCompletion, &phase.RunMode, &phase.BatchEnabled); err != nil {
			rows.Close()
			return nil, err
		}
		phase.Status = "NOT_STARTED"
		phase.Processes = []CDEUProcessView{}
		phases = append(phases, phase)
		phaseIndex[phase.PhaseID] = len(phases) - 1
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(phases) == 0 {
		return phases, nil
	}

	processIndex := map[int64]struct{ phase, process int }{}
	processRows, err := s.pool.Query(ctx, `
SELECT process_id, phase_id, process_code, name, seq_no, critical, affects_completion, run_mode,
       batch_enabled, batch_source, batch_key_field, batch_size, max_parallel, failure_policy
  FROM cdeu.finv_quote_cdeu_process
 WHERE phase_id = ANY($1)
   AND enabled
 ORDER BY phase_id, seq_no`, phaseIDs(phases))
	if err != nil {
		return nil, err
	}
	for processRows.Next() {
		var process CDEUProcessView
		var phaseID int64
		if err := processRows.Scan(&process.ProcessID, &phaseID, &process.ProcessCode, &process.Name,
			&process.SeqNo, &process.Critical, &process.AffectsCompletion, &process.RunMode,
			&process.BatchEnabled, &process.BatchSource, &process.BatchKeyField, &process.BatchSize,
			&process.MaxParallel, &process.FailurePolicy); err != nil {
			processRows.Close()
			return nil, err
		}
		process.Status = "NOT_STARTED"
		process.Jobs = []CDEUJobView{}
		index, ok := phaseIndex[phaseID]
		if !ok {
			continue
		}
		phases[index].Processes = append(phases[index].Processes, process)
		processIndex[process.ProcessID] = struct{ phase, process int }{index, len(phases[index].Processes) - 1}
	}
	processRows.Close()
	if err := processRows.Err(); err != nil {
		return nil, err
	}
	if len(processIndex) == 0 {
		return phases, nil
	}

	jobIndex := map[int64]struct{ phase, process, job int }{}
	jobIDs := make([]int64, 0, 13)
	jobRows, err := s.pool.Query(ctx, `
SELECT job_id, process_id, job_code, name, job_kind, execution_stage, seq_no, critical,
       affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
       schedule_kind, schedule_value
  FROM cdeu.finv_quote_cdeu_job
 WHERE process_id = ANY($1)
   AND enabled AND ui_visible
 ORDER BY process_id, seq_no`, processIDs(phases))
	if err != nil {
		return nil, err
	}
	for jobRows.Next() {
		var job CDEUJobView
		var processID int64
		if err := jobRows.Scan(&job.JobID, &processID, &job.JobCode, &job.Name, &job.JobKind,
			&job.ExecutionStage, &job.SeqNo, &job.Critical, &job.AffectsCompletion,
			&job.TriggerMode, &job.ExecutionMode, &job.ManualRerun, &job.ScheduleKind,
			&job.ScheduleValue); err != nil {
			jobRows.Close()
			return nil, err
		}
		job.Status = "NOT_STARTED"
		job.Steps = []CDEUJobStepView{}
		index, ok := processIndex[processID]
		if !ok {
			continue
		}
		process := &phases[index.phase].Processes[index.process]
		process.Jobs = append(process.Jobs, job)
		jobIndex[job.JobID] = struct{ phase, process, job int }{index.phase, index.process, len(process.Jobs) - 1}
		jobIDs = append(jobIDs, job.JobID)
	}
	jobRows.Close()
	if err := jobRows.Err(); err != nil {
		return nil, err
	}
	if len(jobIDs) == 0 {
		return phases, nil
	}

	stepRows, err := s.pool.Query(ctx, `
SELECT step_id, job_id, step_code, name, seq_no, critical, affects_completion
  FROM cdeu.finv_quote_cdeu_job_step
 WHERE job_id = ANY($1)
   AND enabled
 ORDER BY job_id, seq_no`, jobIDs)
	if err != nil {
		return nil, err
	}
	for stepRows.Next() {
		var step CDEUJobStepView
		var jobID int64
		if err := stepRows.Scan(&step.StepID, &jobID, &step.StepCode, &step.Name, &step.SeqNo,
			&step.Critical, &step.AffectsCompletion); err != nil {
			stepRows.Close()
			return nil, err
		}
		step.Status = "NOT_STARTED"
		index, ok := jobIndex[jobID]
		if !ok {
			continue
		}
		job := &phases[index.phase].Processes[index.process].Jobs[index.job]
		job.Steps = append(job.Steps, step)
	}
	stepRows.Close()
	if err := stepRows.Err(); err != nil {
		return nil, err
	}

	dynamicIDs := make([]int64, 0, 16)
	dynamicRows, err := s.pool.Query(ctx, `
SELECT dj.dynamic_job_id, dj.process_id, dj.job_code, dj.name, dj.usc, dj.biz_month, dj.export_type,
       dj.job_category, dj.file_id, dj.file_type, dj.file_version,
       dj.export_job_id, dj.run_log_id, dj.workflow_run_id, dj.seq_no, dj.status, dj.error,
       dj.gmt_start, dj.gmt_finish,
       COALESCE(f.row_count, rl.processed_rows, 0),
       COALESCE(f.file_size, rl.processed_bytes, 0)
  FROM cdeu.finv_quote_cdeu_job_dynamic dj
  LEFT JOIN cdeu.finv_quote_cdeu_job_run_log_export_file rl ON rl.run_id = dj.run_log_id
  LEFT JOIN cdeu.finv_quote_cdeu_export_file f ON f.file_id = dj.file_id
 WHERE dj.process_id = ANY($1)
 ORDER BY dj.process_id, dj.seq_no, dj.dynamic_job_id`, processIDs(phases))
	if err != nil {
		return nil, err
	}
	for dynamicRows.Next() {
		var job CDEUJobView
		var dynamicJobID, processID int64
		var exportType, jobCategory, fileType *string
		var exportJobID, fileID, dynamicRunLogID, dynamicWorkflowRunID *int64
		var fileVersion *int
		var usc, jobCode, jobName, status string
		var bizMonth, seqNo int
		var jobError *string
		var startedAt, finishedAt *time.Time
		var processedRows, processedBytes int64
		if err := dynamicRows.Scan(&dynamicJobID, &processID, &jobCode, &jobName, &usc, &bizMonth,
			&exportType, &jobCategory, &fileID, &fileType, &fileVersion,
			&exportJobID, &dynamicRunLogID, &dynamicWorkflowRunID, &seqNo, &status, &jobError,
			&startedAt, &finishedAt, &processedRows, &processedBytes); err != nil {
			dynamicRows.Close()
			return nil, err
		}
		index, ok := processIndex[processID]
		if !ok {
			continue
		}
		job.JobID = dynamicJobID
		job.DynamicJobID = &dynamicJobID
		job.JobCode = jobCode
		job.Name = jobName
		job.JobKind = "DYNAMIC_EXPORT"
		if jobCategory != nil && *jobCategory == "UPLOAD" {
			job.JobKind = "DYNAMIC_UPLOAD"
		}
		stage := "EXPORT"
		if exportType != nil && *exportType != "" {
			stage = *exportType
		} else if fileType != nil && *fileType != "" {
			stage = *fileType
		}
		job.ExecutionStage = &stage
		job.SeqNo = seqNo
		job.Critical = false
		job.AffectsCompletion = false
		job.TriggerMode = "DYNAMIC"
		job.ExecutionMode = "DYNAMIC"
		job.ManualRerun = false
		job.Usc = &usc
		job.BizMonth = &bizMonth
		job.ExportType = exportType
		job.JobCategory = jobCategory
		job.FileID = fileID
		job.FileType = fileType
		job.FileVersion = fileVersion
		job.RunLogID = dynamicRunLogID
		job.WorkflowRunID = dynamicWorkflowRunID
		job.IsDynamic = true
		job.Status = status
		job.Error = jobError
		job.StartedAt = startedAt
		job.FinishedAt = finishedAt
		job.ProcessedRows = processedRows
		job.ProcessedBytes = processedBytes
		job.Steps = []CDEUJobStepView{}
		process := &phases[index.phase].Processes[index.process]
		process.Jobs = append(process.Jobs, job)
		dynamicIDs = append(dynamicIDs, dynamicJobID)
	}
	dynamicRows.Close()
	if err := dynamicRows.Err(); err != nil {
		return nil, err
	}
	dynamicSteps := map[int64][]CDEUJobStepView{}
	if len(dynamicIDs) > 0 {
		stepRows, err := s.pool.Query(ctx, `
SELECT dynamic_step_id, dynamic_job_id, step_code, name, seq_no, status,
       started_at, finished_at, error
  FROM cdeu.finv_quote_cdeu_job_step_dynamic
 WHERE dynamic_job_id = ANY($1)
 ORDER BY dynamic_job_id, seq_no`, dynamicIDs)
		if err != nil {
			return nil, err
		}
		for stepRows.Next() {
			var step CDEUJobStepView
			var dynamicJobID int64
			if err := stepRows.Scan(&step.StepID, &dynamicJobID, &step.StepCode, &step.Name,
				&step.SeqNo, &step.Status, &step.StartedAt, &step.FinishedAt, &step.Error); err != nil {
				stepRows.Close()
				return nil, err
			}
			dynamicSteps[dynamicJobID] = append(dynamicSteps[dynamicJobID], step)
		}
		stepRows.Close()
		if err := stepRows.Err(); err != nil {
			return nil, err
		}
	}
	// 回填 Step 时通过动态 JOB ID 查找，避免 append 扩容导致切片元素指针失效。
	for phaseIndex := range phases {
		for processIndex := range phases[phaseIndex].Processes {
			for jobIndex := range phases[phaseIndex].Processes[processIndex].Jobs {
				job := &phases[phaseIndex].Processes[processIndex].Jobs[jobIndex]
				if !job.IsDynamic || job.DynamicJobID == nil {
					continue
				}
				if steps, ok := dynamicSteps[*job.DynamicJobID]; ok {
					job.Steps = steps
				}
			}
		}
	}
	return phases, nil
}

func phaseIDs(phases []CDEUPhaseView) []int64 {
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.PhaseID)
	}
	return ids
}

func processIDs(phases []CDEUPhaseView) []int64 {
	ids := make([]int64, 0, 13)
	for _, phase := range phases {
		for _, process := range phase.Processes {
			ids = append(ids, process.ProcessID)
		}
	}
	return ids
}

func (s *Store) cdeuRunByID(ctx context.Context, runID int64) (*CDEURunView, error) {
	var run CDEURunView
	err := s.pool.QueryRow(ctx, `
SELECT run_id, workflow_code, workflow_version, to_char(biz_date, 'YYYY-MM-DD'),
       trigger_type, status, scheduled_at, started_at, finished_at,
       completion_rule_snapshot::text, completion_snapshot::text, operator, error, gmt_create
  FROM cdeu.finv_quote_cdeu_workflow_run WHERE run_id = $1`, runID).Scan(
		&run.RunID, &run.WorkflowCode, &run.WorkflowVersion, &run.BizDate, &run.TriggerType,
		&run.Status, &run.ScheduledAt, &run.StartedAt, &run.FinishedAt,
		&run.CompletionRule, &run.CompletionResult, &run.Operator, &run.Error, &run.GmtCreate)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) cdeuLatestRun(ctx context.Context, workflowID int64) (*CDEURunView, error) {
	var run CDEURunView
	err := s.pool.QueryRow(ctx, `
SELECT run_id, workflow_code, workflow_version, to_char(biz_date, 'YYYY-MM-DD'),
       trigger_type, status, scheduled_at, started_at, finished_at,
       completion_rule_snapshot::text, completion_snapshot::text, operator, error, gmt_create
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE workflow_id = $1
 ORDER BY run_id DESC LIMIT 1`, workflowID).Scan(
		&run.RunID, &run.WorkflowCode, &run.WorkflowVersion, &run.BizDate, &run.TriggerType,
		&run.Status, &run.ScheduledAt, &run.StartedAt, &run.FinishedAt,
		&run.CompletionRule, &run.CompletionResult, &run.Operator, &run.Error, &run.GmtCreate)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) cdeuRunHistory(ctx context.Context, workflowID int64, limit int) ([]CDEURunView, error) {
	rows, err := s.pool.Query(ctx, `
SELECT run_id, workflow_code, workflow_version, to_char(biz_date, 'YYYY-MM-DD'),
       trigger_type, status, scheduled_at, started_at, finished_at,
       completion_rule_snapshot::text, completion_snapshot::text, operator, error, gmt_create
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE workflow_id = $1
 ORDER BY run_id DESC LIMIT $2`, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := make([]CDEURunView, 0, limit)
	for rows.Next() {
		var run CDEURunView
		if err := rows.Scan(&run.RunID, &run.WorkflowCode, &run.WorkflowVersion, &run.BizDate,
			&run.TriggerType, &run.Status, &run.ScheduledAt, &run.StartedAt, &run.FinishedAt,
			&run.CompletionRule, &run.CompletionResult, &run.Operator, &run.Error,
			&run.GmtCreate); err != nil {
			return nil, err
		}
		history = append(history, run)
	}
	return history, rows.Err()
}

// attachCDEURun 把指定运行的 Phase/Process/JOB/Step 运行状态叠加到定义树。
func (s *Store) attachCDEURun(ctx context.Context, overview *CDEUOverview) error {
	runID := overview.Run.RunID
	// 自 V200045 起 cdeu_job_dynamic 按运行隔离（唯一键含 workflow_run_id），
	// 本次运行持有自己的动态实例，因此只看属于当前运行的实例；
	// 旧实现因表结构是「(usc,biz_month) 全局唯一」而无法过滤，
	// 现在数据模型支持后恢复为严格按 runID 过滤。
	for phaseIndex := range overview.Phases {
		for processIndex := range overview.Phases[phaseIndex].Processes {
			process := &overview.Phases[phaseIndex].Processes[processIndex]
			jobs := process.Jobs[:0]
			for jobIndex := range process.Jobs {
				job := &process.Jobs[jobIndex]
				if job.IsDynamic && (job.WorkflowRunID == nil || *job.WorkflowRunID != runID) {
					continue
				}
				job.FromCurrentRun = job.IsDynamic
				jobs = append(jobs, *job)
			}
			process.Jobs = jobs
		}
	}
	phaseRuns := map[int64]*CDEUPhaseView{}
	rows, err := s.pool.Query(ctx, `
SELECT phase_run_id, phase_id, status, started_at, finished_at,
       total_count, success_count, failed_count, completion_snapshot::text
  FROM cdeu.finv_quote_cdeu_phase_run WHERE workflow_run_id = $1`, runID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var phaseRunID, phaseID int64
		var status string
		var startedAt, finishedAt *time.Time
		var total, success, failed int
		var snapshot *string
		if err := rows.Scan(&phaseRunID, &phaseID, &status, &startedAt, &finishedAt,
			&total, &success, &failed, &snapshot); err != nil {
			rows.Close()
			return err
		}
		phaseRuns[phaseID] = &CDEUPhaseView{
			PhaseRunID: &phaseRunID, Status: status, StartedAt: startedAt, FinishedAt: finishedAt,
			TotalCount: total, SuccessCount: success, FailedCount: failed,
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	processRuns := map[int64]*CDEUProcessView{}
	processRows, err := s.pool.Query(ctx, `
SELECT process_id, status, started_at, finished_at,
       batch_total, batch_success, batch_failed, error
  FROM cdeu.finv_quote_cdeu_process_run
 WHERE workflow_run_id = $1`, runID)
	if err != nil {
		return err
	}
	for processRows.Next() {
		var processID int64
		var status string
		var startedAt, finishedAt *time.Time
		var total, success, failed int
		var processError *string
		if err := processRows.Scan(&processID, &status, &startedAt, &finishedAt,
			&total, &success, &failed, &processError); err != nil {
			processRows.Close()
			return err
		}
		processRuns[processID] = &CDEUProcessView{
			ProcessID: processID, Status: status, StartedAt: startedAt, FinishedAt: finishedAt,
			BatchTotal: total, BatchSuccess: success, BatchFailed: failed, Error: processError,
		}
	}
	processRows.Close()
	if err := processRows.Err(); err != nil {
		return err
	}

	jobRuns := map[int64]CDEUJobView{}
	jobCounts := map[int64]int{}
	jobRows, err := s.pool.Query(ctx, `
SELECT jr.job_id, jr.status, jr.attempt, jr.started_at, jr.finished_at,
       COALESCE(NULLIF(jr.processed_rows, 0), rl.processed_rows, 0),
       COALESCE(NULLIF(jr.processed_bytes, 0), rl.processed_bytes, 0),
       jr.run_log_id, jr.dynamic_job_id, jr.error
  FROM cdeu.finv_quote_cdeu_job_run jr
  LEFT JOIN cdeu.finv_quote_cdeu_job_run_log_export_file rl ON rl.run_id = jr.run_log_id
 WHERE jr.workflow_run_id = $1
 ORDER BY jr.process_id, jr.job_id`, runID)
	if err != nil {
		return err
	}
	for jobRows.Next() {
		var jobView CDEUJobView
		var jobID int64
		if err := jobRows.Scan(&jobID, &jobView.Status, &jobView.Attempt,
			&jobView.StartedAt, &jobView.FinishedAt, &jobView.ProcessedRows, &jobView.ProcessedBytes,
			&jobView.RunLogID, &jobView.DynamicJobID, &jobView.Error); err != nil {
			jobRows.Close()
			return err
		}
		jobCounts[jobID]++
		jobView.Instances = jobCounts[jobID]
		jobRuns[jobID] = jobView
	}
	jobRows.Close()
	if err := jobRows.Err(); err != nil {
		return err
	}

	stepRuns := map[int64]CDEUJobStepView{}
	stepRows, err := s.pool.Query(ctx, `
SELECT jsr.step_id, jsr.status, jsr.attempt, jsr.started_at, jsr.finished_at,
       jsr.processed_rows, jsr.processed_bytes, jsr.error
  FROM cdeu.finv_quote_cdeu_job_step_run jsr
 WHERE jsr.workflow_run_id = $1
 ORDER BY jsr.job_id, jsr.seq_no`, runID)
	if err != nil {
		return err
	}
	for stepRows.Next() {
		var stepView CDEUJobStepView
		var stepID int64
		if err := stepRows.Scan(&stepID, &stepView.Status, &stepView.Attempt,
			&stepView.StartedAt, &stepView.FinishedAt, &stepView.ProcessedRows,
			&stepView.ProcessedBytes, &stepView.Error); err != nil {
			stepRows.Close()
			return err
		}
		stepView.StepID = stepID
		stepRuns[stepID] = stepView
	}
	stepRows.Close()
	if err := stepRows.Err(); err != nil {
		return err
	}

	for phaseIndex := range overview.Phases {
		phase := &overview.Phases[phaseIndex]
		if run, ok := phaseRuns[phase.PhaseID]; ok {
			phase.PhaseRunID = run.PhaseRunID
			phase.Status = run.Status
			phase.StartedAt = run.StartedAt
			phase.FinishedAt = run.FinishedAt
			phase.TotalCount = run.TotalCount
			phase.SuccessCount = run.SuccessCount
			phase.FailedCount = run.FailedCount
		} else {
			phase.Status = CDEUStatusPending
		}
		for processIndex := range phase.Processes {
			process := &phase.Processes[processIndex]
			if run, ok := processRuns[process.ProcessID]; ok {
				process.ProcessID = run.ProcessID
				process.Status = run.Status
				process.StartedAt = run.StartedAt
				process.FinishedAt = run.FinishedAt
				process.BatchTotal = run.BatchTotal
				process.BatchSuccess = run.BatchSuccess
				process.BatchFailed = run.BatchFailed
				process.Error = run.Error
			} else if len(process.Jobs) == 0 || phase.Status == CDEUStatusSuccess {
				// 历史运行可能在占位/新增 Process 登记前创建：
				// 空 Process，或所属阶段已成功的旧运行，按成功回填展示。
				process.Status = CDEUStatusSuccess
			} else {
				process.Status = CDEUStatusPending
			}
			for jobIndex := range process.Jobs {
				job := &process.Jobs[jobIndex]
				if job.IsDynamic {
					continue
				}
				if run, ok := jobRuns[job.JobID]; ok {
					job.Status = run.Status
					job.Attempt = run.Attempt
					job.StartedAt = run.StartedAt
					job.FinishedAt = run.FinishedAt
					job.ProcessedRows = run.ProcessedRows
					job.ProcessedBytes = run.ProcessedBytes
					job.RunLogID = run.RunLogID
					job.DynamicJobID = run.DynamicJobID
					job.Error = run.Error
					job.Instances = run.Instances
				} else if process.Status == CDEUStatusSuccess {
					// 兼容在历史已完成运行之后新增的 JOB（例如证券元数据获取）：
					// 所属 Process 已成功且无该 JOB 运行记录时，按成功回填展示。
					job.Status = CDEUStatusSuccess
				} else {
					job.Status = CDEUStatusPending
				}
				for stepIndex := range job.Steps {
					step := &job.Steps[stepIndex]
					if run, ok := stepRuns[step.StepID]; ok {
						step.StepRunID = run.StepRunID
						step.Status = run.Status
						step.Attempt = run.Attempt
						step.StartedAt = run.StartedAt
						step.FinishedAt = run.FinishedAt
						step.ProcessedRows = run.ProcessedRows
						step.ProcessedBytes = run.ProcessedBytes
						step.Error = run.Error
						continue
					}
					// 兼容 V100050 之前的历史运行：没有 Job Step 运行记录时，按 JOB 状态回填。
					switch job.Status {
					case CDEUStatusSuccess:
						step.Status = CDEUStatusSuccess
					case CDEUStatusFailed, CDEUStatusPartial:
						step.Status = job.Status
					default:
						step.Status = CDEUStatusPending
					}
				}
			}
		}
		// 以当前定义树重算阶段计数，兼容占位 Process 新增前创建的历史运行。
		total, success, failed := 0, 0, 0
		for _, process := range phase.Processes {
			if !process.AffectsCompletion {
				continue
			}
			total++
			if process.Status == CDEUStatusSuccess {
				success++
			}
			if process.Status == CDEUStatusFailed || process.Status == CDEUStatusPartial {
				failed++
			}
		}
		phase.TotalCount = total
		phase.SuccessCount = success
		phase.FailedCount = failed
		if phase.Status == CDEUStatusPending && total > 0 && success == total {
			phase.Status = CDEUStatusSuccess
		}
	}
	return nil
}
