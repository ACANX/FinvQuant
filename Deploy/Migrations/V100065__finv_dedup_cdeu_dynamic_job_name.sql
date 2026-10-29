-- =====================================================================
-- FinvQuant PostgreSQL V65：统一导出动态 JOB 名称格式
--
-- 例：EXPORT_MON_BASE_GCMain_201912
--     EXPORT_MON_INCR_GCMain_201912
-- =====================================================================

UPDATE dedup.cdeu_job_dynamic
   SET name = 'EXPORT_MON_' || export_type || '_' || usc || '_' || biz_month,
       gmt_update = now()
 WHERE job_category = 'EXPORT';
