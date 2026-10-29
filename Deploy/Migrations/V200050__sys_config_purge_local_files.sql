-- =====================================================================
-- FinvQuant PostgreSQL V100050：系统配置 20011 + 本地导出文件清理 JOB（需求 10744）
--
-- 用户决策（2026-10-27）：
--   ① 20011 默认 '1'（清理）；
--   ② MANIFEST .json 与 .mvsv 一起删除；
--   ③ 上传成功后超过 24 小时才删，执行顺序排在 JOB_PURGE_EXPORTED_MONTH 之后；
--   ④ 原 LocalRetention 机制降级为「孤儿文件兜底」。
--
-- 配置口径：
--   config_no = 20011
--   data_type = '2'（char）
--   char_value = '1'（清理）/ '0'（不清理）
--
-- JOB 归属：
--   PHASE_REVIEW / PROCESS_PURGE，seq_no = 2
--     seq 1  JOB_PURGE_EXPORTED_MONTH   删 dedup 主表该月数据（20007 控制）
--     seq 2  JOB_PURGE_LOCAL_FILES      删本地 .mvsv / .json（20011 控制）
-- =====================================================================

BEGIN;

-- 兼容修正（需求 10771，2026-10-28）：
--   原实现硬编码 phase_id = 12（PHASE_REVIEW），全新库中该 id 由序列分配
--   （实测为 7）导致前置校验失败；改为按 phase_code 解析。


-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_purge bigint;
BEGIN
    SELECT process_id INTO v_purge FROM dedup.cdeu_process
     WHERE phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND process_code = 'PROCESS_PURGE';
    IF v_purge IS NULL THEN
        RAISE EXCEPTION 'V100050 前置校验失败：PROCESS_PURGE 不存在（需先执行 V100048）';
    END IF;
    RAISE NOTICE 'V100050 前置校验：PROCESS_PURGE process_id=%', v_purge;
END $$;

-- ---------------------------------------------------------------------
-- 2. 配置 20011（ON CONFLICT DO NOTHING：不覆盖用户已配置的值）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, remark, create_by)
VALUES
    (20011, '归档复核是否清理本地导出文件', '2', 0, '1', '',
     '1-清理（默认） 0-不清理。启用后由 PHASE_REVIEW 的 PROCESS_PURGE / JOB_PURGE_LOCAL_FILES 执行：该证券月 BASE 与全部 PATCH 上传成功且远端校验通过、最后一个文件上传完成超过 24 小时、无待处理补丁时，删除本地 .mvsv 与 Manifest .json。',
     'finv')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. JOB 定义：JOB_PURGE_LOCAL_FILES → PROCESS_PURGE（seq 2）
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled, schedule_kind, schedule_value,
     max_attempts, retry_backoff_seconds, enabled, ui_visible)
SELECT p.process_id, 'JOB_PURGE_LOCAL_FILES', '本地导出文件清理', 'PURGE_LOCAL_EXPORT_FILES',
       2, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '06:15',
       1, 60, true, true
  FROM dedup.cdeu_process p
 WHERE p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND p.process_code = 'PROCESS_PURGE'
ON CONFLICT (process_id, job_code) DO UPDATE
   SET name = EXCLUDED.name,
       job_kind = EXCLUDED.job_kind,
       seq_no = EXCLUDED.seq_no,
       critical = true,
       affects_completion = true,
       schedule_kind = EXCLUDED.schedule_kind,
       schedule_value = EXCLUDED.schedule_value,
       enabled = true,
       ui_visible = true,
       gmt_update = now();

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cfg    text;
    v_jobs   int;
    v_seq    text;
BEGIN
    -- 4.1 20011 存在且默认 '1'
    SELECT char_value INTO v_cfg FROM sys.sys_config WHERE config_no = 20011;
    IF v_cfg IS NULL THEN
        RAISE EXCEPTION 'V100050 收敛失败：20011 未登记';
    END IF;
    IF trim(v_cfg) <> '1' THEN
        RAISE NOTICE 'V100050 提示：20011 当前值为 %（非默认 1，可能已被运维修改）', v_cfg;
    END IF;

    -- 4.2 PROCESS_PURGE 下两个 JOB，按序
    SELECT count(*) INTO v_jobs
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND p.process_code = 'PROCESS_PURGE' AND j.enabled;
    IF v_jobs <> 2 THEN
        RAISE EXCEPTION 'V100050 收敛失败：PROCESS_PURGE 应有 2 个启用 JOB，实际 %', v_jobs;
    END IF;

    SELECT string_agg(j.job_code || ':' || j.seq_no, ',' ORDER BY j.seq_no) INTO v_seq
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.phase_id = (SELECT phase_id FROM dedup.cdeu_phase WHERE phase_code = 'PHASE_REVIEW') AND p.process_code = 'PROCESS_PURGE' AND j.enabled;
    IF v_seq <> 'JOB_PURGE_EXPORTED_MONTH:1,JOB_PURGE_LOCAL_FILES:2' THEN
        RAISE EXCEPTION 'V100050 收敛失败：PROCESS_PURGE 的 JOB 顺序为 %', v_seq;
    END IF;

    RAISE NOTICE 'V100050 收敛校验通过：20011=% PROCESS_PURGE=2 个 JOB（主表清理 → 本地文件清理）', trim(v_cfg);
END $$;

COMMIT;
