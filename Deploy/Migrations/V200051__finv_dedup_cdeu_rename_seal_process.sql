-- =====================================================================
-- FinvQuant PostgreSQL V51：收口冻结名称调整
--
-- 将 PROCESS_SEAL / JOB_MARK_SEALED / STEP_MARK_SEALED 的展示名称统一为
-- 「收口冻结与差异报告生成」，编码与执行逻辑不变。
-- =====================================================================

UPDATE dedup.cdeu_process
   SET name = '收口冻结与差异报告生成', gmt_update = now()
 WHERE process_code = 'PROCESS_SEAL';

UPDATE dedup.cdeu_job
   SET name = '收口冻结与差异报告生成', gmt_update = now()
 WHERE job_code = 'JOB_MARK_SEALED';

UPDATE dedup.cdeu_job_step
   SET name = '收口冻结与差异报告生成', gmt_update = now()
 WHERE step_code = 'STEP_MARK_SEALED';
