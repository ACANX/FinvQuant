-- =====================================================================
-- FinvQuant PostgreSQL V53：阶段命名与归档复核阶段
--
--   PHASE_COLLECT → 历史行情采集
--   PHASE_DEDUP   → 历史行情去重
--   新增 PHASE_REVIEW「归档复核」于上传之后（seq=6）
--
-- 新增阶段当前无 JOB，使用 PROCESS_REVIEW 占位 Process；运行初始化时自动 SUCCESS。
-- =====================================================================

UPDATE dedup.cdeu_phase
   SET name = '历史行情采集', gmt_update = now()
 WHERE phase_code = 'PHASE_COLLECT';

UPDATE dedup.cdeu_phase
   SET name = '历史行情去重', gmt_update = now()
 WHERE phase_code = 'PHASE_DEDUP';

-- 新增归档复核阶段（沿用当前 CDEU_DAILY 工作流版本）。
INSERT INTO dedup.cdeu_phase
    (workflow_id, phase_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, completion_rule, enabled)
SELECT w.workflow_id, 'PHASE_REVIEW', '归档复核', 6, true, true, 'INHERIT',
       false, '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_workflow w
 WHERE w.workflow_code = 'CDEU_DAILY' AND w.version = 1
ON CONFLICT (workflow_id, phase_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    run_mode = EXCLUDED.run_mode,
    batch_enabled = EXCLUDED.batch_enabled,
    completion_rule = EXCLUDED.completion_rule,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

-- 占位 Process：无 JOB，初始化时自动成功，保证工作流可正常完成。
INSERT INTO dedup.cdeu_process
    (phase_id, process_code, name, seq_no, critical, affects_completion, run_mode,
     batch_enabled, batch_source, batch_key_field, batch_size, max_parallel,
     failure_policy, completion_rule, enabled)
SELECT ph.phase_id, 'PROCESS_REVIEW', '归档复核', 1, true, true, 'INHERIT',
       false, NULL, NULL, NULL, 1, 'CONTINUE', '{"mode":"ALL"}'::jsonb, true
  FROM dedup.cdeu_phase ph
  JOIN dedup.cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND w.version = 1
   AND ph.phase_code = 'PHASE_REVIEW'
ON CONFLICT (phase_id, process_code) DO UPDATE SET
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
