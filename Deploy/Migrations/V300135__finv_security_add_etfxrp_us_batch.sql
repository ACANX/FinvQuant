-- =====================================================================
-- FinvQuant PostgreSQL V300135：为 Bitwise XRP ETF 分配独立 usc = ETFXRP（需求 11034）
--
-- 背景（用户决策）：
--   `stock_id = 87660282761853` 是美股 ETF「Bitwise XRP ETF」（交易所代码 `XRP`、富途码 12），
--   其 `usc` 被 cdeu 批次映射成了 `XRP`，与既有加密币 `XRP`（`market_code=10001`，需求 10946）
--   撞车 —— `usc` 是全局主键，同名的两个标的不可能共存于 finv_security。
--   用户明确指定：**XRP ETF 的 usc 改用 `ETFXRP`**（`ETF` + `XRP`），加密币侧 `XRP` 不动。
--
-- 本迁移做三件事（全部幂等）：
--   1. `cdeu.finv_cdeu_secu_realtime_stock`：把该 stock_id 的 `usc` 由 `XRP` 更正为 `ETFXRP`，
--      避免 cdeu 侧继续指向加密币的 usc；
--   2. `futu.finv_futu_quote_collect`：插入 `usc='ETFXRP'` 行（富途侧原码：`futu_symbol='XRP'`、
--      `market_code=12`、`instrument_type=4`、`sub_instrument_type=4002`、`type_symbol='etfs'`）；
--   3. `public.finv_security`：插入 `usc='ETFXRP'` 行（`security_code='XRP'` 保留交易所原始代码，
--      `security_type='ETF'`、`type_secu_sub='ETF.US'`、`currency_type='USD'`、
--      `market_code=0`（未维护，方案 A 对 ETF 不猜交易所码）、`flag_enable='0'` 待人工启用）。
--
-- 数据依据：上游 Supabase `finv_quote_secu_meta` 中 `stock_id=87660282761853` 的记录
--   （`ftmm_symbol='US.XRP'`、`market_code=12`、`instrument_type=4`、`sub_instrument_type=4002`、
--   `name=name_en='Bitwise XRP ETF'`）。
--
-- 安全：
--   · 若 `usc='ETFXRP'` 已被**其它**证券占用（`security_code <> 'XRP'`），直接报错回滚，不猜不覆盖；
--   · 三个写入点都是 `WHERE NOT EXISTS` + `ON CONFLICT DO NOTHING`，重跑影响 0 行；
--   · 加密币 `XRP` 的任何记录都不动（`usc='XRP'` 行仅做存在性校验，不做修改）。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾 NOTICE 输出三处落库结果。
-- 前置：V300134（线上缺失 9 个的就地派生）、V200082（finv_security 扩列）、V200083（futu 表）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('cdeu.finv_cdeu_secu_realtime_stock') IS NULL THEN
        RAISE EXCEPTION 'V300135 缺少 cdeu.finv_cdeu_secu_realtime_stock';
    END IF;

    -- 加密币 XRP 必须仍在（本次只改 ETF 侧，不应影响它）
    IF NOT EXISTS (SELECT 1 FROM public.finv_security WHERE usc = 'XRP') THEN
        RAISE NOTICE 'V300135 提示：finv_security 中已无 usc=''XRP''（加密币行），本迁移仅处理 ETF 侧';
    END IF;

    -- ETFXRP 若已存在，必须就是这只 ETF（security_code='XRP'），否则拒绝执行
    IF EXISTS (SELECT 1 FROM public.finv_security WHERE usc = 'ETFXRP' AND security_code <> 'XRP') THEN
        RAISE EXCEPTION 'V300135 usc=''ETFXRP'' 已被其它证券占用（security_code<>XRP），拒绝执行';
    END IF;
    IF EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect WHERE usc = 'ETFXRP' AND futu_symbol <> 'XRP') THEN
        RAISE EXCEPTION 'V300135 futu 表 usc=''ETFXRP'' 已被其它证券占用（futu_symbol<>XRP），拒绝执行';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. cdeu 侧：把该 stock_id 的 usc 由 XRP 更正为 ETFXRP（幂等）
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_cdeu_secu_realtime_stock
   SET usc = 'ETFXRP'
 WHERE stock_id = 87660282761853
   AND usc IS DISTINCT FROM 'ETFXRP';

-- ---------------------------------------------------------------------
-- 3. futu.finv_futu_quote_collect：插入 ETFXRP 行
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_quote_collect
    (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
     sub_instrument_type, req_section, quote_market, type_symbol, flag_enable, remark)
SELECT 'ETFXRP', 87660282761853, 'XRP', 2, 12, 4, 4002, NULL, 'US', 'etfs', '1',
       '20261109-线上缺失证券补齐-第二批'
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect f WHERE f.usc = 'ETFXRP')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 4. public.finv_security：插入 ETFXRP 行
-- ---------------------------------------------------------------------
INSERT INTO public.finv_security
    (usc, exchange_code, market_code, security_type, type_secu_sub, security_code,
     security_name, security_name_cn, security_name_full, currency_type, init_date,
     timezone, tz, flag_enable, provider, remark)
SELECT 'ETFXRP', 30, 0, 'ETF', 'ETF.US', 'XRP',
       'Bitwise XRP ETF', 'Bitwise XRP ETF', 'Bitwise XRP ETF', 'USD', 20000000,
       '-04:00', 'America/New_York', '0', 'FTMM',
       '20261109-线上缺失证券补齐-第二批'
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = 'ETFXRP')
   AND NOT EXISTS (
        -- 同一交易所内原始代码已存在同一证券时不再插入（避免 (exchange_code, security_code) 冲突）
        SELECT 1 FROM public.finv_security s2
         WHERE s2.exchange_code = 30 AND s2.security_code = 'XRP'
   )
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 5. 收敛输出
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cdeu  text;
    v_futu  int;
    v_secu  int;
    v_xrp   int;
BEGIN
    SELECT usc INTO v_cdeu FROM cdeu.finv_cdeu_secu_realtime_stock
     WHERE stock_id = 87660282761853;
    SELECT count(*) INTO v_futu FROM futu.finv_futu_quote_collect WHERE usc = 'ETFXRP';
    SELECT count(*) INTO v_secu FROM public.finv_security WHERE usc = 'ETFXRP';
    SELECT count(*) INTO v_xrp  FROM public.finv_security WHERE usc = 'XRP';

    RAISE NOTICE 'V300135 cdeu.usc=%（期望 ETFXRP）', COALESCE(v_cdeu, '<无该 stock_id>');
    RAISE NOTICE 'V300135 futu_futu_quote_collect usc=ETFXRP 行数=%（期望 1）', v_futu;
    RAISE NOTICE 'V300135 finv_security usc=ETFXRP 行数=%（期望 1）', v_secu;
    RAISE NOTICE 'V300135 finv_security usc=XRP（加密币）行数=%（应为 1，未被改动）', v_xrp;
END $$;

COMMIT;
