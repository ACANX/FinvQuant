-- =====================================================================
-- FinvQuant PostgreSQL V63：动态 JOB 表改名 + 上传动态 JOB
--
--   cdeu_dynamic_job      → cdeu_job_dynamic
--   cdeu_dynamic_job_step → cdeu_job_dynamic_step
--
-- 动态 JOB 分为两类：
--   EXPORT：单证券月导出 JOB
--   UPLOAD：单月归档文件上传 JOB
-- 上传动态 JOB 基于 finv_dedup_export_file 中已导出且未完成上传的文件生成。
-- =====================================================================

ALTER TABLE dedup.cdeu_dynamic_job RENAME TO cdeu_job_dynamic;
ALTER TABLE dedup.cdeu_dynamic_job_step RENAME TO cdeu_job_dynamic_step;

ALTER INDEX IF EXISTS dedup.idx_cdeu_dynamic_job_process RENAME TO idx_cdeu_job_dynamic_process;
ALTER INDEX IF EXISTS dedup.idx_cdeu_dynamic_job_status RENAME TO idx_cdeu_job_dynamic_status;
ALTER INDEX IF EXISTS dedup.idx_cdeu_dynamic_job_step_job RENAME TO idx_cdeu_job_dynamic_step_job;

ALTER TABLE dedup.cdeu_job_dynamic
    ADD COLUMN IF NOT EXISTS job_category text NOT NULL DEFAULT 'EXPORT'
        CHECK (job_category IN ('EXPORT', 'UPLOAD')),
    ADD COLUMN IF NOT EXISTS file_id bigint,
    ADD COLUMN IF NOT EXISTS file_type text,
    ADD COLUMN IF NOT EXISTS file_version int;

ALTER TABLE dedup.cdeu_job_dynamic
    ALTER COLUMN export_type DROP NOT NULL;

-- 原 (usc,biz_month) 唯一约束只适用于导出动态 JOB；上传按 file_id 唯一。
ALTER TABLE dedup.cdeu_job_dynamic
    DROP CONSTRAINT IF EXISTS cdeu_dynamic_job_usc_biz_month_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeu_job_dynamic_export
    ON dedup.cdeu_job_dynamic (usc, biz_month)
    WHERE job_category = 'EXPORT';
CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeu_job_dynamic_upload
    ON dedup.cdeu_job_dynamic (file_id)
    WHERE job_category = 'UPLOAD';

CREATE INDEX IF NOT EXISTS idx_cdeu_job_dynamic_file
    ON dedup.cdeu_job_dynamic (file_id, job_category);

-- 固定上传执行壳 JOB 对前端隐藏，实际由文件级动态 JOB 展示。
UPDATE dedup.cdeu_job
   SET ui_visible = false, gmt_update = now()
 WHERE job_code = 'JOB_UPLOAD_FILES';
