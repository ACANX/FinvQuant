-- =====================================================================
-- FinvQuant PostgreSQL V64：导出决策阶段并行依赖与调度调整
--
-- PHASE_DECISION 内部拓扑：
--   入口流量统计 ─┐
--                  ├→ 每日导出预算计算 → 导出条件判定
--   行情成熟度扫描 ─┘
--
-- 调度：
--   00:05 入口流量统计 / 行情成熟度扫描（并行）
--   00:10 每日导出预算计算（在前两者之后）
-- =====================================================================

-- 1. 预算先移到临时序号，避免与成熟度交换时唯一约束冲突。
UPDATE dedup.cdeu_process p
   SET seq_no = 99, gmt_update = now()
  FROM dedup.cdeu_phase ph
 WHERE p.phase_id = ph.phase_id
   AND ph.phase_code = 'PHASE_DECISION'
   AND p.process_code = 'PROCESS_BUDGET';

UPDATE dedup.cdeu_process p
   SET seq_no = 2, gmt_update = now()
  FROM dedup.cdeu_phase ph
 WHERE p.phase_id = ph.phase_id
   AND ph.phase_code = 'PHASE_DECISION'
   AND p.process_code = 'PROCESS_MATURITY';

UPDATE dedup.cdeu_process p
   SET seq_no = 3, gmt_update = now()
  FROM dedup.cdeu_phase ph
 WHERE p.phase_id = ph.phase_id
   AND ph.phase_code = 'PHASE_DECISION'
   AND p.process_code = 'PROCESS_BUDGET';

-- 2. 调整调度时间。
UPDATE dedup.cdeu_job
   SET schedule_value = '00:05', gmt_update = now()
 WHERE job_code = 'JOB_REFRESH_MATURITY';

UPDATE dedup.cdeu_job
   SET schedule_value = '00:10', gmt_update = now()
 WHERE job_code = 'JOB_CALC_BUDGET';
