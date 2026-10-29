-- =====================================================================
-- FinvQuant PostgreSQL V100048：归档复核阶段流程/JOB 重排（需求 10716–10723）
--
-- 决策依据：用户明确要求「帮我完成修改」（2026-10-27），实施需求文档
--   10716 20007 主表清理改为归档复核阶段的独立 JOB
--   10717 撤销 CLEAN_EXPIRED_EXPORT_ARTIFACTS，拆为三个独立 JOB
--   10718 PHASE_REVIEW 下新建四个流程并按序执行
--   10719 JOB_CHECK_HEALTH / JOB_RECOVER_TIMEOUT 迁入 PHASE_REVIEW
--   10720 主表清理改为扫描式判据（代码层，本迁移只提供 JOB 定义）
--   10721 PHASE_REVIEW 下五个 JOB 全部为关键 JOB
--   10722 PHASE_UPLOAD 精简为只含 PROCESS_UPLOAD / JOB_UPLOAD_FILES
--   10723 新增 PURGE_HISTORY_RUNS 清理历史流程运行数据 + 20009/20010 配置
--
-- ---------------------------------------------------------------------
-- 目标结构
-- ---------------------------------------------------------------------
--   PHASE_REVIEW 归档复核（phase_id=12）
--   ├─ seq 1  PROCESS_RECOVER         超时任务回收  → JOB_RECOVER_TIMEOUT
--   ├─ seq 2  PROCESS_REVIEW          归档复核      → JOB_CHECK_HEALTH
--   ├─ seq 3  PROCESS_PURGE           归档数据清理  → JOB_PURGE_EXPORTED_MONTH
--   ├─ seq 4  PROCESS_STORAGE_CLEAN   存储清理      → JOB_CLEAN_STORAGE
--   └─ seq 5  PROCESS_RETENTION       运行记录保留  → JOB_PURGE_STALE_DYNAMIC + JOB_PURGE_HISTORY_RUNS
--
--   PHASE_UPLOAD 月度行情上传（phase_id=4）
--   └─ seq 1  PROCESS_UPLOAD 文件上传 → JOB_UPLOAD_FILES
--
-- ---------------------------------------------------------------------
-- 停用（不删除，保留历史运行记录可追溯）
-- ---------------------------------------------------------------------
--   PROCESS_LIFECYCLE_CLEAN（process_id=8）+ JOB_CLEAN_EXPIRED（job_id=38）
--   PROCESS_HEALTH（process_id=10）—— JOB_CHECK_HEALTH 迁走后为空壳
--
-- 注意：定义表的软停用统一用 enabled=false，绝不 DELETE，避免破坏
--       cdeu_job_run / cdeu_process_run 中的历史引用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_review  bigint;
    v_upload  bigint;
BEGIN
    SELECT phase_id INTO v_review FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW';
    SELECT phase_id INTO v_upload FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_UPLOAD';
    IF v_review IS NULL THEN RAISE EXCEPTION 'V100048 前置校验失败：PHASE_REVIEW 不存在'; END IF;
    IF v_upload IS NULL THEN RAISE EXCEPTION 'V100048 前置校验失败：PHASE_UPLOAD 不存在'; END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. PHASE_UPLOAD 精简：停用生命周期清理与健康检查流程
-- ---------------------------------------------------------------------
UPDATE dedup.cdeu_process
   SET enabled = false, gmt_update = now()
 WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_UPLOAD') AND process_code IN ('PROCESS_LIFECYCLE_CLEAN', 'PROCESS_HEALTH');

UPDATE dedup.cdeu_job j
   SET enabled = false, gmt_update = now()
  FROM dedup.cdeu_process p
 WHERE j.process_id = p.process_id
   AND p.process_code = 'PROCESS_LIFECYCLE_CLEAN'
   AND j.job_code = 'JOB_CLEAN_EXPIRED';

-- ---------------------------------------------------------------------
-- 3. PROCESS_RECOVER 迁入 PHASE_REVIEW（seq 1）
--    先把 PROCESS_REVIEW 的 seq_no 让到 2，避免 (phase_id, seq_no) 唯一冲突
-- ---------------------------------------------------------------------
UPDATE dedup.cdeu_process
   SET seq_no = 2, affects_completion = true, critical = true, gmt_update = now()
 WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND process_code = 'PROCESS_REVIEW';

UPDATE dedup.cdeu_process
   SET phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW'), seq_no = 1, affects_completion = true, critical = true, gmt_update = now()
 WHERE process_code = 'PROCESS_RECOVER';

-- ---------------------------------------------------------------------
-- 4. PHASE_REVIEW 新建三个流程（seq 3/4/5）
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_process
    (phase_id, process_code, name, seq_no, critical, affects_completion,
     run_mode, batch_enabled, max_parallel, failure_policy, completion_rule, enabled)
VALUES
    ((SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW'), 'PROCESS_PURGE',         '归档数据清理', 3, true, true, 'INHERIT', false, 1, 'FAIL_FAST', '{"mode":"ALL"}'::jsonb, true),
    ((SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW'), 'PROCESS_STORAGE_CLEAN', '存储清理',     4, true, true, 'INHERIT', false, 1, 'FAIL_FAST', '{"mode":"ALL"}'::jsonb, true),
    ((SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW'), 'PROCESS_RETENTION',     '运行记录保留', 5, true, true, 'INHERIT', false, 1, 'FAIL_FAST', '{"mode":"ALL"}'::jsonb, true)
ON CONFLICT (phase_id, process_code) DO UPDATE
   SET name = EXCLUDED.name,
       seq_no = EXCLUDED.seq_no,
       critical = EXCLUDED.critical,
       affects_completion = EXCLUDED.affects_completion,
       failure_policy = EXCLUDED.failure_policy,
       enabled = true,
       gmt_update = now();

-- ---------------------------------------------------------------------
-- 5. JOB_CHECK_HEALTH 改挂到 PROCESS_REVIEW（原 PROCESS_HEALTH）
-- ---------------------------------------------------------------------
UPDATE dedup.cdeu_job
   SET process_id = (SELECT process_id FROM dedup.cdeu_process WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND process_code = 'PROCESS_REVIEW'),
       seq_no = 1,
       affects_completion = true,
       critical = true,
       gmt_update = now()
 WHERE job_code = 'JOB_CHECK_HEALTH';

-- JOB_RECOVER_TIMEOUT 随 PROCESS_RECOVER 一起进入 PHASE_REVIEW，仅升级为关键 JOB
UPDATE dedup.cdeu_job
   SET affects_completion = true, critical = true, gmt_update = now()
 WHERE job_code = 'JOB_RECOVER_TIMEOUT';

-- ---------------------------------------------------------------------
-- 6. PHASE_REVIEW 新增四个 JOB
--    10721：全部 affects_completion=true（关键 JOB）
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled, schedule_kind, schedule_value,
     max_attempts, retry_backoff_seconds, enabled, ui_visible)
SELECT p.process_id, v.job_code, v.name, v.job_kind, v.seq_no, true, true,
       'SCHEDULED', 'DIRECT', true, 'DAILY', v.daily_at,
       1, 60, true, true
  FROM (VALUES
        ('PROCESS_PURGE',         'JOB_PURGE_EXPORTED_MONTH', '已归档行情数据清理',   'PURGE_EXPORTED_MONTH', 1, '06:10'),
        ('PROCESS_STORAGE_CLEAN', 'JOB_CLEAN_STORAGE',        '中转与本地文件清理',   'CLEAN_EXPORT_STORAGE',  1, '06:20'),
        ('PROCESS_RETENTION',     'JOB_PURGE_STALE_DYNAMIC',  '历史动态JOB实例清理',  'PURGE_STALE_DYNAMIC',   1, '06:30'),
        ('PROCESS_RETENTION',     'JOB_PURGE_HISTORY_RUNS',   '历史流程运行数据清理', 'PURGE_HISTORY_RUNS',    2, '06:40')
       ) AS v(process_code, job_code, name, job_kind, seq_no, daily_at)
  JOIN dedup.cdeu_process p ON p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND p.process_code = v.process_code
ON CONFLICT (process_id, job_code) DO UPDATE
   SET name = EXCLUDED.name,
       job_kind = EXCLUDED.job_kind,
       seq_no = EXCLUDED.seq_no,
       critical = true,
       affects_completion = true,
       schedule_value = EXCLUDED.schedule_value,
       enabled = true,
       ui_visible = true,
       gmt_update = now();

-- ---------------------------------------------------------------------
-- 7. 系统配置 20009 / 20010（需求 10723）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, remark, create_by)
VALUES
    (20009, '历史流程运行数据清理策略', '3', NULL, '', 'Days',
     '取值 Off / Days / Count：Off 不清理；Days 删除超过 20010 天前的 workflow_run；Count 只保留最近 20010 个 workflow_run。由 PURGE_HISTORY_RUNS 执行。',
     'finv'),
    (20010, '历史流程运行数据保留阈值', '1', 30, '', '',
     '配合 20009 使用：Days 策略下为天数，Count 策略下为保留个数；合法区间 1~3650，越界或非法回退默认 30。',
     'finv')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 8. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_proc   int;
    v_seq    text;
    v_jobs   int;
    v_key    int;
    v_upload int;
    v_cfg    int;
BEGIN
    -- 8.1 PHASE_REVIEW 下五个流程按序
    SELECT count(*) INTO v_proc FROM dedup.cdeu_process WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND enabled;
    IF v_proc <> 5 THEN
        RAISE EXCEPTION 'V100048 收敛失败：PHASE_REVIEW 应有 5 个启用流程，实际 %', v_proc;
    END IF;

    SELECT string_agg(process_code || ':' || seq_no, ',' ORDER BY seq_no) INTO v_seq
      FROM dedup.cdeu_process WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND enabled;
    IF v_seq <> 'PROCESS_RECOVER:1,PROCESS_REVIEW:2,PROCESS_PURGE:3,PROCESS_STORAGE_CLEAN:4,PROCESS_RETENTION:5' THEN
        RAISE EXCEPTION 'V100048 收敛失败：PHASE_REVIEW 流程顺序为 %', v_seq;
    END IF;

    -- 8.2 PHASE_REVIEW 下五个 JOB（含两个迁入的）
    SELECT count(*) INTO v_jobs
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND j.enabled;
    IF v_jobs <> 6 THEN
        RAISE EXCEPTION 'V100048 收敛失败：PHASE_REVIEW 应有 6 个启用 JOB（5 流程 + RETENTION 2 个），实际 %', v_jobs;
    END IF;

    -- 8.3 全部为关键 JOB
    SELECT count(*) INTO v_key
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND j.enabled AND j.affects_completion = false;
    IF v_key <> 0 THEN
        RAISE EXCEPTION 'V100048 收敛失败：PHASE_REVIEW 下仍有 % 个非关键 JOB', v_key;
    END IF;

    -- 8.4 PHASE_UPLOAD 只剩一个流程
    SELECT count(*) INTO v_upload FROM dedup.cdeu_process WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_UPLOAD') AND enabled;
    IF v_upload <> 1 THEN
        RAISE EXCEPTION 'V100048 收敛失败：PHASE_UPLOAD 应有 1 个启用流程，实际 %', v_upload;
    END IF;

    -- 8.5 CLEAN_EXPIRED 已停用
    IF EXISTS (SELECT 1 FROM dedup.cdeu_job WHERE job_code = 'JOB_CLEAN_EXPIRED' AND enabled) THEN
        RAISE EXCEPTION 'V100048 收敛失败：JOB_CLEAN_EXPIRED 仍处于启用状态';
    END IF;

    -- 8.6 配置就绪
    SELECT count(*) INTO v_cfg FROM sys.sys_config WHERE config_no IN (20009, 20010);
    IF v_cfg <> 2 THEN
        RAISE EXCEPTION 'V100048 收敛失败：20009/20010 应有 2 条，实际 %', v_cfg;
    END IF;

    RAISE NOTICE 'V100048 收敛校验通过：PHASE_REVIEW 5 流程 / 6 关键 JOB；PHASE_UPLOAD 1 流程；CLEAN_EXPIRED 已停用；20009+20010 已就绪';
END $$;

COMMIT;
