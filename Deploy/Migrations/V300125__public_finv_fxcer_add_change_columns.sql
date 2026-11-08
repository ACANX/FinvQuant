-- =====================================================================
-- V300125：public.finv_fxcer 增加涨跌额 / 涨跌幅两列（需求 11016）
--
-- 背景：表格模式采集来源 moomoo get-fx-rate-list 除最新价外还返回
--   · changeAmount  —— 涨跌额，带符号（如 -0.0012）；
--   · changePercent —— 涨跌幅，含百分号（如 -0.35%）。
--   矩阵模式来源 get-fx-overview 只返回价格与时间，**不提供**这两个字段，
--   因此两列必须可空：overview 写入时保持 NULL（见 store 侧 COALESCE，不清空已有值）。
--
-- 列名沿用仓库既有口径（V200038 的 K 线表）：change_price = 涨跌额、change_ratio = 涨跌幅。
--   · change_price NUMERIC(24,10)：与 rate 同精度，容纳 JPYUSD 这类小数位多的货币对；
--   · change_ratio NUMERIC(12,6)：百分数数值（-0.35 表示 -0.35%），落库前去掉了「%」。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + COMMENT 可重复执行；
--   末尾校验列清单/CHECK 数/注释数，任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 增列
-- ---------------------------------------------------------------------
ALTER TABLE public.finv_fxcer
    ADD COLUMN IF NOT EXISTS change_price NUMERIC(24,10),
    ADD COLUMN IF NOT EXISTS change_ratio NUMERIC(12,6);

-- ---------------------------------------------------------------------
-- 2. 列注释
-- ---------------------------------------------------------------------
COMMENT ON COLUMN public.finv_fxcer.change_price IS
    '涨跌额（带符号）：moomoo get-fx-rate-list 的 changeAmount；只有 rateList 来源提供，overview 写入时保持原值不清空';
COMMENT ON COLUMN public.finv_fxcer.change_ratio IS
    '涨跌幅（百分数数值，-0.35 表示 -0.35%）：moomoo get-fx-rate-list 的 changePercent，落库前去掉「%」；overview 不提供';

-- ---------------------------------------------------------------------
-- 3. 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols text;
    v_want CONSTANT text := 'usc,name,currency_src,currency_dest,rate,ts_quote,source,'
                            'flag_derive,remark,gmt_create,gmt_update,'
                            'change_price,change_ratio';
    v_chk  int;
    v_cmt  int;
    v_type text;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'public' AND table_name = 'finv_fxcer';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300125 列结构不符：期望 %，实际 %', v_want, COALESCE(v_cols, '<表不存在>');
    END IF;

    SELECT data_type || '(' || COALESCE(numeric_precision::text, '') || ',' ||
           COALESCE(numeric_scale::text, '') || ')'
      INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'public' AND table_name = 'finv_fxcer' AND column_name = 'change_price';
    IF v_type IS DISTINCT FROM 'numeric(24,10)' THEN
        RAISE EXCEPTION 'V300125 change_price 类型不符：期望 numeric(24,10)，实际 %', COALESCE(v_type, '<缺失>');
    END IF;

    SELECT data_type || '(' || COALESCE(numeric_precision::text, '') || ',' ||
           COALESCE(numeric_scale::text, '') || ')'
      INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'public' AND table_name = 'finv_fxcer' AND column_name = 'change_ratio';
    IF v_type IS DISTINCT FROM 'numeric(12,6)' THEN
        RAISE EXCEPTION 'V300125 change_ratio 类型不符：期望 numeric(12,6)，实际 %', COALESCE(v_type, '<缺失>');
    END IF;

    SELECT count(*) INTO v_chk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer' AND c.contype = 'c';
    IF v_chk <> 7 THEN
        RAISE EXCEPTION 'V300125 CHECK 约束数不符（应为 7，本迁移不加约束）：实际 %', v_chk;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.objsubid
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer'
       AND a.attname IN ('usc', 'name', 'currency_src', 'currency_dest', 'rate', 'ts_quote',
                         'source', 'flag_derive', 'remark', 'gmt_create', 'gmt_update',
                         'change_price', 'change_ratio');
    IF v_cmt <> 13 THEN
        RAISE EXCEPTION 'V300125 列注释数不符：期望 13，实际 %', v_cmt;
    END IF;
END $$;

COMMIT;
