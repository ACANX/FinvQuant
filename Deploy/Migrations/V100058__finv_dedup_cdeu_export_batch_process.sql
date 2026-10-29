-- =====================================================================
-- FinvQuant PostgreSQL V58：导出前置批次清单 Process + JOB + 输出表
--
--   PHASE_EXPORT：
--     PROCESS_EXPORT_BATCH（新，seq=1）证券导出批次清单获取
--     PROCESS_EXPORT_FILE_MON（原 seq=1 → 2）
--     PROCESS_EXPORT_FILE_MON_INCR（原 seq=2 → 3）
--     PROCESS_RECOVER（原 seq=3 → 4）
--
-- 新 JOB 复用 RUN_DUE_EXPORT_JOBS 的窗口调度，在 META/LOAD 前生成批次清单；
-- 输出写入 dedup.finv_dedup_export_batch，作为后续 BASE/INCR 导出 Process 的输入。
-- =====================================================================

-- 1. 先按倒序腾出 Process 顺序。
UPDATE dedup.cdeu_process p
   SET seq_no = 4, gmt_update = now()
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE p.phase_id = ph.phase_id
   AND w.workflow_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_EXPORT'
   AND p.process_code = 'PROCESS_RECOVER';

UPDATE dedup.cdeu_process p
   SET seq_no = 3, gmt_update = now()
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE p.phase_id = ph.phase_id
   AND w.workflow_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_EXPORT'
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON_INCR';

UPDATE dedup.cdeu_process p
   SET seq_no = 2, gmt_update = now()
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE p.phase_id = ph.phase_id
   AND w.workflow_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_EXPORT'
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON';

-- 2. 新增批次清单 Process。
INSERT INTO dedup.cdeu_process
    (phase_id, process_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
     failure_policy, completion_rule, enabled)
SELECT ph.phase_id, 'PROCESS_EXPORT_BATCH', '证券导出批次清单获取', 1,
       true, true, 'INHERIT', false, NULL, NULL, NULL, 1, 'CONTINUE',
       '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ph.phase_code = 'PHASE_EXPORT'
ON CONFLICT (phase_id, process_code) DO UPDATE SET
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

-- 3. 新增 JOB：复用 RUN_DUE_EXPORT_JOBS，执行阶段 META 之前的 BATCH。
INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, execution_stage, seq_no, critical,
     affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
     schedule_kind, schedule_value, job_params, enabled)
SELECT p.process_id, 'JOB_FETCH_EXPORT_BATCH', '证券导出批次清单获取',
       'RUN_DUE_EXPORT_JOBS', 'BATCH', 1, true, true, 'EMBEDDED', 'EMBEDDED',
       false, 'NONE', NULL, '{}'::jsonb, true
  FROM dedup.cdeu_process p
 WHERE p.process_code = 'PROCESS_EXPORT_BATCH'
ON CONFLICT (process_id, job_code) DO UPDATE SET
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

INSERT INTO dedup.cdeu_job_step
    (job_id, step_code, name, seq_no, critical, affects_completion, enabled)
SELECT j.job_id, 'STEP_FETCH_EXPORT_BATCH', '证券导出批次清单获取', 1,
       true, true, true
  FROM dedup.cdeu_job j
 WHERE j.job_code = 'JOB_FETCH_EXPORT_BATCH'
ON CONFLICT (job_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

-- 4. 批次清单输出表。
CREATE TABLE IF NOT EXISTS dedup.finv_dedup_export_batch (
    batch_id        bigserial PRIMARY KEY,
    batch_no        text NOT NULL,
    biz_date        date NOT NULL,
    usc             text NOT NULL,
    biz_month       int4 NOT NULL,
    export_type     text NOT NULL CHECK (export_type IN ('BASE', 'INCR')),
    source_job_id   bigint,
    status          text NOT NULL DEFAULT 'READY' CHECK (status IN ('READY', 'CONSUMED')),
    gmt_create      timestamptz NOT NULL DEFAULT now(),
    gmt_update      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (batch_no, usc, biz_month, export_type)
);

CREATE INDEX IF NOT EXISTS idx_dedupexportbatch_lookup
    ON dedup.finv_dedup_export_batch (batch_no, export_type, usc, biz_month);
CREATE INDEX IF NOT EXISTS idx_dedupexportbatch_biz
    ON dedup.finv_dedup_export_batch (biz_date, status);

COMMENT ON TABLE dedup.finv_dedup_export_batch IS '导出批次清单：当前批次需要导出的证券、业务年月与 BASE/INCR 类型，作为后续导出 Process 的输入';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.batch_id IS '批次项主键';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.batch_no IS '批次号，当前按业务日期生成 yyyyMMdd';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.biz_date IS '批次业务日期';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.biz_month IS '导出业务年月 yyyyMM';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.export_type IS '导出类型：BASE 基础文件 / INCR 增量文件';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.source_job_id IS 'INCR 条目关联的 finv_dedup_export_job.job_id；BASE 可为空';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.status IS '条目状态：READY 待消费 / CONSUMED 已消费';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.finv_dedup_export_batch.gmt_update IS '更新时间';
