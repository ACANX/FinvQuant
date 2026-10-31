-- 任务运行日志增加「阶段 / 进度 / 心跳」字段。
-- 目的：前端总览页可感知一次 Job 执行当前处于哪一阶段（LOAD / EXPORT / CLEANUP / UPLOAD / PATCH …）、
-- 进度百分比、最近心跳时间，从而区分「正常进行中 / 卡住 / 已完成」。
-- 约束：只新增列与索引，不修改既有列；表仍在 dedup schema，全库无外键。

ALTER TABLE dedup.finv_dedup_job_run_log_export_file
    ADD COLUMN IF NOT EXISTS stage            text,
    ADD COLUMN IF NOT EXISTS progress_percent double precision,
    ADD COLUMN IF NOT EXISTS progress_detail  text,
    ADD COLUMN IF NOT EXISTS gmt_heartbeat    timestamptz;

-- 未结束（gmt_finish IS NULL）的运行记录通常只有个位数，部分索引便于运行快照查询。
CREATE INDEX IF NOT EXISTS idx_dedupjobrunlogexportfile_running
    ON dedup.finv_dedup_job_run_log_export_file (job_kind, gmt_start DESC)
    WHERE gmt_finish IS NULL;

COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.stage IS '当前/最近执行阶段：RUNNING / STATS / BUDGET / MATURITY / SEAL / LOAD / EXPORT / CLEANUP / UPLOAD / PATCH / RECOVER / HEALTH / DONE / FAILED / PAUSED';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.progress_percent IS '进度百分比 0~100；NULL 表示不适用，负值表示无法估算总量的不确定进度';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.progress_detail IS '进度明细（如当前证券月、已装载行数、已上传文件数）';
COMMENT ON COLUMN dedup.finv_dedup_job_run_log_export_file.gmt_heartbeat IS '最近心跳时间，用于判断运行中的 Job 是否卡住';
