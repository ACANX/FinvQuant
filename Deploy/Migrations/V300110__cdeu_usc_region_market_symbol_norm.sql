-- =====================================================================
-- V300110：行情证券登记表 region/market 口径纠正为 market_symbol（需求 10944）
--
-- 背景：
--   生产 cdeu.finv_quote_cdeu_usc 里大量行的 market 存的是
--   public.finv_market.market_flag（US-ARCA、SSE-A、SZSE-A、SSE-STAR、
--   HKEX-H、US-I、US-II、US-NYSE、FX），而 {region}_{market} 目录段与
--   导出文件名契约要求 market 取 market_symbol（ARCA、SH、SZ、HK、US、NY、
--   CER、FTC）。口径混用使生产页面与导出文件名出现 CN_SSE-A_…、
--   US_US-ARCA_…、FX_FX_BTC_… 这类不成立的段。
--
-- 口径（用户确认）：
--   · region = public.finv_exchange.region（经 public.finv_region 校验）；
--   · market = public.finv_market.market_symbol（经 public.finv_security.market_code 关联）。
--
-- 步骤：
--   0) 新增 public.finv_market 的 CBOE 行（market_code=1323，原字典只有
--      exchange 34 而无对应 market），仓单 VXMain 由此可落到 US/CBOE。
--   1) 按品种修正 public.finv_security.market_code（期货被误填成美股市场码：
--      1OZMain/GCCurrent/QOCurrent/QOMain/SICurrent/SIMain/SGUMain=US-ARCA、
--      ESMain/NQMain=US-BATS、CLMain/QMMain/BZMain=US-NYSE、YMMain=NYMEX、
--      VXMain=US-BATS），为第 2/4 步提供正确主数据。
--   2) 按主数据纠正登记表：market_symbol + region。**只在现值不是合法
--      market_symbol 时生效**，已正确的 symbol 值不动。
--   3) 主数据查不到时，退回 market_flag → market_symbol 唯一映射
--      （US-ARCA→ARCA、US-NYSE→NY、US-I/US-II→US、SSE-A/SSE-STAR→SH、
--      SZSE-A→SZ、HKEX-H→HK 等）。
--   4) 显式清单：期货 16 条强制对齐主数据（现值是合法 symbol 但与品种不符）、
--      NOW 的 NYSE→NY、GDRnext 登记（HK/HK）、SH0680 名称纠正。
--
-- 为什么不整表按主数据覆盖：见第 1 步——期货 market_code 修好之前整表覆盖会把
--   生产上已正确的 US/COMEX 改成 US/ARCA；修好之后仍只对「现值非 symbol」
--   与显式清单生效，避免误改人工维护值。
--
-- 幂等：重复执行影响 0 行（各步均带 IS DISTINCT FROM / NOT EXISTS 判断）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. finv_market 新增 CBOE（芝交所期权交易所）
--    en_security_type 沿用期货段占位值 1390（上游类型字典未定版，口径同
--    V300015 的占位先例），market_symbol=CBOE、exchange_code=34。
-- ---------------------------------------------------------------------
INSERT INTO public.finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type,
     base_currency, exchange_code, market_symbol, flag_enable)
VALUES
    (1323, 'CBOE', 'CBOE', '芝加哥期权交易所', '1390', 'USD', 34, 'CBOE', '1')
ON CONFLICT (market_code) DO UPDATE
   SET market_flag      = EXCLUDED.market_flag,
       market_abbr      = EXCLUDED.market_abbr,
       market_name      = EXCLUDED.market_name,
       base_currency    = EXCLUDED.base_currency,
       exchange_code    = EXCLUDED.exchange_code,
       market_symbol    = EXCLUDED.market_symbol,
       flag_enable      = EXCLUDED.flag_enable,
       gmt_update       = now()
 WHERE public.finv_market.market_symbol IS DISTINCT FROM EXCLUDED.market_symbol
    OR public.finv_market.exchange_code IS DISTINCT FROM EXCLUDED.exchange_code
    OR public.finv_market.flag_enable   IS DISTINCT FROM EXCLUDED.flag_enable;

DO $$
DECLARE
    v_fix_code bigint := 0;
    v_by_dict  bigint := 0;
    v_by_flag  bigint := 0;
    v_by_list  bigint := 0;
    v_tmp      bigint;
    v_bad      text;
BEGIN
    -- -----------------------------------------------------------------
    -- 1. 期货 market_code 按品种修正（usc → market_code）
    -- -----------------------------------------------------------------
    UPDATE public.finv_security s
       SET market_code = v.market_code,
           gmt_update  = now()
      FROM (VALUES
                ('1OZMain',   1320),  -- 1 盎司黄金期货 → COMEX
                ('GCCurrent', 1320),  -- 黄金当期期货 → COMEX
                ('GCMain',    1320),  -- 黄金期货主连 → COMEX
                ('MGCMain',   1320),  -- 微黄金期货 → COMEX
                ('QOCurrent', 1320),  -- 迷你黄金当期期货 → COMEX
                ('QOMain',    1320),  -- 迷你黄金期货 → COMEX
                ('SGUMain',   1320),  -- 上海金(美元)期货 → COMEX
                ('SICurrent', 1320),  -- 白银当期期货 → COMEX
                ('SIMain',    1320),  -- 白银期货主连 → COMEX
                ('ESMain',    1321),  -- 标普500指数期货 → CME
                ('NQMain',    1321),  -- 纳斯达克100指数期货 → CME
                ('YMMain',    1322),  -- 道琼斯指数期货 → CBOT
                ('BZMain',    1319),  -- 布伦特原油(现金)期货 → NYMEX
                ('CLMain',    1319),  -- WTI 原油期货 → NYMEX
                ('QMMain',    1319),  -- 迷你原油期货 → NYMEX
                ('VXMain',    1323)   -- VIX 指数期货 → CBOE
           ) AS v(usc, market_code)
     WHERE s.usc = v.usc
       AND s.market_code IS DISTINCT FROM v.market_code;
    GET DIAGNOSTICS v_fix_code = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 2. 按主数据纠正登记表（现值为非 market_symbol 时才改）
    -- -----------------------------------------------------------------
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET market     = m.market_symbol,
           region     = COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region),
           gmt_update = now()
      FROM public.finv_security s
      JOIN public.finv_market m ON m.market_code = s.market_code
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_region rg ON rg.region = e.region
     WHERE u.usc = s.usc
       AND btrim(m.market_symbol) <> ''
       AND (u.market IS NULL OR btrim(u.market) = ''
            OR NOT EXISTS (SELECT 1 FROM public.finv_market x
                            WHERE x.market_symbol = u.market))
       AND (u.market IS DISTINCT FROM m.market_symbol
            OR u.region IS DISTINCT FROM COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region));
    GET DIAGNOSTICS v_by_dict = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. 主数据查不到：market_flag → market_symbol 唯一映射兜底
    -- -----------------------------------------------------------------
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET market     = f.market_symbol,
           gmt_update = now()
      FROM public.finv_market f
     WHERE btrim(f.market_symbol) <> ''
       AND u.market = f.market_flag
       AND u.market IS DISTINCT FROM f.market_symbol
       AND NOT EXISTS (SELECT 1 FROM public.finv_market x WHERE x.market_symbol = u.market);
    GET DIAGNOSTICS v_by_flag = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 4. 显式清单
    -- -----------------------------------------------------------------
    -- 4.1 期货 16 条：现值可能是合法 symbol 但与品种不符（如本地 US/ARCA、
    --     US/BATS、US/NY），按已修正的主数据强制对齐。
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET market     = m.market_symbol,
           region     = COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region),
           gmt_update = now()
      FROM public.finv_security s
      JOIN public.finv_market m ON m.market_code = s.market_code
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_region rg ON rg.region = e.region
     WHERE u.usc = s.usc
       AND u.usc IN ('1OZMain', 'GCCurrent', 'GCMain', 'MGCMain', 'QOCurrent', 'QOMain',
                     'SGUMain', 'SICurrent', 'SIMain', 'ESMain', 'NQMain', 'YMMain',
                     'BZMain', 'CLMain', 'QMMain', 'VXMain')
       AND (u.market IS DISTINCT FROM m.market_symbol
            OR u.region IS DISTINCT FROM COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region));
    GET DIAGNOSTICS v_by_list = ROW_COUNT;

    -- 4.2 NOW：主数据未收录（需求 10930 的本地独有清单），NYSE 是 market_flag，
    --     对应 market_symbol=NY。
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET market = 'NY', region = 'US', gmt_update = now()
     WHERE u.usc = 'NOW' AND (u.market IS DISTINCT FROM 'NY' OR u.region IS DISTINCT FROM 'US');
    GET DIAGNOSTICS v_tmp = ROW_COUNT;
    v_by_list := v_by_list + v_tmp;

    -- 4.3 SH0680 名称纠正为证券主数据口径（原值是把 usc 当名字）。
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET name = s.security_name_cn, gmt_update = now()
      FROM public.finv_security s
     WHERE u.usc = 'SH0680'
       AND btrim(s.security_name_cn) <> ''
       AND u.name IS DISTINCT FROM s.security_name_cn;

    -- 4.4 GDRnext：富途 symbol 小写形态，元数据与 GDRNext 同源。
    --     需求 10943 的补丁规则会把行情迁到 GDRNext；本行只保证迁移前
    --     页面与导出文件名不再是 UNKNOWN_UNKNOWN。day_expect_rows 留空，
    --     由 HK/HK 模板（331）兜底，不在此处编造。
    INSERT INTO cdeu.finv_quote_cdeu_usc
        (usc, category, date_list, flag_auto_discover, region, market, provider,
         name, timezone, currency)
    VALUES ('GDRnext', 'Futures', DATE '2000-01-01', false, 'HK', 'HK', 'FTMM',
            '人民币黄金下月期货', 'Asia/Hong_Kong', 'HKD')
    ON CONFLICT (usc) DO UPDATE
       SET region = EXCLUDED.region, market = EXCLUDED.market, gmt_update = now()
     WHERE cdeu.finv_quote_cdeu_usc.region IS DISTINCT FROM EXCLUDED.region
        OR cdeu.finv_quote_cdeu_usc.market IS DISTINCT FROM EXCLUDED.market;
    GET DIAGNOSTICS v_tmp = ROW_COUNT;
    v_by_list := v_by_list + v_tmp;

    -- -----------------------------------------------------------------
    -- 5. 校验
    -- -----------------------------------------------------------------
    -- 5.1 期货 16 条必须落到预期 market_symbol
    SELECT string_agg(u.usc || '=' || COALESCE(u.region, '') || '/' || COALESCE(u.market, ''),
                      ', ' ORDER BY u.usc)
      INTO v_bad
      FROM cdeu.finv_quote_cdeu_usc u
      JOIN (VALUES
                ('1OZMain', 'US/COMEX'), ('GCCurrent', 'US/COMEX'), ('GCMain', 'US/COMEX'),
                ('MGCMain', 'US/COMEX'), ('QOCurrent', 'US/COMEX'), ('QOMain', 'US/COMEX'),
                ('SGUMain', 'US/COMEX'), ('SICurrent', 'US/COMEX'), ('SIMain', 'US/COMEX'),
                ('ESMain', 'US/CME'), ('NQMain', 'US/CME'), ('YMMain', 'US/CBOT'),
                ('BZMain', 'US/NYMEX'), ('CLMain', 'US/NYMEX'), ('QMMain', 'US/NYMEX'),
                ('VXMain', 'US/CBOE')
           ) AS v(usc, want) ON v.usc = u.usc
     WHERE COALESCE(u.region, '') || '/' || COALESCE(u.market, '') IS DISTINCT FROM v.want;
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION '期货 region/market 未按预期对齐：%', v_bad;
    END IF;

    -- 5.2 输出仍然不是合法 market_symbol 的残留（人工待办，不阻断）
    SELECT string_agg(u.usc || '(' || COALESCE(u.region, '') || '/' || COALESCE(u.market, '') || ')',
                      ', ' ORDER BY u.usc)
      INTO v_bad
      FROM cdeu.finv_quote_cdeu_usc u
     WHERE u.market IS NULL OR btrim(u.market) = ''
        OR NOT EXISTS (SELECT 1 FROM public.finv_market x WHERE x.market_symbol = u.market);

    RAISE NOTICE '完成：期货 market_code 修正 % 行；按主数据纠正 % 行，按 market_flag 映射 % 行，显式清单 % 行',
        v_fix_code, v_by_dict, v_by_flag, v_by_list;
    IF v_bad IS NOT NULL THEN
        RAISE NOTICE '仍有非 market_symbol 口径的证券（人工待办）：%', v_bad;
    ELSE
        RAISE NOTICE '登记表已无非 market_symbol 口径的证券';
    END IF;
END $$;

COMMIT;

-- 执行后核对
-- 1) 字典新行：
-- SELECT market_code, market_flag, market_name, market_symbol, exchange_code, flag_enable
--   FROM public.finv_market WHERE market_code = 1323;
-- 2) 期货对齐：
-- SELECT usc, region, market FROM cdeu.finv_quote_cdeu_usc
--  WHERE usc IN ('1OZMain','GCMain','ESMain','NQMain','YMMain','CLMain','BZMain','QMMain','VXMain')
--  ORDER BY usc;
-- 3) 不应再出现 flag 口径：
-- SELECT count(*) FROM cdeu.finv_quote_cdeu_usc u
--  WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_symbol = u.market);
-- 4) 重点证券：
-- SELECT usc, region, market FROM cdeu.finv_quote_cdeu_usc
--  WHERE usc IN ('BTC','BCHUSDC','EURUSD','XAUUSD','USDIndex','SH0008','SH0980','688050',
--                '000510','BK1191','HTI','01024','AIQ','BOTZ','CHAT','KSTR','NOW','GDRnext')
--  ORDER BY usc;
