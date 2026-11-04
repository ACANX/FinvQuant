-- =====================================================================
-- V300107：USC 补丁相关名称统一（需求 10938）
--
-- 把工作流里的 JOB 名称与菜单名称统一为「过时USC映射到新USC」，
-- 与功能语义一致（不是"打补丁"，而是把过时 usc 的行情映射到新 usc）。
--
-- 只改显示名称，不改编码与行为。
-- =====================================================================

BEGIN;

UPDATE cdeu.finv_quote_cdeu_process
   SET name = '过时USC映射到新USC', gmt_update = now()
 WHERE process_code = 'PROCESS_USC_PATCH';

UPDATE cdeu.finv_quote_cdeu_job
   SET name = '过时USC映射到新USC', gmt_update = now()
 WHERE job_code = 'JOB_PATCH_OUTDATED_USC';

UPDATE cdeu.finv_quote_cdeu_job_step
   SET name = '过时USC映射到新USC'
 WHERE step_code = 'STEP_PATCH_OUTDATED_USC';

DO $$
DECLARE
    v_proc text;
    v_job  text;
    v_step text;
BEGIN
    SELECT name INTO v_proc FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_USC_PATCH';
    SELECT name INTO v_job  FROM cdeu.finv_quote_cdeu_job     WHERE job_code     = 'JOB_PATCH_OUTDATED_USC';
    SELECT name INTO v_step FROM cdeu.finv_quote_cdeu_job_step WHERE step_code   = 'STEP_PATCH_OUTDATED_USC';
    RAISE NOTICE '名称核对：process=%，job=%，step=%', v_proc, v_job, v_step;
    IF v_proc <> '过时USC映射到新USC' OR v_job <> '过时USC映射到新USC' OR v_step <> '过时USC映射到新USC' THEN
        RAISE EXCEPTION '名称未全部更新，回滚';
    END IF;
END $$;

COMMIT;
