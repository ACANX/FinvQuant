-- =====================================================================
-- FinvQuant PostgreSQL V100052：按子级真实时间重算流程/阶段的起止时间（需求 10747）
--
-- 背景：原 EvaluateCDEUProcess 用
--   started_at = COALESCE(pr.started_at, LEAST(min(job_run.started_at),
--                                              min(job_dynamic.gmt_start), now()))
-- 首次评估时子级尚未开始，LEAST 忽略 NULL 后落到 now()，把「评估时刻」写成了流程开始时间；
-- 随后 pr.started_at 非空，COALESCE 永久锁定，子级真实开始时间再也进不来。
--
-- 同理 finished_at 在「暂停后恢复」的 run 里也是错的：
-- run 50 于 16:00:34 判终态，随后 PAUSED，23:44:43 恢复继续跑子 JOB，
-- 于是 pr.finished_at(16:00:34) 早于子 JOB 的 started_at(23:44:43)——
-- 物理上不可能，说明本次迁移必须父子时间一起校正，否则会留下新的自相矛盾。
--
-- 新口径（只由子级推导，绝不编造）：
--   started_at  = LEAST(最早 job_run.started_at, 最早 job_dynamic.gmt_start)
--   finished_at = GREATEST(最晚 job_run.finished_at, 最晚 job_dynamic.gmt_finish, started_at, 原 finished_at)
--   子级无痕迹      → started_at = NULL（没有执行痕迹就不写时间）
--   非终态          → finished_at 保持 NULL
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 重算 Process 起止时间
-- ---------------------------------------------------------------------
WITH derived AS (
    SELECT pr.workflow_run_id,
           pr.process_id,
           pr.status,
           pr.finished_at AS old_finish,
           LEAST(
               (SELECT min(r.started_at)
                  FROM dedup.cdeu_job_run r
                 WHERE r.workflow_run_id = pr.workflow_run_id
                   AND r.process_id = pr.process_id
                   AND r.started_at IS NOT NULL),
               (SELECT min(d.gmt_start)
                  FROM dedup.cdeu_job_dynamic d
                 WHERE d.workflow_run_id = pr.workflow_run_id
                   AND d.process_id = pr.process_id
                   AND d.gmt_start IS NOT NULL)
           ) AS new_start,
           GREATEST(
               (SELECT max(r.finished_at)
                  FROM dedup.cdeu_job_run r
                 WHERE r.workflow_run_id = pr.workflow_run_id
                   AND r.process_id = pr.process_id
                   AND r.finished_at IS NOT NULL),
               (SELECT max(d.gmt_finish)
                  FROM dedup.cdeu_job_dynamic d
                 WHERE d.workflow_run_id = pr.workflow_run_id
                   AND d.process_id = pr.process_id
                   AND d.gmt_finish IS NOT NULL)
           ) AS child_finish
      FROM dedup.cdeu_process_run pr
)
UPDATE dedup.cdeu_process_run pr
   SET started_at = d.new_start,
       finished_at = CASE
           WHEN pr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
               THEN GREATEST(d.child_finish, d.new_start, pr.finished_at)
           ELSE NULL
       END,
       gmt_update = now()
  FROM derived d
 WHERE pr.workflow_run_id = d.workflow_run_id
   AND pr.process_id = d.process_id
   AND (pr.started_at IS DISTINCT FROM d.new_start
        OR pr.finished_at IS DISTINCT FROM CASE
               WHEN pr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
                   THEN GREATEST(d.child_finish, d.new_start, pr.finished_at)
               ELSE NULL
           END);

-- ---------------------------------------------------------------------
-- 2. 重算 Phase 起止时间（依赖第 1 步的结果）
-- ---------------------------------------------------------------------
WITH derived AS (
    SELECT phr.phase_run_id,
           phr.status,
           (SELECT min(r.started_at)
              FROM dedup.cdeu_process_run r
             WHERE r.phase_run_id = phr.phase_run_id
               AND r.started_at IS NOT NULL) AS new_start,
           (SELECT max(r.finished_at)
              FROM dedup.cdeu_process_run r
             WHERE r.phase_run_id = phr.phase_run_id
               AND r.finished_at IS NOT NULL) AS child_finish
      FROM dedup.cdeu_phase_run phr
)
UPDATE dedup.cdeu_phase_run phr
   SET started_at = d.new_start,
       finished_at = CASE
           WHEN phr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
               THEN GREATEST(d.child_finish, d.new_start, phr.finished_at)
           ELSE NULL
       END,
       gmt_update = now()
  FROM derived d
 WHERE phr.phase_run_id = d.phase_run_id
   AND (phr.started_at IS DISTINCT FROM d.new_start
        OR phr.finished_at IS DISTINCT FROM CASE
               WHEN phr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
                   THEN GREATEST(d.child_finish, d.new_start, phr.finished_at)
               ELSE NULL
           END);

-- ---------------------------------------------------------------------
-- 3. 收敛校验：父子时间必须自洽
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad_process bigint;
    v_bad_phase   bigint;
    v_null_start  bigint;
    v_total       bigint;
BEGIN
    SELECT count(*) INTO v_bad_process
      FROM dedup.cdeu_process_run
     WHERE started_at IS NOT NULL AND finished_at IS NOT NULL AND started_at > finished_at;
    IF v_bad_process > 0 THEN
        RAISE EXCEPTION 'V100052 收敛失败：% 条 process_run 的 started_at 晚于 finished_at', v_bad_process;
    END IF;

    SELECT count(*) INTO v_bad_phase
      FROM dedup.cdeu_phase_run
     WHERE started_at IS NOT NULL AND finished_at IS NOT NULL AND started_at > finished_at;
    IF v_bad_phase > 0 THEN
        RAISE EXCEPTION 'V100052 收敛失败：% 条 phase_run 的 started_at 晚于 finished_at', v_bad_phase;
    END IF;

    -- 阶段时间必须落在其子流程区间的外侧
    SELECT count(*) INTO v_bad_phase
      FROM dedup.cdeu_phase_run phr
     WHERE phr.started_at IS NOT NULL
       AND EXISTS (SELECT 1 FROM dedup.cdeu_process_run r
                    WHERE r.phase_run_id = phr.phase_run_id
                      AND r.started_at IS NOT NULL AND r.started_at < phr.started_at);
    IF v_bad_phase > 0 THEN
        RAISE EXCEPTION 'V100052 收敛失败：% 条 phase_run 的开始时间晚于其子流程', v_bad_phase;
    END IF;

    SELECT count(*) INTO v_total FROM dedup.cdeu_process_run;
    SELECT count(*) INTO v_null_start FROM dedup.cdeu_process_run WHERE started_at IS NULL;

    RAISE NOTICE 'V100052 重算完成：process_run 共 % 条，其中 % 条无开始时间（子级无执行痕迹，属正常）；起止自洽校验通过',
        v_total, v_null_start;
END $$;

COMMIT;
