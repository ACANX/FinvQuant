-- =====================================================================
-- V300215：修订存量/增量同步函数，usc 以 public.finv_security 为准（需求 11272）
--
-- 背景：V300213 的同步函数直接把旧表 usc 搬过来，而旧快照的 usc 混用两种口径
--   （富途别名 `BJ.920025` / `US.MSGE.WI` 与标准编码 `920025` / `MSGE.WI`），
--   加上 BDFS 写入的是 finv_security.usc，导致同一证券在新表出现两条不同主键记录。
--
-- 本次只替换函数体（CREATE OR REPLACE），不改数据；数据归一由 V300214 负责。
-- 新口径：目标 usc = 主数据 public.finv_security.usc（按 交易所 + 代码 匹配），
--   匹配不到时才退回旧表 usc。
-- =====================================================================

BEGIN;

CREATE OR REPLACE FUNCTION cdeu.f_finv_quote_secu_realtime_sync(p_limit int DEFAULT 5000)
RETURNS TABLE (synced bigint, skipped_no_usc bigint, batches int)
LANGUAGE plpgsql
AS $$
DECLARE
    v_last    bigint := -9223372036854775808;
    v_rows    int;
    v_next    bigint;
    v_total   bigint := 0;
    v_batches int := 0;
    v_skip    bigint;
BEGIN
    IF p_limit IS NULL OR p_limit <= 0 THEN
        p_limit := 5000;
    END IF;

    -- 源表缺 usc 的行无法定位主键，统计后跳过
    SELECT count(*) INTO v_skip
      FROM cdeu.finv_cdeu_secu_realtime_stock
     WHERE usc IS NULL OR usc = '';

    LOOP
        WITH batch AS (
            SELECT s.*
              FROM cdeu.finv_cdeu_secu_realtime_stock s
             WHERE s.usc IS NOT NULL
               AND s.usc <> ''
               AND s.stock_id > v_last
             ORDER BY s.stock_id
             LIMIT p_limit
        ), done AS (
            INSERT INTO cdeu.finv_quote_secu_realtime (
                usc, stock_id, name, stock_code, market_type, market_label,
                instrument_type, currency, market_value, total_shares,
                circulation_total_shares, trade_trunover, trade_volumn,
                circulation_market_value, dt_update, price_nominal, change_ratio,
                trust_aum, trade_change_ratio, price_amplitude, volumn_ratio,
                buysell_ratio, change_price, price_direct, leverage, page,
                rmb_exchange_rate, rmb_market_value, rmb_circulation_market_value,
                rmb_trade_trunover, rmb_price_nominal, remark, provider
            )
            SELECT COALESCE(
                       (SELECT btrim(s.usc)
                          FROM public.finv_security s
                          JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
                         WHERE upper(btrim(e.exchange_flag)) = upper(btrim(b.market_label))
                           AND btrim(s.security_code) = btrim(b.stock_code)
                           AND s.usc IS NOT NULL AND btrim(s.usc) <> ''
                         LIMIT 1),
                       b.usc),
                   b.stock_id::text,
                   b.name, b.stock_code, b.market_type, b.market_label,
                   b.instrument_type, b.currency, b.market_value, b.total_shares,
                   b.circulation_total_shares, b.trade_trunover, b.trade_volumn,
                   b.circulation_market_value, b.dt_update, b.price_nominal, b.change_ratio,
                   b.trust_aum, b.trade_change_ratio, b.price_amplitude, b.volumn_ratio,
                   b.buysell_ratio, b.change_price,
                   CASE WHEN b.price_direct = '0' THEN '=' ELSE b.price_direct END,
                   b.leverage, b.page,
                   b.rmb_exchange_rate, b.rmb_market_value, b.rmb_circulation_market_value,
                   b.rmb_trade_trunover, b.rmb_price_nominal, b.remark,
                   'FTMM'
              FROM batch b
            ON CONFLICT (usc) DO UPDATE SET
                stock_id                 = EXCLUDED.stock_id,
                name                     = EXCLUDED.name,
                stock_code               = EXCLUDED.stock_code,
                market_type              = EXCLUDED.market_type,
                market_label             = EXCLUDED.market_label,
                instrument_type          = EXCLUDED.instrument_type,
                currency                 = EXCLUDED.currency,
                market_value             = EXCLUDED.market_value,
                total_shares             = EXCLUDED.total_shares,
                circulation_total_shares = EXCLUDED.circulation_total_shares,
                trade_trunover           = EXCLUDED.trade_trunover,
                trade_volumn             = EXCLUDED.trade_volumn,
                circulation_market_value = EXCLUDED.circulation_market_value,
                dt_update                = EXCLUDED.dt_update,
                price_nominal            = EXCLUDED.price_nominal,
                change_ratio             = EXCLUDED.change_ratio,
                trust_aum                = EXCLUDED.trust_aum,
                trade_change_ratio       = EXCLUDED.trade_change_ratio,
                price_amplitude          = EXCLUDED.price_amplitude,
                volumn_ratio             = EXCLUDED.volumn_ratio,
                buysell_ratio            = EXCLUDED.buysell_ratio,
                change_price             = EXCLUDED.change_price,
                price_direct             = EXCLUDED.price_direct,
                leverage                 = EXCLUDED.leverage,
                page                     = EXCLUDED.page,
                rmb_exchange_rate        = EXCLUDED.rmb_exchange_rate,
                rmb_market_value         = EXCLUDED.rmb_market_value,
                rmb_circulation_market_value = EXCLUDED.rmb_circulation_market_value,
                rmb_trade_trunover       = EXCLUDED.rmb_trade_trunover,
                rmb_price_nominal        = EXCLUDED.rmb_price_nominal,
                remark                   = EXCLUDED.remark,
                provider                 = EXCLUDED.provider
            -- 谁更新（dt_update 更大）谁赢，不设 provider 优先级（需求 11270）
            WHERE cdeu.finv_quote_secu_realtime.dt_update IS NULL
               OR cdeu.finv_quote_secu_realtime.dt_update < EXCLUDED.dt_update
            RETURNING 1
        )
        SELECT (SELECT count(*) FROM done), (SELECT max(stock_id) FROM batch)
          INTO v_rows, v_next;

        EXIT WHEN v_rows = 0 AND v_next IS NULL;

        v_total   := v_total + v_rows;
        v_batches := v_batches + 1;
        v_last    := COALESCE(v_next, v_last);

        EXIT WHEN v_next IS NULL;   -- 已到源表末尾
    END LOOP;

    synced         := v_total;
    skipped_no_usc := v_skip;
    batches        := v_batches;
    RETURN NEXT;
END;
$$;

COMMENT ON FUNCTION cdeu.f_finv_quote_secu_realtime_sync(int)
    IS '把 cdeu.finv_cdeu_secu_realtime_stock 的存量/增量快照复制到 cdeu.finv_quote_secu_realtime（provider=FTMM，usc 以 public.finv_security 为准，源表只读；按 stock_id 水位分批，按 dt_update 决胜、不设 provider 优先级）';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
         WHERE n.nspname = 'cdeu' AND p.proname = 'f_finv_quote_secu_realtime_sync'
    ) THEN
        RAISE EXCEPTION 'V300215 校验失败：函数 cdeu.f_finv_quote_secu_realtime_sync 不存在';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
         WHERE n.nspname = 'cdeu' AND p.proname = 'f_finv_quote_secu_realtime_sync'
           AND pg_get_functiondef(p.oid) LIKE '%public.finv_security s%'
    ) THEN
        RAISE EXCEPTION 'V300215 校验失败：函数体未包含 finv_security 归一逻辑';
    END IF;
    RAISE NOTICE 'V300215 校验通过：同步函数已按 finv_security.usc 归一';
END $$;

COMMIT;
