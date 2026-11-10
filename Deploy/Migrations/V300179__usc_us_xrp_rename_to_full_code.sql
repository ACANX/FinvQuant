-- =====================================================================
-- FinvQuant PostgreSQL V300179：美股 XRP ETF 的 usc 由 ETFXRP 改为 US.XRP（需求 11097）
--
-- 背景：
--   · 需求 11034 曾给该 ETF 分配独立读法 `ETFXRP`（ETF+XRP），用来解除与加密币 `XRP` 的撞车。
--   · 现在加密现货的口径已定：`CC.XRP → XRP`（P40 精确例外，V300178），
--     用户要求美股这只 ETF 改用**富途全码 `US.XRP`**（与加拿大 `CA.XRP` 的全码风格一致），
--     不再用自造的 `ETFXRP` 读法。
--
-- 动作（幂等，全部按当前状态重算）：
--   ① 规则层：`P40 / market=US / code=XRP` 由 `LITERAL ETFXRP` 改为 `FTMM`（= ftmm_symbol `US.XRP`）；
--   ② 6 张表的存量值 `ETFXRP` → `US.XRP`：结果表、键表、采集表、指标表、证券表、实时表；
--   ③ 改前快照写入既有归档表（采集/指标/证券）并在规则日志留痕；
--   ④ 收敛校验：`ETFXRP` 归零、规则胜出条目为 P40、结果表与键表口径一致、无重复 usc。
--
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 改前快照（既有归档表，唯一键冲突则跳过）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT 'ETFXRP', 'US.XRP', c.stock_id, c.ftmm_symbol, to_jsonb(c),
       '需求 11097 usc 改名：ETFXRP → US.XRP（美股 XRP ETF 改用富途全码）'
  FROM futu.finv_futu_quote_collect c
 WHERE c.usc = 'ETFXRP'
ON CONFLICT (usc_from, stock_id) DO NOTHING;

INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT 'ETFXRP', 'US.XRP', i.stock_id, i.ftmm_symbol, to_jsonb(i),
       '需求 11097 usc 改名：ETFXRP → US.XRP（美股 XRP ETF 改用富途全码）'
  FROM cdeu.finv_quote_secu_indicator i
 WHERE i.usc = 'ETFXRP'
ON CONFLICT (usc_from) DO NOTHING;

INSERT INTO archive.finv_security_purge_20261110
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, init_date, timezone, tz, gmt_create, gmt_update,
        flag_enable, market_code, type_secu_sub, provider, remark, src_table, reason, backup_at)
SELECT s.usc, s.exchange_code, s.security_type, s.security_code, s.security_name, s.security_name_cn,
       s.security_name_full, s.currency_type, s.init_date, s.timezone, s.tz, s.gmt_create, s.gmt_update,
       s.flag_enable, s.market_code, s.type_secu_sub, s.provider, s.remark,
       'public.finv_security',
       '需求 11097 usc 改名：ETFXRP → US.XRP（改前快照，行仍在册）', now()
  FROM public.finv_security s
 WHERE s.usc = 'ETFXRP'
   AND NOT EXISTS (SELECT 1 FROM archive.finv_security_purge_20261110 a
                    WHERE a.usc = 'ETFXRP' AND a.security_code = s.security_code
                      AND a.reason LIKE '需求 11097%');

-- ---------------------------------------------------------------------
-- ② 规则层：P40/US/XRP 由 LITERAL ETFXRP 改为 FTMM（全码 US.XRP）
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_rule_item
   SET action      = 'FTMM',
       usc_literal = NULL,
       rewrite_tpl = NULL,
       status      = 'ACTIVE',
       item_desc   = '美股 XRP ETF 用富途全码 US.XRP（加密现货 CC.XRP 占住 XRP 读法，不再自造 ETFXRP）',
       source_ref  = '需求 11097',
       remark      = '2026-11-10 用户要求：美股 XRP ETF 的 usc 由 ETFXRP 改为 US.XRP（与 CA.XRP 全码风格一致；需求 11034 的 ETFXRP 口径作废）'
 WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP'
   AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*';

INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
SELECT 'finv_futu_usc_rule_item', 'UPDATE', 'P40/US/XRP',
       jsonb_build_object('action', 'LITERAL', 'usc_literal', 'ETFXRP', 'status', 'ACTIVE'),
       jsonb_build_object('action', 'FTMM', 'usc_literal', NULL, 'status', 'ACTIVE'),
       'migration', '需求 11097 美股 XRP ETF：usc ETFXRP → US.XRP（P40 改 FTMM）';

-- ---------------------------------------------------------------------
-- ③ 存量值改名：ETFXRP → US.XRP
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_map
   SET usc = 'US.XRP',
       rule_no = 'P40',
       basis = '美股 XRP ETF 用富途全码 US.XRP：P40 精确例外（加密现货占住 XRP，需求 11097）',
       dt_compute = now()
 WHERE ftmm_symbol = 'US.XRP' AND usc = 'ETFXRP';

UPDATE futu.finv_futu_secu_key
   SET usc = 'US.XRP'
 WHERE ftmm_symbol = 'US.XRP' AND usc = 'ETFXRP';

UPDATE futu.finv_futu_quote_collect SET usc = 'US.XRP' WHERE usc = 'ETFXRP';

UPDATE cdeu.finv_quote_secu_indicator SET usc = 'US.XRP' WHERE usc = 'ETFXRP';

UPDATE public.finv_security
   SET usc = 'US.XRP'
 WHERE usc = 'ETFXRP' AND security_code = 'XRP';

UPDATE cdeu.finv_cdeu_secu_realtime_stock SET usc = 'US.XRP' WHERE usc = 'ETFXRP';

-- ---------------------------------------------------------------------
-- ③b 文案同步：V300178 给 CC.XRP 写的 item_desc / basis 里仍写着「美股让位为 ETFXRP」，
--     本次改名后该表述已过期，一并刷新（纯文案，不动 usc）
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_rule_item
   SET item_desc = '加密 XRP 现货占住主读法 XRP（美股 XRP ETF 用富途全码 US.XRP、加拿大 XRP ETF 走 P21 全码）'
 WHERE rule_no = 'P40' AND market = 'CC' AND code = 'XRP'
   AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*'
   AND item_desc IS DISTINCT FROM '加密 XRP 现货占住主读法 XRP（美股 XRP ETF 用富途全码 US.XRP、加拿大 XRP ETF 走 P21 全码）';

UPDATE futu.finv_futu_usc_map
   SET basis = '加密 XRP 现货固定 XRP：P40 精确例外（美股 XRP ETF 用富途全码 US.XRP，需求 11096/11097）'
 WHERE ftmm_symbol = 'CC.XRP' AND usc = 'XRP'
   AND basis IS DISTINCT FROM '加密 XRP 现货固定 XRP：P40 精确例外（美股 XRP ETF 用富途全码 US.XRP，需求 11096/11097）';

-- ---------------------------------------------------------------------
-- ④ 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_item      BIGINT;
    v_action    TEXT;
    v_literal   TEXT;
    v_winner    TEXT;
    v_canon     TEXT;
    v_map_usc   TEXT;
    v_map_rule  TEXT;
    v_key_usc   TEXT;
    v_left      BIGINT;
    v_dup       BIGINT;
    v_sec       BIGINT;
    v_has_key   BOOLEAN;
BEGIN
    -- ① 规则条目
    SELECT count(*), min(action), min(usc_literal) INTO v_item, v_action, v_literal
      FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND code = 'XRP'
       AND instrument IS NULL AND code_regex IS NULL AND scope_env = '*'
       AND status = 'ACTIVE';
    IF v_item <> 1 OR v_action IS DISTINCT FROM 'FTMM' OR v_literal IS NOT NULL THEN
        RAISE EXCEPTION 'V300179 校验失败：P40/US/XRP 应为 1 条 FTMM/usc_literal=NULL/ACTIVE，实际 % 条 action=% usc_literal=%',
            v_item, v_action, v_literal;
    END IF;

    -- ② 引擎分层命中（精确 code 层）应为 P40
    SELECT i.rule_no INTO v_winner
      FROM futu.finv_futu_usc_rule_item i
      JOIN futu.finv_futu_usc_rule ru ON ru.rule_no = i.rule_no
     WHERE ru.flag_enable = '1'
       AND ru.match_scope = 'NON_GATE'
       AND i.status = 'ACTIVE'
       AND i.scope_env IN ('*', 'prod')
       AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
       AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
       AND (i.market     IS NULL OR i.market     = 'US')
       AND (i.instrument IS NULL OR i.instrument = 'etf')
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
        RAISE EXCEPTION 'V300179 校验失败：US/XRP 的胜出条目应为 P40，实际 %', v_winner;
    END IF;

    -- ③ 旧值必须归零
    SELECT (SELECT count(*) FROM futu.finv_futu_quote_collect       WHERE usc = 'ETFXRP')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_indicator     WHERE usc = 'ETFXRP')
         + (SELECT count(*) FROM futu.finv_futu_secu_key            WHERE usc = 'ETFXRP')
         + (SELECT count(*) FROM futu.finv_futu_usc_map             WHERE usc = 'ETFXRP')
         + (SELECT count(*) FROM public.finv_security                WHERE usc = 'ETFXRP')
         + (SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE usc = 'ETFXRP')
      INTO v_left;
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300179 未收敛：仍有 % 行 usc=ETFXRP', v_left;
    END IF;

    -- ④ 结果表不得出现重复 usc（部分唯一索引保证 ACTIVE 行唯一，这里显式复核）
    SELECT count(*) INTO v_dup FROM (
      SELECT lower(usc) FROM futu.finv_futu_usc_map WHERE status = 'ACTIVE'
       GROUP BY 1 HAVING count(*) > 1) t;
    IF v_dup <> 0 THEN
        RAISE EXCEPTION 'V300179 引入重复：结果表有 % 组 lower(usc) 重复', v_dup;
    END IF;

    -- ⑤ 存量数据口径（键表/结果表在册时）
    SELECT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key WHERE ftmm_symbol = 'US.XRP') INTO v_has_key;
    IF v_has_key THEN
        SELECT canon_code INTO v_canon FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'US.XRP';
        IF v_canon IS DISTINCT FROM 'XRP' THEN
            RAISE EXCEPTION 'V300179 校验失败：US.XRP 前处理后 canon_code 应为 XRP，实际 %', v_canon;
        END IF;

        SELECT usc, rule_no INTO v_map_usc, v_map_rule
          FROM futu.finv_futu_usc_map WHERE ftmm_symbol = 'US.XRP' AND status = 'ACTIVE';
        IF FOUND AND (v_map_usc IS DISTINCT FROM 'US.XRP' OR v_map_rule IS DISTINCT FROM 'P40') THEN
            RAISE EXCEPTION 'V300179 校验失败：结果表 US.XRP 应为 usc=US.XRP/rule_no=P40，实际 % / %',
                v_map_usc, v_map_rule;
        END IF;

        SELECT usc INTO v_key_usc FROM futu.finv_futu_secu_key WHERE ftmm_symbol = 'US.XRP';
        IF v_key_usc IS DISTINCT FROM 'US.XRP' THEN
            RAISE EXCEPTION 'V300179 校验失败：键表 US.XRP 的 usc 应为 US.XRP，实际 %', v_key_usc;
        END IF;
        RAISE NOTICE 'V300179：US.XRP canon=% 结果表 usc=% 规则=% 键表 usc=%',
            v_canon, v_map_usc, v_map_rule, v_key_usc;
    ELSE
        RAISE NOTICE 'V300179：键表暂无 US.XRP（全新库），跳过存量校验';
    END IF;

    -- ⑥ 文案：CC.XRP 的条目说明不得再出现已作废的 ETFXRP 读法
    IF EXISTS (SELECT 1 FROM futu.finv_futu_usc_rule_item
                WHERE rule_no = 'P40' AND market = 'CC' AND code = 'XRP'
                  AND item_desc LIKE '%ETFXRP%') THEN
        RAISE EXCEPTION 'V300179 校验失败：P40/CC/XRP 的 item_desc 仍提到已作废的 ETFXRP';
    END IF;
    IF EXISTS (SELECT 1 FROM futu.finv_futu_usc_map
                WHERE ftmm_symbol = 'CC.XRP' AND basis LIKE '%ETFXRP%') THEN
        RAISE EXCEPTION 'V300179 校验失败：CC.XRP 的结果依据仍提到已作废的 ETFXRP';
    END IF;

    -- ⑦ 证券表：只有那只 ETF 该持有 US.XRP
    SELECT count(*) INTO v_sec FROM public.finv_security WHERE usc = 'US.XRP';
    IF v_sec > 1 OR EXISTS (SELECT 1 FROM public.finv_security WHERE usc = 'US.XRP' AND security_code <> 'XRP') THEN
        RAISE EXCEPTION 'V300179 校验失败：finv_security 中 usc=US.XRP 的行 % 条（且必须 security_code=XRP）', v_sec;
    END IF;

    RAISE NOTICE 'V300179 就绪：美股 XRP ETF usc = US.XRP（P40 FTMM），ETFXRP 已归零';
END $$;

COMMIT;
