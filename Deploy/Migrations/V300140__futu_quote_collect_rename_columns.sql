-- =====================================================================
-- FinvQuant PostgreSQL V300140：futu.finv_futu_quote_collect 列改名（需求 11039）
--
-- 授权：列改名属 DbSchemaSpec §0 红线操作（RENAME），本次由用户**明确点名**：
--   「type_symbol 字段改为 instrument；quote_market 字段改为 market_label」。
--
-- 改动：
--   · `type_symbol`  → `instrument`   （证券类型标识：stock / etfs / index / currency / crypto / futures / sectors）
--   · `quote_market` → `market_label` （行情市场标识：SZ / SH / BJ / HK / US / FX / CC）
--   仅改名，类型 / NOT NULL / 数据一律不变；表内 20,741 行数据不受影响。
--
-- 为什么这样命名：
--   `instrument` 与已建的键表 `futu.finv_quote_secu_key.instrument`、上游
--   `finv_quote_secu_meta.instrument` 对齐；`market_label` 与上游同名字段、
--   及 `cdeu.finv_cdeu_secu_realtime_stock.market_label` 对齐，便于三处 join 与同步任务复用。
--
-- 注意：本次只改 `futu.finv_futu_quote_collect`。
--   · 历史迁移（V200083 建表、V300063/V300131/V300133/V300134/V300135/V300136 等写入）
--     仍在**本迁移之前**执行，仍使用旧列名，顺序上不冲突，故一律不改（迁移文件不可回改）；
--   · `public.finv_security` / `cdeu.*` 等其它表的同名字段不在本次范围。
--
-- 幂等：用 `DO` 块判断「旧列存在且新列不存在」才改名，重跑为 no-op；
--   末尾收敛校验（旧列必须已消失、新列必须存在且 NOT NULL、列数不变）。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V200083（建表）、V300139（键表补列，仅顺序参考）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置：表必须存在
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('futu.finv_futu_quote_collect') IS NULL THEN
        RAISE EXCEPTION 'V300140 缺少表 futu.finv_futu_quote_collect（需 V200083）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 列改名（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
                  AND column_name = 'type_symbol')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                        WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
                          AND column_name = 'instrument') THEN
        ALTER TABLE futu.finv_futu_quote_collect RENAME COLUMN type_symbol TO instrument;
        RAISE NOTICE 'V300140 type_symbol → instrument 已改名';
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
                  AND column_name = 'quote_market')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                        WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
                          AND column_name = 'market_label') THEN
        ALTER TABLE futu.finv_futu_quote_collect RENAME COLUMN quote_market TO market_label;
        RAISE NOTICE 'V300140 quote_market → market_label 已改名';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 列注释（改名后重写，保持与新列名一致）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN futu.finv_futu_quote_collect.instrument IS
    '证券类型标识（原 type_symbol）：stock / etfs / index / currency / crypto / futures / sectors';
COMMENT ON COLUMN futu.finv_futu_quote_collect.market_label IS
    '行情市场标识（原 quote_market）：SZ / SH / BJ / HK / US / FX / CC';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_old int;
    v_new int;
    v_cnt int;
    v_bad text;
BEGIN
    SELECT count(*) INTO v_old
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
       AND column_name IN ('type_symbol', 'quote_market');
    IF v_old <> 0 THEN
        RAISE EXCEPTION 'V300140 旧列仍存在（type_symbol / quote_market 共 % 列）', v_old;
    END IF;

    SELECT count(*) INTO v_new
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect'
       AND column_name IN ('instrument', 'market_label')
       AND is_nullable = 'NO';
    IF v_new <> 2 THEN
        RAISE EXCEPTION 'V300140 新列缺失或可空性不符（命中 % 列，期望 2 且 NOT NULL）', v_new;
    END IF;

    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect';
    IF v_cnt <> 14 THEN
        RAISE EXCEPTION 'V300140 列数不符：期望 14，实际 %', v_cnt;
    END IF;

    SELECT count(*)::text INTO v_bad FROM futu.finv_futu_quote_collect;
    RAISE NOTICE 'V300140 改名完成：共 14 列，表内 % 行数据未受影响', v_bad;
END $$;

COMMIT;
