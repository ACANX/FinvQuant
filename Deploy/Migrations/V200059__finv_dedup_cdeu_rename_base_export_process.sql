-- =====================================================================
-- FinvQuant PostgreSQL V59：BASE 导出 Process 编码调整
--
--   PROCESS_EXPORT_FILE_MON → PROCESS_EXPORT_FILE_MON_BASE
-- 名称与执行逻辑不变；JOB 通过 process_id 关联。
-- =====================================================================

UPDATE dedup.cdeu_process
   SET process_code = 'PROCESS_EXPORT_FILE_MON_BASE',
       gmt_update = now()
 WHERE process_code = 'PROCESS_EXPORT_FILE_MON';
