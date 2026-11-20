-- =====================================================================
-- FinvQuant PostgreSQL V70：动态 JOB 关联运行日志与工作流运行
--
-- run_log_id      关联 finv_dedup_job_run_log_export_file.run_id
-- workflow_run_id 关联 cdeu_workflow_run.run_id，用于当前工作流只展示本轮动态 JOB
-- =====================================================================

ALTER TABLE dedup.cdeu_job_dynamic
    ADD COLUMN IF NOT EXISTS run_log_id bigint,
    ADD COLUMN IF NOT EXISTS workflow_run_id bigint;

CREATE INDEX IF NOT EXISTS idx_cdeu_job_dynamic_workflow_run
    ON dedup.cdeu_job_dynamic (workflow_run_id, job_category);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_dynamic_run_log
    ON dedup.cdeu_job_dynamic (run_log_id);

COMMENT ON COLUMN dedup.cdeu_job_dynamic.run_log_id IS '最近一次执行对应的 Job 运行日志 ID，逻辑关联 finv_dedup_job_run_log_export_file.run_id，无外键';
COMMENT ON COLUMN dedup.cdeu_job_dynamic.workflow_run_id IS '生成本动态 JOB 的 CDEU 工作流运行 ID，逻辑关联 cdeu_workflow_run.run_id，无外键';
