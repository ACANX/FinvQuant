-- =====================================================================
-- FinvQuant PostgreSQL V55：导出 Process 名称调整
--
-- PROCESS_EXPORT_JOB 展示名称由「单证券月导出」调整为
-- 「证券历史月度行情文件导出」；编码与执行逻辑不变。
-- =====================================================================

UPDATE dedup.cdeu_process
   SET name = '证券历史月度行情文件导出', gmt_update = now()
 WHERE process_code = 'PROCESS_EXPORT_JOB';
