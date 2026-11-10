-- =====================================================================
-- V300156：修正 V300155 对 US.HSTI 的误改（需求 11071 修正）
--
-- 背景：V300155 首版把 `futu.finv_futu_secu_key` 中「usc='HSTI'」的行无条件改成 'HTI'，
--   但那一行其实是 **US.HSTI（美股，code='HSTI'）**，按 USC 规则 L0「US 市场默认 usc=code」
--   它的 usc 就应该是 'HSTI'，与港股恒生科技指数（HK.800700 → HTI）无关。
--   上游 public.finv_quote_secu_key 中 US.HSTI 的 usc 也是 'HSTI'（2026-11-10 实测），
--   因此本地必须恢复为 'HSTI'。
--
-- 说明：V300155 已同步收口为「只改 market='HK'」，新装库不会再出现此问题；
--   本迁移只负责把已受影响的存量库改回来。
--
-- 幂等：UPDATE 命中 0 行即已恢复；末尾校验两条 usc 各自唯一存在。
-- =====================================================================

BEGIN;

UPDATE futu.finv_futu_secu_key
   SET usc = 'HSTI'
 WHERE ftmm_symbol = 'US.HSTI'
   AND market = 'US'
   AND usc IS DISTINCT FROM 'HSTI';

DO $$
DECLARE
    v_us  bigint;
    v_hk  bigint;
BEGIN
    SELECT count(*) INTO v_us FROM futu.finv_futu_secu_key
     WHERE ftmm_symbol = 'US.HSTI' AND usc = 'HSTI';
    SELECT count(*) INTO v_hk FROM futu.finv_futu_secu_key
     WHERE ftmm_symbol = 'HK.800700' AND usc = 'HTI';
    -- 全新库（首次部署）键表为空：本迁移是存量数据修复，没有数据就跳过校验，
    -- 否则会让首次部署的迁移链中断（键表数据由运行时从上游同步写入）。
    IF NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key LIMIT 1) THEN
        RAISE NOTICE 'V300156：键表为空（全新库），跳过存量数据校验';
        RETURN;
    END IF;

    IF v_us <> 1 OR v_hk <> 1 THEN
        RAISE EXCEPTION 'V300156 校验失败：应为 US.HSTI→HSTI 1 行、HK.800700→HTI 1 行，实际 % / %', v_us, v_hk;
    END IF;
    RAISE NOTICE 'V300156：US.HSTI 的 usc 已恢复为 HSTI（HK.800700 保持 HTI）';
END $$;

COMMIT;
