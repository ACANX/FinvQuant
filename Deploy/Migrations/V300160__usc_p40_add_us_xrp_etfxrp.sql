-- =====================================================================
-- FinvQuant PostgreSQL V300160：P40 补一条 US/XRP → ETFXRP（需求 11075）
--
-- 背景（用户 2026-11-10 决定）：
--   V300159 修复分层排序、P22 提为前处理之后，全表仅剩 149 行 SH 指数（方案 A 的预期改写）
--   与 1 行 `US.XRP` 属「语义级差异」：
--     · `US.XRP` 是美股 XRP ETF，现有 usc = `ETFXRP`（`V300135` 建行时定的写法）；
--     · 按现行规则它会走 P20（US 市场默认 CODE）得到 `XRP`；
--     · 加密币 XRP 在 P40 里被显式指到 `CC.XRP`（非权威主体），于是裸 `XRP` 会落到美股 ETF 头上 —— 语义颠倒。
--   决定：给 P40 补一条单点例外，让美股 ETF 沿用既有 `ETFXRP` 写法，`XRP` 这个读法不被占用。
--
-- 本条目的口径：
--   rule_no=P40（单点例外，tier=1 精确 code，优先于 P20 市场默认）
--   market=US, code=XRP, action=LITERAL, usc_literal=ETFXRP
--   与 `finv_security.usc='ETFXRP'`、`futu.finv_futu_secu_key.usc='ETFXRP'` 的既有数据一致。
--
-- 幂等：ON CONFLICT DO NOTHING（唯一键 (rule_no, scope_env, market, instrument, code, code_regex)，
--   NULLS NOT DISTINCT），重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 补条目
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule_item
      (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
       priority, status, scope_env, item_desc, source_ref, remark, creator)
VALUES ('P40', 'US', NULL, 'XRP', NULL, 'LITERAL', 'ETFXRP', NULL,
        100, 'ACTIVE', '*',
        '美股 XRP ETF：沿用既有 ETFXRP 写法，避免占用加密币 XRP 的读法',
        'USCMappingRule.md §5', '需求 11075', 'migration')
ON CONFLICT (rule_no, scope_env, market, instrument, code, code_regex) DO NOTHING;

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt      INT;
    v_literal  TEXT;
    v_action   TEXT;
    v_desc     TEXT;
    v_canon    TEXT;
BEGIN
    SELECT count(*), min(usc_literal), min(action), min(item_desc)
      INTO v_cnt, v_literal, v_action, v_desc
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP' AND status = 'ACTIVE';

    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V300160 校验失败：P40/US/XRP 应有且仅有 1 条 ACTIVE 条目，实际 %', v_cnt;
    END IF;
    IF v_literal IS DISTINCT FROM 'ETFXRP' THEN
        RAISE EXCEPTION 'V300160 校验失败：usc_literal 应为 ETFXRP，实际 %', v_literal;
    END IF;
    IF v_action IS DISTINCT FROM 'LITERAL' THEN
        RAISE EXCEPTION 'V300160 校验失败：action 应为 LITERAL，实际 %', v_action;
    END IF;
    IF v_desc IS NULL OR v_desc = '' THEN
        RAISE EXCEPTION 'V300160 校验失败：item_desc 必填';
    END IF;

    -- 前处理不应被这条例外影响：US.XRP 的 canon_code 仍是 XRP
    SELECT canon_code INTO v_canon FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'US.XRP';
    IF v_canon IS DISTINCT FROM 'XRP' THEN
        RAISE EXCEPTION 'V300160 校验失败：US.XRP 的 canon_code 应仍为 XRP，实际 %', v_canon;
    END IF;

    RAISE NOTICE 'V300160 就绪：P40/US/XRP → ETFXRP（canon_code 保持 %）', v_canon;
END $$;

COMMIT;
