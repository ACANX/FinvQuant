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
    sch TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quote_secu_kline_min',
        'finv_quote_ingest_batches',
        'finv_quote_revision_log',
        'finv_quant_backtest_strategy',
        'finv_quant_backtest_run',
        'finv_quant_backtest_run_archive'
    ] LOOP
        -- 按实际所在 schema 定位（兼容修正：不再用 current_schema()，避免命中 public
        -- 中其它服务的同名表）；优先本程序域 quote/quant，老库回退 public。
        sch := NULL;
        IF to_regclass(format('quote.%I', t)) IS NOT NULL THEN sch := 'quote';
        ELSIF to_regclass(format('quant.%I', t)) IS NOT NULL THEN sch := 'quant';
        ELSIF to_regclass(format('public.%I', t)) IS NOT NULL THEN sch := 'public';
        END IF;
        IF sch IS NOT NULL AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = sch
              AND table_name = t
              AND column_name = 'secu_code'
        ) THEN
            EXECUTE format('ALTER TABLE %I.%I RENAME COLUMN secu_code TO usc', sch, t);
            RAISE NOTICE 'V29: %.%.secu_code -> usc', sch, t;
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
    idx_sch TEXT;
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
        -- 只在目标 schema 内定位并改名（避免按 search_path 命中 public 中其它服务的同名索引）
        SELECT n.nspname INTO idx_sch
          FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE c.relkind = 'i' AND c.relname = r.old_name
           AND n.nspname IN ('quote', 'quant', 'public')
         LIMIT 1;
        IF idx_sch IS NOT NULL
           AND NOT EXISTS (
                SELECT 1 FROM pg_class c2 JOIN pg_namespace n2 ON n2.oid = c2.relnamespace
                 WHERE c2.relkind = 'i' AND c2.relname = r.new_name AND n2.nspname = idx_sch
           ) THEN
            EXECUTE format('ALTER INDEX %I.%I RENAME TO %I', idx_sch, r.old_name, r.new_name);
            RAISE NOTICE 'V29: 索引 %.% -> %', idx_sch, r.old_name, r.new_name;
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
    sch TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quote_secu_kline_min',
        'finv_quote_ingest_batches',
        'finv_quote_revision_log',
        'finv_quant_backtest_strategy',
        'finv_quant_backtest_run',
        'finv_quant_backtest_run_archive'
    ] LOOP
        sch := NULL;
        IF to_regclass(format('quote.%I', t)) IS NOT NULL THEN sch := 'quote';
        ELSIF to_regclass(format('quant.%I', t)) IS NOT NULL THEN sch := 'quant';
        ELSIF to_regclass(format('public.%I', t)) IS NOT NULL THEN sch := 'public';
        END IF;
        IF sch IS NOT NULL AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = sch
              AND table_name = t
              AND column_name = 'usc'
        ) THEN
            EXECUTE format(
                'COMMENT ON COLUMN %I.%I.usc IS %L',
                sch, t,
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
