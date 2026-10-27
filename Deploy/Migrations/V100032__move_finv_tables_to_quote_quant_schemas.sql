-- =====================================================================
-- FinvQuant PostgreSQL V32：按业务域把表迁入 quote / quant schema
--
-- 决策（ACANX 2026-10-24）：
--   - 需求：finv_quote 开头的表迁入 quote schema，finv_quant 开头的表迁入
--     quant schema，实现「业务域 = schema」的分包；字典/基础数据表
--     （finv_security / finv_market / finv_exchange / finv_region /
--     finv_currency / finv_futu_mapping_*）与 schema_version 保留 public；
--   - 只改 schema、不改表名、不改列：ALTER TABLE ... SET SCHEMA 为 catalog
--     级操作，索引/约束/identity 序列/触发器自动跟随，不扫描数据、不重写表；
--   - 物理外键为 0，无需处理跨 schema 引用；
--   - search_path：除对当前库设置默认值外，应用连接串也携带同样的
--     search_path（见 internal/database/postgres.go），原因是本迁移（V32）
--     早于种子段 V100xxx 执行，而历史种子 SQL 写的是不带 schema 的表名，
--     必须依赖 search_path 才能解析到新 schema；连接串设置在 V1 起即生效，
--     quote/quant 尚未创建时 PostgreSQL 会静默忽略不存在的 schema；
--   - 幂等：逐表用 pg_tables.schemaname 判断，已迁出则跳过；末尾以断言
--     收敛校验，任何一张表未迁出即整事务回滚。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- 前置：V29（secu_code -> usc）、V30（prev_close -> last_close）、V31（open 回填）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建业务域 schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS quote;
CREATE SCHEMA IF NOT EXISTS quant;

-- ---------------------------------------------------------------------
-- 2. finv_quote_* → quote schema（3 张，幂等）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quote_secu_kline_min',
        'finv_quote_ingest_batches',
        'finv_quote_revision_log'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            EXECUTE format('ALTER TABLE public.%I SET SCHEMA quote', t);
            RAISE NOTICE 'V32: public.% -> quote.%', t, t;
        ELSE
            RAISE NOTICE 'V32: public.% 不存在或已迁出，跳过', t;
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. finv_quant_* → quant schema（13 张，幂等）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_quant_backtest_account',
        'finv_quant_backtest_cashflow',
        'finv_quant_backtest_del_log',
        'finv_quant_backtest_del_task',
        'finv_quant_backtest_equity',
        'finv_quant_backtest_event_trace',
        'finv_quant_backtest_position_log',
        'finv_quant_backtest_run',
        'finv_quant_backtest_run_archive',
        'finv_quant_backtest_strategy',
        'finv_quant_backtest_trade',
        'finv_quant_environment',
        'finv_quant_template'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            EXECUTE format('ALTER TABLE public.%I SET SCHEMA quant', t);
            RAISE NOTICE 'V32: public.% -> quant.%', t, t;
        ELSE
            RAISE NOTICE 'V32: public.% 不存在或已迁出，跳过', t;
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 4. 数据库默认 search_path（对新连接生效，供手工 psql 与历史种子迁移兜底）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path TO public, quote, quant', current_database());
    RAISE NOTICE 'V32: 数据库 % 的 search_path 设为 public, quote, quant', current_database();
END $$;

-- ---------------------------------------------------------------------
-- 5. 收敛校验：已存在的表必须都在目标 schema；未创建的表（种子段 V100023
--    才建的 finv_quant_backtest_del_task / del_log / run_archive）跳过
-- ---------------------------------------------------------------------
DO $$
DECLARE
    misplaced_quote INT;
    misplaced_quant INT;
    missing_quote   TEXT;
BEGIN
    SELECT count(*), string_agg(tablename, ',')
      INTO misplaced_quant, missing_quote
      FROM pg_tables
     WHERE tablename LIKE 'finv\_quant\_%'
       AND schemaname <> 'quant';

    SELECT count(*) INTO misplaced_quote
      FROM pg_tables
     WHERE tablename LIKE 'finv\_quote\_%'
       AND schemaname <> 'quote';

    IF misplaced_quote > 0 THEN
        RAISE EXCEPTION 'V32: 有 % 张 finv_quote_* 表不在 quote schema', misplaced_quote;
    END IF;
    IF misplaced_quant > 0 THEN
        RAISE EXCEPTION 'V32: 有 % 张 finv_quant_* 表不在 quant schema（%）', misplaced_quant, missing_quote;
    END IF;

    RAISE NOTICE 'V32: 校验通过（quote=%, quant=%, public 残留=0）',
        (SELECT count(*) FROM pg_tables WHERE schemaname = 'quote' AND tablename LIKE 'finv\_quote\_%'),
        (SELECT count(*) FROM pg_tables WHERE schemaname = 'quant' AND tablename LIKE 'finv\_quant\_%');
END $$;

COMMIT;
