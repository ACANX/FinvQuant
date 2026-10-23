-- =====================================================================
-- FinvQuant PostgreSQL V100024：把种子段新建的 3 张 quant 表迁入 quant schema
--
-- 背景（ACANX 2026-10-24，改造项 C）：
--   - 表按业务域归属：finv_quote_* → quote schema，finv_quant_* → quant schema；
--   - V32 负责迁移存量表，但它早于种子段（V100xxx）执行，而
--     finv_quant_backtest_del_task / finv_quant_backtest_del_log /
--     finv_quant_backtest_run_archive 三张表由种子迁移 V100023 用
--     CREATE TABLE IF NOT EXISTS 创建（不带 schema 前缀），在全新库中会落到
--     search_path 的首个 schema（public）；
--   - 因此需要本迁移（编号 > V100023）在其后把这三张表移入 quant；
--     存量库中它们已由 V32 迁走，本脚本判定为空操作（幂等）。
-- 编号说明：本文件属种子段编号区间，但内容为结构收尾迁移——原因如上，
--   必须排在 V100023 之后执行；历史种子迁移（V100019～V100023）保持原样。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    t TEXT;
    moved INT := 0;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quant_backtest_del_task',
        'finv_quant_backtest_del_log',
        'finv_quant_backtest_run_archive'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            EXECUTE format('ALTER TABLE public.%I SET SCHEMA quant', t);
            RAISE NOTICE 'V100024: public.% -> quant.%', t, t;
            moved := moved + 1;
        ELSE
            RAISE NOTICE 'V100024: public.% 不存在（已由 V32 迁走或未创建），跳过', t;
        END IF;
    END LOOP;

    RAISE NOTICE 'V100024: 本次移动 % 张表', moved;
END $$;

-- 收敛校验：已存在的 finv_quant_* 表不得留在 quant 之外
DO $$
DECLARE
    misplaced INT;
BEGIN
    SELECT count(*) INTO misplaced
      FROM pg_tables
     WHERE tablename LIKE 'finv\_quant\_%'
       AND schemaname <> 'quant';

    IF misplaced > 0 THEN
        RAISE EXCEPTION 'V100024: 仍有 % 张 finv_quant_* 表不在 quant schema', misplaced;
    END IF;

    RAISE NOTICE 'V100024: 校验通过（quant 中 finv_quant_* 共 % 张）',
        (SELECT count(*) FROM pg_tables WHERE schemaname = 'quant' AND tablename LIKE 'finv\_quant\_%');
END $$;

COMMIT;
