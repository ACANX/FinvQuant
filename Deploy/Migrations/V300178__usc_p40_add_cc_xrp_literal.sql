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
-- ② 派生数据跟随规则：结果表 + 键表把 CC.XRP 落成 usc=XRP
--
-- 不假设「环境里已经对齐过」：有的环境（如从上游同步过来的库）结果表/键表里
-- CC.XRP 仍是旧口径 `CC.XRP`（P40 当年是 FTMM），只补 rule_no 会留下错误值。
-- 这里按 ftmm_symbol 直接落值，等价于引擎 apply 之后的状态。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_clash TEXT;
BEGIN
    -- 结果表的部分唯一索引只约束 ACTIVE 行的 usc_lower，先确认目标值没被别的行占着
    SELECT m2.ftmm_symbol INTO v_clash
      FROM futu.finv_futu_usc_map m2
     WHERE m2.usc_lower = 'xrp' AND m2.status = 'ACTIVE' AND m2.ftmm_symbol <> 'CC.XRP'
     LIMIT 1;
    IF v_clash IS NOT NULL THEN
        RAISE EXCEPTION 'V300178 中止：usc=XRP 已被结果表在册行 % 占用，需先处理该行', v_clash;
    END IF;
END $$;

UPDATE futu.finv_futu_usc_map
   SET usc        = 'XRP',
       rule_no    = 'P40',
       basis      = '加密 XRP 现货固定 XRP：P40 精确例外（美股 XRP ETF 用富途全码 US.XRP，需求 11096/11097）',
       dt_compute = now()
 WHERE ftmm_symbol = 'CC.XRP'
   AND status = 'ACTIVE'
   AND (usc     IS DISTINCT FROM 'XRP'
     OR rule_no IS DISTINCT FROM 'P40'
     OR basis   IS DISTINCT FROM '加密 XRP 现货固定 XRP：P40 精确例外（美股 XRP ETF 用富途全码 US.XRP，需求 11096/11097）');

-- 键表 usc 是引擎 apply 会回写的派生值，这里同步落值（幂等）
UPDATE futu.finv_futu_secu_key
   SET usc = 'XRP'
 WHERE ftmm_symbol = 'CC.XRP'
   AND usc IS DISTINCT FROM 'XRP';

-- 下游「当前状态表」跟随改名：CC.XRP → XRP
--
-- 有的环境（结果表仍是旧全码、键表来自上游同步）在这些表里还留着 `CC.XRP`；
-- 本地库因需求 11085 的一次性对齐早已是 `XRP`，所以这里通常改动 0 行。
-- 先查冲突（`finv_futu_quote_collect` 以 usc 为主键，改名可能撞上已存在的同名行）。
DO $$
DECLARE
    v_clash TEXT;
BEGIN
    SELECT 'finv_futu_quote_collect' INTO v_clash
      FROM futu.finv_futu_quote_collect c
     WHERE c.usc = 'XRP' AND c.ftmm_symbol <> 'CC.XRP'
     LIMIT 1;
    IF v_clash IS NOT NULL THEN
        RAISE EXCEPTION 'V300178 中止：采集表已存在 usc=XRP 行（非 CC.XRP），无法改名 CC.XRP → XRP';
    END IF;
    SELECT 'finv_quote_secu_indicator' INTO v_clash
      FROM cdeu.finv_quote_secu_indicator i
     WHERE i.usc = 'XRP' AND coalesce(i.ftmm_symbol, '') <> 'CC.XRP'
     LIMIT 1;
    IF v_clash IS NOT NULL THEN
        RAISE EXCEPTION 'V300178 中止：指标表已存在 usc=XRP 行（非 CC.XRP），无法改名 CC.XRP → XRP';
    END IF;
END $$;

UPDATE futu.finv_futu_quote_collect      SET usc = 'XRP' WHERE usc = 'CC.XRP';
UPDATE cdeu.finv_quote_secu_indicator    SET usc = 'XRP' WHERE usc = 'CC.XRP';
UPDATE public.finv_security              SET usc = 'XRP' WHERE usc = 'CC.XRP' AND security_code = 'XRP';
UPDATE cdeu.finv_cdeu_secu_realtime_stock SET usc = 'XRP' WHERE usc = 'CC.XRP';
UPDATE cdeu.finv_quote_cdeu_usc          SET usc = 'XRP' WHERE usc = 'CC.XRP';

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
    v_old_left  BIGINT;
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

    -- ② 美股 XRP ETF 的例外不得被本迁移影响：
    --    它在本迁移时点的形态取决于环境版本（可能是 `LITERAL ETFXRP`，也可能已被 V300179 改成 `FTMM`），
    --    所以不能断言具体形态；只断言「仍然存在且生效」+「没有被塞成 LITERAL XRP」（那会和加密现货撞车）。
    SELECT count(*) INTO v_us_item
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP' AND status = 'ACTIVE';
    IF v_us_item <> 1 THEN
        RAISE EXCEPTION 'V300178 校验失败：P40/US/XRP 在册条目应为 1 条，实际 %', v_us_item;
    END IF;
    IF EXISTS (SELECT 1 FROM futu.finv_futu_usc_rule_item
                WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP'
                  AND action = 'LITERAL' AND usc_literal = 'XRP') THEN
        RAISE EXCEPTION 'V300178 校验失败：P40/US/XRP 被误设成 LITERAL XRP（会与加密现货撞车）';
    END IF;
    v_us_lit := NULL;

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

    -- ④b 下游当前状态表不得残留旧值 CC.XRP（历史/日志/导出/K 线表不在本次范围）
    SELECT (SELECT count(*) FROM futu.finv_futu_quote_collect       WHERE usc = 'CC.XRP')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_indicator     WHERE usc = 'CC.XRP')
         + (SELECT count(*) FROM public.finv_security                WHERE usc = 'CC.XRP')
         + (SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE usc = 'CC.XRP')
         + (SELECT count(*) FROM cdeu.finv_quote_cdeu_usc           WHERE usc = 'CC.XRP')
      INTO v_old_left;
    IF v_old_left <> 0 THEN
        RAISE EXCEPTION 'V300178 未收敛：下游当前状态表仍有 % 行 usc=CC.XRP', v_old_left;
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
