-- =====================================================================
-- FinvQuant PostgreSQL V68：统一上传动态 JOB 编码/名称
--
-- 例：JOB_UPLOAD_BASE_V1_GCMAIN_201910
--     JOB_UPLOAD_MANIFEST_V1_GCMAIN_201910
-- =====================================================================

UPDATE dedup.cdeu_job_dynamic
   SET job_code = 'JOB_UPLOAD_' || file_type || '_V' || file_version || '_' ||
                  upper(usc) || '_' || biz_month,
       name = 'JOB_UPLOAD_' || file_type || '_V' || file_version || '_' ||
              upper(usc) || '_' || biz_month,
       gmt_update = now()
 WHERE job_category = 'UPLOAD';
