-- =====================================================================
-- FinvQuant PostgreSQL V300178：把 CC.XRP 的 usc 变更写进 USC 规则表（需求 11096）
--
-- 背景：
--   · 历史上加密 XRP 与美股 XRP ETF 同名，规则层给**加密侧用全码**：
--     P40 条目 `CC/XRP → FTMM`（usc = CC.XRP），P50 还有一条同类备份条目。
--   · 后来美股 XRP ETF 让位成 `ETFXRP`（P40 单点例外），`XRP` 这个读法腾了出来；
--     需求 11085 的一次性对齐把 `CC.XRP` 的 usc 从 `CC.XRP` 改成了 `XRP`。
--   · 但这次改值**没有落到规则表**：现在 `CC.XRP → XRP` 只是被
--     P20 市场默认（「CC 市场天然唯一率高，usc 直接用交易所代码」）"顺带"算出来的，
--     一旦 CC 的默认动作调整、或出现同 code 的第二个 CC 证券，读法就会漂回去。
--
-- 本迁移按用户要求把这条判定固化进规则表，作为后续发放 usc 的依据：
--     P40 / market=CC / code=XRP / action=LITERAL / usc_literal=XRP / status=ACTIVE
--
-- 引擎分层：精确 code（P40/P50）→ 正则 → 市场默认（P20/P21）→ 兜底（P60），
-- 因此该条目稳定压过 P20；P50 的同名条目保持 RETIRED（P50 整层也已停用）。
--
-- 幂等：条目不存在则补插、存在则校正为预期值（最终状态恒定）；结果表只补 rule_no/basis。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 规则条目：P40/CC/XRP 校正为 LITERAL XRP 并启用
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists BOOLEAN;
BEGIN
    SELECT EXISTS (SELECT 1 FROM futu.finv_futu_usc_rule_item
                    WHERE rule_no = 'P40' AND market = 'CC' AND code = 'XRP'
                      AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*')
      INTO v_exists;
    IF NOT v_exists THEN
        INSERT INTO futu.finv_futu_usc_rule_item
              (rule_no, market, instrument, code, code_regex, action, usc_literal,
               priority, status, scope_env, item_desc, source_ref, remark)
        VALUES ('P40', 'CC', NULL, 'XRP', NULL, 'LITERAL', 'XRP',
                100, 'ACTIVE', '*',
                '加密 XRP 现货占住主读法 XRP（美股 XRP ETF 让位为 ETFXRP、加拿大 XRP ETF 走 P21 全码）',
                '需求 11096',
                '2026-11-10 用户要求：把 CC.XRP 的 usc 由全码 CC.XRP 改为 XRP 写进规则表，作为后续发放依据');
    END IF;
END $$;

UPDATE futu.finv_futu_usc_rule_item
   SET action      = 'LITERAL',
       usc_literal = 'XRP',
       rewrite_tpl = NULL,
       status      = 'ACTIVE',
       item_desc   = '加密 XRP 现货占住主读法 XRP（美股 XRP ETF 让位为 ETFXRP、加拿大 XRP ETF 走 P21 全码）',
       source_ref  = '需求 11096',
       remark      = '2026-11-10 用户要求：把 CC.XRP 的 usc 由全码 CC.XRP 改为 XRP 写进规则表，作为后续发放依据（原先仅靠 P20 市场默认顺带得出；P50 同名条目保持 RETIRED）'
 WHERE rule_no = 'P40' AND market = 'CC' AND code = 'XRP'
   AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*'
   AND (action      IS DISTINCT FROM 'LITERAL'
     OR usc_literal IS DISTINCT FROM 'XRP'
     OR rewrite_tpl IS NOT NULL
     OR status      IS DISTINCT FROM 'ACTIVE'
     OR item_desc   IS DISTINCT FROM '加密 XRP 现货占住主读法 XRP（美股 XRP ETF 让位为 ETFXRP、加拿大 XRP ETF 走 P21 全码）'
     OR source_ref  IS DISTINCT FROM '需求 11096'
     OR remark      IS DISTINCT FROM '2026-11-10 用户要求：把 CC.XRP 的 usc 由全码 CC.XRP 改为 XRP 写进规则表，作为后续发放依据（原先仅靠 P20 市场默认顺带得出；P50 同名条目保持 RETIRED）');

-- ---------------------------------------------------------------------
-- ② 结果表口径同步：已在册的 CC.XRP 把命中规则改成 P40（只补规则与依据，不动 usc）
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_map m
   SET rule_no    = 'P40',
       basis      = '加密 XRP 现货固定 XRP：P40 精确例外（与美股 ETFXRP 区分，需求 11096）',
       dt_compute = now()
 WHERE m.ftmm_symbol = 'CC.XRP'
   AND m.usc = 'XRP'
   AND (m.rule_no IS DISTINCT FROM 'P40'
     OR m.basis   IS DISTINCT FROM '加密 XRP 现货固定 XRP：P40 精确例外（与美股 ETFXRP 区分，需求 11096）');

-- ---------------------------------------------------------------------
-- ③ 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_item      BIGINT;
    v_action    TEXT;
    v_literal   TEXT;
    v_status    TEXT;
    v_us_item   BIGINT;
    v_us_lit    TEXT;
    v_p50       TEXT;
    v_winner    TEXT;
    v_canon     TEXT;
    v_map_usc   TEXT;
    v_map_rule  TEXT;
    v_has_key   BOOLEAN;
BEGIN
    -- ① P40/CC/XRP 条目
    SELECT count(*), min(action), min(usc_literal), min(status)
      INTO v_item, v_action, v_literal, v_status
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'CC' AND code = 'XRP'
       AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*';
    IF v_item <> 1 OR v_action IS DISTINCT FROM 'LITERAL'
       OR v_literal IS DISTINCT FROM 'XRP' OR v_status IS DISTINCT FROM 'ACTIVE' THEN
        RAISE EXCEPTION 'V300178 校验失败：P40/CC/XRP 应为唯一 1 条 LITERAL/XRP/ACTIVE，实际 % 条 action=% usc_literal=% status=%',
            v_item, v_action, v_literal, v_status;
    END IF;

    -- ② 美股 XRP ETF 的例外不得被影响
    SELECT count(*), min(usc_literal) INTO v_us_item, v_us_lit
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP'
       AND action = 'LITERAL' AND status = 'ACTIVE';
    IF v_us_item <> 1 OR v_us_lit IS DISTINCT FROM 'ETFXRP' THEN
        RAISE EXCEPTION 'V300178 校验失败：P40/US/XRP 应仍为 1 条且 usc_literal=ETFXRP，实际 % 条 / %',
            v_us_item, v_us_lit;
    END IF;

    -- ③ P50 的同名条目保持 RETIRED（P50 整层已停用，不做启用）
    SELECT min(status) INTO v_p50
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P50' AND market = 'CC' AND code = 'XRP';
    IF v_p50 IS NOT NULL AND v_p50 <> 'RETIRED' THEN
        RAISE EXCEPTION 'V300178 校验失败：P50/CC/XRP 应保持 RETIRED，实际 %', v_p50;
    END IF;

    -- ④ 引擎分层命中：按与 store.go 相同的 ORDER BY 取 CC/XRP 的胜出条目
    SELECT i.rule_no INTO v_winner
      FROM futu.finv_futu_usc_rule_item i
      JOIN futu.finv_futu_usc_rule ru ON ru.rule_no = i.rule_no
     WHERE ru.flag_enable = '1'
       AND ru.match_scope = 'NON_GATE'
       AND i.status = 'ACTIVE'
       AND i.scope_env IN ('*', 'prod')
       AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
       AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
       AND (i.market     IS NULL OR i.market     = 'CC')
       AND (i.instrument IS NULL OR i.instrument = 'crypto')
       AND ( (i.code IS NOT NULL AND lower(i.code) = lower('XRP'))
          OR (i.code IS NULL AND i.code_regex IS NOT NULL AND 'XRP' ~* i.code_regex)
          OR (i.code IS NULL AND i.code_regex IS NULL) )
     ORDER BY CASE WHEN i.code IS NOT NULL THEN 1
                   WHEN i.code_regex IS NOT NULL THEN 2
                   WHEN i.market IS NOT NULL THEN 3
                   ELSE 4 END,
              ru.priority, i.priority
     LIMIT 1;
    IF v_winner IS DISTINCT FROM 'P40' THEN
        RAISE EXCEPTION 'V300178 校验失败：CC/XRP 的胜出条目应为 P40，实际 %', v_winner;
    END IF;

    -- ⑤ 存量数据（键表有 CC.XRP 时）：前处理后 canon 仍为 XRP，结果表口径一致
    SELECT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key WHERE ftmm_symbol = 'CC.XRP') INTO v_has_key;
    IF v_has_key THEN
        SELECT canon_code INTO v_canon FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'CC.XRP';
        IF v_canon IS DISTINCT FROM 'XRP' THEN
            RAISE EXCEPTION 'V300178 校验失败：CC.XRP 前处理后 canon_code 应为 XRP，实际 %', v_canon;
        END IF;

        SELECT usc, rule_no INTO v_map_usc, v_map_rule
          FROM futu.finv_futu_usc_map
         WHERE ftmm_symbol = 'CC.XRP' AND status = 'ACTIVE';
        IF FOUND AND (v_map_usc IS DISTINCT FROM 'XRP' OR v_map_rule IS DISTINCT FROM 'P40') THEN
            RAISE EXCEPTION 'V300178 校验失败：结果表 CC.XRP 应为 usc=XRP/rule_no=P40，实际 % / %',
                v_map_usc, v_map_rule;
        END IF;
        RAISE NOTICE 'V300178：CC.XRP canon=% 结果表 usc=% 规则=%', v_canon, v_map_usc, v_map_rule;
    ELSE
        RAISE NOTICE 'V300178：键表暂无 CC.XRP（全新库），跳过存量校验';
    END IF;

    RAISE NOTICE 'V300178 就绪：P40/CC/XRP → XRP（胜出条目 %）', v_winner;
END $$;

COMMIT;
