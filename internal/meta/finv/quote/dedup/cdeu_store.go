package dedup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 本文件是 CDEU 运行层的数据访问实现（cdeu.finv_quote_cdeu_*）。
// 约束：所有表名 schema 全限定；不加外键；定义表只读，运行表按运行实例写入。
// 新模型：Workflow → Phase → Process → JOB → Step。

// cdeuChild 完成条件评估时的子级快照。
type cdeuChild struct {
	Code     string
	Status   string
	Critical bool
	Affects  bool
}

// CDEUJobDefinitions 读取指定 job_kind 的启用 JOB 定义（含工作流/阶段/Process/Step）。
func (s *Store) CDEUJobDefinitions(ctx context.Context, kind string) ([]CDEUJobDefinition, error) {
	rows, err := s.pool.Query(ctx, `
SELECT w.workflow_id, w.workflow_code, w.version,
       ph.phase_id, ph.phase_code,
       pr.process_id, pr.process_code,
       j.job_id, j.job_code, j.job_kind, COALESCE(j.execution_stage, ''),
       j.seq_no, j.critical, j.affects_completion, j.manual_rerun_enabled
  FROM cdeu.finv_quote_cdeu_job j
  JOIN cdeu.finv_quote_cdeu_process pr ON pr.process_id = j.process_id
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = pr.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE j.job_kind = $1
   AND j.enabled AND pr.enabled AND ph.enabled AND w.enabled
   AND w.version = (
       SELECT max(w2.version) FROM cdeu.finv_quote_cdeu_workflow w2
        WHERE w2.workflow_code = w.workflow_code AND w2.enabled
   )
 ORDER BY ph.seq_no, pr.seq_no, j.seq_no`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := make([]CDEUJobDefinition, 0, 4)
	for rows.Next() {
		var definition CDEUJobDefinition
		if err := rows.Scan(&definition.WorkflowID, &definition.WorkflowCode, &definition.WorkflowVersion,
			&definition.PhaseID, &definition.PhaseCode, &definition.ProcessID, &definition.ProcessCode,
			&definition.JobID, &definition.JobCode, &definition.JobKind, &definition.ExecutionStage,
			&definition.SeqNo, &definition.Critical, &definition.AffectsCompletion,
			&definition.ManualRerun); err != nil {
			return nil, err
		}
		definition.Steps = []CDEUJobStepDefinition{}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return definitions, s.attachCDEUJobSteps(ctx, definitions)
}

// CDEUWorkflowJobDefinitions 读取工作流下全部启用 JOB 定义（按阶段/Process/JOB 顺序）。
func (s *Store) CDEUWorkflowJobDefinitions(ctx context.Context, workflowCode string) ([]CDEUJobDefinition, error) {
	rows, err := s.pool.Query(ctx, `
SELECT w.workflow_id, w.workflow_code, w.version,
       ph.phase_id, ph.phase_code,
       pr.process_id, pr.process_code,
       j.job_id, j.job_code, j.job_kind, COALESCE(j.execution_stage, ''),
       j.seq_no, j.critical, j.affects_completion, j.manual_rerun_enabled
  FROM cdeu.finv_quote_cdeu_job j
  JOIN cdeu.finv_quote_cdeu_process pr ON pr.process_id = j.process_id
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = pr.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = $1
   AND j.enabled AND pr.enabled AND ph.enabled AND w.enabled
   AND w.version = (
       SELECT max(w2.version) FROM cdeu.finv_quote_cdeu_workflow w2
        WHERE w2.workflow_code = w.workflow_code AND w2.enabled
   )
 ORDER BY ph.seq_no, pr.seq_no, j.seq_no`, workflowCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := make([]CDEUJobDefinition, 0, 13)
	for rows.Next() {
		var definition CDEUJobDefinition
		if err := rows.Scan(&definition.WorkflowID, &definition.WorkflowCode, &definition.WorkflowVersion,
			&definition.PhaseID, &definition.PhaseCode, &definition.ProcessID, &definition.ProcessCode,
			&definition.JobID, &definition.JobCode, &definition.JobKind, &definition.ExecutionStage,
			&definition.SeqNo, &definition.Critical, &definition.AffectsCompletion,
			&definition.ManualRerun); err != nil {
			return nil, err
		}
		definition.Steps = []CDEUJobStepDefinition{}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return definitions, s.attachCDEUJobSteps(ctx, definitions)
}

// attachCDEUJobSteps 一次性补齐 JOB 下的启用 Step。
func (s *Store) attachCDEUJobSteps(ctx context.Context, definitions []CDEUJobDefinition) error {
	if len(definitions) == 0 {
		return nil
	}
	jobIDs := make([]int64, 0, len(definitions))
	index := map[int64][]int{}
	for position := range definitions {
		jobIDs = append(jobIDs, definitions[position].JobID)
		index[definitions[position].JobID] = append(index[definitions[position].JobID], position)
	}
	rows, err := s.pool.Query(ctx, `
SELECT job_id, step_id, step_code, name, seq_no, critical, affects_completion, timeout_seconds
  FROM cdeu.finv_quote_cdeu_job_step
 WHERE job_id = ANY($1) AND enabled
 ORDER BY job_id, seq_no`, jobIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var jobID int64
		var step CDEUJobStepDefinition
		if err := rows.Scan(&jobID, &step.StepID, &step.StepCode, &step.Name, &step.SeqNo,
			&step.Critical, &step.AffectsCompletion, &step.TimeoutSeconds); err != nil {
			return err
		}
		for _, position := range index[jobID] {
			definitions[position].Steps = append(definitions[position].Steps, step)
		}
	}
	return rows.Err()
}

// CreateCDEUManualRun 为「启动工作流」显式创建 MANUAL 运行；已有活动运行时返回 ErrCDEUWorkflowActive。
func (s *Store) CreateCDEUManualRun(ctx context.Context, definition CDEUJobDefinition,
	bizDate time.Time, operator string) (CDEURunState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CDEURunState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`,
		fmt.Sprintf("%s:%s", definition.WorkflowCode, bizDate.Format("2006-01-02"))); err != nil {
		return CDEURunState{}, err
	}
	var activeRunID int64
	err = tx.QueryRow(ctx, `
SELECT run_id FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE workflow_id = $1 AND biz_date = $2
   AND status IN ('PENDING','READY','RUNNING')
 ORDER BY run_id DESC LIMIT 1`, definition.WorkflowID, bizDate).Scan(&activeRunID)
	if err == nil {
		return CDEURunState{}, ErrCDEUWorkflowActive
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CDEURunState{}, err
	}

	runKey := fmt.Sprintf("%s:%s:MANUAL:%d", definition.WorkflowCode,
		bizDate.Format("2006-01-02"), time.Now().UnixNano())
	var runID int64
	if err := tx.QueryRow(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_workflow_run
    (workflow_id, workflow_code, workflow_version, run_key, biz_date, trigger_type,
     status, scheduled_at, completion_rule_snapshot, operator)
SELECT workflow_id, workflow_code, version, $3, $4::date, 'MANUAL',
       'PENDING', now(), completion_rule, NULLIF($5, '')
  FROM cdeu.finv_quote_cdeu_workflow
 WHERE workflow_id = $1 AND version = $2
RETURNING run_id`, definition.WorkflowID, definition.WorkflowVersion, runKey, bizDate, operator).Scan(&runID); err != nil {
		return CDEURunState{}, err
	}
	if err := seedCDEUHierarchyTx(ctx, tx, runID, definition.WorkflowID); err != nil {
		return CDEURunState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CDEURunState{}, err
	}
	return CDEURunState{RunID: runID, WorkflowID: definition.WorkflowID, Status: CDEUStatusPending}, nil
}

// EnsureCDEUWorkflowRun 找到当天活动工作流运行；没有则创建，并初始化 Phase/Process 运行树。
//
// 自动运行使用 workflow_code:biz_date:SCHEDULE 幂等键；手动运行追加纳秒时间戳，
// 允许同一业务日多次补跑。若当天已有活动运行，手动/自动都复用它，避免并发写同一工作流。
func (s *Store) EnsureCDEUWorkflowRun(ctx context.Context, definition CDEUJobDefinition,
	bizDate time.Time, triggerType string) (CDEURunState, error) {
	if triggerType != CDEUTriggerManual {
		triggerType = CDEUTriggerSchedule
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CDEURunState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 同一工作流 + 业务日串行化，避免并发 JOB 同时创建运行实例。
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`,
		fmt.Sprintf("%s:%s", definition.WorkflowCode, bizDate.Format("2006-01-02"))); err != nil {
		return CDEURunState{}, err
	}

	var state CDEURunState
	err = tx.QueryRow(ctx, `
SELECT run_id, workflow_id, status
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE workflow_id = $1 AND biz_date = $2
   AND status IN ('PENDING','READY','RUNNING')
 ORDER BY run_id DESC LIMIT 1`, definition.WorkflowID, bizDate).
		Scan(&state.RunID, &state.WorkflowID, &state.Status)
	if err == nil {
		if err := seedCDEUHierarchyTx(ctx, tx, state.RunID, definition.WorkflowID); err != nil {
			return CDEURunState{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return CDEURunState{}, err
		}
		return state, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CDEURunState{}, err
	}

	// 当天没有活动运行，但已有终态运行：JOB 重试/补跑只登记只读运行，不再新建工作流实例。
	err = tx.QueryRow(ctx, `
SELECT run_id, workflow_id, status
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE workflow_id = $1 AND biz_date = $2
 ORDER BY run_id DESC LIMIT 1`, definition.WorkflowID, bizDate).
		Scan(&state.RunID, &state.WorkflowID, &state.Status)
	if err == nil {
		if !isCDEUTerminal(state.Status) {
			if err := seedCDEUHierarchyTx(ctx, tx, state.RunID, definition.WorkflowID); err != nil {
				return CDEURunState{}, err
			}
		}
		state.ReadOnly = isCDEUTerminal(state.Status)
		if err := tx.Commit(ctx); err != nil {
			return CDEURunState{}, err
		}
		return state, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CDEURunState{}, err
	}

	runKey := fmt.Sprintf("%s:%s:SCHEDULE", definition.WorkflowCode, bizDate.Format("2006-01-02"))
	if triggerType == CDEUTriggerManual {
		runKey = fmt.Sprintf("%s:%s:MANUAL:%d", definition.WorkflowCode,
			bizDate.Format("2006-01-02"), time.Now().UnixNano())
	}
	var runID int64
	err = tx.QueryRow(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_workflow_run
    (workflow_id, workflow_code, workflow_version, run_key, biz_date, trigger_type,
     status, scheduled_at, completion_rule_snapshot)
SELECT workflow_id, workflow_code, version, $3, $4::date, $5, 'PENDING', now(), completion_rule
  FROM cdeu.finv_quote_cdeu_workflow
 WHERE workflow_id = $1 AND version = $2
ON CONFLICT (run_key) DO NOTHING
RETURNING run_id`, definition.WorkflowID, definition.WorkflowVersion, runKey, bizDate, triggerType).
		Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		// 幂等键已存在（可能是上一次自动运行已终态）：读取该运行，保持只读。
		err = tx.QueryRow(ctx, `
SELECT run_id, workflow_id, status FROM cdeu.finv_quote_cdeu_workflow_run WHERE run_key = $1`, runKey).
			Scan(&state.RunID, &state.WorkflowID, &state.Status)
		if err != nil {
			return CDEURunState{}, err
		}
		state.ReadOnly = isCDEUTerminal(state.Status)
		if err := tx.Commit(ctx); err != nil {
			return CDEURunState{}, err
		}
		return state, nil
	}
	if err != nil {
		return CDEURunState{}, err
	}
	state = CDEURunState{RunID: runID, WorkflowID: definition.WorkflowID, Status: CDEUStatusPending}
	if err := seedCDEUHierarchyTx(ctx, tx, runID, definition.WorkflowID); err != nil {
		return CDEURunState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CDEURunState{}, err
	}
	return state, nil
}

// SeedCDEUProcessRuns 幂等补齐某次工作流运行的 Phase/Process 运行实例。
func (s *Store) SeedCDEUProcessRuns(ctx context.Context, runID int64, definitions []CDEUJobDefinition) error {
	if len(definitions) == 0 {
		return nil
	}
	return seedCDEUHierarchyTx(ctx, s.pool, runID, definitions[0].WorkflowID)
}

// cdeuExecer 统一 *pgxpool.Pool 与 pgx.Tx 的最小 Exec 接口。
type cdeuExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// seedCDEUHierarchyTx 在事务或连接池上初始化阶段/Process 运行实例（幂等）。
func seedCDEUHierarchyTx(ctx context.Context, db cdeuExecer, runID, workflowID int64) error {
	if _, err := db.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_phase_run
    (workflow_run_id, phase_id, phase_code, seq_no, status, completion_rule_snapshot)
SELECT $1, ph.phase_id, ph.phase_code, ph.seq_no, 'PENDING', ph.completion_rule
  FROM cdeu.finv_quote_cdeu_phase ph
 WHERE ph.workflow_id = $2 AND ph.enabled
ON CONFLICT (workflow_run_id, phase_code) DO NOTHING`, runID, workflowID); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_process_run
    (workflow_run_id, phase_run_id, process_id, process_code, seq_no, status, completion_rule_snapshot)
SELECT pr.workflow_run_id, pr.phase_run_id, ps.process_id, ps.process_code, ps.seq_no,
       'PENDING', ps.completion_rule
  FROM cdeu.finv_quote_cdeu_process ps
  JOIN cdeu.finv_quote_cdeu_phase_run pr ON pr.workflow_run_id = $1 AND pr.phase_id = ps.phase_id
 WHERE ps.enabled
ON CONFLICT (workflow_run_id, process_id) DO NOTHING`, runID); err != nil {
		return err
	}
	// 占位 Process（当前无启用的 JOB）永远不会被 JOB 触发评估，但**不得因此标成功**（需求 10737）：
	// 原实现直接把它们写成 SUCCESS，导致「归档复核（PROCESS_REVIEW 目前 0 个 JOB）」
	// 在流程还没跑到该阶段时就显示为已完成。改为 SKIPPED：语义是「无可执行工作，已跳过」。
	if _, err := db.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run pr
   SET status = 'SKIPPED', finished_at = now(), completion_snapshot = $2::jsonb, gmt_update = now()
  FROM cdeu.finv_quote_cdeu_phase_run phr
 WHERE pr.phase_run_id = phr.phase_run_id
   AND phr.workflow_run_id = $1
   AND pr.status = 'PENDING'
   AND NOT EXISTS (
       SELECT 1 FROM cdeu.finv_quote_cdeu_job j
        WHERE j.process_id = pr.process_id AND j.enabled
   )`,
		runID, `{"status":"SKIPPED","completed":0,"total":0,"failed":0}`); err != nil {
		return err
	}
	// 阶段同理：全部子流程都是占位时，阶段标 SKIPPED 而非 SUCCESS。
	_, err := db.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run phr
   SET status = 'SKIPPED',
       finished_at = now(),
       total_count = sub.total,
       success_count = 0,
       failed_count = 0,
       completion_snapshot = jsonb_build_object(
           'status', 'SKIPPED', 'completed', 0, 'total', sub.total, 'failed', 0
       ),
       gmt_update = now()
  FROM (
      SELECT pr.phase_run_id,
             count(*) FILTER (WHERE ps.affects_completion) AS total,
             count(*) FILTER (WHERE ps.affects_completion AND pr.status IN ('SUCCESS','SKIPPED')) AS done
        FROM cdeu.finv_quote_cdeu_process_run pr
        JOIN cdeu.finv_quote_cdeu_process ps ON ps.process_id = pr.process_id
        JOIN cdeu.finv_quote_cdeu_phase_run phr ON phr.phase_run_id = pr.phase_run_id
       WHERE phr.workflow_run_id = $1
       GROUP BY pr.phase_run_id
  ) sub
 WHERE phr.phase_run_id = sub.phase_run_id
   AND phr.status = 'PENDING'
   AND sub.total > 0
   AND sub.total = sub.done`, runID)
	return err
}

// CDEUPhaseRunID 查询一次运行中指定阶段的运行 ID。
//
// 自 V200044 起 Process 层不再有自增 process_run_id，Process 运行实例改由
// (workflow_run_id, process_id) 定位，因此本方法只返回阶段运行 ID。
func (s *Store) CDEUPhaseRunID(ctx context.Context, runID, phaseID int64) (int64, error) {
	var phaseRunID int64
	err := s.pool.QueryRow(ctx, `
SELECT phase_run_id
  FROM cdeu.finv_quote_cdeu_phase_run
 WHERE workflow_run_id = $1 AND phase_id = $2`, runID, phaseID).Scan(&phaseRunID)
	return phaseRunID, err
}

// InsertCDEUJobRun 登记一次 JOB 运行；status=RUNNING 时写开始时间。
//
// 主键为 (workflow_run_id, process_id, job_id)，重复登记时覆盖为本次状态。
func (s *Store) InsertCDEUJobRun(ctx context.Context, key cdeuRunKey, definition CDEUJobDefinition,
	triggerType, status string, attempt int, runLogID *int64) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_job_run
    (workflow_run_id, process_id, job_id, job_code, job_kind, execution_stage, trigger_type, status,
     attempt, started_at, run_log_id)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9,
        CASE WHEN $8 = 'RUNNING' THEN now() ELSE NULL END, $10)
ON CONFLICT (workflow_run_id, process_id, job_id) DO UPDATE SET
    job_code = EXCLUDED.job_code,
    job_kind = EXCLUDED.job_kind,
    execution_stage = EXCLUDED.execution_stage,
    trigger_type = EXCLUDED.trigger_type,
    status = EXCLUDED.status,
    attempt = EXCLUDED.attempt,
    started_at = EXCLUDED.started_at,
    finished_at = NULL,
    processed_rows = 0,
    processed_bytes = 0,
    run_log_id = COALESCE(EXCLUDED.run_log_id, cdeu.finv_quote_cdeu_job_run.run_log_id),
    error = NULL,
    gmt_update = now()`, key.WorkflowRunID, key.ProcessID, key.JobID, definition.JobCode, definition.JobKind,
		definition.ExecutionStage, triggerType, status, attempt, runLogID)
	return err
}

// InsertCDEUJobStepRuns 为一次 JOB 运行登记 JOB 内全部 Step（PENDING）。
func (s *Store) InsertCDEUJobStepRuns(ctx context.Context, key cdeuRunKey,
	steps []CDEUJobStepDefinition) ([]cdeuJobStepRun, error) {
	states := make([]cdeuJobStepRun, 0, len(steps))
	for _, step := range steps {
		var state cdeuJobStepRun
		if err := s.pool.QueryRow(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_job_step_run
    (workflow_run_id, process_id, job_id, step_id, step_code, seq_no, status)
VALUES ($1, $2, $3, $4, $5, $6, 'PENDING')
ON CONFLICT (workflow_run_id, process_id, job_id, step_code) DO UPDATE SET
    step_id = EXCLUDED.step_id,
    seq_no = EXCLUDED.seq_no,
    status = 'PENDING',
    attempt = 0,
    started_at = NULL,
    finished_at = NULL,
    processed_rows = 0,
    processed_bytes = 0,
    error = NULL,
    gmt_update = now()
RETURNING step_run_id`, key.WorkflowRunID, key.ProcessID, key.JobID, step.StepID, step.StepCode, step.SeqNo).
			Scan(&state.StepRunID); err != nil {
			return nil, err
		}
		state.StepCode = step.StepCode
		state.Status = CDEUStatusPending
		states = append(states, state)
	}
	return states, nil
}

// ResetCDEUJobStepRuns 重置终态工作流后复用的 JOB Step 运行（语义同 InsertCDEUJobStepRuns）。
func (s *Store) ResetCDEUJobStepRuns(ctx context.Context, key cdeuRunKey,
	steps []CDEUJobStepDefinition) ([]cdeuJobStepRun, error) {
	return s.InsertCDEUJobStepRuns(ctx, key, steps)
}

// UpsertReadOnlyCDEUJobRun 在流程终态后刷新该 JOB 本次运行的状态（不存在则新建）。
//
// 需求 10747：只读重放路径下 JOB **并没有真正执行**（工作流已结束，只是查看/重跑界面数据），
// 因此不得写入 now() 当作开始时间——那是编造。新行 started_at 保持 NULL；
// 已有行保留其真实开始时间，绝不用当前时刻覆盖。
func (s *Store) UpsertReadOnlyCDEUJobRun(ctx context.Context, key cdeuRunKey,
	runLogID *int64) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_job_run
    (workflow_run_id, process_id, job_id, job_code, job_kind, execution_stage, trigger_type, status,
     attempt, started_at, run_log_id)
SELECT $1, $2, $3, j.job_code, j.job_kind, COALESCE(j.execution_stage, ''), 'PROCESS', 'RUNNING',
       1, NULL, $4
  FROM cdeu.finv_quote_cdeu_job j
 WHERE j.job_id = $3
ON CONFLICT (workflow_run_id, process_id, job_id) DO UPDATE SET
    status = 'RUNNING',
    attempt = cdeu.finv_quote_cdeu_job_run.attempt + 1,
    started_at = cdeu.finv_quote_cdeu_job_run.started_at,
    finished_at = NULL,
    processed_rows = 0,
    processed_bytes = 0,
    run_log_id = COALESCE(EXCLUDED.run_log_id, cdeu.finv_quote_cdeu_job_run.run_log_id),
    error = NULL,
    gmt_update = now()`, key.WorkflowRunID, key.ProcessID, key.JobID, runLogID)
	return err
}

// StartCDEUJobRun 把待执行 JOB 置为 RUNNING。
func (s *Store) StartCDEUJobRun(ctx context.Context, key cdeuRunKey, runLogID int64) error {
	var logArg *int64
	if runLogID > 0 {
		logArg = &runLogID
	}
	_, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_run
   SET status = 'RUNNING', started_at = COALESCE(started_at, now()),
       attempt = GREATEST(attempt, 1), run_log_id = COALESCE($4, run_log_id), gmt_update = now()
 WHERE workflow_run_id = $1 AND process_id = $2 AND job_id = $3`,
		key.WorkflowRunID, key.ProcessID, key.JobID, logArg)
	return err
}

// FinishCDEUJobRun 结束一次 JOB 运行。
func (s *Store) FinishCDEUJobRun(ctx context.Context, key cdeuRunKey, status string,
	rows, bytes int64, runLogID int64, errText string) error {
	var logArg *int64
	if runLogID > 0 {
		logArg = &runLogID
	}
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_run
   SET status = $4, finished_at = now(), processed_rows = $5, processed_bytes = $6,
       run_log_id = COALESCE($7, run_log_id), error = $8, gmt_update = now()
 WHERE workflow_run_id = $1 AND process_id = $2 AND job_id = $3`,
		key.WorkflowRunID, key.ProcessID, key.JobID, status, rows, bytes, logArg, errArg)
	return err
}

// StartCDEUJobStepRun 把 JOB 内 Step 置为 RUNNING。
func (s *Store) StartCDEUJobStepRun(ctx context.Context, stepRunID int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_step_run
   SET status = 'RUNNING', started_at = COALESCE(started_at, now()),
       attempt = GREATEST(attempt + 1, 1), gmt_update = now()
 WHERE step_run_id = $1 AND status IN ('PENDING','RUNNING')`, stepRunID)
	return err
}

// FinishCDEUJobStepRun 结束 JOB 内 Step。
func (s *Store) FinishCDEUJobStepRun(ctx context.Context, stepRunID int64, status string,
	rows, bytes int64, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_step_run
   SET status = $2, finished_at = now(), processed_rows = $3, processed_bytes = $4,
       error = $5, gmt_update = now()
 WHERE step_run_id = $1`, stepRunID, status, rows, bytes, errArg)
	return err
}

// TouchCDEUHierarchy 把工作流/阶段/Process 置为 RUNNING（不覆盖已终态）。
func (s *Store) TouchCDEUHierarchy(ctx context.Context, runID, phaseID, phaseRunID, processID int64) error {
	// 允许从终态 SUCCESS/SKIPPED 回退为 RUNNING：
	// 子级从定义表取之后，已完成父级仍可能被拉到 RUNNING（例如同一 JOB 的后续阶段在当前
	// Process 下继续执行）。仍不允许覆盖 FAILED/PARTIAL/CANCELLED，保留失败语义。
	const revivable = `status IN ('PENDING','READY','RUNNING','SUCCESS','SKIPPED')`
	if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_workflow_run
   SET status = 'RUNNING', started_at = COALESCE(started_at, now()),
       finished_at = NULL, current_phase_id = $2, gmt_update = now()
 WHERE run_id = $1 AND `+revivable,
		runID, phaseID); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run
   SET status = 'RUNNING', started_at = COALESCE(started_at, now()),
       finished_at = NULL, gmt_update = now()
 WHERE phase_run_id = $1 AND `+revivable, phaseRunID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run
   SET status = 'RUNNING', started_at = COALESCE(started_at, now()),
       finished_at = NULL, gmt_update = now()
 WHERE workflow_run_id = $1 AND process_id = $2 AND `+revivable, runID, processID)
	return err
}

// cdeuPhasePredecessorState 返回同一工作流内 seq_no 最小且非成功的前序阶段状态（需求 10733）。
//
// blocked=true 表示本阶段不应继续判定完成：
//   - 前序 PENDING/READY/RUNNING → 尚未轮到本阶段，保持 PENDING；
//   - 前序 FAILED/PARTIAL/CANCELLED → 主链路已失败，本阶段标 SKIPPED。
//
// 旁路阶段（affects_completion=false）不参与门禁。
func (s *Store) cdeuPhasePredecessorState(ctx context.Context, runID, workflowID int64, seqNo int) (string, bool, error) {
	var prev string
	err := s.pool.QueryRow(ctx, `
SELECT pr.status
  FROM cdeu.finv_quote_cdeu_phase ph
  JOIN cdeu.finv_quote_cdeu_phase_run pr
    ON pr.phase_id = ph.phase_id AND pr.workflow_run_id = $1
 WHERE ph.workflow_id = $2
   AND ph.enabled
   AND ph.affects_completion
   AND ph.seq_no < $3
   AND pr.status NOT IN ('SUCCESS','SKIPPED')
 ORDER BY ph.seq_no DESC
 LIMIT 1`, runID, workflowID, seqNo).Scan(&prev)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return prev, true, nil
}

// cdeuProcessHasEarlierUnfinished 判断同一 Phase 内是否存在 seq_no 更小、参与完成判定
// 且尚未进入终态的流程（需求 10729 的阶段内串行门禁）。
//
// 只统计 affects_completion=true 的流程：旁路流程（如 PROCESS_RECOVER 超时回收）
// 本身与主链路无关，不应阻塞后续流程。
func (s *Store) cdeuProcessHasEarlierUnfinished(ctx context.Context, runID, phaseID int64, seqNo int) (bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_process p
    JOIN cdeu.finv_quote_cdeu_process_run pr
      ON pr.process_id = p.process_id AND pr.workflow_run_id = $1
   WHERE p.phase_id = $2
     AND p.enabled
     AND p.affects_completion
     AND p.seq_no < $3
     AND pr.status NOT IN ('SUCCESS','SKIPPED','PARTIAL','FAILED','PAUSED','CANCELLED')
)`, runID, phaseID, seqNo).Scan(&blocked)
	return blocked, err
}

// cdeuProcessHasActiveChild 判断 Process 下是否仍有未终态的子级（JOB 或动态实例）。
// 用于让已判 SUCCESS 的父级在子级重新开跑时能够重新评估。
func (s *Store) cdeuProcessHasActiveChild(ctx context.Context, runID, processID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_job j
    LEFT JOIN cdeu.finv_quote_cdeu_job_run r
           ON r.workflow_run_id = $1 AND r.process_id = j.process_id AND r.job_id = j.job_id
   WHERE j.process_id = $2 AND j.enabled
     AND COALESCE(r.status, 'PENDING') IN ('PENDING','READY','RUNNING')
  UNION ALL
  SELECT 1
    FROM cdeu.finv_quote_cdeu_job_dynamic dj
   WHERE dj.process_id = $2 AND dj.workflow_run_id = $1
     AND dj.status IN ('PENDING','READY','RUNNING')
)`, runID, processID).Scan(&exists)
	return exists, err
}

// cdeuPhaseHasActiveChild 判断 Phase 下是否仍有未终态的 Process。
func (s *Store) cdeuPhaseHasActiveChild(ctx context.Context, runID, phaseID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_process ps
    LEFT JOIN cdeu.finv_quote_cdeu_process_run r
           ON r.workflow_run_id = $1 AND r.process_id = ps.process_id
   WHERE ps.phase_id = $2 AND ps.enabled
     AND COALESCE(r.status, 'PENDING') IN ('PENDING','READY','RUNNING')
)`, runID, phaseID).Scan(&exists)
	return exists, err
}

// LatestActiveCDEURunID 返回最近一次未终态的 CDEU 运行 ID；没有则返回 0。
func (s *Store) LatestActiveCDEURunID(ctx context.Context) (int64, error) {
	var runID int64
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(max(run_id), 0)
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE status IN ('PENDING','READY','RUNNING')`).Scan(&runID)
	if err != nil {
		return 0, err
	}
	return runID, nil
}

// CDEUTerminateResult 强制终止一次运行的结果统计。
type CDEUTerminateResult struct {
	RunID          int64  `json:"run_id"`
	PreviousStatus string `json:"previous_status"`
	Status         string `json:"status"`
	PhaseRuns      int64  `json:"phase_runs"`
	ProcessRuns    int64  `json:"process_runs"`
	JobRuns        int64  `json:"job_runs"`
	JobStepRuns    int64  `json:"job_step_runs"`
	DynamicJobs    int64  `json:"dynamic_jobs"`
	DynamicSteps   int64  `json:"dynamic_steps"`
	ReleasedJobs   int64  `json:"released_jobs"`
}

// ErrCDEURunTerminal 该运行已是终态，无需（也无法）再次终止。
var ErrCDEURunTerminal = errors.New("该 CDEU 运行已处于终态，无需终止")

// TerminateCDEURun 强制终止一次运行，把其下所有未终态的阶段 / 流程 / JOB / 步骤置为 CANCELLED。
//
// 用途：调度器或人工启动的流程卡死（例如某 JOB 陷入空转、或 BASE 上传长期无响应）时，
// 由运维一键收口，避免「已有运行中的工作流」一直挡住下一次启动。
//
// 安全约束：
//   - 只处理 PENDING / READY / RUNNING 的记录，已有终态（SUCCESS/FAILED/PARTIAL/...）一律不动；
//   - 单事务完成，任何一步失败整体回滚，不会留下半截状态；
//   - 同时释放 finv_dedup_export_job 上的任务锁，避免残留锁挡住后续 ClaimJob；
//   - 不触碰 finv_dedup_export_file 的上传状态（由 RecoverTimeoutUploads 负责）。
func (s *Store) TerminateCDEURun(ctx context.Context, runID int64) (CDEUTerminateResult, error) {
	result := CDEUTerminateResult{RunID: runID, Status: CDEUStatusCancelled}
	const activeStatusSQL = `('PENDING','READY','RUNNING')`

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `
SELECT status FROM cdeu.finv_quote_cdeu_workflow_run WHERE run_id = $1`, runID).Scan(&result.PreviousStatus); err != nil {
		return result, err
	}
	if isCDEUTerminal(result.PreviousStatus) {
		return result, ErrCDEURunTerminal
	}

	// 1) 动态步骤 → 动态 JOB（先子后父）
	tag, err := tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_step_dynamic
   SET status = 'CANCELLED', finished_at = now(), error = '强制终止'
 WHERE status IN `+activeStatusSQL+`
   AND dynamic_job_id IN (
       SELECT dynamic_job_id FROM cdeu.finv_quote_cdeu_job_dynamic WHERE workflow_run_id = $1)`, runID)
	if err != nil {
		return result, err
	}
	result.DynamicSteps = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_dynamic
   SET status = 'CANCELLED', gmt_finish = now(), error = '强制终止'
 WHERE workflow_run_id = $1 AND status IN `+activeStatusSQL, runID)
	if err != nil {
		return result, err
	}
	result.DynamicJobs = tag.RowsAffected()

	// 2) 静态 JOB 步骤 → JOB 运行 → 流程运行 → 阶段运行
	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_step_run
   SET status = 'CANCELLED', finished_at = now(), error = '强制终止'
 WHERE workflow_run_id = $1 AND status IN `+activeStatusSQL, runID)
	if err != nil {
		return result, err
	}
	result.JobStepRuns = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_job_run
   SET status = 'CANCELLED', finished_at = now(), error = '强制终止'
 WHERE workflow_run_id = $1 AND status IN `+activeStatusSQL, runID)
	if err != nil {
		return result, err
	}
	result.JobRuns = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run
   SET status = 'CANCELLED', finished_at = now()
 WHERE workflow_run_id = $1 AND status IN `+activeStatusSQL, runID)
	if err != nil {
		return result, err
	}
	result.ProcessRuns = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run
   SET status = 'CANCELLED', finished_at = now()
 WHERE workflow_run_id = $1 AND status IN `+activeStatusSQL, runID)
	if err != nil {
		return result, err
	}
	result.PhaseRuns = tag.RowsAffected()

	// 3) 释放导出任务锁：终止后这些 JOB 不应再被原 worker 占用。
	tag, err = tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_export_job
   SET status = CASE WHEN status IN ('LOADING','LOADED','EXPORTING') THEN 'WAITING' ELSE status END,
       locked_by = NULL,
       locked_until = NULL,
       gmt_heartbeat = NULL,
       gmt_update = now()
 WHERE locked_by IS NOT NULL`)
	if err != nil {
		return result, err
	}
	result.ReleasedJobs = tag.RowsAffected()

	// 4) 运行本体
	if _, err := tx.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_workflow_run
   SET status = 'CANCELLED', finished_at = now(), current_phase_id = NULL, gmt_update = now()
 WHERE run_id = $1`, runID); err != nil {
		return result, err
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// EvaluateCDEUProcess 评估 Process 完成条件，并向上级联评估阶段/工作流。
//
// 自 V200044 起 Process 运行实例由 (workflow_run_id, process_id) 定位。
func (s *Store) EvaluateCDEUProcess(ctx context.Context, runID, processID int64) error {
	var status string
	var ruleJSON *string
	var phaseRunID int64
	var seqNo int
	var affectsCompletion bool
	var phaseID int64
	if err := s.pool.QueryRow(ctx, `
SELECT pr.status, pr.completion_rule_snapshot::text, pr.phase_run_id,
       p.seq_no, p.affects_completion, p.phase_id
  FROM cdeu.finv_quote_cdeu_process_run pr
  JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = pr.process_id
 WHERE pr.workflow_run_id = $1 AND pr.process_id = $2`, runID, processID).
		Scan(&status, &ruleJSON, &phaseRunID, &seqNo, &affectsCompletion, &phaseID); err != nil {
		return err
	}
	// 阶段内串行门禁（需求 10729）：同一 Phase 下 affects_completion=true 的流程必须按 seq_no 串行。
	// 前序流程尚未终态时，本流程不得进入 RUNNING/SUCCESS，一律保持 PENDING。
	//
	// 历史缺陷：PROCESS_EXPORT_FILE_MON_BASE 与 PROCESS_EXPORT_FILE_MON_INCR 会同时 RUNNING——
	// 两者各自的子级（BASE 动态实例 / JOB_SCAN_PATCH 补丁扫描）由不同触发点独立驱动，
	// 而评估逻辑只看自身子级，从不检查前序流程，于是并行状态被如实反映到界面上。
	if affectsCompletion {
		blocked, blockErr := s.cdeuProcessHasEarlierUnfinished(ctx, runID, phaseID, seqNo)
		if blockErr != nil {
			return blockErr
		}
		if blocked {
			if status != CDEUStatusPending {
				if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run
   SET status = $3, started_at = NULL, finished_at = NULL,
       completion_snapshot = $4, gmt_update = now()
 WHERE workflow_run_id = $1 AND process_id = $2`,
					runID, processID, CDEUStatusPending,
					marshalCDEUCompletion(CDEUStatusPending, 0, 0, 0)); err != nil {
					return err
				}
			}
			return nil
		}
	}
	// 已终态时：只有存在仍在进行的动态实例，或仍存在未终态子级时，才重新评估。
	// 后者是「父级提前完成」的回退修正路径（子级从定义表取，job_run 未创建时为 PENDING）。
	if isCDEUTerminal(status) {
		pending, pendingErr := s.cdeuProcessHasPendingDynamic(ctx, runID, processID)
		if pendingErr != nil {
			return pendingErr
		}
		if !pending {
			if active, activeErr := s.cdeuProcessHasActiveChild(ctx, runID, processID); activeErr != nil {
				return activeErr
			} else if !active {
				return nil
			}
		}
	}
	children, err := s.cdeuProcessChildren(ctx, runID, processID)
	if err != nil {
		return err
	}
	// 空流程保护：该 Process 在 cdeu_job 中没有任何 enabled 定义（也无动态实例）时，
	// 不得判完成。否则「有流程定义但尚未配置 JOB」的空流程会瞬间 SUCCESS，
	// 并向上传染使 Phase / Workflow 提前完成（run 59 的 PROCESS_REVIEW → PHASE_REVIEW 即此情况）。
	if len(children) == 0 {
		if !isCDEUTerminal(status) {
			return nil
		}
		// 已是终态但定义被清空：回退为 PENDING，等待后续补齐定义后再评估。
		if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run
   SET status = $3, finished_at = NULL,
       batch_total = 0, batch_success = 0, batch_failed = 0,
       completion_snapshot = $4, gmt_update = now()
 WHERE workflow_run_id = $1 AND process_id = $2`,
			runID, processID, CDEUStatusPending,
			marshalCDEUCompletion(CDEUStatusPending, 0, 0, 0)); err != nil {
			return err
		}
		return s.EvaluateCDEUPhase(ctx, phaseRunID)
	}
	next, completed, total, failed := decideCDEUStatus(parseCDEUCompletionRule(ruleJSON), children)
	// started_at 完全由子级推导（需求 10747）。
	//
	// 原实现为 COALESCE(pr.started_at, LEAST(min(job_run.started_at), min(dyn.gmt_start), now()))：
	// 首次评估时子级尚未开始（min 全为 NULL），LEAST 忽略 NULL 后落到 now()，
	// 于是流程被写入「评估时刻」当作开始时间；随后 pr.started_at 非空，COALESCE 永久锁定，
	// 子级真实开始时间再也进不来。表现是同一阶段内先后执行的流程 started_at 挤在几十毫秒内，
	// 甚至早于其子 JOB 数秒（run 62 的 PROCESS_RECOVER / PROCESS_UPLOAD 早 9.7 秒）。
	//
	// 现口径：Process 自身不执行任何动作，它的「开始」只能是某个子级真的开始了。
	//   · 有子 JOB   → 子 JOB 最早 started_at
	//   · 有动态实例 → 动态实例最早 gmt_start
	//   · 两者兼有   → 取更早者（LEAST 忽略 NULL）
	//   · 都没有     → NULL（没有执行痕迹就不写时间，绝不编造）
	// LEAST 两者皆 NULL 时返回 NULL，因此不会产生假时间。
	if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_process_run pr
   SET status = $3,
       started_at = LEAST(
           (SELECT min(r.started_at) FROM cdeu.finv_quote_cdeu_job_run r
             WHERE r.workflow_run_id = $1 AND r.process_id = $2 AND r.started_at IS NOT NULL),
           (SELECT min(d.gmt_start) FROM cdeu.finv_quote_cdeu_job_dynamic d
             WHERE d.workflow_run_id = $1 AND d.process_id = $2 AND d.gmt_start IS NOT NULL)),
       finished_at = CASE WHEN $3 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE finished_at END,
       batch_total = $4, batch_success = $5, batch_failed = $6,
       completion_snapshot = $7, gmt_update = now()
 WHERE pr.workflow_run_id = $1 AND pr.process_id = $2`, runID, processID, next, total, completed, failed,
		marshalCDEUCompletion(next, completed, total, failed)); err != nil {
		return err
	}
	if !isCDEUTerminal(next) {
		return nil
	}
	return s.EvaluateCDEUPhase(ctx, phaseRunID)
}

// EvaluateCDEUPhase 评估阶段完成条件，并向上级联评估工作流。
func (s *Store) EvaluateCDEUPhase(ctx context.Context, phaseRunID int64) error {
	var status string
	var ruleJSON *string
	var runID int64
	var phaseID int64
	var seqNo int
	var workflowID int64
	if err := s.pool.QueryRow(ctx, `
SELECT pr.status, pr.completion_rule_snapshot::text, pr.workflow_run_id, pr.phase_id,
       ph.seq_no, ph.workflow_id
  FROM cdeu.finv_quote_cdeu_phase_run pr
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = pr.phase_id
 WHERE pr.phase_run_id = $1`, phaseRunID).
		Scan(&status, &ruleJSON, &runID, &phaseID, &seqNo, &workflowID); err != nil {
		return err
	}
	// 阶段串行与失败传播门禁（需求 10733）：
	// 同工作流内按 seq_no 串行，前序阶段未成功时后续阶段不得判成功——
	// 原实现只看自身子级，于是 PHASE_EXPORT=FAILED 时 PHASE_UPLOAD / PHASE_REVIEW
	// 仍会因自己子级全 SUCCESS 而被判 SUCCESS（run 59 即此现象）。
	if prevStatus, blocked, gateErr := s.cdeuPhasePredecessorState(ctx, runID, workflowID, seqNo); gateErr != nil {
		return gateErr
	} else if blocked {
		next := CDEUStatusPending
		switch prevStatus {
		case CDEUStatusFailed, CDEUStatusPartial, CDEUStatusCancelled:
			// 前序已失败：本阶段业务上不会真正执行，标为 SKIPPED 而非 SUCCESS。
			next = CDEUStatusSkipped
		case CDEUStatusSkipped:
			next = CDEUStatusSkipped
		}
		if status != next {
			if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run
   SET status = $2, finished_at = CASE WHEN $2 = 'SKIPPED' THEN now() ELSE NULL END,
       completion_snapshot = $3, gmt_update = now()
 WHERE phase_run_id = $1`, phaseRunID, next,
				marshalCDEUCompletion(next, 0, 0, 0)); err != nil {
				return err
			}
		}
		return s.EvaluateCDEUWorkflow(ctx, runID)
	}
	// 已终态时：只有存在仍在进行的动态实例才重新评估（否则会无谓地反复计算）。
	// 注：子级从定义表取，若仍有 PENDING/RUNNING 子级，下面的 decideCDEUStatus 会
	// 自然给出非终态结果，并通过 TouchCDEUHierarchy 回退已完成标记。
	if isCDEUTerminal(status) {
		pending, pendingErr := s.cdeuPhaseHasPendingDynamic(ctx, phaseRunID)
		if pendingErr != nil {
			return pendingErr
		}
		if !pending {
			if running, runningErr := s.cdeuPhaseHasActiveChild(ctx, runID, phaseID); runningErr != nil {
				return runningErr
			} else if !running {
				return nil
			}
		}
	}
	children, err := s.cdeuPhaseChildren(ctx, runID, phaseID)
	if err != nil {
		return err
	}
	// 空阶段保护：该 Phase 在 cdeu_process 中没有任何 enabled 定义时，
	// 同样不得判完成（与 Process 层同口径）。
	if len(children) == 0 {
		if !isCDEUTerminal(status) {
			return nil
		}
		if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run
   SET status = $2, finished_at = NULL,
       total_count = 0, success_count = 0, failed_count = 0,
       completion_snapshot = $3, gmt_update = now()
 WHERE phase_run_id = $1`,
			phaseRunID, CDEUStatusPending,
			marshalCDEUCompletion(CDEUStatusPending, 0, 0, 0)); err != nil {
			return err
		}
		return nil
	}
	next, completed, total, failed := decideCDEUStatus(parseCDEUCompletionRule(ruleJSON), children)
	// 与 Process 层同口径（需求 10747）：阶段的开始时间取**子流程最早的真实开始时间**。
	// 子流程全未开始（全 NULL）时阶段没有开始时间，保持 NULL，不用 now() 填充。
	if _, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_phase_run pr
   SET status = $2,
       started_at = (SELECT min(r.started_at) FROM cdeu.finv_quote_cdeu_process_run r
                      WHERE r.phase_run_id = $1 AND r.started_at IS NOT NULL),
       finished_at = CASE WHEN $2 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE finished_at END,
       total_count = $3, success_count = $4, failed_count = $5,
       completion_snapshot = $6, gmt_update = now()
 WHERE pr.phase_run_id = $1`, phaseRunID, next, total, completed, failed,
		marshalCDEUCompletion(next, completed, total, failed)); err != nil {
		return err
	}
	if !isCDEUTerminal(next) {
		return nil
	}
	return s.EvaluateCDEUWorkflow(ctx, runID)
}

// EvaluateCDEUWorkflow 评估工作流完成条件；已终态工作流不再改写。
func (s *Store) EvaluateCDEUWorkflow(ctx context.Context, runID int64) error {
	var status string
	var ruleJSON *string
	if err := s.pool.QueryRow(ctx, `
SELECT status, completion_rule_snapshot::text
  FROM cdeu.finv_quote_cdeu_workflow_run WHERE run_id = $1`, runID).
		Scan(&status, &ruleJSON); err != nil {
		return err
	}
	if isCDEUTerminal(status) {
		pending, pendingErr := s.cdeuWorkflowHasPendingDynamic(ctx, runID)
		if pendingErr != nil {
			return pendingErr
		}
		if !pending {
			return nil
		}
	}
	children, err := s.cdeuWorkflowChildren(ctx, runID)
	if err != nil {
		return err
	}
	next, completed, total, failed := decideCDEUStatus(parseCDEUCompletionRule(ruleJSON), children)
	_, err = s.pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_workflow_run
   SET status = $2,
       finished_at = CASE WHEN $2 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE finished_at END,
       completion_snapshot = $3, gmt_update = now()
 WHERE run_id = $1`, runID, next,
		marshalCDEUCompletion(next, completed, total, failed))
	return err
}

// EvaluateCDEURunProcesses 评估一次工作流运行下所有未终态 Process，兼容同一种 JOB
// 跨多个 Process 的编排（例如 BATCH / META / LOAD 分布在不同 Process）。
func (s *Store) EvaluateCDEURunProcesses(ctx context.Context, runID int64) error {
	rows, err := s.pool.Query(ctx, `
SELECT pr.process_id
  FROM cdeu.finv_quote_cdeu_process_run pr
 WHERE pr.workflow_run_id = $1
 ORDER BY pr.process_id`, runID)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, 8)
	for rows.Next() {
		var processID int64
		if err := rows.Scan(&processID); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, processID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, processID := range ids {
		if err := s.EvaluateCDEUProcess(ctx, runID, processID); err != nil {
			return err
		}
	}
	return nil
}

// cdeuProcessChildren 返回 Process 下的子级状态，**以任务定义表为基准**。
//
// 修复点（2026-10-27）：原实现从 `cdeu_job_run` 取子级，而 job_run 只在 JOB 真正
// 开始执行时才创建——在 SeedCDEUProcessRuns 之后、job_run 建立之前，子级集合为空，
// decideCDEUStatus 会按“total=0”判为 SUCCESS，导致父级在子级开跑前就完成（run 58
// 中 PROCESS_EXPORT_BATCH finished_at 早于 JOB_FETCH_EXPORT_BATCH started_at 6 秒）。
// 现改为以 `cdeu_job` 定义为基准 LEFT JOIN 运行记录，缺失运行记录时视为 PENDING。
func (s *Store) cdeuProcessChildren(ctx context.Context, runID, processID int64) ([]cdeuChild, error) {
	rows, err := s.pool.Query(ctx, `
SELECT j.job_code, COALESCE(r.status, 'PENDING'), j.critical, j.affects_completion
  FROM cdeu.finv_quote_cdeu_job j
  LEFT JOIN cdeu.finv_quote_cdeu_job_run r
         ON r.workflow_run_id = $1 AND r.process_id = j.process_id AND r.job_id = j.job_id
 WHERE j.process_id = $2 AND j.enabled
UNION ALL
SELECT dj.job_code, dj.status, false, true
  FROM cdeu.finv_quote_cdeu_job_dynamic dj
 WHERE dj.process_id = $2 AND dj.workflow_run_id = $1`, runID, processID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCDEUChildren(rows)
}

func (s *Store) cdeuProcessHasPendingDynamic(ctx context.Context, runID, processID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_job_dynamic dj
   WHERE dj.process_id = $2
     AND dj.workflow_run_id = $1
     AND dj.status IN ('PENDING','READY','RUNNING')
)`, runID, processID).Scan(&exists)
	return exists, err
}

func (s *Store) cdeuPhaseHasPendingDynamic(ctx context.Context, phaseRunID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_job_dynamic dj
    JOIN cdeu.finv_quote_cdeu_process_run pr ON pr.process_id = dj.process_id
    JOIN cdeu.finv_quote_cdeu_phase_run phr ON phr.phase_run_id = pr.phase_run_id
   WHERE pr.phase_run_id = $1
     AND dj.workflow_run_id = phr.workflow_run_id
     AND dj.status IN ('PENDING','READY','RUNNING')
)`, phaseRunID).Scan(&exists)
	return exists, err
}

func (s *Store) cdeuWorkflowHasPendingDynamic(ctx context.Context, runID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
    FROM cdeu.finv_quote_cdeu_job_dynamic dj
    JOIN cdeu.finv_quote_cdeu_process_run pr ON pr.process_id = dj.process_id
    JOIN cdeu.finv_quote_cdeu_phase_run phr ON phr.phase_run_id = pr.phase_run_id
   WHERE phr.workflow_run_id = $1
     AND dj.workflow_run_id = phr.workflow_run_id
     AND dj.status IN ('PENDING','READY','RUNNING')
)`, runID).Scan(&exists)
	return exists, err
}

// cdeuPhaseChildren 返回 Phase 下的子级（Process）状态，同样以定义为基准。
func (s *Store) cdeuPhaseChildren(ctx context.Context, runID, phaseID int64) ([]cdeuChild, error) {
	rows, err := s.pool.Query(ctx, `
SELECT ps.process_code, COALESCE(r.status, 'PENDING'), ps.critical, ps.affects_completion
  FROM cdeu.finv_quote_cdeu_process ps
  LEFT JOIN cdeu.finv_quote_cdeu_process_run r
         ON r.workflow_run_id = $1 AND r.process_id = ps.process_id
 WHERE ps.phase_id = $2 AND ps.enabled`, runID, phaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCDEUChildren(rows)
}

// cdeuWorkflowChildren 返回 Workflow 下的子级（Phase）状态，同样以定义为基准。
func (s *Store) cdeuWorkflowChildren(ctx context.Context, runID int64) ([]cdeuChild, error) {
	rows, err := s.pool.Query(ctx, `
SELECT ph.phase_code, COALESCE(r.status, 'PENDING'), ph.critical, ph.affects_completion
  FROM cdeu.finv_quote_cdeu_phase ph
  LEFT JOIN cdeu.finv_quote_cdeu_phase_run r
         ON r.workflow_run_id = $1 AND r.phase_id = ph.phase_id
 WHERE ph.workflow_id = (
       SELECT workflow_id FROM cdeu.finv_quote_cdeu_workflow_run WHERE run_id = $1)
   AND ph.enabled`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCDEUChildren(rows)
}

func scanCDEUChildren(rows pgx.Rows) ([]cdeuChild, error) {
	children := make([]cdeuChild, 0, 8)
	for rows.Next() {
		var child cdeuChild
		if err := rows.Scan(&child.Code, &child.Status, &child.Critical, &child.Affects); err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	return children, rows.Err()
}

// parseCDEUCompletionRule 解析完成条件；空值按默认 ALL 处理。
func parseCDEUCompletionRule(raw *string) CDEUCompletionRule {
	rule := CDEUCompletionRule{Mode: "ALL"}
	if raw == nil || *raw == "" {
		return rule
	}
	if err := json.Unmarshal([]byte(*raw), &rule); err != nil {
		rule.Mode = "ALL"
	}
	if rule.Mode == "" {
		rule.Mode = "ALL"
	}
	return rule
}

// decideCDEUStatus 根据完成条件与子级状态给出下一状态。
func decideCDEUStatus(rule CDEUCompletionRule, rawChildren []cdeuChild) (status string, completed, total, failed int) {
	children := make([]cdeuChild, 0, len(rawChildren))
	for _, child := range rawChildren {
		if child.Affects {
			children = append(children, child)
		}
	}
	total = len(children)
	if total == 0 {
		return CDEUStatusSuccess, 0, 0, 0
	}

	hasRunning := false
	hasPaused := false
	failedCritical := false
	selected := map[string]bool{}
	if len(rule.Items) > 0 {
		for _, item := range rule.Items {
			selected[item] = true
		}
	}
	selectedTotal := 0
	selectedCompleted := 0
	for _, child := range children {
		switch child.Status {
		case CDEUStatusPending, CDEUStatusReady, CDEUStatusRunning:
			hasRunning = true
		case CDEUStatusPaused:
			hasPaused = true
			failed++
		case CDEUStatusFailed, CDEUStatusCancelled:
			failed++
			if child.Critical {
				failedCritical = true
			}
		}
		inSelected := len(selected) == 0 || selected[child.Code]
		if inSelected {
			selectedTotal++
		}
		if cdeuChildCompleted(rule, child.Status) {
			completed++
			if inSelected {
				selectedCompleted++
			}
		}
	}
	if hasRunning {
		return CDEUStatusRunning, completed, total, failed
	}
	if failedCritical {
		return CDEUStatusFailed, completed, total, failed
	}
	if hasPaused {
		return CDEUStatusPaused, completed, total, failed
	}

	ok := false
	switch rule.Mode {
	case "CRITICAL_ALL":
		ok = true
		for _, child := range children {
			if child.Critical && !cdeuChildCompleted(rule, child.Status) {
				ok = false
				break
			}
		}
	case "AT_LEAST":
		minCount := 1
		if rule.MinCount != nil {
			minCount = *rule.MinCount
		}
		ok = selectedCompleted >= minCount
		if ok && rule.MinRatio != nil && selectedTotal > 0 {
			ok = float64(selectedCompleted)/float64(selectedTotal) >= *rule.MinRatio
		}
	case "ANY_OF":
		ok = selectedCompleted > 0
	default: // ALL
		ok = selectedCompleted == selectedTotal
	}
	if !ok {
		return CDEUStatusFailed, completed, total, failed
	}
	if failed > 0 || completed < total {
		return CDEUStatusPartial, completed, total, failed
	}
	return CDEUStatusSuccess, completed, total, failed
}

// cdeuChildCompleted 判断子级是否算「已完成」。
//
// 需求 10740：SKIPPED 必须算完成。它表示「主动跳过」而非失败——典型场景是
// 占位流程（如 PROCESS_EXPORT_DECISION，V100047 起就没配过 JOB）：初始化时被标
// SKIPPED，若不计入 completed，父阶段会因 3/4 而被误判 FAILED。
func cdeuChildCompleted(rule CDEUCompletionRule, status string) bool {
	switch status {
	case CDEUStatusSuccess, CDEUStatusSkipped:
		return true
	}
	if rule.CountMode == "TERMINAL" {
		return isCDEUTerminal(status)
	}
	return false
}

func isCDEUTerminal(status string) bool {
	switch status {
	case CDEUStatusSuccess, CDEUStatusPartial, CDEUStatusFailed,
		CDEUStatusPaused, CDEUStatusCancelled, CDEUStatusSkipped:
		return true
	default:
		return false
	}
}
