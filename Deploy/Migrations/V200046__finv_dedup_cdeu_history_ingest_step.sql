-- =====================================================================
-- FinvQuant PostgreSQL V46：数据采集阶段增加「历史行情数据采集」占位步骤
--
-- 背景：历史行情数据采集当前尚未接入 CDEU 执行引擎，先以非关键占位步骤登记，
--       STEP 下无 JOB；流程运行初始化时由 seed 逻辑自动置为 SUCCESS。
-- 位置：PHASE_COLLECT 第 1 步，原有入口统计/预算顺延为第 2/3 步。
-- 约束：只新增步骤定义，不改 JOB；step 下无 JOB 时运行表自动成功。
-- =====================================================================

-- 先为新增步骤腾出 seq=1：预算 2 → 3，入口统计 1 → 2。
UPDATE dedup.cdeu_step AS st
   SET seq_no = 3, gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE st.phase_id = ph.phase_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_COLLECT'
   AND st.step_code = 'STEP_BUDGET';

UPDATE dedup.cdeu_step AS st
   SET seq_no = 2, gmt_update = now()
  FROM dedup.cdeu_phase AS ph
  JOIN dedup.cdeu_process AS p ON p.process_id = ph.process_id
 WHERE st.phase_id = ph.phase_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_COLLECT'
   AND st.step_code = 'STEP_INGEST_STATS';

INSERT INTO dedup.cdeu_step
    (phase_id, step_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
     failure_policy, completion_rule, enabled)
SELECT ph.phase_id, 'STEP_HISTORY_INGEST', '历史行情数据采集', 1,
       false, true, 'INHERIT', false, NULL, NULL, NULL, 1,
       'CONTINUE', '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_process p ON p.process_id = ph.process_id
 WHERE p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_COLLECT'
ON CONFLICT (phase_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    failure_policy = EXCLUDED.failure_policy,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();
