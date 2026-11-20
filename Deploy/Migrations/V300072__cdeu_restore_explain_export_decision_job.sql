-- =====================================================================
-- FinvQuant PostgreSQL V300072：补回 JOB_EXPLAIN_EXPORT_DECISION（全新库丢失修复）
--
-- 问题（2026-10-31 独立库 finv_migtest 全链验证发现）：
--   V200085 在 `PROCESS_EXPORT_DECISION` 下插入 JOB_EXPLAIN_EXPORT_DECISION，
--   但 V300057（版本号更晚）在「cdeu 配置落后于 dedup」时会
--   `DELETE FROM cdeu.finv_quote_cdeu_job` 后从 `dedup.cdeu_job` 全量重灌配置。
--   全新库按版本顺序执行时：V200085 先插 → V300057 清空重灌（dedup 侧无该 JOB）
--   → 该 JOB 丢失，最终 job 数 22；生产库因 V200085 是后来补跑的（已越过 V300057），
--   所以是 23，两条路径终态不一致。
--
-- 本迁移只做一件事：把该 JOB 幂等补回（存在则按 V200085 的口径更新），
-- 使全新库与生产库都收敛到 23 个 JOB。
--
-- 幂等：INSERT ... ON CONFLICT (process_id, job_code) DO UPDATE；重跑无副作用。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾收敛校验（该 JOB 必须存在且挂在
--           PROCESS_EXPORT_DECISION 下）。
-- 前置：V200085（JOB 定义来源）、V300057（会清空重灌配置）。
-- =====================================================================

BEGIN;

INSERT INTO cdeu.finv_quote_cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled, schedule_kind, schedule_value,
     max_attempts, retry_backoff_seconds, enabled)
SELECT p.process_id, 'JOB_EXPLAIN_EXPORT_DECISION', '导出决策明细',
       'EXPLAIN_EXPORT_DECISION', 2, true, true,
       'SCHEDULED', 'DIRECT', true, 'DAILY', '00:16',
       1, 60, true
  FROM cdeu.finv_quote_cdeu_process p
 WHERE p.process_code = 'PROCESS_EXPORT_DECISION'
ON CONFLICT (process_id, job_code) DO UPDATE
   SET name = EXCLUDED.name,
       job_kind = EXCLUDED.job_kind,
       seq_no = EXCLUDED.seq_no,
       critical = EXCLUDED.critical,
       affects_completion = EXCLUDED.affects_completion,
       schedule_kind = EXCLUDED.schedule_kind,
       schedule_value = EXCLUDED.schedule_value,
       enabled = EXCLUDED.enabled;

DO $$
DECLARE
    v_job   bigint;
    v_proc  text;
BEGIN
    SELECT j.job_id, p.process_code
      INTO v_job, v_proc
      FROM cdeu.finv_quote_cdeu_job j
      JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = j.process_id
     WHERE j.job_code = 'JOB_EXPLAIN_EXPORT_DECISION';

    IF v_job IS NULL THEN
        RAISE EXCEPTION 'V300072 收敛失败：JOB_EXPLAIN_EXPORT_DECISION 未补回';
    END IF;
    IF v_proc IS DISTINCT FROM 'PROCESS_EXPORT_DECISION' THEN
        RAISE EXCEPTION 'V300072 收敛失败：JOB 挂在 % 下（预期 PROCESS_EXPORT_DECISION）', v_proc;
    END IF;

    RAISE NOTICE 'V300072 完成：JOB_EXPLAIN_EXPORT_DECISION 已补回（job_id=%）', v_job;
END $$;

COMMIT;
