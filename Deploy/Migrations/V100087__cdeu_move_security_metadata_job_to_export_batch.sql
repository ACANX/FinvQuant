-- =====================================================================
-- FinvQuant PostgreSQL V100087：证券元数据 JOB 归入导出批次流程（需求 10837）
--
-- 授权：用户在会话中明确要求「JOB_FETCH_SECURITY_METADATA 要改到 PROCESS_EXPORT_BATCH 中执行」。
--
-- 背景：
--   该 JOB 原先挂在 PROCESS_EXPORT_FILE_MON_BASE（每证券月一个动态实例的流程）下，
--   语义上却是「导出前把证券字典里的 region/market/provider/name/timezone/currency 补齐」——
--   这是批次级的准备工作，与单个证券月的导出实例无关：
--     · 它由 RUN_DUE_EXPORT_JOBS 在批次生成之后、装载之前执行一次，不随动态实例重复；
--     · 挂在 BASE 流程下会让「每证券月」的流程图里出现一个全局步骤，容易被误读成每实例都跑。
--   因此改挂到 PROCESS_EXPORT_BATCH（批次清单获取流程）。
--
-- 改动：
--   1. JOB_FETCH_SECURITY_METADATA 的 process_id → PROCESS_EXPORT_BATCH，seq_no = 2
--      （排在 JOB_FETCH_EXPORT_BATCH 之后：先算批次，再补元数据，再进入装载）；
--   2. PROCESS_EXPORT_FILE_MON_BASE 剩余 JOB 的 seq_no 重排为连续 1..N
--      （原为 2..5，留空洞会让人误以为少了步骤）；
--   3. execution_stage 保持 'META' 不变（前端按阶段显示装载/导出/登记/清理的进度）。
--
-- 注意：PROCESS_EXPORT_FILE_MON_BASE 的 seq_no 有 UNIQUE (process_id, seq_no) 约束，
--       重排分两步做（先整体 +100 腾出区间，再按原相对顺序写入 1..N），避免中途冲突。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_batch  bigint;
    v_base   bigint;
    v_job    bigint;
    v_cur_proc bigint;
BEGIN
    SELECT process_id INTO v_batch FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_EXPORT_BATCH';
    SELECT process_id INTO v_base  FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_EXPORT_FILE_MON_BASE';
    IF v_batch IS NULL OR v_base IS NULL THEN
        RAISE EXCEPTION 'V100087 前置校验失败：PROCESS_EXPORT_BATCH 或 PROCESS_EXPORT_FILE_MON_BASE 不存在';
    END IF;

    SELECT job_id, process_id INTO v_job, v_cur_proc
      FROM cdeu.finv_quote_cdeu_job WHERE job_code = 'JOB_FETCH_SECURITY_METADATA';
    IF v_job IS NULL THEN
        RAISE NOTICE 'V100087：JOB_FETCH_SECURITY_METADATA 不存在，跳过（幂等）';
        RETURN;
    END IF;
    IF v_cur_proc = v_batch THEN
        RAISE NOTICE 'V100087：JOB_FETCH_SECURITY_METADATA 已在 PROCESS_EXPORT_BATCH，跳过（幂等）';
        RETURN;
    END IF;
    IF v_cur_proc <> v_base THEN
        RAISE EXCEPTION 'V100087 前置校验失败：JOB_FETCH_SECURITY_METADATA 当前不属于预期流程（process_id=%）', v_cur_proc;
    END IF;
    RAISE NOTICE 'V100087 前置校验通过：job_id=% batch=% base=%', v_job, v_batch, v_base;
END $$;

-- ---------------------------------------------------------------------
-- 2. 迁移 JOB 归属
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_job j
   SET process_id = b.process_id,
       seq_no     = 2,
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_process b
 WHERE b.process_code = 'PROCESS_EXPORT_BATCH'
   AND j.job_code = 'JOB_FETCH_SECURITY_METADATA'
   AND j.process_id <> b.process_id;

-- ---------------------------------------------------------------------
-- 3. PROCESS_EXPORT_FILE_MON_BASE 剩余 JOB 重排 seq_no（两步，规避 UNIQUE 冲突）
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_job j
   SET seq_no = j.seq_no + 100,
       gmt_update = now()
  FROM cdeu.finv_quote_cdeu_process p
 WHERE p.process_id = j.process_id
   AND p.process_code = 'PROCESS_EXPORT_FILE_MON_BASE';

WITH ranked AS (
    SELECT j.job_id,
           row_number() OVER (ORDER BY j.seq_no, j.job_id) AS rn
      FROM cdeu.finv_quote_cdeu_job j
      JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_EXPORT_FILE_MON_BASE'
)
UPDATE cdeu.finv_quote_cdeu_job j
   SET seq_no = r.rn,
       gmt_update = now()
  FROM ranked r
 WHERE j.job_id = r.job_id;

-- ---------------------------------------------------------------------
-- 4. 后置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_proc bigint;
    v_seq  int;
    v_cnt  int;
    v_gap  int;
BEGIN
    SELECT j.process_id, j.seq_no INTO v_proc, v_seq
      FROM cdeu.finv_quote_cdeu_job j WHERE j.job_code = 'JOB_FETCH_SECURITY_METADATA';

    IF v_proc <> (SELECT process_id FROM cdeu.finv_quote_cdeu_process WHERE process_code = 'PROCESS_EXPORT_BATCH') THEN
        RAISE EXCEPTION 'V100087 后置校验失败：JOB_FETCH_SECURITY_METADATA 未挂到 PROCESS_EXPORT_BATCH';
    END IF;
    IF v_seq <> 2 THEN
        RAISE EXCEPTION 'V100087 后置校验失败：JOB_FETCH_SECURITY_METADATA 的 seq_no 应为 2，实际 %', v_seq;
    END IF;

    -- PROCESS_EXPORT_BATCH 内 seq_no 必须从 1 连续：1=批次清单，2=证券元数据
    SELECT count(*) INTO v_gap
      FROM cdeu.finv_quote_cdeu_job j
      JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_EXPORT_BATCH' AND j.enabled;
    IF v_gap <> 2 THEN
        RAISE EXCEPTION 'V100087 后置校验失败：PROCESS_EXPORT_BATCH 启用 JOB 数应为 2，实际 %', v_gap;
    END IF;

    -- PROCESS_EXPORT_FILE_MON_BASE 内 seq_no 必须连续（1..N，无空洞）
    SELECT count(*), coalesce(max(j.seq_no), 0) INTO v_cnt, v_gap
      FROM cdeu.finv_quote_cdeu_job j
      JOIN cdeu.finv_quote_cdeu_process p ON p.process_id = j.process_id
     WHERE p.process_code = 'PROCESS_EXPORT_FILE_MON_BASE';
    IF v_cnt <> v_gap THEN
        RAISE EXCEPTION 'V100087 后置校验失败：PROCESS_EXPORT_FILE_MON_BASE 的 seq_no 有空洞（job 数 %，最大 seq %）', v_cnt, v_gap;
    END IF;

    RAISE NOTICE 'V100087 完成：元数据 JOB 已归入导出批次流程，BASE 流程 seq 连续 1..%', v_gap;
END $$;

COMMIT;
