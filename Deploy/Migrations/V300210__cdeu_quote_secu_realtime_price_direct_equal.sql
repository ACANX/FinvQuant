-- =====================================================================
-- V300210：修正 cdeu.finv_quote_secu_realtime.price_direct 的取值口径注释
--
-- 背景（需求 11262 调整）：
--   V300209 建表时该列注释沿用参考表 cdeu.finv_cdeu_secu_realtime_stock 的写法
--   「+ 涨 / - 跌 / 0 平」；实际写入方（BDFS 板块成分股采集）用 "=" 表示持平，
--   与参考表不一致，必须把注释改成与实际写入一致，避免下游按 "0" 解析。
--
-- 范围：只改注释，不改列类型 / 默认值 / 约束；参考表
--   cdeu.finv_cdeu_secu_realtime_stock 的 price_direct 仍为 "+ / - / 0"，本次不动。
--
-- 幂等：COMMENT ON COLUMN 可重复执行；末尾校验注释内容，不符整体回滚。
-- =====================================================================

BEGIN;

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.price_direct
    IS '价格变动方向符号（+ 涨 / - 跌 / = 平）；本表用 = 表示持平，与参考表 finv_cdeu_secu_realtime_stock 的 0 不同';

DO $$
DECLARE
    v_cmt text;
BEGIN
    SELECT col_description('cdeu.finv_quote_secu_realtime'::regclass,
                           (SELECT attnum FROM pg_attribute
                             WHERE attrelid = 'cdeu.finv_quote_secu_realtime'::regclass
                               AND attname = 'price_direct'
                               AND NOT attisdropped))
      INTO v_cmt;
    IF v_cmt IS NULL OR v_cmt NOT LIKE '%= 平%' THEN
        RAISE EXCEPTION 'V300210 price_direct 注释不符：期望含「= 平」，实际 %', COALESCE(v_cmt, '<无注释>');
    END IF;
    IF v_cmt LIKE '%0 平%' THEN
        RAISE EXCEPTION 'V300210 price_direct 注释仍含旧的「0 平」口径：%', v_cmt;
    END IF;

    RAISE NOTICE 'V300210 校验通过：price_direct 注释为「+ / - / = 平」';
END $$;

COMMIT;
