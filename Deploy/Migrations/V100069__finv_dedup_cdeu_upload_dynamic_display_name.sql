-- =====================================================================
-- FinvQuant PostgreSQL V69：上传动态 JOB 名称与编码分离
--
-- job_code : JOB_UPLOAD_BASE_V1_GCMAIN_201910
-- name     : UPLOAD_BASE_V1_GCMain_201910
-- =====================================================================

UPDATE dedup.cdeu_job_dynamic
   SET name = 'UPLOAD_' || file_type || '_V' || file_version || '_' ||
              usc || '_' || biz_month,
       gmt_update = now()
 WHERE job_category = 'UPLOAD';
