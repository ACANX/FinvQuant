-- =====================================================================
-- V300230：期货快照表回填 usc（需求 11326）
--
-- 背景：cdeu.finv_quote_secu_realtime_future 的 usc 由采集侧按
--   stock_id → futu.finv_futu_quote_collect
--   派生键 → futu.finv_futu_secu_key
-- 两级解析；但期货合约几乎都不在键表（instrument='future' 的键表行为 0），
-- 导致 335 行里 100 行 usc 为 NULL（用户 2026-11-18 指出）。
--
-- 修复：用 usc 引擎结果表 futu.finv_futu_usc_map（status=ACTIVE）按
--   ftmm_symbol = upper(市场标签) || '.' || upper(合约代码)
-- 回填一次。实测 100/100 命中（如 SG.CNMAIN → CNMain、US.XAMD0MAIN → XAMD0Main、
-- JP.NK225MAIN → NK225Main）。
--
-- 幂等：只写 usc 仍为空的行；重跑影响 0 行。
-- 说明：仍解析不到的行保持 NULL（「尚未录入」信号），不用合约代码冒充 usc。
-- =====================================================================

BEGIN;

WITH src AS (
    SELECT t.stock_id, btrim(m.usc) AS usc
      FROM cdeu.finv_quote_secu_realtime_future t
      JOIN futu.finv_futu_usc_map m
        ON upper(btrim(m.ftmm_symbol)) = upper(btrim(t.market_label)) || '.' || upper(btrim(t.contract_code))
       AND m.status = 'ACTIVE'
     WHERE NULLIF(btrim(t.usc), '') IS NULL
       AND NULLIF(btrim(t.market_label), '') IS NOT NULL
       AND NULLIF(btrim(t.contract_code), '') IS NOT NULL
       AND NULLIF(btrim(m.usc), '') IS NOT NULL
)
UPDATE cdeu.finv_quote_secu_realtime_future t
   SET usc = src.usc
  FROM src
 WHERE t.stock_id = src.stock_id;

-- 校验：打印仍为空的合约数（保持 NULL 属正常，但本次应为 0）
DO $$
DECLARE
    v_empty int;
    v_total int;
BEGIN
    SELECT count(*) FILTER (WHERE NULLIF(btrim(usc), '') IS NULL), count(*)
      INTO v_empty, v_total
      FROM cdeu.finv_quote_secu_realtime_future;
    RAISE NOTICE 'V300230：期货快照 usc 回填完成，共 % 行，仍为空 % 行', v_total, v_empty;
    IF v_empty > 0 THEN
        RAISE WARNING 'V300230：仍有 % 行 usc 为空（usc 引擎结果表无对应 ACTIVE 映射），保留 NULL 待录入', v_empty;
    END IF;
END $$;

COMMIT;
