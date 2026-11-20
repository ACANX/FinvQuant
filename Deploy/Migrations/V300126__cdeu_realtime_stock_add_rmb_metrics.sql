-- =====================================================================
-- V300126：cdeu.finv_cdeu_secu_realtime_stock 增加 5 个人民币计价字段（需求 11023）
--
-- 背景：实时快照里的价格/市值/成交额是**证券本币（currency 列）**计价
--   （HKD / USD / JPY …），跨币种无法直接比较大小。本次先补上人民币计价字段，
--   供后续「补全字段」的任务按这些指标做证券排行；本迁移只加列，不写数据、不改口径。
--
-- 列语义：
--   · rmb_exchange_rate            —— 转换汇率：1 单位本币折合多少人民币
--                                      （口径与 public.finv_fxcer 的 rate 一致：1 src = N dest，dest 为 CNY）；
--   · rmb_market_value             —— 人民币计价的市值（元）；
--   · rmb_circulation_market_value —— 人民币计价的流通市值（元）；
--   · rmb_trade_trunover           —— 人民币计价的成交额（元）；
--   · rmb_price_nominal            —— 人民币计价的单价（元/股）。
--
-- 约定：
--   · 全部可空 —— NULL 表示「尚未补全」，不是 0；排行时不要把 NULL 当 0 参与比较；
--   · 由写入方补全（补全任务后续实现），可比口径：rmb_* = 本币值 × rmb_exchange_rate；
--   · 汇率必须为正，故加 CHECK（rmb_exchange_rate IS NULL OR rmb_exchange_rate > 0）；
--   · 列序按用户指定：汇率在前，其后市值 / 流通市值 / 成交额 / 单价（ADD COLUMN 顺序即列序）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + COMMENT 可重复执行；
--   末尾校验列清单/类型/列序/注释数，任一不符整事务回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS rmb_exchange_rate            NUMERIC(24,10),
    ADD COLUMN IF NOT EXISTS rmb_market_value             NUMERIC(24,6),
    ADD COLUMN IF NOT EXISTS rmb_circulation_market_value NUMERIC(24,6),
    ADD COLUMN IF NOT EXISTS rmb_trade_trunover           NUMERIC(24,6),
    ADD COLUMN IF NOT EXISTS rmb_price_nominal            NUMERIC(20,6);

-- 汇率必须为正（NULL 表示未补全，允许）
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_cdeu_secu_realtime_stock_rmb_rate_chk'
           AND conrelid = 'cdeu.finv_cdeu_secu_realtime_stock'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
            ADD CONSTRAINT finv_cdeu_secu_realtime_stock_rmb_rate_chk
            CHECK (rmb_exchange_rate IS NULL OR rmb_exchange_rate > 0);
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 列注释：口径写进注释，避免下游各自理解
-- ---------------------------------------------------------------------
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.rmb_exchange_rate IS
    '人民币转换汇率：1 单位本币（currency 列）折合的人民币数量；口径同 public.finv_fxcer.rate（1 src = N CNY）；NULL=未补全';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.rmb_market_value IS
    '人民币计价市值（元）= market_value × rmb_exchange_rate；NULL=未补全，不等于 0';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.rmb_circulation_market_value IS
    '人民币计价流通市值（元）= circulation_market_value × rmb_exchange_rate；NULL=未补全，不等于 0';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.rmb_trade_trunover IS
    '人民币计价成交额（元）= trade_trunover × rmb_exchange_rate；NULL=未补全，不等于 0';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.rmb_price_nominal IS
    '人民币计价单价（元/股）= price_nominal × rmb_exchange_rate；NULL=未补全，不等于 0';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_order CONSTANT text := 'rmb_exchange_rate,rmb_market_value,rmb_circulation_market_value,'
                             'rmb_trade_trunover,rmb_price_nominal';
    v_order_tail text;
    v_type  text;
    v_chk   int;
    v_cmt   int;
    v_cnt   int;
BEGIN
    -- 1) 五列存在且类型正确
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name IN ('rmb_exchange_rate', 'rmb_market_value', 'rmb_circulation_market_value',
                           'rmb_trade_trunover', 'rmb_price_nominal');
    IF v_cnt <> 5 THEN
        RAISE EXCEPTION 'V300126 新增列缺失：期望 5 列，实际 %', v_cnt;
    END IF;

    SELECT data_type || '(' || COALESCE(numeric_precision::text, '') || ',' ||
           COALESCE(numeric_scale::text, '') || ')'
      INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'rmb_exchange_rate';
    IF v_type IS DISTINCT FROM 'numeric(24,10)' THEN
        RAISE EXCEPTION 'V300126 rmb_exchange_rate 类型不符：期望 numeric(24,10)，实际 %', COALESCE(v_type, '<缺失>');
    END IF;

    SELECT data_type || '(' || COALESCE(numeric_precision::text, '') || ',' ||
           COALESCE(numeric_scale::text, '') || ')'
      INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'rmb_price_nominal';
    IF v_type IS DISTINCT FROM 'numeric(20,6)' THEN
        RAISE EXCEPTION 'V300126 rmb_price_nominal 类型不符：期望 numeric(20,6)，实际 %', COALESCE(v_type, '<缺失>');
    END IF;

    -- 2) 列序：按用户指定顺序排在原有列之后
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_order_tail
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND ordinal_position > 26;
    IF v_order_tail IS DISTINCT FROM v_order THEN
        RAISE EXCEPTION 'V300126 列序不符：期望 %，实际 %', v_order, COALESCE(v_order_tail, '<空>');
    END IF;

    -- 3) 列总数：原 26 列 + 新增 5 列
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock';
    IF v_cnt <> 31 THEN
        RAISE EXCEPTION 'V300126 列总数不符：期望 31，实际 %', v_cnt;
    END IF;

    -- 4) 汇率 CHECK 约束存在
    SELECT count(*) INTO v_chk
      FROM pg_constraint
     WHERE conname = 'finv_cdeu_secu_realtime_stock_rmb_rate_chk'
       AND conrelid = 'cdeu.finv_cdeu_secu_realtime_stock'::regclass;
    IF v_chk <> 1 THEN
        RAISE EXCEPTION 'V300126 缺少 rmb_exchange_rate 正数 CHECK 约束';
    END IF;

    -- 5) 五条列注释齐全
    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.objsubid
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND a.attname IN ('rmb_exchange_rate', 'rmb_market_value', 'rmb_circulation_market_value',
                         'rmb_trade_trunover', 'rmb_price_nominal');
    IF v_cmt <> 5 THEN
        RAISE EXCEPTION 'V300126 列注释数不符：期望 5，实际 %', v_cmt;
    END IF;
END $$;

COMMIT;
