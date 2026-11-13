-- =====================================================================
-- FinvQuant PostgreSQL V300138：futu.finv_quote_secu_key.status 增加取值约束（需求 11037）
--
-- 背景（用户确认 2026-11-09）：
--   `status` 列的取值集合固定为 5 个：`PENDING` / `QUEUED` / `DONE` / `FAILED` / `SKIP`，
--   不允许写入其它值，避免下游同步任务分支出未定义行为。
--   本迁移只加约束与注释，**不写入任何初始数据**（用户明确：后续走同步）。
--
-- 幂等：约束用 `DO` 块判断 `pg_constraint` 后新增，重跑不重复创建也不报错；
--   历史数据若不满足约束会在此处报错回滚（当前表为空，不会触发）。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾收敛校验。
-- 前置：V300137（建表）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 取值约束（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint c
          JOIN pg_class t ON t.oid = c.conrelid
          JOIN pg_namespace n ON n.oid = t.relnamespace
         WHERE n.nspname = 'futu'
           AND t.relname = 'finv_quote_secu_key'
           AND c.conname = 'finv_quote_secu_key_status_check'
    ) THEN
        -- 先给出存量违规行数量，便于报错时定位（为空表时恒 0）
        PERFORM 1 FROM futu.finv_quote_secu_key
         WHERE status NOT IN ('PENDING', 'QUEUED', 'DONE', 'FAILED', 'SKIP')
         LIMIT 1;
        IF FOUND THEN
            RAISE EXCEPTION 'V300138 存量数据存在非约定 status 值，请先纠正后再加约束';
        END IF;

        ALTER TABLE futu.finv_quote_secu_key
            ADD CONSTRAINT finv_quote_secu_key_status_check
            CHECK (status IN ('PENDING', 'QUEUED', 'DONE', 'FAILED', 'SKIP'));
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 列注释同步（写明 5 个合法取值）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN futu.finv_quote_secu_key.status IS
    '处理状态，取值固定 5 个：PENDING（待处理，缺省）/ QUEUED（已入队）/ DONE（已完成）/ FAILED（失败）/ SKIP（跳过）';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt int;
    v_def text;
BEGIN
    SELECT count(*) INTO v_cnt
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_quote_secu_key'
       AND c.conname = 'finv_quote_secu_key_status_check' AND c.contype = 'c';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V300138 status 取值约束缺失';
    END IF;

    SELECT column_default INTO v_def
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_quote_secu_key' AND column_name = 'status';
    IF v_def IS NULL OR v_def NOT LIKE '%PENDING%' THEN
        RAISE EXCEPTION 'V300138 status 默认值异常：%', COALESCE(v_def, '<NULL>');
    END IF;

    RAISE NOTICE 'V300138 status 约束已生效：PENDING / QUEUED / DONE / FAILED / SKIP（默认 PENDING）';
END $$;

COMMIT;
