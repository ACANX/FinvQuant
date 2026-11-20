-- =====================================================================
-- FinvQuant PostgreSQL V300136：5 个证券的 usc 更正与初始数据发放（需求 11035）
--
-- 背景（用户指定 usc）：
--   以下 5 个 stock_id 在 cdeu 快照里的 usc 与既有证券撞车或用错，用户明确指定改用新 usc：
--
--   | stock_id       | 旧 usc  | 新 usc  | 标的                                   | 撞车/错因 |
--   |----------------|---------|---------|----------------------------------------|-----------|
--   | 80199924522296 | BFLY    | BFLYN   | Butterfly Network（美股股票，富途码10） | `BFLY` 已被 Cboe S&P 500铁蝴蝶指数占用 |
--   | 85070417466455 | DJT     | DJTG    | 特朗普媒体科技集团（美股股票，富途码11）| `DJT` 已被道琼斯交通运输平均指数占用 |
--   | 85611583331825 | USBTC   | BTCMT   | Grayscale Bitcoin Mini Trust（ETF，码12）| `USBTC` 是上批按 usc 生成的占位代码 |
--   | 85577223574361 | USETH   | LETH    | Grayscale Ethereum Mini Trust（ETF，码12）| `USETH` 同上 |
--   | 87660282761853 | XRP     | ETFXRP  | Bitwise XRP ETF（ETF，码12）             | 已在 V300135 完成（本迁移幂等跳过） |
--
-- 本迁移做四件事（全部幂等）：
--   1. `cdeu.finv_cdeu_secu_realtime_stock`：按 stock_id 把 usc 更正为新值；
--   2. `public.finv_security` / `futu.finv_futu_quote_collect`：把上批生成的别名行
--      （`USBTC` / `USETH`）**改名为新 usc**，避免出现两条同标的记录；
--   3. 两表按新 usc 补齐缺失行（`BFLYN` / `DJTG` 属首次发放）；
--   4. `security_code` 口径：
--      · `BTCMT` / `LETH`：修正为上批遗留的占位值（`USBTC`/`USETH`）→ 交易所原始代码 `BTC` / `ETH`；
--      · `BFLYN` / `DJTG`：交易所原始代码 `BFLY` / `DJT` 已被同名指数占用（`(exchange_code, security_code)`
--        唯一约束），故 `security_code` 直接用新 usc，与既有「usc 去撞车」口径一致；
--      · `ETFXRP`：`security_code='XRP'`（V300135 已建，本迁移不动）。
--
-- 市场码口径沿用方案 A（交易所专码）：股票 富途码 10→1316(NYSE) / 11→1315(NSDAQ)；
--   ETF 的 `finv_security.market_code` 置 0（未维护），`futu` 侧保留富途原码 12。
-- 安全：新 usc 若已被**其它**证券占用（不匹配本清单）则报错回滚；所有写入带 `WHERE NOT EXISTS`
--   与 `IS DISTINCT FROM`，重跑 0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾 NOTICE 输出 5 条结果。
-- 前置：V300132/V300133（417 个发放）、V300135（ETFXRP）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 清单
-- ---------------------------------------------------------------------
CREATE TEMP TABLE _usc_fix (
    stock_id             BIGINT PRIMARY KEY,
    old_usc              TEXT NOT NULL,
    new_usc              TEXT NOT NULL,
    security_code        TEXT NOT NULL,
    security_name        TEXT NOT NULL,
    security_name_cn     TEXT NOT NULL,
    security_type        TEXT NOT NULL,
    type_secu_sub        TEXT NOT NULL,
    security_market_code INTEGER NOT NULL,
    futu_symbol          TEXT NOT NULL,
    futu_market_code     INTEGER NOT NULL,
    instrument_type      INTEGER NOT NULL,
    sub_instrument_type  INTEGER NOT NULL,
    type_symbol          TEXT NOT NULL,
    note                 TEXT NOT NULL
) ON COMMIT DROP;

INSERT INTO _usc_fix VALUES
    (80199924522296, 'BFLY',  'BFLYN',  'BFLYN', 'Butterfly Network', 'Butterfly Network',
     'Stock', 'Stock.US', 1316, 'BFLY', 10, 3, 3002, 'stock', 'usc 撞车：BFLY 属 Cboe 铁蝴蝶指数'),
    (85070417466455, 'DJT',   'DJTG',   'DJTG',  'Trump Media & Technology', '特朗普媒体科技集团',
     'Stock', 'Stock.US', 1315, 'DJT',  11, 3, 3002, 'stock', 'usc 撞车：DJT 属道琼斯交通运输指数'),
    (85611583331825, 'USBTC', 'BTCMT',  'BTC',   'Grayscale Bitcoin Mini Trust', 'Grayscale Bitcoin Mini Trust',
     'ETF',   'ETF.US',      0, 'BTC', 12, 4, 4002, 'etfs',  '上批占位 usc=USBTC，改为用户指定 BTCMT'),
    (85577223574361, 'USETH', 'LETH',   'ETH',   'Grayscale Ethereum Mini Trust', '低成本以太坊ETF-Grayscale',
     'ETF',   'ETF.US',      0, 'ETH', 12, 4, 4002, 'etfs',  '上批占位 usc=USETH，改为用户指定 LETH'),
    (87660282761853, 'XRP',   'ETFXRP', 'XRP',   'Bitwise XRP ETF', 'Bitwise XRP ETF',
     'ETF',   'ETF.US',      0, 'XRP', 12, 4, 4002, 'etfs',  'usc 撞车：XRP 属加密币；V300135 已建');

-- ---------------------------------------------------------------------
-- 1. 前置校验：新 usc 若已存在，必须是本清单这只证券；否则拒绝
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad text;
BEGIN
    SELECT string_agg(f.new_usc, ',') INTO v_bad
      FROM _usc_fix f
     WHERE EXISTS (
            SELECT 1 FROM public.finv_security s
             WHERE s.usc = f.new_usc AND s.security_name <> f.security_name)
        OR EXISTS (
            SELECT 1 FROM futu.finv_futu_quote_collect q
             WHERE q.usc = f.new_usc AND q.futu_symbol <> f.futu_symbol);
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300136 新 usc 已被其它证券占用，拒绝执行：%', v_bad;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. cdeu 侧 usc 更正
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_cdeu_secu_realtime_stock c
   SET usc = f.new_usc
  FROM _usc_fix f
 WHERE c.stock_id = f.stock_id
   AND c.usc IS DISTINCT FROM f.new_usc;

-- ---------------------------------------------------------------------
-- 3. 上批别名行改名（USBTC→BTCMT、USETH→LETH）
--    关键：只认「同一标的」的行（`security_name` / `futu_symbol` 必须与清单一致），
--    否则会误改同名指数行（`BFLY` 属 Cboe 铁蝴蝶指数、`DJT` 属道琼斯交通运输指数、`XRP` 属加密币）。
-- ---------------------------------------------------------------------
UPDATE public.finv_security s
   SET usc = f.new_usc
  FROM _usc_fix f
 WHERE s.usc = f.old_usc
   AND s.security_name = f.security_name
   AND f.old_usc <> f.new_usc
   AND NOT EXISTS (SELECT 1 FROM public.finv_security x WHERE x.usc = f.new_usc);

UPDATE futu.finv_futu_quote_collect q
   SET usc = f.new_usc
  FROM _usc_fix f
 WHERE q.usc = f.old_usc
   AND q.futu_symbol = f.futu_symbol
   AND f.old_usc <> f.new_usc
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect x WHERE x.usc = f.new_usc);

-- 3.1 security_code 修正：上批把 usc 当成了源代码（USBTC / USETH），改回交易所真码（BTC / ETH）
UPDATE public.finv_security s
   SET security_code = f.security_code
  FROM _usc_fix f
 WHERE s.usc = f.new_usc
   AND s.security_code = f.old_usc
   AND f.old_usc <> f.security_code
   AND NOT EXISTS (
        SELECT 1 FROM public.finv_security x
         WHERE x.exchange_code = 30 AND x.security_code = f.security_code AND x.usc <> f.new_usc);

-- ---------------------------------------------------------------------
-- 4. futu.finv_futu_quote_collect 补齐
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_quote_collect
    (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
     sub_instrument_type, req_section, quote_market, type_symbol, flag_enable, remark)
SELECT f.new_usc, f.stock_id, f.futu_symbol, 2, f.futu_market_code, f.instrument_type,
       f.sub_instrument_type, NULL, 'US', f.type_symbol, '1', '20261109-线上缺失证券补齐-第三批'
  FROM _usc_fix f
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect q WHERE q.usc = f.new_usc)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect q2
                    WHERE q2.market_code = f.futu_market_code AND q2.futu_symbol = f.futu_symbol)
 ORDER BY f.new_usc
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 5. public.finv_security 补齐
-- ---------------------------------------------------------------------
INSERT INTO public.finv_security
    (usc, exchange_code, market_code, security_type, type_secu_sub, security_code,
     security_name, security_name_cn, security_name_full, currency_type, init_date,
     timezone, tz, flag_enable, provider, remark)
SELECT f.new_usc, 30, f.security_market_code, f.security_type, f.type_secu_sub, f.security_code,
       f.security_name, f.security_name_cn, f.security_name, 'USD', 20000000,
       '-04:00', 'America/New_York', '0', 'FTMM', '20261109-线上缺失证券补齐-第三批'
  FROM _usc_fix f
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = f.new_usc)
   AND NOT EXISTS (SELECT 1 FROM public.finv_security s2
                    WHERE s2.exchange_code = 30 AND s2.security_code = f.security_code)
 ORDER BY f.new_usc
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 6. 收敛输出
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT f.stock_id, f.new_usc, f.note,
                    (SELECT c.usc FROM cdeu.finv_cdeu_secu_realtime_stock c WHERE c.stock_id = f.stock_id) AS cdeu_usc,
                    (SELECT s.security_type FROM public.finv_security s WHERE s.usc = f.new_usc) AS secu_type,
                    (SELECT s.security_code FROM public.finv_security s WHERE s.usc = f.new_usc) AS secu_code,
                    (SELECT q.futu_symbol FROM futu.finv_futu_quote_collect q WHERE q.usc = f.new_usc) AS futu_symbol,
                    (SELECT q.market_code FROM futu.finv_futu_quote_collect q WHERE q.usc = f.new_usc) AS futu_mc
               FROM _usc_fix f ORDER BY f.new_usc
    LOOP
        RAISE NOTICE 'V300136 % → usc=% | cdeu=% | finv_security=%/% | futu=%/mc% | %',
            r.stock_id, r.new_usc, COALESCE(r.cdeu_usc, '<无>'),
            COALESCE(r.secu_type, '<缺失>'), COALESCE(r.secu_code, '-'),
            COALESCE(r.futu_symbol, '<缺失>'), r.futu_mc, r.note;
    END LOOP;
END $$;

COMMIT;
