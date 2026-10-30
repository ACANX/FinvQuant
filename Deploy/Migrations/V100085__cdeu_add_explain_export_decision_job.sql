-- =====================================================================
-- FinvQuant PostgreSQL V100085：新增「导出决策明细」JOB（需求 10832）
--
-- 背景（用户实测反馈）：
--   流程跑了很多次，但一直没有生成预期的动态导出 JOB；cdue 表里明明有上个月的数据，
--   调度器里也没有任何日志说明为什么没有生成。运维只能自己逐表比对（成熟度表、
--   证券主数据、导出任务表、批次表），排查成本极高。
--
--   现有 EVALUATE_EXPORT_CONDITION 只输出「各排除原因的数量」，导出 JOB 最多打一行
--   「候选 0 个」，两者都无法回答：
--     · 是哪几个证券月不能导出？
--     · 具体是因为缺了哪个元数据字段 / 差哪几个交易日 / 已经导出过？
--     · 能导出的那些，会生成什么文件、放在哪里？
--     · 有没有「行情表有数据但成熟度表无记录」的月份（判定侧完全看不到）？
--
-- 本次改动：
--   在 PHASE_DECISION / PROCESS_EXPORT_DECISION 下新增 JOB_EXPLAIN_EXPORT_DECISION
--   （job_kind = EXPLAIN_EXPORT_DECISION，seq_no = 2，排在导出条件判定之后）。
--   该 JOB 逐条打印每个 (usc, biz_month) 的结论与具体原因，并给出可导出项的文件名、
--   本地路径、远端路径；同时扫描成熟度缺失的月份。只读，不写批次表、不生成动态 JOB。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_process bigint;
BEGIN
    SELECT process_id INTO v_process
      FROM cdeu.finv_quote_cdeu_process
     WHERE process_code = 'PROCESS_EXPORT_DECISION';
    IF v_process IS NULL THEN
        RAISE EXCEPTION 'V100085 前置校验失败：PROCESS_EXPORT_DECISION 不存在';
    END IF;
    RAISE NOTICE 'V100085 前置校验：decision_process_id=%', v_process;
END $$;

-- ---------------------------------------------------------------------
-- 2. 导出决策明细 JOB
-- ---------------------------------------------------------------------
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

-- ---------------------------------------------------------------------
-- 3. 后置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_count int;
BEGIN
    SELECT count(*) INTO v_count
      FROM cdeu.finv_quote_cdeu_job j
      JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_EXPORT_DECISION'
       AND j.job_kind = 'EXPLAIN_EXPORT_DECISION'
       AND j.enabled;
    IF v_count <> 1 THEN
        RAISE EXCEPTION 'V100085 后置校验失败：EXPLAIN_EXPORT_DECISION 启用数应为 1，实际 %', v_count;
    END IF;
    RAISE NOTICE 'V100085 完成：已启用 EXPLAIN_EXPORT_DECISION';
END $$;

COMMIT;
