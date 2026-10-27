-- =====================================================================
-- FinvQuant PostgreSQL V44：CDEU 端到端流程模型（复用 dedup schema）
--
-- 依据：Docs/Refactor/2610-007-CDEUProcessModelDesign.md
--   Process（流程）→ Phase（阶段）→ Step（步骤）→ Job（作业）
--   定义表 4 张 + 运行表 4 张，全部落在 dedup schema，统一 cdeu_* 前缀。
--   全库不加外键，表间通过 *_id 逻辑关联。
--
-- 初始数据：CDEU_DAILY 流程 + 5 个阶段 + 10 个步骤 + 13 个作业登记。
--   1–4、9–13 为 DIRECT（对应现有 10 个调度 Job 中的 9 个）；
--   5–8 为 RUN_DUE_EXPORT_JOBS 的 LOAD / EXPORT / REGISTER / CLEANUP 四个内嵌阶段。
--   ⑩ 网盘归档当前由外部自动完成但实现暂缺，不登记、不参与完成判定。
--
-- 说明：业务日与调度时区默认取 FINV_QUOTE_DEDUB_TIMEZONE；批处理失败策略默认
--   CONTINUE；流程完成条件默认 ALL。迁移由 database.Migrate 在单事务中执行，
--   本文件不写 BEGIN/COMMIT。
-- =====================================================================

-- ---------------------------------------------------------------------
-- 1. 流程定义
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_process (
    process_id          bigserial PRIMARY KEY,
    process_code        text NOT NULL,
    name                text NOT NULL,
    description         text,
    version             int  NOT NULL DEFAULT 1,
    run_mode            text NOT NULL DEFAULT 'MANUAL'
                        CHECK (run_mode IN ('AUTO_DAILY','MANUAL','BOTH')),
    schedule_time       time,
    schedule_timezone   text NOT NULL DEFAULT 'Asia/Shanghai',
    schedule_enabled    boolean NOT NULL DEFAULT false,
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (process_code, version)
);

COMMENT ON TABLE dedup.cdeu_process IS 'CDEU 流程定义：一条端到端流程（采集→去重→导出→上传）';
COMMENT ON COLUMN dedup.cdeu_process.process_id IS '流程主键';
COMMENT ON COLUMN dedup.cdeu_process.process_code IS '流程编码，如 CDEU_DAILY';
COMMENT ON COLUMN dedup.cdeu_process.name IS '流程中文名称';
COMMENT ON COLUMN dedup.cdeu_process.description IS '流程说明';
COMMENT ON COLUMN dedup.cdeu_process.version IS '定义版本号，同编码可多版本';
COMMENT ON COLUMN dedup.cdeu_process.run_mode IS '运行模式：AUTO_DAILY 每日定时 / MANUAL 手动 / BOTH 两者';
COMMENT ON COLUMN dedup.cdeu_process.schedule_time IS '每日自动触发时间（按 schedule_timezone 解释）';
COMMENT ON COLUMN dedup.cdeu_process.schedule_timezone IS '调度时区，默认取 FINV_QUOTE_DEDUB_TIMEZONE';
COMMENT ON COLUMN dedup.cdeu_process.schedule_enabled IS '是否启用每日自动触发';
COMMENT ON COLUMN dedup.cdeu_process.completion_rule IS '完成条件 JSON：ALL / CRITICAL_ALL / AT_LEAST / ANY_OF，默认 ALL';
COMMENT ON COLUMN dedup.cdeu_process.enabled IS '流程定义是否启用';
COMMENT ON COLUMN dedup.cdeu_process.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_process.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 2. 阶段定义
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_phase (
    phase_id            bigserial PRIMARY KEY,
    process_id          bigint NOT NULL,
    phase_code          text NOT NULL,
    name                text NOT NULL,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    run_mode            text NOT NULL DEFAULT 'INHERIT'
                        CHECK (run_mode IN ('INHERIT','CONTINUOUS','MANUAL')),
    batch_enabled       boolean NOT NULL DEFAULT false,
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (process_id, phase_code),
    UNIQUE (process_id, seq_no)
);

COMMENT ON TABLE dedup.cdeu_phase IS 'CDEU 阶段定义：流程下的大阶段';
COMMENT ON COLUMN dedup.cdeu_phase.phase_id IS '阶段主键';
COMMENT ON COLUMN dedup.cdeu_phase.process_id IS '所属流程 ID，逻辑关联 cdeu_process.process_id';
COMMENT ON COLUMN dedup.cdeu_phase.phase_code IS '阶段编码，如 PHASE_EXPORT';
COMMENT ON COLUMN dedup.cdeu_phase.name IS '阶段中文名称';
COMMENT ON COLUMN dedup.cdeu_phase.seq_no IS '阶段执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_phase.critical IS '是否关键阶段，关键阶段失败或未完成会使流程失败';
COMMENT ON COLUMN dedup.cdeu_phase.affects_completion IS '是否参与流程完成判定';
COMMENT ON COLUMN dedup.cdeu_phase.run_mode IS '运行模式：INHERIT 随流程 / CONTINUOUS 独立循环 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_phase.batch_enabled IS '该阶段是否包含批处理步骤';
COMMENT ON COLUMN dedup.cdeu_phase.completion_rule IS '阶段完成条件 JSON，默认全部步骤完成';
COMMENT ON COLUMN dedup.cdeu_phase.enabled IS '阶段定义是否启用';
COMMENT ON COLUMN dedup.cdeu_phase.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_phase.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 3. 步骤定义
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_step (
    step_id             bigserial PRIMARY KEY,
    phase_id            bigint NOT NULL,
    step_code           text NOT NULL,
    name                text NOT NULL,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    run_mode            text NOT NULL DEFAULT 'INHERIT'
                        CHECK (run_mode IN ('INHERIT','CONTINUOUS','MANUAL')),
    batch_enabled       boolean NOT NULL DEFAULT false,
    batch_source        text,
    batch_key_field     text,
    batch_size          int,
    max_parallel        int NOT NULL DEFAULT 1,
    failure_policy      text NOT NULL DEFAULT 'CONTINUE'
                        CHECK (failure_policy IN ('FAIL_FAST','CONTINUE')),
    completion_rule     jsonb NOT NULL DEFAULT '{"mode":"ALL"}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (phase_id, step_code),
    UNIQUE (phase_id, seq_no)
);

COMMENT ON TABLE dedup.cdeu_step IS 'CDEU 步骤定义：阶段下的可编排步骤，支持批处理';
COMMENT ON COLUMN dedup.cdeu_step.step_id IS '步骤主键';
COMMENT ON COLUMN dedup.cdeu_step.phase_id IS '所属阶段 ID，逻辑关联 cdeu_phase.phase_id';
COMMENT ON COLUMN dedup.cdeu_step.step_code IS '步骤编码，如 STEP_EXPORT_JOB';
COMMENT ON COLUMN dedup.cdeu_step.name IS '步骤中文名称';
COMMENT ON COLUMN dedup.cdeu_step.seq_no IS '步骤执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_step.critical IS '是否关键步骤，关键步骤失败或未完成会使阶段失败';
COMMENT ON COLUMN dedup.cdeu_step.affects_completion IS '是否参与阶段完成判定';
COMMENT ON COLUMN dedup.cdeu_step.run_mode IS '运行模式：INHERIT 随阶段 / CONTINUOUS 独立循环 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_step.batch_enabled IS '是否按批次展开执行';
COMMENT ON COLUMN dedup.cdeu_step.batch_source IS '批次来源：EXPORT_JOB_CANDIDATES / EXPORT_FILES / PATCH_CANDIDATES / USC_LIST / CUSTOM_SQL';
COMMENT ON COLUMN dedup.cdeu_step.batch_key_field IS '批次键字段，如 usc,biz_month';
COMMENT ON COLUMN dedup.cdeu_step.batch_size IS '单批处理条数';
COMMENT ON COLUMN dedup.cdeu_step.max_parallel IS '最大并发批次数';
COMMENT ON COLUMN dedup.cdeu_step.failure_policy IS '批处理失败策略：CONTINUE 继续其余批次（默认）/ FAIL_FAST 失败即停';
COMMENT ON COLUMN dedup.cdeu_step.completion_rule IS '步骤完成条件 JSON，默认全部作业完成';
COMMENT ON COLUMN dedup.cdeu_step.enabled IS '步骤定义是否启用';
COMMENT ON COLUMN dedup.cdeu_step.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_step.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 4. 作业定义
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_job (
    job_id              bigserial PRIMARY KEY,
    step_id             bigint NOT NULL,
    job_code            text NOT NULL,
    name                text NOT NULL,
    job_kind            text NOT NULL,
    execution_stage     text,
    seq_no              int  NOT NULL,
    critical            boolean NOT NULL DEFAULT true,
    affects_completion  boolean NOT NULL DEFAULT true,
    trigger_mode        text NOT NULL DEFAULT 'SCHEDULED'
                        CHECK (trigger_mode IN ('SCHEDULED','EMBEDDED','MANUAL')),
    execution_mode      text NOT NULL DEFAULT 'DIRECT'
                        CHECK (execution_mode IN ('DIRECT','EMBEDDED')),
    manual_rerun_enabled boolean NOT NULL DEFAULT true,
    schedule_kind       text NOT NULL DEFAULT 'NONE'
                        CHECK (schedule_kind IN ('DAILY','INTERVAL','NONE')),
    schedule_value      text,
    timeout_seconds     int,
    max_attempts        int NOT NULL DEFAULT 1,
    retry_backoff_seconds int NOT NULL DEFAULT 60,
    job_params          jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled             boolean NOT NULL DEFAULT true,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (step_id, job_code),
    UNIQUE (step_id, seq_no)
);

COMMENT ON TABLE dedup.cdeu_job IS 'CDEU 作业定义：步骤下的可执行作业，按 seq_no 顺序执行';
COMMENT ON COLUMN dedup.cdeu_job.job_id IS '作业主键';
COMMENT ON COLUMN dedup.cdeu_job.step_id IS '所属步骤 ID，逻辑关联 cdeu_step.step_id';
COMMENT ON COLUMN dedup.cdeu_job.job_code IS '作业编码，如 JOB_EXPORT_LOAD';
COMMENT ON COLUMN dedup.cdeu_job.name IS '作业中文名称';
COMMENT ON COLUMN dedup.cdeu_job.job_kind IS '实际调度标识，如 RUN_DUE_EXPORT_JOBS';
COMMENT ON COLUMN dedup.cdeu_job.execution_stage IS '内嵌执行阶段：LOAD / EXPORT / REGISTER / CLEANUP；不拆分则为空';
COMMENT ON COLUMN dedup.cdeu_job.seq_no IS '作业执行顺序，数值小的先执行';
COMMENT ON COLUMN dedup.cdeu_job.critical IS '是否关键作业，关键作业失败或未完成会使步骤失败';
COMMENT ON COLUMN dedup.cdeu_job.affects_completion IS '是否参与步骤完成判定';
COMMENT ON COLUMN dedup.cdeu_job.trigger_mode IS '触发方式：SCHEDULED 定时 / EMBEDDED 父作业内嵌 / MANUAL 手动';
COMMENT ON COLUMN dedup.cdeu_job.execution_mode IS '执行方式：DIRECT 直接调用 / EMBEDDED 由父作业阶段驱动';
COMMENT ON COLUMN dedup.cdeu_job.manual_rerun_enabled IS '是否允许单阶段手动重跑（5–8 导出阶段默认开启）';
COMMENT ON COLUMN dedup.cdeu_job.schedule_kind IS '调度类型：DAILY 每日 / INTERVAL 固定间隔 / NONE 无';
COMMENT ON COLUMN dedup.cdeu_job.schedule_value IS '调度值：DAILY 为 HH:MM，INTERVAL 为秒数';
COMMENT ON COLUMN dedup.cdeu_job.timeout_seconds IS '单次执行超时秒数，空表示不限制';
COMMENT ON COLUMN dedup.cdeu_job.max_attempts IS '最大尝试次数';
COMMENT ON COLUMN dedup.cdeu_job.retry_backoff_seconds IS '重试退避基数（秒）';
COMMENT ON COLUMN dedup.cdeu_job.job_params IS '作业参数 JSON';
COMMENT ON COLUMN dedup.cdeu_job.enabled IS '作业定义是否启用';
COMMENT ON COLUMN dedup.cdeu_job.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 5. 流程运行
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_process_run (
    run_id              bigserial PRIMARY KEY,
    process_id          bigint NOT NULL,
    process_code        text NOT NULL,
    process_version     int  NOT NULL,
    run_key             text NOT NULL,
    biz_date            date NOT NULL,
    trigger_type        text NOT NULL
                        CHECK (trigger_type IN ('SCHEDULE','MANUAL','CONTINUOUS')),
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    scheduled_at        timestamptz,
    started_at          timestamptz,
    finished_at         timestamptz,
    current_phase_id    bigint,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    operator            text,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_key)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeuprocessrun_running
    ON dedup.cdeu_process_run (process_id)
    WHERE status IN ('PENDING','READY','RUNNING');
CREATE INDEX IF NOT EXISTS idx_cdeuprocessrun_biz
    ON dedup.cdeu_process_run (biz_date, status);

COMMENT ON TABLE dedup.cdeu_process_run IS 'CDEU 流程运行实例';
COMMENT ON COLUMN dedup.cdeu_process_run.run_id IS '流程运行主键';
COMMENT ON COLUMN dedup.cdeu_process_run.process_id IS '流程定义 ID，逻辑关联 cdeu_process.process_id';
COMMENT ON COLUMN dedup.cdeu_process_run.process_code IS '流程编码快照';
COMMENT ON COLUMN dedup.cdeu_process_run.process_version IS '流程定义版本快照';
COMMENT ON COLUMN dedup.cdeu_process_run.run_key IS '幂等键：自动 process_code:biz_date:SCHEDULE；手动追加时间戳/ULID';
COMMENT ON COLUMN dedup.cdeu_process_run.biz_date IS '业务日期，按 FINV_QUOTE_DEDUB_TIMEZONE 计算';
COMMENT ON COLUMN dedup.cdeu_process_run.trigger_type IS '触发类型：SCHEDULE 定时 / MANUAL 手动 / CONTINUOUS 连续循环';
COMMENT ON COLUMN dedup.cdeu_process_run.status IS '运行状态';
COMMENT ON COLUMN dedup.cdeu_process_run.scheduled_at IS '计划触发时间';
COMMENT ON COLUMN dedup.cdeu_process_run.started_at IS '实际开始时间';
COMMENT ON COLUMN dedup.cdeu_process_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_process_run.current_phase_id IS '当前执行阶段 ID';
COMMENT ON COLUMN dedup.cdeu_process_run.completion_rule_snapshot IS '完成条件快照';
COMMENT ON COLUMN dedup.cdeu_process_run.completion_snapshot IS '完成判定结果快照';
COMMENT ON COLUMN dedup.cdeu_process_run.operator IS '手动触发人';
COMMENT ON COLUMN dedup.cdeu_process_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_process_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_process_run.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 6. 阶段运行
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_phase_run (
    phase_run_id        bigserial PRIMARY KEY,
    run_id              bigint NOT NULL,
    phase_id            bigint NOT NULL,
    phase_code          text NOT NULL,
    seq_no              int NOT NULL,
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    started_at          timestamptz,
    finished_at         timestamptz,
    success_count       int NOT NULL DEFAULT 0,
    failed_count        int NOT NULL DEFAULT 0,
    total_count         int NOT NULL DEFAULT 0,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, phase_code)
);

COMMENT ON TABLE dedup.cdeu_phase_run IS 'CDEU 阶段运行实例';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_run_id IS '阶段运行主键';
COMMENT ON COLUMN dedup.cdeu_phase_run.run_id IS '流程运行 ID，逻辑关联 cdeu_process_run.run_id';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_id IS '阶段定义 ID';
COMMENT ON COLUMN dedup.cdeu_phase_run.phase_code IS '阶段编码快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.seq_no IS '阶段顺序快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.status IS '阶段运行状态';
COMMENT ON COLUMN dedup.cdeu_phase_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.success_count IS '成功步骤数';
COMMENT ON COLUMN dedup.cdeu_phase_run.failed_count IS '失败步骤数';
COMMENT ON COLUMN dedup.cdeu_phase_run.total_count IS '参与判定步骤总数';
COMMENT ON COLUMN dedup.cdeu_phase_run.completion_rule_snapshot IS '阶段完成条件快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.completion_snapshot IS '阶段完成判定快照';
COMMENT ON COLUMN dedup.cdeu_phase_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_phase_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_phase_run.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 7. 步骤运行
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_step_run (
    step_run_id         bigserial PRIMARY KEY,
    phase_run_id        bigint NOT NULL,
    step_id             bigint NOT NULL,
    step_code           text NOT NULL,
    seq_no              int NOT NULL,
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','READY','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    started_at          timestamptz,
    finished_at         timestamptz,
    batch_total         int NOT NULL DEFAULT 0,
    batch_success       int NOT NULL DEFAULT 0,
    batch_failed        int NOT NULL DEFAULT 0,
    completion_rule_snapshot jsonb,
    completion_snapshot jsonb,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (phase_run_id, step_code)
);

COMMENT ON TABLE dedup.cdeu_step_run IS 'CDEU 步骤运行实例，记录批处理计数';
COMMENT ON COLUMN dedup.cdeu_step_run.step_run_id IS '步骤运行主键';
COMMENT ON COLUMN dedup.cdeu_step_run.phase_run_id IS '阶段运行 ID，逻辑关联 cdeu_phase_run.phase_run_id';
COMMENT ON COLUMN dedup.cdeu_step_run.step_id IS '步骤定义 ID';
COMMENT ON COLUMN dedup.cdeu_step_run.step_code IS '步骤编码快照';
COMMENT ON COLUMN dedup.cdeu_step_run.seq_no IS '步骤顺序快照';
COMMENT ON COLUMN dedup.cdeu_step_run.status IS '步骤运行状态';
COMMENT ON COLUMN dedup.cdeu_step_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_step_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_total IS '批次总数；非批处理步骤为 1';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_success IS '成功批次数';
COMMENT ON COLUMN dedup.cdeu_step_run.batch_failed IS '失败批次数';
COMMENT ON COLUMN dedup.cdeu_step_run.completion_rule_snapshot IS '步骤完成条件快照';
COMMENT ON COLUMN dedup.cdeu_step_run.completion_snapshot IS '步骤完成判定快照';
COMMENT ON COLUMN dedup.cdeu_step_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_step_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_step_run.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 8. 作业运行（批处理实例）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_job_run (
    job_run_id          bigserial PRIMARY KEY,
    step_run_id         bigint NOT NULL,
    job_id              bigint NOT NULL,
    job_code            text NOT NULL,
    job_kind            text NOT NULL,
    execution_stage     text,
    batch_key           text,
    trigger_type        text NOT NULL DEFAULT 'PROCESS'
                        CHECK (trigger_type IN ('PROCESS','MANUAL_STAGE')),
    status              text NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    attempt             int NOT NULL DEFAULT 0,
    started_at          timestamptz,
    finished_at         timestamptz,
    processed_rows      bigint NOT NULL DEFAULT 0,
    processed_bytes     bigint NOT NULL DEFAULT 0,
    run_log_id          bigint,
    error               text,
    gmt_create          timestamptz NOT NULL DEFAULT now(),
    gmt_update          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_step ON dedup.cdeu_job_run (step_run_id, job_code);
CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_log ON dedup.cdeu_job_run (run_log_id);

COMMENT ON TABLE dedup.cdeu_job_run IS 'CDEU 作业运行实例（批处理项）';
COMMENT ON COLUMN dedup.cdeu_job_run.job_run_id IS '作业运行主键';
COMMENT ON COLUMN dedup.cdeu_job_run.step_run_id IS '步骤运行 ID，逻辑关联 cdeu_step_run.step_run_id';
COMMENT ON COLUMN dedup.cdeu_job_run.job_id IS '作业定义 ID';
COMMENT ON COLUMN dedup.cdeu_job_run.job_code IS '作业编码快照';
COMMENT ON COLUMN dedup.cdeu_job_run.job_kind IS '实际调度标识快照';
COMMENT ON COLUMN dedup.cdeu_job_run.execution_stage IS '执行阶段快照';
COMMENT ON COLUMN dedup.cdeu_job_run.batch_key IS '批处理键，如 US.AAPL:202604';
COMMENT ON COLUMN dedup.cdeu_job_run.trigger_type IS '触发类型：PROCESS 流程内 / MANUAL_STAGE 单阶段手动重跑';
COMMENT ON COLUMN dedup.cdeu_job_run.status IS '作业运行状态';
COMMENT ON COLUMN dedup.cdeu_job_run.attempt IS '尝试次数';
COMMENT ON COLUMN dedup.cdeu_job_run.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_job_run.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_job_run.processed_rows IS '处理行数';
COMMENT ON COLUMN dedup.cdeu_job_run.processed_bytes IS '处理字节数';
COMMENT ON COLUMN dedup.cdeu_job_run.run_log_id IS '逻辑关联 dedup.finv_dedup_job_run_log_export_file.run_id';
COMMENT ON COLUMN dedup.cdeu_job_run.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_job_run.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_job_run.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- 9. 初始数据：CDEU_DAILY 流程
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_process
    (process_code, name, description, version, run_mode, schedule_time, schedule_timezone,
     schedule_enabled, completion_rule, enabled)
VALUES
    ('CDEU_DAILY', 'CDEU 每日全链路',
     '采集统计 → 去重收口 → 导出（含补丁）→ 上传清理；保障任务为独立循环。',
     1, 'BOTH', '00:05:00', 'Asia/Shanghai', true, '{"mode":"ALL"}'::jsonb, true)
ON CONFLICT (process_code, version) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    run_mode = EXCLUDED.run_mode,
    schedule_time = EXCLUDED.schedule_time,
    schedule_timezone = EXCLUDED.schedule_timezone,
    schedule_enabled = EXCLUDED.schedule_enabled,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

INSERT INTO dedup.cdeu_phase
    (process_id, phase_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, completion_rule, enabled)
SELECT p.process_id, v.phase_code, v.name, v.seq_no, v.critical, v.affects_completion,
       v.run_mode, v.batch_enabled, v.completion_rule::jsonb, true
  FROM dedup.cdeu_process p
 CROSS JOIN (VALUES
    ('PHASE_COLLECT', '数据采集', 1, true, true, 'INHERIT', false, '{"mode":"ALL"}'),
    ('PHASE_DEDUP',   '去重收口', 2, true, true, 'INHERIT', false, '{"mode":"ALL"}'),
    ('PHASE_EXPORT',  '导出',     3, true, true, 'INHERIT', true,  '{"mode":"ALL"}'),
    ('PHASE_UPLOAD',  '上传',     4, true, true, 'INHERIT', true,  '{"mode":"ALL"}'),
    ('PHASE_OPS',     '保障（旁路）', 5, false, false, 'CONTINUOUS', false, '{"mode":"ALL"}')
 ) AS v(phase_code, name, seq_no, critical, affects_completion, run_mode, batch_enabled, completion_rule)
 WHERE p.process_code = 'CDEU_DAILY' AND p.version = 1
ON CONFLICT (process_id, phase_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

INSERT INTO dedup.cdeu_step
    (phase_id, step_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
     failure_policy, completion_rule, enabled)
SELECT ph.phase_id, v.step_code, v.name, v.seq_no, v.critical, v.affects_completion,
       v.run_mode, v.batch_enabled, v.batch_source, v.batch_key_field, v.batch_size,
       v.max_parallel, v.failure_policy, v.completion_rule::jsonb, true
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_process p ON p.process_id = ph.process_id
 CROSS JOIN (VALUES
    ('PHASE_COLLECT', 'STEP_INGEST_STATS', '每日入口流量统计', 1, true, true, 'INHERIT', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_COLLECT', 'STEP_BUDGET', '每日导出预算计算', 2, true, true, 'INHERIT', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_DEDUP', 'STEP_MATURITY', '行情成熟度扫描', 1, true, true, 'INHERIT', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_DEDUP', 'STEP_SEAL', '收口冻结与差异报告', 2, true, true, 'INHERIT', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_EXPORT', 'STEP_EXPORT_JOB', '单证券月导出', 1, true, true, 'INHERIT', true, 'EXPORT_JOB_CANDIDATES', 'usc,biz_month', 1, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_EXPORT', 'STEP_PATCH', '收口后补丁', 2, true, true, 'INHERIT', true, 'PATCH_CANDIDATES', 'usc,biz_month', 1, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_UPLOAD', 'STEP_UPLOAD', '文件上传', 1, true, true, 'CONTINUOUS', true, 'EXPORT_FILES', 'file_id', 200, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_UPLOAD', 'STEP_LIFECYCLE_CLEAN', '生命周期清理', 2, true, true, 'CONTINUOUS', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_OPS', 'STEP_RECOVER', '超时任务回收', 1, false, false, 'CONTINUOUS', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'),
    ('PHASE_OPS', 'STEP_HEALTH', '归档健康检查', 2, false, false, 'CONTINUOUS', false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}')
 ) AS v(phase_code, step_code, name, seq_no, critical, affects_completion, run_mode,
        batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
        failure_policy, completion_rule)
 WHERE p.process_code = 'CDEU_DAILY' AND p.version = 1 AND ph.phase_code = v.phase_code
ON CONFLICT (phase_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    batch_source = EXCLUDED.batch_source,
    batch_key_field = EXCLUDED.batch_key_field,
    batch_size = EXCLUDED.batch_size,
    max_parallel = EXCLUDED.max_parallel,
    failure_policy = EXCLUDED.failure_policy,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

INSERT INTO dedup.cdeu_job
    (step_id, job_code, name, job_kind, execution_stage, seq_no, critical,
     affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
     schedule_kind, schedule_value, job_params, enabled)
SELECT st.step_id, v.job_code, v.name, v.job_kind, v.execution_stage, v.seq_no,
       v.critical, v.affects_completion, v.trigger_mode, v.execution_mode,
       v.manual_rerun_enabled, v.schedule_kind, v.schedule_value, v.job_params::jsonb, true
  FROM dedup.cdeu_step st
  JOIN dedup.cdeu_phase ph ON ph.phase_id = st.phase_id
  JOIN dedup.cdeu_process p ON p.process_id = ph.process_id
 CROSS JOIN (VALUES
    ('STEP_INGEST_STATS', 'JOB_CALC_INGEST_STATS', '每日入口流量统计', 'CALC_DAILY_INGEST_STATS', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:05', '{}'),
    ('STEP_BUDGET', 'JOB_CALC_BUDGET', '每日导出预算计算', 'CALC_DAILY_EXPORT_BUDGET', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:05', '{}'),
    ('STEP_MATURITY', 'JOB_REFRESH_MATURITY', '行情成熟度扫描', 'REFRESH_QUOTE_MATURITY', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:10', '{}'),
    ('STEP_SEAL', 'JOB_MARK_SEALED', '收口冻结与差异报告', 'MARK_SEALED_JOBS_FROZEN', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:20', '{}'),
    ('STEP_EXPORT_JOB', 'JOB_EXPORT_LOAD', '导出装载（LOAD）', 'RUN_DUE_EXPORT_JOBS', 'LOAD', 1, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}'),
    ('STEP_EXPORT_JOB', 'JOB_EXPORT_EXPORT', '导出文件生成（EXPORT）', 'RUN_DUE_EXPORT_JOBS', 'EXPORT', 2, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}'),
    ('STEP_EXPORT_JOB', 'JOB_EXPORT_REGISTER', '导出文件登记（REGISTER）', 'RUN_DUE_EXPORT_JOBS', 'REGISTER', 3, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}'),
    ('STEP_EXPORT_JOB', 'JOB_EXPORT_CLEANUP', '中转表清理（CLEANUP）', 'RUN_DUE_EXPORT_JOBS', 'CLEANUP', 4, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}'),
    ('STEP_PATCH', 'JOB_SCAN_PATCH', '收口后补丁扫描', 'SCAN_POST_SEAL_PATCHES', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '03:00', '{}'),
    ('STEP_UPLOAD', 'JOB_UPLOAD_FILES', '待上传文件上传', 'UPLOAD_PENDING_EXPORT_FILES', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '300', '{}'),
    ('STEP_LIFECYCLE_CLEAN', 'JOB_CLEAN_EXPIRED', '生命周期清理', 'CLEAN_EXPIRED_EXPORT_ARTIFACTS', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '06:00', '{}'),
    ('STEP_RECOVER', 'JOB_RECOVER_TIMEOUT', '超时任务回收', 'RECOVER_TIMEOUT_EXPORT_JOBS', NULL, 1, false, false, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '300', '{}'),
    ('STEP_HEALTH', 'JOB_CHECK_HEALTH', '归档健康检查', 'CHECK_EXPORT_JOB_HEALTH', NULL, 1, false, false, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '900', '{}')
 ) AS v(step_code, job_code, name, job_kind, execution_stage, seq_no, critical,
        affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
        schedule_kind, schedule_value, job_params)
 WHERE p.process_code = 'CDEU_DAILY' AND p.version = 1 AND st.step_code = v.step_code
ON CONFLICT (step_id, job_code) DO UPDATE SET
    name = EXCLUDED.name,
    job_kind = EXCLUDED.job_kind,
    execution_stage = EXCLUDED.execution_stage,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    trigger_mode = EXCLUDED.trigger_mode,
    execution_mode = EXCLUDED.execution_mode,
    manual_rerun_enabled = EXCLUDED.manual_rerun_enabled,
    schedule_kind = EXCLUDED.schedule_kind,
    schedule_value = EXCLUDED.schedule_value,
    job_params = EXCLUDED.job_params,
    enabled = EXCLUDED.enabled,
    gmt_update = now();
