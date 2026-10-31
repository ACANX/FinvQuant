-- =====================================================================
-- FinvQuant PostgreSQL V61：按证券 × 年月动态生成导出 JOB
--
-- 固定导出阶段 JOB（LOAD/EXPORT/REGISTER/CLEANUP、补丁扫描）作为内部执行壳，
-- 对前端隐藏；真实 JOB 由批次清单在运行时按 (usc, biz_month) 生成：
--   BASE → PROCESS_EXPORT_FILE_MON_BASE
--   INCR → PROCESS_EXPORT_FILE_MON_INCR
-- 每个动态 JOB 的 (usc, biz_month) 全局唯一。
-- =====================================================================

ALTER TABLE dedup.cdeu_job
    ADD COLUMN IF NOT EXISTS ui_visible boolean NOT NULL DEFAULT true;

UPDATE dedup.cdeu_job
   SET ui_visible = false, gmt_update = now()
 WHERE job_code IN ('JOB_EXPORT_LOAD', 'JOB_EXPORT_EXPORT',
                    'JOB_EXPORT_REGISTER', 'JOB_EXPORT_CLEANUP',
                    'JOB_SCAN_PATCH');

CREATE TABLE IF NOT EXISTS dedup.cdeu_dynamic_job (
    dynamic_job_id  bigserial PRIMARY KEY,
    process_id      bigint NOT NULL,
    job_code        text NOT NULL,
    name            text NOT NULL,
    usc             text NOT NULL,
    biz_month       int4 NOT NULL,
    export_type     text NOT NULL CHECK (export_type IN ('BASE', 'INCR')),
    export_job_id   bigint,
    seq_no          int NOT NULL DEFAULT 1,
    status          text NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    error           text,
    gmt_start       timestamptz,
    gmt_finish      timestamptz,
    gmt_create      timestamptz NOT NULL DEFAULT now(),
    gmt_update      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (usc, biz_month)
);

CREATE INDEX IF NOT EXISTS idx_cdeu_dynamic_job_process
    ON dedup.cdeu_dynamic_job (process_id, seq_no);
CREATE INDEX IF NOT EXISTS idx_cdeu_dynamic_job_status
    ON dedup.cdeu_dynamic_job (status, gmt_update);

COMMENT ON TABLE dedup.cdeu_dynamic_job IS '按批次动态生成的单证券导出 JOB；(usc, biz_month) 全局唯一';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.dynamic_job_id IS '动态 JOB 主键';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.process_id IS '所属 Process，逻辑关联 cdeu_process.process_id';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.job_code IS '动态 JOB 编码，如 JOB_EXPORT_BASE_GCMAIN_202501';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.name IS '动态 JOB 名称，包含证券/年月/类型';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.biz_month IS '导出业务年月 yyyyMM';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.export_type IS '导出类型：BASE / INCR';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.export_job_id IS '关联 finv_dedup_export_job.job_id';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.seq_no IS '同 Process 内展示顺序';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.status IS '动态 JOB 最近一次运行状态';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.error IS '最近一次失败原因';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.gmt_start IS '最近一次开始时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.gmt_finish IS '最近一次结束时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job.gmt_update IS '更新时间';
