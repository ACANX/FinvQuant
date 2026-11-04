-- =====================================================================
-- FinvQuant PostgreSQL V50：CDEU 工作流模型重构
--
-- 新层级：Workflow → Phase → Process → JOB → Step
--   Process（原流程）    → Workflow
--   Step（原步骤）       → Process
--   Job（作业）          → JOB
--   新增 JOB 下的串行原子步骤 → Step
--
-- 本迁移只做模型改名与补齐 JOB Step，不改变现有 Job 的调度与执行语义。
-- =====================================================================

-- ---------------------------------------------------------------------
-- 1. 表改名（先腾出 process / process_run 名称，再接管 step / step_run）
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_process_run RENAME TO cdeu_workflow_run;
ALTER TABLE dedup.cdeu_process     RENAME TO cdeu_workflow;
ALTER TABLE dedup.cdeu_step_run    RENAME TO cdeu_process_run;
ALTER TABLE dedup.cdeu_step        RENAME TO cdeu_process;

-- ---------------------------------------------------------------------
-- 2. 列改名
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_workflow
    RENAME COLUMN process_id TO workflow_id;
ALTER TABLE dedup.cdeu_workflow
    RENAME COLUMN process_code TO workflow_code;

ALTER TABLE dedup.cdeu_phase
    RENAME COLUMN process_id TO workflow_id;

ALTER TABLE dedup.cdeu_process
    RENAME COLUMN step_id TO process_id;
ALTER TABLE dedup.cdeu_process
    RENAME COLUMN step_code TO process_code;

ALTER TABLE dedup.cdeu_workflow_run
    RENAME COLUMN process_id TO workflow_id;
ALTER TABLE dedup.cdeu_workflow_run
    RENAME COLUMN process_code TO workflow_code;
ALTER TABLE dedup.cdeu_workflow_run
    RENAME COLUMN process_version TO workflow_version;

ALTER TABLE dedup.cdeu_phase_run
    RENAME COLUMN run_id TO workflow_run_id;

ALTER TABLE dedup.cdeu_process_run
    RENAME COLUMN step_run_id TO process_run_id;
ALTER TABLE dedup.cdeu_process_run
    RENAME COLUMN step_id TO process_id;
ALTER TABLE dedup.cdeu_process_run
    RENAME COLUMN step_code TO process_code;

ALTER TABLE dedup.cdeu_job
    RENAME COLUMN step_id TO process_id;

ALTER TABLE dedup.cdeu_job_run
    RENAME COLUMN step_run_id TO process_run_id;

-- 索引名同步到新模型。
ALTER INDEX IF EXISTS dedup.uq_cdeuprocessrun_running RENAME TO uq_cdeuworkflowrun_running;
ALTER INDEX IF EXISTS dedup.idx_cdeuprocessrun_biz RENAME TO idx_cdeuworkflowrun_biz;
ALTER INDEX IF EXISTS dedup.idx_cdeu_job_run_step RENAME TO idx_cdeu_job_run_process;

-- 原有 Process/Step 编码同步改成 PROCESS_ 前缀，避免与新 JOB Step 混淆。
UPDATE dedup.cdeu_process
   SET process_code = 'PROCESS_' || substring(process_code FROM 6)
 WHERE process_code LIKE 'STEP_%';

-- ---------------------------------------------------------------------
-- 3. JOB 下的串行原子 Step 定义
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_job_step (
    step_id             bigserial PRIMARY KEY,
    job_id              bigint NOT NULL,
    step_code           text NOT NULL,
    name                text NOT NULL,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    timeout_seconds     int,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_id, step_code),
    UNIQUE (job_id, seq_no)
);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_step_job
    ON dedup.cdeu_job_step (job_id, seq_no);

COMMENT ON TABLE dedup.cdeu_job_step IS 'CDEU JOB 下的串行原子步骤定义';
COMMENT ON COLUMN dedup.cdeu_job_step.step_id IS '步骤主键';
COMMENT ON COLUMN dedup.cdeu_job_step.job_id IS '所属 JOB ID，逻辑关联 cdeu_job.job_id';
COMMENT ON COLUMN dedup.cdeu_job_step.step_code IS '步骤编码，如 STEP_LOAD';
COMMENT ON COLUMN dedup.cdeu_job_step.name IS '步骤中文名称';
COMMENT ON COLUMN dedup.cdeu_job_step.seq_no IS 'JOB 内执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_job_step.critical IS '是否关键步骤';
COMMENT ON COLUMN dedup.cdeu_job_step.affects_completion IS '是否参与 JOB 完成判定';
COMMENT ON COLUMN dedup.cdeu_job_step.timeout_seconds IS '单步超时秒数，空表示沿用 JOB';
COMMENT ON COLUMN dedup.cdeu_job_step.enabled IS '步骤定义是否启用';
COMMENT ON COLUMN dedup.cdeu_job_step.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job_step.gmt_update IS '更新时间';

-- 现有 JOB 先按 1:1 建立默认 Step；内嵌导出阶段使用 LOAD/EXPORT/REGISTER/CLEANUP 作为 Step 编码。
INSERT INTO dedup.cdeu_job_step
    (job_id, step_code, name, seq_no, critical, affects_completion, enabled)
SELECT j.job_id,
       CASE
           WHEN COALESCE(j.execution_stage, '') <> '' THEN 'STEP_' || j.execution_stage
           ELSE 'STEP_' || regexp_replace(j.job_code, '^JOB_', '')
       END,
       j.name,
       1,
       j.critical,
       j.affects_completion,
       j.enabled
  FROM dedup.cdeu_job j
ON CONFLICT (job_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

-- ---------------------------------------------------------------------
-- 4. JOB Step 运行实例
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_job_step_run (
    step_run_id         bigserial PRIMARY KEY,
    job_run_id          bigint NOT NULL,
    step_id             bigint NOT NULL,
    step_code           text NOT NULL,
    seq_no              int NOT NULL,
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    attempt             int NOT NULL DEFAULT 0,
    started_at          timestamptz,
    finished_at         timestamptz,
    processed_rows      bigint NOT NULL DEFAULT 0,
    processed_bytes     bigint NOT NULL DEFAULT 0,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (job_run_id, step_code)
);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_step_run_job
    ON dedup.cdeu_job_step_run (job_run_id, seq_no);

COMMENT ON TABLE dedup.cdeu_job_step_run IS 'CDEU JOB 内 Step 运行实例';
COMMENT ON COLUMN dedup.cdeu_job_step_run.step_run_id IS 'JOB Step 运行主键';
COMMENT ON COLUMN dedup.cdeu_job_step_run.job_run_id IS 'JOB 运行 ID，逻辑关联 cdeu_job_run.job_run_id';
COMMENT ON COLUMN dedup.cdeu_job_step_run.step_id IS 'JOB Step 定义 ID';
COMMENT ON COLUMN dedup.cdeu_job_step_run.step_code IS '步骤编码快照';
COMMENT ON COLUMN dedup.cdeu_job_step_run.seq_no IS 'JOB 内顺序快照';
COMMENT ON COLUMN dedup.cdeu_job_step_run.status IS '步骤运行状态';
COMMENT ON COLUMN dedup.cdeu_job_step_run.attempt IS '尝试次数';
COMMENT ON COLUMN dedup.cdeu_job_step_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_job_step_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_job_step_run.processed_rows IS '处理行数';
COMMENT ON COLUMN dedup.cdeu_job_step_run.processed_bytes IS '处理字节数';
COMMENT ON COLUMN dedup.cdeu_job_step_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_job_step_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job_step_run.gmt_update IS '更新时间';
