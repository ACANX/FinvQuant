-- =====================================================================
-- FinvQuant PostgreSQL V100049：为两个占位流程配置真实 JOB（需求 10741）
--
-- 背景：
--   全库有两个「有流程定义但 0 个 JOB」的占位节点，其状态只能靠
--   seedCDEUHierarchyTx 的空流程分支直接标 SKIPPED，界面上既不产生执行日志，
--   也看不到任何业务结果：
--
--     PHASE_COLLECT  / PROCESS_HISTORY_INGEST   历史行情数据采集
--     PHASE_DECISION / PROCESS_EXPORT_DECISION  导出条件判定
--
--   用户决定「给它们配真实 JOB」（选项 B）。
--
-- 本次改动：
--   1. PROCESS_HISTORY_INGEST（process_id=21）
--        ← JOB_CHECK_INGEST_READINESS「采集就绪检查」
--        只读体检 quote schema 的数据新鲜度（最新交易日 / 滞后天数 / 行数 / 证券数）。
--        采集本身仍由外部程序负责，本 JOB 提供可观测的判定点与执行日志。
--
--   2. PROCESS_EXPORT_DECISION（process_id=22）
--        ← JOB_EVALUATE_EXPORT_CONDITION「导出条件判定」
--        用与 GenerateExportBatch 的 BASE 分支完全一致的口径做只读评估，
--        输出候选证券月数与各排除原因分布（元数据不全 / 已退市 / 当月 / 未成熟 / 已导出）。
--        真正的批次清单仍由 PHASE_EXPORT 的 JOB_FETCH_EXPORT_BATCH 生成。
--
-- 两个 JOB 均 affects_completion = true（关键 JOB）：判定或体检失败应阻断阶段完成。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_ingest   bigint;
    v_decision bigint;
BEGIN
    SELECT process_id INTO v_ingest   FROM dedup.cdeu_process WHERE process_code = 'PROCESS_HISTORY_INGEST';
    SELECT process_id INTO v_decision FROM dedup.cdeu_process WHERE process_code = 'PROCESS_EXPORT_DECISION';
    IF v_ingest IS NULL THEN RAISE EXCEPTION 'V100049 前置校验失败：PROCESS_HISTORY_INGEST 不存在'; END IF;
    IF v_decision IS NULL THEN RAISE EXCEPTION 'V100049 前置校验失败：PROCESS_EXPORT_DECISION 不存在'; END IF;
    RAISE NOTICE 'V100049 前置校验：ingest_process_id=% decision_process_id=%', v_ingest, v_decision;
END $$;

-- ---------------------------------------------------------------------
-- 2. 采集就绪检查 → PROCESS_HISTORY_INGEST
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled, schedule_kind, schedule_value,
     max_attempts, retry_backoff_seconds, enabled, ui_visible)
SELECT p.process_id, 'JOB_CHECK_INGEST_READINESS', '采集就绪检查', 'CHECK_INGEST_READINESS',
       1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:03',
       1, 60, true, true
  FROM dedup.cdeu_process p
 WHERE p.process_code = 'PROCESS_HISTORY_INGEST'
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
-- 3. 导出条件判定 → PROCESS_EXPORT_DECISION
-- ---------------------------------------------------------------------
INSERT INTO dedup.cdeu_job
    (process_id, job_code, name, job_kind, seq_no, critical, affects_completion,
     trigger_mode, execution_mode, manual_rerun_enabled, schedule_kind, schedule_value,
     max_attempts, retry_backoff_seconds, enabled, ui_visible)
SELECT p.process_id, 'JOB_EVALUATE_EXPORT_CONDITION', '导出条件判定', 'EVALUATE_EXPORT_CONDITION',
       1, true, true, 'SCHEDULED', 'DIRECT', true, 'DAILY', '00:15',
       1, 60, true, true
  FROM dedup.cdeu_process p
 WHERE p.process_code = 'PROCESS_EXPORT_DECISION'
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
    v_empty    int;
    v_ingest   int;
    v_decision int;
BEGIN
    -- 4.1 不再存在「启用但 0 个 JOB」的流程
    SELECT count(*) INTO v_empty
      FROM dedup.cdeu_process p
     WHERE p.enabled
       AND NOT EXISTS (SELECT 1 FROM dedup.cdeu_job j WHERE j.process_id = p.process_id AND j.enabled);
    IF v_empty <> 0 THEN
        RAISE EXCEPTION 'V100049 收敛失败：仍有 % 个启用流程没有任何 JOB', v_empty;
    END IF;

    -- 4.2 采集就绪检查 JOB
    SELECT count(*) INTO v_ingest
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_HISTORY_INGEST'
       AND j.job_kind = 'CHECK_INGEST_READINESS' AND j.enabled AND j.affects_completion;
    IF v_ingest <> 1 THEN
        RAISE EXCEPTION 'V100049 收敛失败：CHECK_INGEST_READINESS 未正确登记（%）', v_ingest;
    END IF;

    -- 4.3 导出条件判定 JOB
    SELECT count(*) INTO v_decision
      FROM dedup.cdeu_job j
      JOIN dedup.cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_EXPORT_DECISION'
       AND j.job_kind = 'EVALUATE_EXPORT_CONDITION' AND j.enabled AND j.affects_completion;
    IF v_decision <> 1 THEN
        RAISE EXCEPTION 'V100049 收敛失败：EVALUATE_EXPORT_CONDITION 未正确登记（%）', v_decision;
    END IF;

    RAISE NOTICE 'V100049 收敛校验通过：两个占位流程已配置关键 JOB，全库无空流程';
END $$;

COMMIT;
