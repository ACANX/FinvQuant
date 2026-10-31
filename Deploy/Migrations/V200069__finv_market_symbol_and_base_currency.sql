-- =====================================================================
-- FinvQuant PostgreSQL V200069：补齐 finv_market 的 market_symbol 与 base_currency
--
-- 背景：
--   · V100075 引入 market_symbol（市场对外标识，与行情链路 market 取值对齐，
--     需求 10606 的 dedup.usc.market 优先取该列、为空时回退 market_flag），
--     但 V200015/V200017/V200018 三批字典种子以「暂空串，待按市场补充」入库，
--     导致部分有证券数据的市场一直回退成 N/A_xxxx 这类占位标识；
--   · V100003 的 base_currency（基础计价货币）同样在种子阶段留空。
--
-- 取值依据（不编造）：
--   · market_symbol 沿用既有约定「交易所/区域的对外标识」，如 SH / SZ / HK / US /
--     CER / CFD / CIX / NSDQ / USOTC / PINK / TK / KR / SGX / FTC：
--     - 10011 / 10013 / 10014：加密现货，与 10001（CRYPTO_10001，symbol=FTC）同一
--       对外市场标识，故同取 FTC；
--     - 8000：板块/行业分类（其下 usc 全为 LIST* 板块代码），取 SECTOR；
--     - 2001 债券取 BD、2003 国债收益率取 BDY（两者 market_abbr 同为 BD，需可区分）、
--     - 3001 基金（主）取 FD（沿用 market_abbr）；
--     - 1521 印度 SENSEX 指数取 IN（与 1508 韩国 KOSPI 用 KR 同风格）。
--   · base_currency 只按「该市场当前所辖证券的计价货币唯一」时补值：
--     - 1501→JPY、1508→USD、1511→SGD、1521→USD、2001→HKD、3001→USD、
--       10001/10011/10013/10014→USD；
--     - 跨币种市场保持空串（1251 FX 含 45 种货币、2003 含 CNY+USD、8000 板块含
--       AUD/CNY/HKD/USD）：一个市场只有一列基础计价货币，跨币种市场按证券层
--       finv_security.currency_type 区分，不拆市场、不在 market_symbol 上叠加货币。
--
-- 幂等策略：UPDATE 带 IS DISTINCT FROM 条件，已改过的库上重跑影响 0 行；
--           末尾收敛校验，清单内取值不符则整事务回滚。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V100075（market_symbol 列）、V100020（exchange_code 列）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. market_symbol：有证券数据但符号未维护的市场
-- ---------------------------------------------------------------------
UPDATE public.finv_market AS m
   SET market_symbol = v.market_symbol
  FROM (VALUES
    (10011, 'FTC'),
    (10013, 'FTC'),
    (10014, 'FTC'),
    (8000,  'SECTOR'),
    (2001,  'BD'),
    (2003,  'BDY'),
    (3001,  'FD'),
    (1521,  'IN')
  ) AS v(market_code, market_symbol)
 WHERE m.market_code = v.market_code
   AND m.market_symbol IS DISTINCT FROM v.market_symbol;

-- ---------------------------------------------------------------------
-- 2. base_currency：所辖证券计价货币唯一的市场
-- ---------------------------------------------------------------------
UPDATE public.finv_market AS m
   SET base_currency = v.base_currency
  FROM (VALUES
    (1501,  'JPY'),
    (1508,  'USD'),
    (1511,  'SGD'),
    (1521,  'USD'),
    (2001,  'HKD'),
    (3001,  'USD'),
    (10001, 'USD'),
    (10011, 'USD'),
    (10013, 'USD'),
    (10014, 'USD')
  ) AS v(market_code, base_currency)
 WHERE m.market_code = v.market_code
   AND m.base_currency IS DISTINCT FROM v.base_currency;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad int;
BEGIN
    SELECT count(*) INTO v_bad
      FROM (VALUES
        (10011, 'FTC'), (10013, 'FTC'), (10014, 'FTC'), (8000, 'SECTOR'),
        (2001, 'BD'), (2003, 'BDY'), (3001, 'FD'), (1521, 'IN')
      ) AS v(market_code, market_symbol)
      JOIN public.finv_market m ON m.market_code = v.market_code
     WHERE m.market_symbol IS DISTINCT FROM v.market_symbol;
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V200069 收敛失败：% 个市场的 market_symbol 与清单不一致', v_bad;
    END IF;

    SELECT count(*) INTO v_bad
      FROM (VALUES
        (1501, 'JPY'), (1508, 'USD'), (1511, 'SGD'), (1521, 'USD'),
        (2001, 'HKD'), (3001, 'USD'), (10001, 'USD'), (10011, 'USD'),
        (10013, 'USD'), (10014, 'USD')
      ) AS v(market_code, base_currency)
      JOIN public.finv_market m ON m.market_code = v.market_code
     WHERE m.base_currency IS DISTINCT FROM v.base_currency;
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V200069 收敛失败：% 个市场的 base_currency 与清单不一致', v_bad;
    END IF;

    RAISE NOTICE 'V200069 完成：8 个市场补 market_symbol，10 个市场补 base_currency（跨币种市场保持空串）';
END $$;

COMMIT;
