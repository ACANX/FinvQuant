-- =====================================================================
-- V300117：cdeu.finv_cdeu_secu_realtime_stock 增加 7 个结构化数值指标
--
-- 背景：上游 secu_realtime_quote 的同名列是展示用文本（含 +/-、%、万/亿/万亿），
--   无法直接统计与排序；本迁移增加对应数值列，字符串解析在写入侧完成
--   （Python 同步脚本 / API 调用方），库内只存规范化数值。
--
-- 口径：
--   · 百分数列存「百分点数值」：1.03 表示 1.03%（与日K表 change_ratio 口径一致）；
--   · trust_aum 存「元」：万=1e4、亿=1e8、万亿=1e12 换算后写入；
--   · trade_change_ratio 由上游拼写 trade_changeraio 规范化命名；
--   · price_nominal 为纯十进制文本，直接转数值。
--
-- 列序：新增列紧随原 15 列之后，顺序为
--   price_nominal, change_ratio, trust_aum, trade_change_ratio,
--   price_amplitude, volumn_ratio, buysell_ratio
--   （按用户要求 price_nominal 排新增列第一位；ADD COLUMN 顺序即列序）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；末尾校验新增列数量/类型/列序与注释数量，不符回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS price_nominal      NUMERIC(20,6),
    ADD COLUMN IF NOT EXISTS change_ratio       NUMERIC(20,6),
    ADD COLUMN IF NOT EXISTS trust_aum          NUMERIC(24,6),
    ADD COLUMN IF NOT EXISTS trade_change_ratio NUMERIC(20,6),
    ADD COLUMN IF NOT EXISTS price_amplitude    NUMERIC(20,6),
    ADD COLUMN IF NOT EXISTS volumn_ratio       NUMERIC(20,6),
    ADD COLUMN IF NOT EXISTS buysell_ratio      NUMERIC(20,6);

-- ---------------------------------------------------------------------
-- 列注释：口径写进注释，避免下游各自理解
-- ---------------------------------------------------------------------
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.price_nominal
    IS '名义价格/单价（源值为纯十进制文本，直接转数值）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.change_ratio
    IS '涨跌幅（百分点数值，1.03 表示 +1.03%；源值 "+1.03%" 去符号后写入）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.trust_aum
    IS '信托/基金资产规模（单位：元；源值带单位，万=1e4、亿=1e8、万亿=1e12 换算后写入）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.trade_change_ratio
    IS '成交额变化率（百分点数值，0.12 表示 +0.12%；由上游列 trade_changeraio 规范化命名）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.price_amplitude
    IS '价格振幅（百分点数值，1.30 表示 1.30%）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.volumn_ratio
    IS '量比（无量纲数值，1.15 即 1.15 倍）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.buysell_ratio
    IS '买卖比（百分点数值，可为负；0.00 表示 0.00%）';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_new   CONSTANT text[] := ARRAY['price_nominal','change_ratio','trust_aum',
                                     'trade_change_ratio','price_amplitude','volumn_ratio','buysell_ratio'];
    v_order CONSTANT text   := 'price_nominal,change_ratio,trust_aum,trade_change_ratio,'
                               'price_amplitude,volumn_ratio,buysell_ratio';
    v_cnt   int;
    v_got   text;
    v_bad   text;
    v_cmt   int;
BEGIN
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = ANY(v_new) AND data_type = 'numeric';
    IF v_cnt <> 7 THEN
        RAISE EXCEPTION 'V300117 新增数值列数量不符：期望 7，实际 %', v_cnt;
    END IF;

    SELECT string_agg(column_name, ',') INTO v_bad
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = ANY(v_new) AND data_type <> 'numeric';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300117 列类型不符（非 numeric）：%', v_bad;
    END IF;

    -- 列序：新增列必须紧跟原 15 列，且 price_nominal 在第一位
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position) INTO v_got
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND ordinal_position > 15;
    IF v_got IS DISTINCT FROM v_order THEN
        RAISE EXCEPTION 'V300117 新增列顺序不符：期望 %，实际 %', v_order, COALESCE(v_got, '<无>');
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 22 THEN
        RAISE EXCEPTION 'V300117 列注释数量不符：期望 22（原 15 + 新增 7），实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300117 校验通过：7 个数值列已就绪（price_nominal 在前），列注释 22';
END $$;

COMMIT;
