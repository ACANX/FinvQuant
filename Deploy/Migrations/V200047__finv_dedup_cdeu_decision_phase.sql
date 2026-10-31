-- =====================================================================
-- FinvQuant PostgreSQL V47：去重收口与导出之间增加「导出判定」阶段
--
-- 定位：在去重收口完成后、真正导出前，统一完成「是否有数据符合导出要求」的
--       检查与决策，为后续是否执行导出/上传提供明确的流程阶段。
-- 暂不登记 JOB：先以占位步骤接入流程；无 JOB 的步骤由运行初始化逻辑自动
--       置为 SUCCESS。后续接入具体判定 Job 时，只需新增 cdeu_job 定义。
-- 阶段顺序：采集(1) → 去重收口(2) → 导出判定(3) → 导出(4) → 上传(5)。
-- =====================================================================

-- 先为导出/上传阶段腾出 seq：上传 4 → 5，导出 3 → 4。
UPDATE dedup.cdeu_phase AS ph
   SET seq_no = 5, gmt_update = now()
  FROM dedup.cdeu_process AS p
 WHERE ph.process_id = p.process_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_UPLOAD';

UPDATE dedup.cdeu_phase AS ph
   SET seq_no = 4, gmt_update = now()
  FROM dedup.cdeu_process AS p
 WHERE ph.process_id = p.process_id
   AND p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_EXPORT';

INSERT INTO dedup.cdeu_phase
    (process_id, phase_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, completion_rule, enabled)
SELECT p.process_id, 'PHASE_DECISION', '导出判定', 3,
       true, true, 'INHERIT', false, '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_process p
 WHERE p.process_code = 'CDEU_DAILY'
ON CONFLICT (process_id, phase_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

INSERT INTO dedup.cdeu_step
    (phase_id, step_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
     failure_policy, completion_rule, enabled)
SELECT ph.phase_id, 'STEP_EXPORT_DECISION', '导出条件判定', 1,
       true, true, 'INHERIT', false, NULL, NULL, NULL, 1,
       'CONTINUE', '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_process p ON p.process_id = ph.process_id
 WHERE p.process_code = 'CDEU_DAILY'
   AND ph.phase_code = 'PHASE_DECISION'
ON CONFLICT (phase_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    batch_source = EXCLUDED.batch_source,
    batch_key_field = EXCLUDED.batch_key_field,
    batch_size = EXCLUDED.batch_size,
    max_parallel = EXCLUDED.max_parallel,
    failure_policy = EXCLUDED.failure_policy,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();
