-- =====================================================================
-- FinvQuant PostgreSQL V300161：指数代码去前导点（前处理）+ 14 条同市场撞名例外（需求 11077）
--
-- 背景（用户 2026-11-10 要求）：
--   「指数的代码前面的 `.` 需要统一去掉；如果去掉后与其它代码冲突，就设例外规避。」
--
-- 现状实测：641 行带前导点的代码，全部 `instrument='index'`：
--     US 577 / JP 41 / CA 11 / AU 7 / SG 4 / THA 1
--   它们的 usc 现在是两种难看形态：
--     · 裸代码口径（P20 市场，如 US/THA）：走 CODE 后 canon 带点，usc 会拿 `SPX`? 否——
--       现在 canon 保留点，直接拿不到裸码，被 P40/P50 兜成 `US..SET` 这种**双点**；
--     · 全码口径（P21 市场，如 AU/CA/JP/SG）：usc = ftmm_symbol = `AU..XFL`（同样双点）。
--
-- 本迁移做三件事：
--   1. 新增前处理规则 P16「指数代码去前导点」：`instrument='index'` 且 `code ~ '^\.'`
--      → 正则改写去掉首个点（`.SPX` → `SPX`），排在 P10 闸门之前；
--   2. 为此前提到的冲突补 14 条 P40 单点例外：美股指数去点后与本市场同码的股票/ETF 撞名
--      （`.BFLY`↔`BFLY[stock]` 等），指数侧改用 `US.xxx`（市场前缀形态），股票侧保持裸码；
--      注意：候选是按**前处理后的 canon_code** 精确匹配的，所以条目里 `code` 写去点后的 `BFLY`，
--      再用 `instrument='index'` 与同码的股票区分（否则会把股票也一起命中）。
--   3. 把已有 3 条以「带点代码」为匹配键的例外改成去点后的形态（否则 P16 之后它们再也匹配不上）：
--        P40 US `.SET` → `SET`、P50 THA `.SET` → `SET`、P50 US `.SPTSX` → `SPTSX`，并补 `instrument='index'`；
--   4. 收敛校验：前处理结果、例外条目数量、已有例外改写结果、以及去点后无残留前导点。
--
-- 与之配套的代码改动（同需求 11077，非本迁移）：
--   `internal/meta/finv/quote/uscrule/store.go` 里 FTMM 动作与冲突升级兜底改用
--   `regexp_replace(ftmm_symbol, '^([^.]+)\.\.', '\1.')` —— 即把 `AU..XFL` 收敛成 `AU.XFL`，
--   保证 P21 市场的指数 usc 也只带一个点。（数据库侧没有依赖该表达式的地方。）
--
-- 幂等：规则 UPDATE 带目标值判断、条目 INSERT ... ON CONFLICT DO NOTHING，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 新增前处理规则 P16（priority 16：P14 之后、P22/P20 之前）
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule
      (rule_no, rule_name, rule_type, priority, match_scope, default_action,
       flag_enable, rule_desc, remark, creator)
VALUES
 ('P16', '前处理：指数代码去前导点', 'NORMALIZE', 16, 'ALL', 'REWRITE', '1',
  E'指数的上游代码带前导点（如 `.SPX`、`.XFL`），统一去掉，usc 不再带点。\n'
  || E'只在 instrument=''index'' 上生效，正则 ''^\\.'' → 空串。\n'
  || E'去点后与同市场同码的股票/ETF 撞名的，由 P40 单点例外改用市场前缀形态（如 US.BFLY）。',
  '需求 11077', 'migration')
ON CONFLICT (rule_no) DO NOTHING;

INSERT INTO futu.finv_futu_usc_rule_item
      (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
       priority, status, scope_env, item_desc, source_ref, remark, creator)
VALUES
 ('P16', NULL, 'index', NULL, '^\.', 'REWRITE', NULL, '', 100, 'ACTIVE', '*',
  '指数代码去前导点：.SPX → SPX、.XFL → XFL（仅 instrument=index）',
  'USCMappingRule.md §3.4', '需求 11077', 'migration')
ON CONFLICT (rule_no, scope_env, market, instrument, code, code_regex) DO NOTHING;

-- ---------------------------------------------------------------------
-- 2. 14 条同市场撞名例外（美股指数 vs 美股同码股票/ETF）
--    指数侧用 `US.xxx`（市场前缀），股票侧保持裸码，两者不再抢同一个 usc。
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule_item
      (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
       priority, status, scope_env, item_desc, source_ref, remark, creator)
VALUES
 ('P40', 'US', 'index', 'BFLY', NULL, 'LITERAL', 'US.BFLY', NULL, 100, 'ACTIVE', '*',
  '美股指数 .BFLY 去点后与美股 BFLY（股票）撞名，指数侧改为 US.BFLY', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'CAC',  NULL, 'LITERAL', 'US.CAC',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .CAC 去点后与美股 CAC（股票）撞名，指数侧改为 US.CAC',   'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'CMBO', NULL, 'LITERAL', 'US.CMBO', NULL, 100, 'ACTIVE', '*',
  '美股指数 .CMBO 去点后与美股 CMBO（ETF）撞名，指数侧改为 US.CMBO', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'DAX',  NULL, 'LITERAL', 'US.DAX',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .DAX 去点后与美股 DAX（ETF）撞名，指数侧改为 US.DAX',    'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'DJT',  NULL, 'LITERAL', 'US.DJT',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .DJT 去点后与美股 DJT（股票）撞名，指数侧改为 US.DJT',   'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'HXC',  NULL, 'LITERAL', 'US.HXC',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .HXC 去点后与美股 HXC（ETF）撞名，指数侧改为 US.HXC',    'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'IBEX', NULL, 'LITERAL', 'US.IBEX', NULL, 100, 'ACTIVE', '*',
  '美股指数 .IBEX 去点后与美股 IBEX（股票）撞名，指数侧改为 US.IBEX', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'IBIG', NULL, 'LITERAL', 'US.IBIG', NULL, 100, 'ACTIVE', '*',
  '美股指数 .IBIG 去点后与美股 IBIG（ETF）撞名，指数侧改为 US.IBIG', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'JCI',  NULL, 'LITERAL', 'US.JCI',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .JCI 去点后与美股 JCI（股票）撞名，指数侧改为 US.JCI',   'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'OBX',  NULL, 'LITERAL', 'US.OBX',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .OBX 去点后与美股 OBX（股票）撞名，指数侧改为 US.OBX',   'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'SPAI', NULL, 'LITERAL', 'US.SPAI', NULL, 100, 'ACTIVE', '*',
  '美股指数 .SPAI 去点后与美股 SPAI（股票）撞名，指数侧改为 US.SPAI', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'SPRO', NULL, 'LITERAL', 'US.SPRO', NULL, 100, 'ACTIVE', '*',
  '美股指数 .SPRO 去点后与美股 SPRO（股票）撞名，指数侧改为 US.SPRO', 'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'UZF',  NULL, 'LITERAL', 'US.UZF',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .UZF 去点后与美股 UZF（股票）撞名，指数侧改为 US.UZF',   'USCMappingRule.md §5', '需求 11077', 'migration'),
 ('P40', 'US', 'index', 'UZX',  NULL, 'LITERAL', 'US.UZX',  NULL, 100, 'ACTIVE', '*',
  '美股指数 .UZX 去点后与美股 UZX（股票）撞名，指数侧改为 US.UZX',   'USCMappingRule.md §5', '需求 11077', 'migration')
ON CONFLICT (rule_no, scope_env, market, instrument, code, code_regex) DO NOTHING;

-- ---------------------------------------------------------------------
-- 2.1 已有 3 条以带点代码为匹配键的例外改成去点后的形态
--     候选按前处理后的 canon_code 精确匹配，P16 去点后旧的 `.SET` / `.SPTSX` 键永远命中不了；
--     这 3 行在键表里都是 instrument='index'，因此同时把 instrument 由 NULL 收紧为 'index'，
--     避免误命中同码的股票（AU.SET 股票走它自己的 P40 条目）。
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_rule_item
   SET code = ltrim(code, '.'), instrument = 'index', remark = trim(both ' ' FROM COALESCE(remark, '') || ' | 需求 11077：P16 去点后改为匹配去点代码')
 WHERE rule_no IN ('P40', 'P50')
   AND code LIKE '.%'
   AND market IN ('US', 'THA');

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rule      INT;
    v_item      INT;
    v_exc       INT;
    v_spx       TEXT;
    v_xfl       TEXT;
    v_set_us    TEXT;
    v_bad       INT;
BEGIN
    SELECT count(*) INTO v_rule FROM futu.finv_futu_usc_rule WHERE rule_no = 'P16' AND rule_type = 'NORMALIZE';
    IF v_rule <> 1 THEN
        RAISE EXCEPTION 'V300161 校验失败：规则 P16 未插入或类型不对';
    END IF;

    SELECT count(*) INTO v_item FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P16' AND code_regex = '^\.' AND instrument = 'index' AND status = 'ACTIVE';
    IF v_item <> 1 THEN
        RAISE EXCEPTION 'V300161 校验失败：P16 条目应为 1 条，实际 %', v_item;
    END IF;

    SELECT count(*) INTO v_exc FROM futu.finv_futu_usc_rule_item
     WHERE rule_no = 'P40' AND market = 'US' AND instrument = 'index' AND status = 'ACTIVE'
       AND code IN ('BFLY','CAC','CMBO','DAX','DJT','HXC','IBEX','IBIG','JCI','OBX','SPAI','SPRO','UZF','UZX');
    IF v_exc <> 14 THEN
        RAISE EXCEPTION 'V300161 校验失败：同市场撞名例外应为 14 条，实际 %', v_exc;
    END IF;

    -- 前处理抽样：点被去掉，且非指数不受影响。
    -- 全新库键表为空 → 引擎无行，抽样无从比对，跳过（规则/条目的断言在上方已覆盖）。
    IF NOT EXISTS (SELECT 1 FROM futu.f_finv_futu_usc_calc('*') LIMIT 1) THEN
        RAISE NOTICE 'V300161：引擎无计算行（全新库），跳过存量抽样校验';
        RETURN;
    END IF;

    SELECT canon_code INTO v_spx FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'US..SPX';
    SELECT canon_code INTO v_xfl FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'AU..XFL';
    IF v_spx IS DISTINCT FROM 'SPX' THEN
        RAISE EXCEPTION 'V300161 校验失败：US..SPX 的 canon_code 应为 SPX，实际 %', v_spx;
    END IF;
    IF v_xfl IS DISTINCT FROM 'XFL' THEN
        RAISE EXCEPTION 'V300161 校验失败：AU..XFL 的 canon_code 应为 XFL，实际 %', v_xfl;
    END IF;

    -- .SET 也是指数（instrument='index'）：去点后 canon 应为 SET
    SELECT canon_code INTO v_set_us FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'US..SET';
    IF v_set_us IS DISTINCT FROM 'SET' THEN
        RAISE EXCEPTION 'V300161 校验失败：US..SET 的 canon_code 应为 SET，实际 %', v_set_us;
    END IF;

    -- 例外条目里不得再残留带点代码（P16 之后匹配不上）
    SELECT count(*) INTO v_bad FROM futu.finv_futu_usc_rule_item WHERE code LIKE '.%';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300161 校验失败：仍有 % 条例外条目以带点代码为匹配键', v_bad;
    END IF;

    -- 已有 3 条例外已改写为去点形态且收紧到 index
    SELECT count(*) INTO v_bad FROM futu.finv_futu_usc_rule_item
     WHERE rule_no IN ('P40','P50') AND instrument = 'index' AND code IN ('SET', 'SPTSX');
    IF v_bad <> 3 THEN
        RAISE EXCEPTION 'V300161 校验失败：应改写 3 条已有例外为去点形态，实际 %', v_bad;
    END IF;

    -- 去点后仍带点的指数行数应为 0
    SELECT count(*) INTO v_bad
      FROM futu.f_finv_futu_usc_calc('*') c
      JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
     WHERE k.instrument = 'index' AND k.code LIKE '.%' AND c.canon_code LIKE '.%';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300161 校验失败：仍有 % 行指数 code 去点后保留前导点', v_bad;
    END IF;

    RAISE NOTICE 'V300161 就绪：P16 去前导点（US..SPX→%, AU..XFL→%），14 条同市场例外已登记', v_spx, v_xfl;
END $$;

COMMIT;
