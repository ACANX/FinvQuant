-- =====================================================================
-- FinvQuant PostgreSQL V100036：finv_dedup_usc 的 region/market 按字典重新绑定
--
-- 背景（ACANX 2026-10-26）：
--   行情归档路径与 MVsv 文件名使用 {region}_{market}，但 dedup.finv_dedup_usc
--   里的两列此前口径不符合预期：
--     1) region 取自 finv_exchange.region 原文，历史上存在 USA（字典已改为 US）、
--        以及不在 finv_region 字典里的 N/A、FX；
--     2) market 取自 finv_market.market_flag（板块级标识，如 SSE-A / US-NSDQ），
--        而归档目录需要的是市场级符号——现由新增的 finv_market.market_symbol
--        （SH / SZ / BJ / HK / NSDQ / CFD …）表达。
--
-- 本次调整：
--   - 先把 finv_exchange.region 的 'USA' 统一成字典值 'US'（幂等）。
--   - 按 RefreshSecurityMetadata 的新口径重算 dedup.finv_dedup_usc：
--       region = COALESCE(finv_region.region, exchange.region)   -- 字典优先
--       market = COALESCE(NULLIF(market_symbol,''), market_flag, exchange_flag)
--     只更新 region/market 实际发生变化的行，不触碰 category/name/timezone 等
--     人工维护列。
--   - 说明：exchange.region 为 'N/A'、'FX' 的行在 finv_region 中没有对应项，
--     按「字典无项时回退原值」策略保留，不擅自映射（待业务确认后另行迁移）。
--   - 幂等：UPDATE 带值差异判断，重复执行影响 0 行；末尾校验无 USA 残留且
--     market 与 market_symbol 对齐。
-- 迁移策略：单事务、失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 交易所 region 与字典对齐：USA -> US
-- ---------------------------------------------------------------------
UPDATE public.finv_exchange
   SET region = 'US'
 WHERE region = 'USA';

-- ---------------------------------------------------------------------
-- 2. 按新口径重算 dedup.finv_dedup_usc 的 region / market
-- ---------------------------------------------------------------------
WITH computed AS (
    SELECT s.usc,
           COALESCE(r.region, NULLIF(e.region, '')) AS region,
           COALESCE(NULLIF(m.market_symbol, ''),
                    NULLIF(m.market_flag, ''),
                    NULLIF(e.exchange_flag, ''))    AS market
      FROM public.finv_security s
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_market m   ON m.market_code   = s.market_code
      LEFT JOIN public.finv_region r   ON r.region        = e.region
     WHERE COALESCE(s.flag_enable, '1') = '1'
)
UPDATE dedup.finv_dedup_usc u
   SET region = COALESCE(c.region, u.region),
       market = COALESCE(c.market, u.market),
       gmt_update = now()
  FROM computed c
 WHERE c.usc = u.usc
   AND (u.region IS DISTINCT FROM COALESCE(c.region, u.region)
     OR u.market IS DISTINCT FROM COALESCE(c.market, u.market));

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad   INT;
    v_rows  INT;
BEGIN
    -- 3.1 交易所侧不再有 USA
    SELECT count(*) INTO v_bad FROM public.finv_exchange WHERE region = 'USA';
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100036: finv_exchange 仍有 % 行 region=USA', v_bad;
    END IF;

    -- 3.2 证券主数据不再有 USA
    SELECT count(*) INTO v_bad FROM dedup.finv_dedup_usc WHERE region = 'USA';
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100036: finv_dedup_usc 仍有 % 行 region=USA', v_bad;
    END IF;

    -- 3.3 market 已与 market_symbol 对齐（仅校验 symbol 非空的证券）
    SELECT count(*) INTO v_bad
      FROM dedup.finv_dedup_usc u
      JOIN public.finv_security s ON s.usc = u.usc
      JOIN public.finv_market   m ON m.market_code = s.market_code
     WHERE NULLIF(m.market_symbol, '') IS NOT NULL
       AND COALESCE(u.market, '') <> m.market_symbol;
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100036: % 行的 market 未与 finv_market.market_symbol 对齐', v_bad;
    END IF;

    -- 3.4 region/market 均不应为空（原值非空 + 新值为空时保留旧值）
    SELECT count(*) INTO v_bad
      FROM dedup.finv_dedup_usc
     WHERE NULLIF(region, '') IS NULL OR NULLIF(market, '') IS NULL;
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100036: % 行的 region/market 为空', v_bad;
    END IF;

    SELECT count(*) INTO v_rows FROM dedup.finv_dedup_usc;
    RAISE NOTICE 'V100036: 证券主数据重绑完成（共 % 行；region 按字典规范化，market 取 market_symbol）', v_rows;
END $$;

COMMIT;
