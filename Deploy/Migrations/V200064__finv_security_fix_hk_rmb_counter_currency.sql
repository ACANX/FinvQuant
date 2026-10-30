-- =====================================================================
-- V200064：finv_security 港股人民币柜台 currency_type 修正
--
-- 需求 10817：港交所 8 开头 5 位代码为人民币柜台（名称带 -R / -WR，如 80700 腾讯控股-R、
--   81810 小米集团-WR），计价货币应为 CNY，不能按市场默认写 HKD；
--   6 位 800100 一类港元计价指数（国企指数、恒指波幅指数）保持 HKD。
--
-- 背景：V200009 的历史种子把 81810 写成 HKD，V200062 因 ON CONFLICT DO NOTHING
--   不会覆盖这些已有行，故此迁移在数据发放之后统一收敛，保证全新库与生产库一致。
--
-- 幂等：条件含 currency_type <> 'CNY'，重复执行影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾收敛校验，不满足则整批回滚。
-- 前置：V100082（finv_security 增加 type_secu_sub / provider / remark）已应用。
-- 规则 hkRmbCounterToCny：CNY（港股人民币柜台（港交所 8 开头 5 位代码，名称带 -R / -WR）计价货币改为 CNY；6 位 8 开头指数不适用）
-- 生成工具：FinvQuantDesign/Scripts/SecuSync/FixSecuCurrency.py（2026-10-30 03:30:11）
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 前置校验：currency_type 列必须存在
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = 'public'
           AND table_name   = 'finv_security'
           AND column_name  = 'currency_type'
    ) THEN
        RAISE EXCEPTION '缺少列 public.finv_security.currency_type';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 1. 港股人民币柜台（港交所 8 开头 5 位代码，名称带 -R / -WR）计价货币改为 CNY；6 位 8 开头指数不适用
--    快照侧命中 26 条：
--      80016(新鸿基地产-R), 80020(商汤-WR), 80175(吉利汽车-R), 80291(华润啤酒-R), 80388(香港交易所-R), 80700(腾讯控股-R), 80737(湾区发展-R), 80883(中国海洋石油-R)
--      80941(中国移动-R), 80992(联想集团-R), 81024(快手-WR), 81211(比亚迪股-R一百), 81299(友邦保险-R), 81810(小米集团-WR), 82020(安踏体育-R), 82318(中国平安-R)
--      82331(李宁-R), 82333(长城汽车-R), 82388(中银香港-R), 82840(SPDR金-R), 83168(恒生人币金ETF), 83690(美团-WR), 86618(京东健康-R), 89618(京东集团-SWR)
--      89888(百度集团-SWR), 89988(阿里巴巴-WR)
--
--    变更前留证：psql 会打印即将受影响的行（需留档时用 -o 重定向到文件）
-- ---------------------------------------------------------------------
SELECT usc, security_code, security_type, currency_type
  FROM public.finv_security
 WHERE exchange_code = 21 AND security_code ~ '^8[0-9]{4}$' AND security_type NOT IN ('StockIndex', 'VolatilityIndex', 'BondIndex', 'CommodityIndex') AND currency_type <> 'CNY'
 ORDER BY security_code;

UPDATE public.finv_security
   SET currency_type = 'CNY'
 WHERE exchange_code = 21 AND security_code ~ '^8[0-9]{4}$' AND security_type NOT IN ('StockIndex', 'VolatilityIndex', 'BondIndex', 'CommodityIndex') AND currency_type <> 'CNY';

-- ---------------------------------------------------------------------
-- 收敛校验：规则条件下不得再存在非目标货币，否则整批回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rule RECORD;
    v_cnt  BIGINT;
    v_all  BIGINT;
BEGIN
    -- hkRmbCounterToCny
    SELECT count(*), count(*) FILTER (WHERE currency_type <> 'CNY') INTO v_all, v_cnt
      FROM public.finv_security WHERE exchange_code = 21 AND security_code ~ '^8[0-9]{4}$' AND security_type NOT IN ('StockIndex', 'VolatilityIndex', 'BondIndex', 'CommodityIndex');
    IF v_cnt > 0 THEN
        RAISE EXCEPTION 'hkRmbCounterToCny 收敛失败：仍有 % 条非 CNY 记录', v_cnt;
    END IF;
    RAISE NOTICE 'hkRmbCounterToCny：命中范围内共 % 条，currency_type 已统一为 CNY', v_all;
END $$;

COMMIT;
