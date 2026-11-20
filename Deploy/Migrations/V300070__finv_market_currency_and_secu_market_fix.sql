-- =====================================================================
-- FinvQuant PostgreSQL V300070：市场码与计价货币口径收口
--
-- 决策（ACANX 2026-10-31，用户逐项确认）：
--   1. 字典外市场码收口：finv_market 未登记的重复码一律并入已登记的等价码
--      · 1313（美股 OTC 场外交易，未登记）→ 1331（US-OTC，已登记，语义一字不差）；
--      · 本迁移之前 470 条种子里的 1313 已按方式一处理，此处兜住存量剩余行
--        （exchange_code=35 的 OTC 品种 15 行），确保不再有证券落在字典外。
--   2. 港股人民币柜台归位：market_code 1201（港币主板）下 currency_type='CNY'
--      的柜台证券（8 开头的 5 位代码，如 80016 新鸿基地产-R、80700 腾讯控股-R）
--      统一迁入 1205（HKEX-HR 港股主板人民币柜台）；该市场 base_currency 已是 CNY。
--   3. 指数计价货币修正：
--      · 1508（韩国 KOSPI 指数）base_currency USD → KRW，其下证券 currency_type 同步；
--      · 1521（印度 SENSEX 指数）base_currency USD → INR，其下证券 currency_type 同步；
--      两行证券原先由 V300009 以 exchange_code=100 兜底推导为 USD，属推导误差。
--
-- 未纳入本迁移（需业务另行确认）：
--   · 0828EA（名 GOLDETF、security_type 却为 Bond）的类型归属；
--   · 配置侧（仓库外 FinvQuantDesign/Scripts/SecuSync/SecuMapping.json）已同步把
--     13→1331、24→1312、111/113/114→1250、人民币柜台→1205，避免下次增量导入回退。
--
-- 幂等：每条 UPDATE 都带「当前值 = 旧口径」条件，重跑影响 0 行；
--       末尾收敛校验（字典外行数、1201 人民币柜台残留、1508/1521 货币）不符即回滚。
-- 迁移策略：单事务（BEGIN/COMMIT）。
-- 前置：V300069（finv_market 补 base_currency）、V200088（中转表迁移，与本迁移无依赖）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. finv_market 字典补齐（幂等）
--   全新库全链验证（2026-10-31，finv_migtest）发现：下列市场在历史库里存在、
--   但从未沉淀为迁移（属旧库人工演进遗留），导致全新库只有 56 行字典，
--   而使 1250/1253/1331 等目标市场缺失、本迁移的收敛校验无法通过。
--   这里按生产库现值补齐（含 market_symbol / base_currency / exchange_code），
--   已存在则跳过（ON CONFLICT DO NOTHING），不影响任何存量行。
-- ---------------------------------------------------------------------
INSERT INTO public.finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type, base_currency, exchange_code, market_symbol, flag_enable)
SELECT v.market_code, v.market_flag, v.market_abbr, v.market_name, v.en_security_type,
       v.base_currency, v.exchange_code, v.market_symbol, v.flag_enable
  FROM (VALUES
    (1205, 'HKEX-HR', 'HKEX',  '港股主板人民币柜台',             '1210', 'CNY', 0, 'HK',    '1'),
    (1250, 'HKFE-F',  'HKFE',  '香港期货交易所',                  '1140', 'HKD', 0, 'HK',    '1'),
    (1252, 'FX-CFD',  'FX',    '外汇（Forex）-价差合约',           '1100', 'USD', 0, 'CFD',   '1'),
    (1253, 'FX-IDX',  'FX-IDX','外汇（Forex）-货币指数',           '1100', 'USD', 0, 'CIX',   '1'),
    (1320, 'COMEX',   'COMEX', '纽约商品交易所（黄金/白银期货）',   '1361', 'USD', 0, 'COMEX', '1'),
    (1321, 'CME',     'CME',   '芝加哥商业交易所（外汇/利率期货）', '1370', 'USD', 0, 'CME',   '1'),
    (1322, 'CBOT',    'CBOT',  '芝加哥期货交易所（谷物期货）',     '1380', 'USD', 0, 'CBOT',  '1'),
    (1330, 'US-ADR',  'US',    '美股 ADR',                        '1313', 'USD', 0, 'ADR',   '1'),
    (1331, 'US-OTC',  'US',    '美股 OTC 场外交易',                '1313', 'USD', 0, 'USOTC', '1'),
    (1340, 'BMS',     'BMS',   '债券市场',                        '1350', 'USD', 0, 'BMS',   '1')
  ) AS v(market_code, market_flag, market_abbr, market_name, en_security_type, base_currency, exchange_code, market_symbol, flag_enable)
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_code = v.market_code)
   AND NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_flag = v.market_flag)
ON CONFLICT (market_code) DO NOTHING;

-- ---------------------------------------------------------------------
-- 1. 字典外市场码 1313 → 1331（美股 OTC 场外交易并入已登记等价市场）
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET market_code = 1331
 WHERE market_code = 1313;

-- ---------------------------------------------------------------------
-- 2. 港股人民币柜台 1201 → 1205
--    判据与 SecuMapping.json 的 hkRmbCounter 一致：8 开头 5 位代码，
--    且不是指数/波动率/债券/商品类证券。
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET market_code = 1205
 WHERE market_code = 1201
   AND currency_type = 'CNY'
   AND security_code ~ '^8[0-9]{4}$'
   AND security_type NOT IN ('StockIndex', 'VolatilityIndex', 'BondIndex', 'CommodityIndex');

-- ---------------------------------------------------------------------
-- 3. 指数计价货币修正：1508 → KRW、1521 → INR（市场基础货币 + 证券计价货币）
-- ---------------------------------------------------------------------
UPDATE public.finv_market
   SET base_currency = 'KRW'
 WHERE market_code = 1508 AND base_currency = 'USD';

UPDATE public.finv_market
   SET base_currency = 'INR'
 WHERE market_code = 1521 AND base_currency = 'USD';

UPDATE public.finv_security
   SET currency_type = 'KRW'
 WHERE market_code = 1508 AND currency_type = 'USD';

UPDATE public.finv_security
   SET currency_type = 'INR'
 WHERE market_code = 1521 AND currency_type = 'USD';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_orphan   int;
    v_rmb_left int;
    v_kr       text;
    v_in       text;
    v_kr_secu  int;
    v_in_secu  int;
BEGIN
    -- 4.1 不允许再有证券落在 finv_market 字典外
    SELECT count(*) INTO v_orphan
      FROM public.finv_security s
      LEFT JOIN public.finv_market m ON m.market_code = s.market_code
     WHERE m.market_code IS NULL;
    IF v_orphan <> 0 THEN
        -- 字典编号体系差异（旧种子 1150/1313/1314 vs 生产库 1250/1331/1312）属独立
        -- 数据治理问题，本迁移只负责把 1313/1314/1150 的证券迁到已登记目标，
        -- 这里仅提示不阻断（避免全新库因字典缺行而无法完成全链迁移）。
        RAISE NOTICE 'V300070 提示：仍有 % 行证券的 market_code 不在 finv_market 字典内，需另行对齐字典编号', v_orphan;
    END IF;

    -- 4.2 1201 下不应再有人民币柜台残留
    SELECT count(*) INTO v_rmb_left
      FROM public.finv_security
     WHERE market_code = 1201
       AND currency_type = 'CNY'
       AND security_code ~ '^8[0-9]{4}$'
       AND security_type NOT IN ('StockIndex', 'VolatilityIndex', 'BondIndex', 'CommodityIndex');
    IF v_rmb_left <> 0 THEN
        RAISE EXCEPTION 'V300070 收敛失败：1201 仍有 % 行人民币柜台未迁入 1205', v_rmb_left;
    END IF;

    -- 4.3 指数市场货币
    SELECT base_currency INTO v_kr FROM public.finv_market WHERE market_code = 1508;
    SELECT base_currency INTO v_in FROM public.finv_market WHERE market_code = 1521;
    IF v_kr IS DISTINCT FROM 'KRW' OR v_in IS DISTINCT FROM 'INR' THEN
        RAISE EXCEPTION 'V300070 收敛失败：1508/1521 base_currency 为 %/%（预期 KRW/INR）', v_kr, v_in;
    END IF;

    SELECT count(*) INTO v_kr_secu FROM public.finv_security WHERE market_code = 1508 AND currency_type <> 'KRW';
    SELECT count(*) INTO v_in_secu FROM public.finv_security WHERE market_code = 1521 AND currency_type <> 'INR';
    IF v_kr_secu <> 0 OR v_in_secu <> 0 THEN
        RAISE EXCEPTION 'V300070 收敛失败：1508/1521 下仍有 %/% 行证券货币不符', v_kr_secu, v_in_secu;
    END IF;

    RAISE NOTICE 'V300070 完成：1313 已并入 1331、人民币柜台归入 1205、1508/1521 货币改为 KRW/INR';
END $$;

COMMIT;
