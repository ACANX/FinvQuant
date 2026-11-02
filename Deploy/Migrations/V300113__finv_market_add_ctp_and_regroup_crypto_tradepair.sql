-- =====================================================================
-- V300113：加密区域新增 CTP（加密货币交易对）分类并归位交易对数据（需求 10948）
--
-- 背景：
--   CC 区域此前只有一个 market=FTC（`finv_market` 10001「加密货币 - Binance 等交易所」），
--   现货币（BTC/ETH/XRP…）与交易对（BTC/USDC、AAVE/USD…）混在同一分类里：
--   · `public.finv_security` 里 172 行 `type_secu_sub='TradePair.CC'` 散落在
--     10013/10014 两个「加密货币现货（通用）」码上，market_symbol 仍是 FTC；
--   · `cdeu.finv_quote_cdeu_usc` 里 12 个已登记的交易对（名字形如 `BTC/USDC`）
--     也挂在 FTC 上。
--   两者在 {region}_{market} 目录段与导出文件名里无法与现货币区分。
--
-- 做法：
--   1) `finv_market` 新增 CTP：market_code=10016（10001–10015 已占用）、
--      market_symbol='CTP'、market_flag='CRYPTO_CTP'、base_currency='USD'、
--      exchange_code=10001、flag_enable='1'；
--   2) `public.finv_security`：交易所 10001 下 `type_secu_sub='TradePair.CC'` 的行
--      全量改到 10016；`cdeu.finv_quote_cdeu_usc` 里 12 个交易对（历史种子未写
--      type_secu_sub，按显式清单 + 名称含 '/' 双重确认）也改到 10016 并补上
--      type_secu_sub='TradePair.CC'。现货币（BTC/DOGE/ETH/PAXG/SOL/UNI/USDC/USDT/XRP）
--      保持 FTC 不变；
--   3) `cdeu.finv_quote_cdeu_usc`：上述 12 个交易对的 market 由 FTC 改为 CTP
--      （region 仍为 CC，market 取 market_symbol 口径，与需求 10944 一致）；
--   4) 模板表补 `CC / CTP / Crypto = 1440`（24 小时连续行情），
--      避免只落到 `CC / ''` 通配模板；
--   5) 校验：CTP 行、184 条交易对 market_code、12 个登记行 market、9 个现货币仍为
--      FTC、模板行；任一项不符整事务回滚。
--
-- 幂等：全部带 IS DISTINCT FROM / NOT EXISTS，重复执行影响 0 行。
-- 本迁移只改分类与 market 归属，不动任何行情数据。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. finv_market 新增 CTP
--    en_security_type 记交易对类型（该列语义为富途侧类型口径，交易对统一写
--    TradePair.CC，与 finv_security.type_secu_sub 同名值，便于互查）。
-- ---------------------------------------------------------------------
INSERT INTO public.finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type,
     base_currency, exchange_code, market_symbol, flag_enable)
VALUES
    (10016, 'CRYPTO_CTP', 'CRYPTO', '加密货币交易对', 'TradePair.CC', 'USD', 10001, 'CTP', '1')
ON CONFLICT (market_code) DO UPDATE
   SET market_flag   = EXCLUDED.market_flag,
       market_abbr   = EXCLUDED.market_abbr,
       market_name   = EXCLUDED.market_name,
       base_currency = EXCLUDED.base_currency,
       exchange_code = EXCLUDED.exchange_code,
       market_symbol = EXCLUDED.market_symbol,
       flag_enable   = EXCLUDED.flag_enable,
       gmt_update    = now()
 WHERE public.finv_market.market_symbol IS DISTINCT FROM EXCLUDED.market_symbol
    OR public.finv_market.flag_enable   IS DISTINCT FROM EXCLUDED.flag_enable
    OR public.finv_market.market_name   IS DISTINCT FROM EXCLUDED.market_name;

DO $$
DECLARE
    v_sec   bigint := 0;
    v_usc   bigint := 0;
    v_bad   text;
    v_cnt   bigint;
    -- 已登记到 cdeu.finv_quote_cdeu_usc 的 12 个交易对（名字形如 A/B）
    c_tp_scu CONSTANT text[] := ARRAY[
        'BCHUSDC', 'BTCHKD', 'BTCUSD', 'BTCUSDC', 'BTCUSDT', 'ETHBTC',
        'ETHUSDC', 'PAXGUSD', 'SOLBTC', 'USDCUSD', 'USDCUSDT', 'USDTUSD'];
    -- 同一批里的现货币，必须保持 FTC
    c_coin   CONSTANT text[] := ARRAY[
        'BTC', 'DOGE', 'ETH', 'PAXG', 'SOL', 'UNI', 'USDC', 'USDT', 'XRP'];
BEGIN
    -- -----------------------------------------------------------------
    -- 2. public.finv_security 归位交易对
    -- -----------------------------------------------------------------
    UPDATE public.finv_security s
       SET market_code   = 10016,
           type_secu_sub = COALESCE(NULLIF(s.type_secu_sub, ''), 'TradePair.CC'),
           gmt_update    = now()
     WHERE s.exchange_code = 10001
       AND (s.type_secu_sub = 'TradePair.CC'
            OR (s.usc = ANY (c_tp_scu) AND s.security_name_cn LIKE '%/%'))
       AND (s.market_code IS DISTINCT FROM 10016
            OR COALESCE(s.type_secu_sub, '') = '');
    GET DIAGNOSTICS v_sec = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. 登记表：12 个交易对 market 改为 CTP
    -- -----------------------------------------------------------------
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET market     = 'CTP',
           gmt_update = now()
     WHERE u.usc = ANY (c_tp_scu)
       AND u.region = 'CC'
       AND u.market IS DISTINCT FROM 'CTP';
    GET DIAGNOSTICS v_usc = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 4. 模板：CC / CTP / Crypto = 1440
    -- -----------------------------------------------------------------
    INSERT INTO cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
        (region, market, category, day_expect_rows, remark)
    VALUES ('CC', 'CTP', 'Crypto', 1440, '需求 10948：加密货币交易对（24 小时连续行情）')
    ON CONFLICT (region, market, category) DO NOTHING;

    -- -----------------------------------------------------------------
    -- 5. 校验
    -- -----------------------------------------------------------------
    -- 5.1 CTP 必须登记且启用
    IF NOT EXISTS (SELECT 1 FROM public.finv_market
                    WHERE market_code = 10016 AND market_symbol = 'CTP' AND flag_enable = '1') THEN
        RAISE EXCEPTION 'CTP 市场未按预期登记，回滚';
    END IF;

    -- 5.2 交易所 10001 下所有交易对必须落在 10016
    SELECT count(*) INTO v_cnt
      FROM public.finv_security
     WHERE exchange_code = 10001
       AND type_secu_sub = 'TradePair.CC'
       AND market_code = 10016;
    IF v_cnt <> 184 THEN
        RAISE EXCEPTION '交易对应为 184 条落在 CTP（172 条 TradePair.CC + 12 个已登记交易对），实际 %，回滚', v_cnt;
    END IF;

    -- 5.3 现货币必须保持 FTC（10001 或 10011 等 symbol=FTC 的码）
    SELECT string_agg(s.usc || '=' || s.market_code::text, ', ' ORDER BY s.usc) INTO v_bad
      FROM public.finv_security s
      JOIN public.finv_market m ON m.market_code = s.market_code
     WHERE s.exchange_code = 10001
       AND s.usc = ANY (c_coin)
       AND m.market_symbol IS DISTINCT FROM 'FTC';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION '现货币被误改到非 FTC：%，回滚', v_bad;
    END IF;

    -- 5.4 登记表：12 个交易对为 CTP、现货币仍为 FTC
    SELECT count(*) INTO v_cnt
      FROM cdeu.finv_quote_cdeu_usc
     WHERE usc = ANY (c_tp_scu) AND region = 'CC' AND market = 'CTP';
    IF v_cnt <> 12 THEN
        RAISE EXCEPTION '登记表交易对应为 12 行 CTP，实际 %，回滚', v_cnt;
    END IF;
    SELECT count(*) INTO v_cnt
      FROM cdeu.finv_quote_cdeu_usc
     WHERE usc = ANY (c_coin) AND region = 'CC' AND market = 'FTC';
    IF v_cnt <> 9 THEN
        RAISE EXCEPTION '登记表现货币应为 9 行 FTC，实际 %，回滚', v_cnt;
    END IF;

    -- 5.5 模板必须存在
    IF NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
                    WHERE region = 'CC' AND market = 'CTP' AND category = 'Crypto'
                      AND day_expect_rows = 1440) THEN
        RAISE EXCEPTION 'CC/CTP 模板行缺失，回滚';
    END IF;

    RAISE NOTICE '完成：finv_security 交易对归位 % 行；登记表 market 改 CTP % 行；CTP 模板已就绪',
        v_sec, v_usc;
END $$;

COMMIT;

-- 执行后核对
-- 1) 新市场：
-- SELECT market_code, market_flag, market_name, market_symbol, exchange_code, flag_enable
--   FROM public.finv_market WHERE market_code = 10016;
-- 2) 交易对 / 现货币分布（exchange_code=10001）：
-- SELECT m.market_symbol, count(*) FROM public.finv_security s
--   JOIN public.finv_market m ON m.market_code = s.market_code
--  WHERE s.exchange_code = 10001 GROUP BY 1 ORDER BY 1;
-- 3) 登记表 CC 区域：
-- SELECT market, count(*) FROM cdeu.finv_quote_cdeu_usc WHERE region = 'CC' GROUP BY 1;
