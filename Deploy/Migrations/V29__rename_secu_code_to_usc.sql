-- =====================================================================
-- FinvQuant PostgreSQL V29：证券代码列 secu_code 正名为 usc
--
-- 决策（ACANX 2026-10-24）：
--   - finv_security.usc 是统一证券代码字典主键；V21 注释已写明
--     「secu_code 实际为统一证券代码（usc，全局唯一）」，本次为**正名**，
--     语义不变，仅改名为 usc；
--   - 范围：全库统一，quote 与 backtest 两侧共 6 张表同时改名；
--   - 索引：依赖该列的索引一并改名，保持命名与列名一致
--     （PostgreSQL 的 RENAME COLUMN 会自动跟随索引/主键/外键中的列引用，
--       索引仅改名，不重建，无数据扫描）；
--   - 主键 finv_quote_secu_kline_min_pkey 名称保持不变，列引用自动跟随；
--   - 幂等：列改名用 information_schema.columns 判断，索引改名用 pg_class
--     判断「旧名存在且新名不存在」，重复执行安全；
--   - 历史迁移（V1/V21/V22/V24/V100019/V100023）保持原样，不回改。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 列改名：6 张表的 secu_code -> usc（幂等）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quote_secu_kline_min',
        'finv_quote_ingest_batches',
        'finv_quote_revision_log',
        'finv_quant_backtest_strategy',
        'finv_quant_backtest_run',
        'finv_quant_backtest_run_archive'
    ] LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema()
              AND table_name = t
              AND column_name = 'secu_code'
        ) THEN
            EXECUTE format('ALTER TABLE %I RENAME COLUMN secu_code TO usc', t);
            RAISE NOTICE 'V29: %.secu_code -> usc', t;
        ELSE
            RAISE NOTICE 'V29: %.secu_code 不存在，跳过', t;
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 2. 索引改名：保持索引名与新列名一致（幂等，不重建）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('idx_finv_quote_secu_secu_ts',           'idx_finv_quote_usc_ts'),
            ('idx_finv_quote_ingest_batches_secu',    'idx_finv_quote_ingest_batches_usc'),
            ('idx_finv_quote_revision_log_secu',      'idx_finv_quote_revision_log_usc'),
            ('idx_finv_quant_backtest_strategy_secu', 'idx_finv_quant_backtest_strategy_usc'),
            ('idx_finv_quant_backtest_run_secu',      'idx_finv_quant_backtest_run_usc')
        ) AS v(old_name, new_name)
    LOOP
        IF EXISTS (SELECT 1 FROM pg_class WHERE relname = r.old_name AND relkind = 'i')
           AND NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = r.new_name AND relkind = 'i') THEN
            EXECUTE format('ALTER INDEX %I RENAME TO %I', r.old_name, r.new_name);
            RAISE NOTICE 'V29: 索引 % -> %', r.old_name, r.new_name;
        ELSE
            RAISE NOTICE 'V29: 索引 % 无需改名（已改名或不存在）', r.old_name;
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 列注释：统一为「统一证券代码」，与字典表 finv_security.usc 对齐
-- ---------------------------------------------------------------------
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quote_secu_kline_min',
        'finv_quote_ingest_batches',
        'finv_quote_revision_log',
        'finv_quant_backtest_strategy',
        'finv_quant_backtest_run',
        'finv_quant_backtest_run_archive'
    ] LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema()
              AND table_name = t
              AND column_name = 'usc'
        ) THEN
            EXECUTE format(
                'COMMENT ON COLUMN %I.usc IS %L',
                t,
                '统一证券代码（usc，全局唯一，关联字典表 finv_security.usc）'
            );
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 4. 主键不重建
--    RENAME COLUMN 会自动更新 finv_quote_secu_kline_min_pkey 的列引用，
--    此处不重建主键，避免全表重建与长时间锁表。
-- ---------------------------------------------------------------------

COMMIT;
