-- =====================================================================
-- FinvQuant PostgreSQL V66：Job 运行详细日志行
--
-- 运行汇总仍保存在 dedup.finv_dedup_job_run_log_export_file；
-- 本表按运行记录追加执行阶段、进度明细、错误等日志行。
-- =====================================================================

CREATE TABLE IF NOT EXISTS dedup.finv_dedup_job_run_log_line (
    line_id          bigserial PRIMARY KEY,
    run_id           bigint NOT NULL,
    level            text NOT NULL DEFAULT 'INFO',
    stage            text,
    message          text NOT NULL,
    progress_percent double precision,
    gmt_create       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_finv_dedup_job_run_log_line_run
    ON dedup.finv_dedup_job_run_log_line (run_id, line_id);

COMMENT ON TABLE dedup.finv_dedup_job_run_log_line IS 'Job 运行详细日志行，run_id 逻辑关联 finv_dedup_job_run_log_export_file.run_id，无外键';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_line.level IS '日志级别：INFO / WARN / ERROR';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_line.stage IS '发生日志时的执行阶段';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_line.message IS '日志正文';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_line.progress_percent IS '写入日志时的进度百分比，NULL 表示不适用';
