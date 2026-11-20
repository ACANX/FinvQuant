-- =====================================================================
-- FinvQuant PostgreSQL V300081：收敛「工作流已终态、子级仍非终态」的残留记录
--
-- 需求 10884（2026-11-01）
--
-- 背景：
--   旧版本在运行被终止/失败/完成后，子级的收尾并不彻底，留下了「父级已终态、
--   子级仍停在 PENDING / READY / RUNNING」的孤儿记录：
--     · run 59（FAILED，2026-10-27）残留 2 个 RUNNING 的 job_run
--       （JOB_UPLOAD_FILES / JOB_EXPORT_LOAD）、61 条未终态的 GCMAIN 上传动态实例、
--       4 个未终态的 process_run；
--     · run 55（SUCCESS）残留 1 个 RUNNING 的 process_run（PROCESS_EXPORT_FILE_MON_BASE）。
--   这些残留会让「运行中」永远无法收敛、干扰 Job 门禁与界面状态。
--
-- 处置口径：
--   只把「所属 workflow_run 已进入终态（SUCCESS/FAILED/PARTIAL/PAUSED/CANCELLED）
--   而自身仍为 PENDING/READY/RUNNING」的子级改为 CANCELLED，并补齐结束时间。
--     · 不删除任何记录；
--     · 不改动已终态的行；
--     · 不改动 workflow_run / phase_run 的既定结论（只收敛其未终态子级）。
--
-- 幂等：每个 UPDATE 都带 `status IN ('PENDING','READY','RUNNING')` 条件，
--   重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验，仍有残留则抛错回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. Job Step 运行：未终态 → CANCELLED
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_job_step_run s
   SET status = 'CANCELLED',
       finished_at = COALESCE(s.finished_at, now()),
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_workflow_run r
 WHERE r.run_id = s.workflow_run_id
   AND r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
   AND s.status IN ('PENDING', 'READY', 'RUNNING');

-- ---------------------------------------------------------------------
-- 2. Job 运行：未终态 → CANCELLED（保留真实开始时间，不编造）
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_job_run j
   SET status = 'CANCELLED',
       finished_at = COALESCE(j.finished_at, now()),
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_workflow_run r
 WHERE r.run_id = j.workflow_run_id
   AND r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
   AND j.status IN ('PENDING', 'READY', 'RUNNING');

-- ---------------------------------------------------------------------
-- 3. 动态实例（导出 / 上传）：未终态 → CANCELLED
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_job_dynamic d
   SET status = 'CANCELLED',
       gmt_finish = COALESCE(d.gmt_finish, now()),
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_workflow_run r
 WHERE r.run_id = d.workflow_run_id
   AND r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
   AND d.status IN ('PENDING', 'READY', 'RUNNING');

-- ---------------------------------------------------------------------
-- 4. Process 运行：未终态 → CANCELLED
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_process_run p
   SET status = 'CANCELLED',
       finished_at = COALESCE(p.finished_at, now()),
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_workflow_run r
 WHERE r.run_id = p.workflow_run_id
   AND r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
   AND p.status IN ('PENDING', 'READY', 'RUNNING');

-- ---------------------------------------------------------------------
-- 5. Phase 运行：未终态 → CANCELLED
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_phase_run ph
   SET status = 'CANCELLED',
       finished_at = COALESCE(ph.finished_at, now()),
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_workflow_run r
 WHERE r.run_id = ph.workflow_run_id
   AND r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
   AND ph.status IN ('PENDING', 'READY', 'RUNNING');

-- ---------------------------------------------------------------------
-- 6. 收敛校验：任一子级仍残留非终态即回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    stale_step     INT;
    stale_job      INT;
    stale_dynamic  INT;
    stale_process  INT;
    stale_phase    INT;
BEGIN
    SELECT count(*) INTO stale_step
      FROM cdeu.finv_quote_cdeu_job_step_run s
      JOIN cdeu.finv_quote_cdeu_workflow_run r ON r.run_id = s.workflow_run_id
     WHERE r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
       AND s.status IN ('PENDING', 'READY', 'RUNNING');
    SELECT count(*) INTO stale_job
      FROM cdeu.finv_quote_cdeu_job_run j
      JOIN cdeu.finv_quote_cdeu_workflow_run r ON r.run_id = j.workflow_run_id
     WHERE r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
       AND j.status IN ('PENDING', 'READY', 'RUNNING');
    SELECT count(*) INTO stale_dynamic
      FROM cdeu.finv_quote_cdeu_job_dynamic d
      JOIN cdeu.finv_quote_cdeu_workflow_run r ON r.run_id = d.workflow_run_id
     WHERE r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
       AND d.status IN ('PENDING', 'READY', 'RUNNING');
    SELECT count(*) INTO stale_process
      FROM cdeu.finv_quote_cdeu_process_run p
      JOIN cdeu.finv_quote_cdeu_workflow_run r ON r.run_id = p.workflow_run_id
     WHERE r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
       AND p.status IN ('PENDING', 'READY', 'RUNNING');
    SELECT count(*) INTO stale_phase
      FROM cdeu.finv_quote_cdeu_phase_run ph
      JOIN cdeu.finv_quote_cdeu_workflow_run r ON r.run_id = ph.workflow_run_id
     WHERE r.status IN ('SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED')
       AND ph.status IN ('PENDING', 'READY', 'RUNNING');

    IF stale_step + stale_job + stale_dynamic + stale_process + stale_phase > 0 THEN
        RAISE EXCEPTION
            'V300081 收敛未完成：step=% job=% dynamic=% process=% phase=%',
            stale_step, stale_job, stale_dynamic, stale_process, stale_phase;
    END IF;

    RAISE NOTICE 'V300081 收敛完成：已终态运行的所有子级均处于终态';
END $$;

COMMIT;
