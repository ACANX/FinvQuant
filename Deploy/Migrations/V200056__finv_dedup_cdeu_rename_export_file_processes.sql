-- =====================================================================
-- FinvQuant PostgreSQL V56：导出阶段 Process 编码与名称调整
--
--   PROCESS_EXPORT_JOB → PROCESS_EXPORT_FILE_MON（名称保持「证券历史月度行情文件导出」）
--   PROCESS_PATCH      → PROCESS_EXPORT_FILE_MON_INCR
--   名称「收口后补丁」→「证券历史月度行情增量文件导出」
-- JOB 通过 process_id 关联，无需修改；编码与执行逻辑不变。
-- =====================================================================

UPDATE dedup.cdeu_process
   SET process_code = 'PROCESS_EXPORT_FILE_MON_INCR',
       name = '证券历史月度行情增量文件导出',
       gmt_update = now()
 WHERE process_code = 'PROCESS_PATCH';

UPDATE dedup.cdeu_process
   SET process_code = 'PROCESS_EXPORT_FILE_MON',
       gmt_update = now()
 WHERE process_code = 'PROCESS_EXPORT_JOB';
