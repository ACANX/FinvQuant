-- =====================================================================
-- V300106：USC 补丁纳入 CDEU 工作流「历史行情去重」阶段（需求 10932）
--
-- 目的：
--   把 PATCH_OUTDATED_USC 登记成 PHASE_DEDUP（历史行情去重）下的阶段内 JOB，
--   使 CDEU 工作流控制面板能显示并调度它。它同时保留 jobmeta 里的独立
--   每日 05:00 定时（作为工作流未启动时的兜底），两者共用 runner 的单实例约束。
--
-- 顺序：
--   PROCESS_USC_PATCH（seq 1，先补 usc）→ PROCESS_SEAL（seq 2，再收口冻结）。
--   先补 usc 再收口，否则收口冻结记下的分片清单里仍是旧 usc。
--
-- 幂等：按 process_code / job_code / step_code 判存在，重复执行影响 0 行。
-- =====================================================================

BEGIN;

-- 1. 收口冻结先顺延到第 2 位（(phase_id, seq_no) 唯一，必须先腾位再插入）
UPDATE cdeu.finv_quote_cdeu_process
   SET seq_no = 2, gmt_update = now()
 WHERE process_code = 'PROCESS_SEAL' AND seq_no = 1;

-- 2. 新增流程（phase_id=2 即 PHASE_DEDUP）
INSERT INTO cdeu.finv_quote_cdeu_process
    (phase_id, process_code, name, seq_no, critical, affects_completion,
     run_mode, batch_enabled, max_parallel, failure_policy, completion_rule, enabled)
SELECT 2, 'PROCESS_USC_PATCH', '过时 USC 补丁', 1, true, true,
       'INHERIT', false, 1, 'CONTINUE', '{"mode":"ALL"}'::jsonb, true
 WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_USC_PATCH');

-- 3. 新增 JOB
INSERT INTO cdeu.finv_quote_cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled,
     schedule_kind, schedule_value, enabled, ui_visible)
SELECT pr.process_id, 'JOB_PATCH_OUTDATED_USC', '过时 USC 补丁', 'PATCH_OUTDATED_USC', 1, true, true,
       'SCHEDULED', 'DIRECT', true, 'DAILY', '05:00', true, true
  FROM cdeu.finv_quote_cdeu_process pr
 WHERE pr.process_code = 'PROCESS_USC_PATCH'
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_job WHERE job_code = 'JOB_PATCH_OUTDATED_USC');

-- 4. 新增步骤
INSERT INTO cdeu.finv_quote_cdeu_job_step
    (job_id, step_code, name, seq_no, critical, affects_completion, enabled)
SELECT j.job_id, 'STEP_PATCH_OUTDATED_USC', '过时 USC 补丁', 1, true, true, true
  FROM cdeu.finv_quote_cdeu_job j
 WHERE j.job_code = 'JOB_PATCH_OUTDATED_USC'
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_job_step WHERE step_code = 'STEP_PATCH_OUTDATED_USC');

DO $$
DECLARE
    v_process int;
    v_job     int;
    v_step    int;
    v_seal    int;
BEGIN
    SELECT count(*) INTO v_process FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_USC_PATCH';
    SELECT count(*) INTO v_job     FROM cdeu.finv_quote_cdeu_job     WHERE job_code     = 'JOB_PATCH_OUTDATED_USC';
    SELECT count(*) INTO v_step    FROM cdeu.finv_quote_cdeu_job_step WHERE step_code   = 'STEP_PATCH_OUTDATED_USC';
    SELECT seq_no INTO v_seal FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_SEAL';
    RAISE NOTICE '校验：process=%，job=%，step=%，PROCESS_SEAL.seq_no=%', v_process, v_job, v_step, v_seal;
    IF v_process <> 1 OR v_job <> 1 OR v_step <> 1 OR v_seal <> 2 THEN
        RAISE EXCEPTION '登记结果不符合预期，回滚';
    END IF;
END $$;

COMMIT;

-- 执行后核对
-- SELECT pr.seq_no AS proc_seq, pr.process_code, j.job_code, j.job_kind, j.seq_no AS job_seq
--   FROM cdeu.finv_quote_cdeu_process pr
--   LEFT JOIN cdeu.finv_quote_cdeu_job j ON j.process_id = pr.process_id
--  WHERE pr.phase_id = 2 ORDER BY pr.seq_no, j.seq_no;
