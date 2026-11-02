-- =====================================================================
-- V300112：补发 XRP 的 finv_security 初始数据（需求 10946）
--
-- 背景：
--   XRP 与 899050 / AMAT / MRVL / NOW / PLTR / SMCI 同属「仅本地有」清单
--   （需求 10930 的 M11、10945），但 10945 只发了后 6 只；XRP 至今没有
--   public.finv_security 记录，主数据页面查不到，也无法参与
--   RefreshSecurityMetadata 与 BackfillUscDefaults。
--
-- 取值：
--   · cdeu.finv_quote_cdeu_usc 现网值（V300096 发放、需求 10924/10935 已核对）：
--     category=Crypto、region=CC、market=FTC、timezone=UTC、currency=USD；
--   · 与同属 market_code=10001（CRYPTO_10001 → symbol FTC）的 BTC 行同市场口径，
--     exchange_code=10001、market_code=10001；
--   · tz 记 UTC 偏移、timezone 记时区名（与 A 股行 tz='+08:00'/timezone='Asia/Shanghai'
--     的列语义一致）；type_secu_sub 用加密对口径 'Crypto.CC'。
--
-- 幂等：ON CONFLICT DO NOTHING，重复执行影响 0 行。
-- =====================================================================

BEGIN;

INSERT INTO public.finv_security
    (usc, exchange_code, market_code, security_type, type_secu_sub,
     security_code, security_name, security_name_cn, security_name_full,
     currency_type, provider, remark, init_date, tz, timezone, flag_enable)
VALUES
    ('XRP', 10001, 10001, 'Crypto', 'Crypto.CC', 'XRP',
     'XRP', 'XRP', 'XRP', 'USD', 'FTMM',
     '需求 10946：本地独有证券初始数据发放', 20000000, '+00:00', 'UTC', '1')
ON CONFLICT DO NOTHING;

DO $$
DECLARE
    v_bad text;
BEGIN
    -- 1. 主数据必须落库且启用
    IF NOT EXISTS (SELECT 1 FROM public.finv_security
                    WHERE usc = 'XRP'
                      AND security_type = 'Crypto'
                      AND exchange_code = 10001
                      AND market_code = 10001
                      AND currency_type = 'USD'
                      AND timezone = 'UTC'
                      AND flag_enable = '1') THEN
        RAISE EXCEPTION 'XRP 主数据未按 CC/FTC/Crypto/UTC/USD 落库，回滚';
    END IF;

    -- 2. 登记表必须与主数据口径一致（region/market 由 V300110 统一为 symbol 口径）
    SELECT string_agg(u.usc || '=' || COALESCE(u.region, '') || '/' || COALESCE(u.market, '')
                      || '/' || COALESCE(u.category, ''), ', ')
      INTO v_bad
      FROM cdeu.finv_quote_cdeu_usc u
      JOIN public.finv_security s ON s.usc = u.usc
      JOIN public.finv_market m ON m.market_code = s.market_code
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_region rg ON rg.region = e.region
     WHERE u.usc = 'XRP'
       AND (u.market IS DISTINCT FROM NULLIF(m.market_symbol, '')
            OR u.region IS DISTINCT FROM COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region)
            OR u.category IS DISTINCT FROM s.security_type);
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'XRP 登记表与主数据口径不一致：%，回滚', v_bad;
    END IF;

    RAISE NOTICE '完成：XRP 主数据已就绪（CC/FTC/Crypto/UTC/USD），登记表口径一致';
END $$;

COMMIT;

-- 执行后核对
-- SELECT usc, exchange_code, market_code, security_type, type_secu_sub,
--        security_name, currency_type, tz, timezone, init_date, flag_enable
--   FROM public.finv_security WHERE usc = 'XRP';
-- SELECT usc, category, region, market, name, timezone, currency FROM cdeu.finv_quote_cdeu_usc
--  WHERE usc = 'XRP';
