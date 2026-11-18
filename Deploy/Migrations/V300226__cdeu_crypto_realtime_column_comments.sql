-- =====================================================================
-- V300226：补齐 cdeu.finv_quote_secu_realtime_crypto 的列中文注释（需求 11317）
--
-- 背景：V300225 建表时只给 8 列写了 COMMENT，其余 25 列为空（前端/排障看不到中文口径）。
-- 本迁移把 33 列全部补上中文注释，并统一用「字段含义（接口来源）」的写法。
--
-- 幂等：COMMENT ON COLUMN 为覆盖写；末尾校验「33 列均有非空注释」，不符则整事务回滚。
-- 回滚：恢复为 V300225 时的注释集合（8 列有值、25 列为空）——本迁移只改注释，不影响数据。
-- =====================================================================

BEGIN;

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.usc IS
    '证券统一编码（对齐 public.finv_security.usc，如 BTCUSD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.code IS
    'moomoo OpenAPI 代码（CC.BTCUSD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.name IS
    '标的名称（接口 name，如 BTC/USD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.quote_currency IS
    '计价货币（从 usc 后缀推断，如 USD / HKD / EUR；无后缀时为空）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_nominal IS
    '最新价（接口 last_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_open IS
    '开盘价（接口 open_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_highest IS
    '最高价（接口 high_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_lowest IS
    '最低价（接口 low_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_last_close IS
    '上一收盘价（接口 prev_close_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_avg IS
    '均价（接口 avg_price）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.change_price IS
    '涨跌额（last_price − prev_close_price，带符号）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.change_ratio IS
    '涨跌幅（百分点数值：1.23 表示 1.23%，基准为 prev_close_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_amplitude IS
    '振幅（百分点数值，接口 amplitude）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_direct IS
    '涨跌方向（+ 涨 / - 跌 / = 持平）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.trade_volumn IS
    '成交量（接口 volume）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.trade_trunover IS
    '成交额（接口 turnover，计价货币）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.volumn_ratio IS
    '量比（接口 volume_ratio）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.bid_price IS
    '买一价（接口 bid_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.ask_price IS
    '卖一价（接口 ask_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.bid_vol IS
    '买一量（接口 bid_vol）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.ask_vol IS
    '卖一量（接口 ask_vol）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.buysell_ratio IS
    '买卖比（接口 bid_ask_ratio，百分点数值）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.highest52weeks IS
    '52 周最高价（接口 highest52weeks_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.lowest52weeks IS
    '52 周最低价（接口 lowest52weeks_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.highest_history IS
    '历史最高价（接口 highest_history_price）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.lowest_history IS
    '历史最低价（接口 lowest_history_price）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.sec_status IS
    '证券状态（接口 sec_status，如 NORMAL）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.suspension IS
    '是否停牌（接口 suspension）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.data_date IS
    '行情数据日期（接口 data_date，YYYY-MM-DD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.dt_update IS
    '接口 update_time（Unix 毫秒）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.provider IS
    '数据提供方（固定 MMAPI = moomoo OpenAPI）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.gmt_create IS
    '创建时间（默认 now()）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.gmt_update IS
    '更新时间（由触发器 trg_finv_quote_secu_realtime_crypto_gmt_update 维护）';

-- 收敛校验：33 列必须全部有非空注释
DO $$
DECLARE
    v_missing text;
BEGIN
    SELECT string_agg(a.attname, ', ' ORDER BY a.attnum)
      INTO v_missing
      FROM pg_attribute a
     WHERE a.attrelid = 'cdeu.finv_quote_secu_realtime_crypto'::regclass
       AND a.attnum > 0
       AND NOT a.attisdropped
       AND COALESCE(btrim(col_description(a.attrelid, a.attnum)), '') = '';
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300226 校验失败：仍缺少中文注释的列 = %', v_missing;
    END IF;
    RAISE NOTICE 'V300226 完成：33 列中文注释齐全';
END $$;

COMMIT;
