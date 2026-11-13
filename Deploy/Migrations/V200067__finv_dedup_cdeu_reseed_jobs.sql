-- =====================================================================
-- FinvQuant PostgreSQL V67：修复 CDEU JOB / JOB Step 定义缺失
--
-- 背景：历史迁移只在首次建表时 seed JOB 定义；人工清理 cdeu_job 后，
--       CDEU_DAILY 工作流虽然存在，但 StartCDEUWorkflow 查不到 JOB，
--       会报「CDEU 未登记 CDEU_DAILY 工作流定义」。
-- 本迁移按当前 Workflow → Phase → Process → JOB → Step 模型幂等重建。
-- =====================================================================

-- 清理指向已不存在 JOB 的孤立 Step，避免旧 job_id 残留。
DELETE FROM dedup.cdeu_job_step s
 WHERE NOT EXISTS (
       SELECT 1 FROM dedup.cdeu_job j WHERE j.job_id = s.job_id
 );

INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, execution_stage, seq_no, critical,
     affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
     schedule_kind, schedule_value, job_params, enabled, ui_visible)
SELECT p.process_id, v.job_code, v.name, v.job_kind, v.execution_stage, v.seq_no,
       v.critical, v.affects_completion, v.trigger_mode, v.execution_mode,
       v.manual_rerun_enabled, v.schedule_kind, v.schedule_value,
       v.job_params::jsonb, true, v.ui_visible
  FROM (VALUES
    ('PROCESS_INGEST_STATS', 'JOB_CALC_INGEST_STATS', '每日入口流量统计', 'CALC_DAILY_INGEST_STATS', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:05', '{}', true),
    ('PROCESS_BUDGET', 'JOB_CALC_BUDGET', '每日导出预算计算', 'CALC_DAILY_EXPORT_BUDGET', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:10', '{}', true),
    ('PROCESS_MATURITY', 'JOB_REFRESH_MATURITY', '行情成熟度扫描', 'REFRESH_QUOTE_MATURITY', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:05', '{}', true),
    ('PROCESS_SEAL', 'JOB_MARK_SEALED', '收口冻结与差异报告生成', 'MARK_SEALED_JOBS_FROZEN', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:20', '{}', true),
    ('PROCESS_EXPORT_FILE_MON_BASE', 'JOB_EXPORT_LOAD', '导出装载（LOAD）', 'RUN_DUE_EXPORT_JOBS', 'LOAD', 2, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}', false),
    ('PROCESS_EXPORT_FILE_MON_BASE', 'JOB_EXPORT_EXPORT', '导出文件生成（EXPORT）', 'RUN_DUE_EXPORT_JOBS', 'EXPORT', 3, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}', false),
    ('PROCESS_EXPORT_FILE_MON_BASE', 'JOB_EXPORT_REGISTER', '导出文件登记（REGISTER）', 'RUN_DUE_EXPORT_JOBS', 'REGISTER', 4, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}', false),
    ('PROCESS_EXPORT_FILE_MON_BASE', 'JOB_EXPORT_CLEANUP', '中转表清理（CLEANUP）', 'RUN_DUE_EXPORT_JOBS', 'CLEANUP', 5, true, true, 'EMBEDDED', 'EMBEDDED', true, 'NONE', NULL, '{"windowed":true}', false),
    ('PROCESS_EXPORT_FILE_MON_INCR', 'JOB_SCAN_PATCH', '收口后补丁扫描', 'SCAN_POST_SEAL_PATCHES', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '03:00', '{}', false),
    ('PROCESS_UPLOAD', 'JOB_UPLOAD_FILES', '待上传文件上传', 'UPLOAD_PENDING_EXPORT_FILES', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '300', '{}', false),
    ('PROCESS_LIFECYCLE_CLEAN', 'JOB_CLEAN_EXPIRED', '生命周期清理', 'CLEAN_EXPIRED_EXPORT_ARTIFACTS', NULL, 1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '06:00', '{}', true),
    ('PROCESS_RECOVER', 'JOB_RECOVER_TIMEOUT', '超时任务回收', 'RECOVER_TIMEOUT_EXPORT_JOBS', NULL, 1, false, false, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '300', '{}', true),
    ('PROCESS_HEALTH', 'JOB_CHECK_HEALTH', '归档健康检查', 'CHECK_EXPORT_JOB_HEALTH', NULL, 1, false, false, 'SCHEDULED', 'DIRECT', true, 'INTERVAL', '900', '{}', true),
    ('PROCESS_EXPORT_FILE_MON_BASE', 'JOB_FETCH_SECURITY_METADATA', '证券元数据获取', 'RUN_DUE_EXPORT_JOBS', 'META', 1, true, true, 'EMBEDDED', 'EMBEDDED', false, 'NONE', NULL, '{}', true),
    ('PROCESS_EXPORT_BATCH', 'JOB_FETCH_EXPORT_BATCH', '证券导出批次清单获取', 'RUN_DUE_EXPORT_JOBS', 'BATCH', 1, true, true, 'EMBEDDED', 'EMBEDDED', false, 'NONE', NULL, '{}', true)
  ) AS v(process_code, job_code, name, job_kind, execution_stage, seq_no, critical,
         affects_completion, trigger_mode, execution_mode, manual_rerun_enabled,
         schedule_kind, schedule_value, job_params, ui_visible)
  JOIN dedup.cdeu_process p ON p.process_code = v.process_code
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
    ui_visible = EXCLUDED.ui_visible,
    gmt_update = now();

INSERT INTO dedup.cdeu_job_step
    (job_id, step_code, name, seq_no, critical, affects_completion, enabled)
SELECT j.job_id, v.step_code, v.name, v.seq_no, v.critical, v.affects_completion, true
  FROM (VALUES
    ('JOB_CALC_INGEST_STATS', 'STEP_CALC_INGEST_STATS', '每日入口流量统计', 1, true, true),
    ('JOB_CALC_BUDGET', 'STEP_CALC_BUDGET', '每日导出预算计算', 1, true, true),
    ('JOB_REFRESH_MATURITY', 'STEP_REFRESH_MATURITY', '行情成熟度扫描', 1, true, true),
    ('JOB_MARK_SEALED', 'STEP_MARK_SEALED', '收口冻结与差异报告生成', 1, true, true),
    ('JOB_EXPORT_LOAD', 'STEP_LOAD', '导出装载（LOAD）', 1, true, true),
    ('JOB_EXPORT_EXPORT', 'STEP_EXPORT', '导出文件生成（EXPORT）', 1, true, true),
    ('JOB_EXPORT_REGISTER', 'STEP_REGISTER', '导出文件登记（REGISTER）', 1, true, true),
    ('JOB_EXPORT_CLEANUP', 'STEP_CLEANUP', '中转表清理（CLEANUP）', 1, true, true),
    ('JOB_SCAN_PATCH', 'STEP_SCAN_PATCH', '收口后补丁扫描', 1, true, true),
    ('JOB_UPLOAD_FILES', 'STEP_UPLOAD_FILES', '待上传文件上传', 1, true, true),
    ('JOB_CLEAN_EXPIRED', 'STEP_CLEAN_EXPIRED', '生命周期清理', 1, true, true),
    ('JOB_RECOVER_TIMEOUT', 'STEP_RECOVER_TIMEOUT', '超时任务回收', 1, false, false),
    ('JOB_CHECK_HEALTH', 'STEP_CHECK_HEALTH', '归档健康检查', 1, false, false),
    ('JOB_FETCH_SECURITY_METADATA', 'STEP_FETCH_SECURITY_METADATA', '证券元数据获取', 1, true, true),
    ('JOB_FETCH_EXPORT_BATCH', 'STEP_FETCH_EXPORT_BATCH', '证券导出批次清单获取', 1, true, true)
  ) AS v(job_code, step_code, name, seq_no, critical, affects_completion)
  JOIN dedup.cdeu_job j ON j.job_code = v.job_code
ON CONFLICT (job_id, step_code) DO UPDATE SET
    name = EXCLUDED.name,
    seq_no = EXCLUDED.seq_no,
    critical = EXCLUDED.critical,
    affects_completion = EXCLUDED.affects_completion,
    enabled = EXCLUDED.enabled,
    gmt_update = now();
