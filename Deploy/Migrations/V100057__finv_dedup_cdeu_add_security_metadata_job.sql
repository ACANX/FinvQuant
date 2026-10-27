-- =====================================================================
-- FinvQuant PostgreSQL V57：月度导出 Process 增加「证券元数据获取」JOB
--
-- PROCESS_EXPORT_FILE_MON 下新增 JOB_FETCH_SECURITY_METADATA，位于导出阶段最前：
--   seq=1 证券元数据获取（META）
--   seq=2..5 原 LOAD / EXPORT / REGISTER / CLEANUP
-- 该 JOB 复用 RUN_DUE_EXPORT_JOBS 的窗口调度，在导出前从证券字典刷新
-- dedup.finv_dedup_usc 的缺失元数据。
-- =====================================================================

-- 先按倒序给原有 4 个 JOB 腾出 seq=2..5，避免唯一约束冲突。
UPDATE dedup.cdeu_job j
   SET seq_no = 5, gmt_update = now()
  FROM dedup.cdeu_process p
 WHERE j.process_id = p.process_id
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON'
   AND j.job_code = 'JOB_EXPORT_CLEANUP';

UPDATE dedup.cdeu_job j
   SET seq_no = 4, gmt_update = now()
  FROM dedup.cdeu_process p
 WHERE j.process_id = p.process_id
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON'
   AND j.job_code = 'JOB_EXPORT_REGISTER';

UPDATE dedup.cdeu_job j
   SET seq_no = 3, gmt_update = now()
  FROM dedup.cdeu_process p
 WHERE j.process_id = p.process_id
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON'
   AND j.job_code = 'JOB_EXPORT_EXPORT';

UPDATE dedup.cdeu_job j
   SET seq_no = 2, gmt_update = now()
  FROM dedup.cdeu_process p
 WHERE j.process_id = p.process_id
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON'
   AND j.job_code = 'JOB_EXPORT_LOAD';

INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, execution_stage, seq_no, critical,
     affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
     schedule_kind, schedule_value, job_params, enabled)
SELECT p.process_id, 'JOB_FETCH_SECURITY_METADATA', '证券元数据获取',
       'RUN_DUE_EXPORT_JOBS', 'META', 1, true, true, 'EMBEDDED', 'EMBEDDED',
       false, 'NONE', NULL, '{}'::jsonb, true
  FROM dedup.cdeu_process p
 WHERE p.process_code = 'PROCESS_EXPORT_FILE_MON'
ON CONFLICT (process_id, job_code) DO UPDATE SET
    name = EXCLUDED.name,
    job_kind = EXCLUDED.job_kind,
    execution_stage = EXCLUDED.execution_stage,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    trigger_mode = EXCLUDED.trigger_mode,
    execution_mode = EXCLUDED.execution_mode,
    manual_rerun_enabled = EXCLUDED.manual_rerun_enabled,
    schedule_kind = EXCLUDED.schedule_kind,
    schedule_value = EXCLUDED.schedule_value,
    job_params = EXCLUDED.job_params,
    enabled = EXCLUDED.enabled,
    gmt_update = now();

INSERT INTO dedup.cdeu_job_step
    (job_id, step_code, name, seq_no, critical, affects_completion, enabled)
SELECT j.job_id, 'STEP_FETCH_SECURITY_METADATA', '证券元数据获取', 1,
       true, true, true
  FROM dedup.cdeu_job j
 WHERE j.job_code = 'JOB_FETCH_SECURITY_METADATA'
ON CONFLICT (job_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    enabled = EXCLUDED.enabled,
    gmt_update = now();
