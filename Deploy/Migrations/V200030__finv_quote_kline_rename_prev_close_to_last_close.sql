-- =====================================================================
-- FinvQuant PostgreSQL V30：finv_quote_secu_kline_min.prev_close 重命名为 last_close
--
-- 决策（ACANX 2026-10-24）：
--   - 字段语义为「前收盘价」（数据源 MVSV 的 PrevClose / p 列），与 open
--     （开盘价）是独立字段，历史命名 prev_close 易被误解为「上一根 K 线的收盘价」，
--     本次正名为 last_close；
--   - 本次只改名、不删列：change / change_pct 仍由 close - last_close 计算，
--     计算逻辑不变，前端不展示该列，故前端零改动；
--   - 该列无索引、无约束、无视图依赖，RENAME 为 catalog 级操作（O(1)），不扫描数据；
--   - 幂等：以 information_schema.columns 判断旧列是否存在，重复执行安全；
--   - 历史迁移 V1 保留原名，不回改（已应用版本会被 schema_version 跳过）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- 前置：V29（secu_code -> usc）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 列改名：prev_close -> last_close（幂等）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    sch TEXT;
BEGIN
    -- 兼容修正：按实际所在 schema 定位（优先 quote，老库回退 public），
    -- 避免 current_schema()=public 时命中其它服务的同名表。
    IF to_regclass('quote.finv_quote_secu_kline_min') IS NOT NULL THEN sch := 'quote';
    ELSIF to_regclass('public.finv_quote_secu_kline_min') IS NOT NULL THEN sch := 'public';
    END IF;
    IF sch IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = sch
          AND table_name = 'finv_quote_secu_kline_min'
          AND column_name = 'prev_close'
    ) THEN
        EXECUTE format('ALTER TABLE %I.finv_quote_secu_kline_min RENAME COLUMN prev_close TO last_close', sch);
        RAISE NOTICE 'V30: %.finv_quote_secu_kline_min.prev_close -> last_close', sch;
    ELSE
        RAISE NOTICE 'V30: prev_close 不存在，跳过（已改名为 last_close 或表结构已更新）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 列注释：写明口径，避免再被误当作回调价/开盘价
-- ---------------------------------------------------------------------
DO $$
DECLARE
    sch TEXT;
BEGIN
    IF to_regclass('quote.finv_quote_secu_kline_min') IS NOT NULL THEN sch := 'quote';
    ELSIF to_regclass('public.finv_quote_secu_kline_min') IS NOT NULL THEN sch := 'public';
    END IF;
    IF sch IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = sch
          AND table_name = 'finv_quote_secu_kline_min'
          AND column_name = 'last_close'
    ) THEN
        EXECUTE format('COMMENT ON COLUMN %I.finv_quote_secu_kline_min.last_close IS %L',
                       sch,
                       '前收盘价（数据源 MVSV 的 PrevClose / p 列）；与 open 独立，不可作为开盘价或开盘价的替代');
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 不重建任何索引/约束
--    RENAME COLUMN 自动跟随列引用；该列本身无索引依赖。
-- ---------------------------------------------------------------------

COMMIT;
