-- =====================================================================
-- FinvQuant PostgreSQL V300159：P22 提为「前处理」+ 引擎函数支持 SH4（需求 11072 修订）
--
-- 背景（用户 2026-11-10 选择方案 A）：
--   `P20 市场默认·可读型`（SH：usc = 交易所代码）与 `P22 市场默认·上交所指数`
--   （SH 的 000* 指数：usc = SH + 交易所代码后四位）在层上是同一个「市场默认」层。
--   实测（V300157 口径）两者协调失败：
--     · SH 000* 共 319 行，历史数据是「170 行 SH0001 形态 + 149 行裸 000001 形态」两种混杂；
--     · 原因是 `P22` 排在 `P10` 闸门**之后**：天然唯一的（如 SH.000300）被闸门直接定稿成裸代码，
--       只有与 SZ 撞名的（如 SH.000001 与 SZ.000001 平安银行）才轮到 P22；
--     · 且候选层按 rule.priority 排序，P20(20) < P22(22)，P22 连撞名的那些也抢不到。
--
-- 决定（方案 A）：把 P22 从「市场默认」提升为「前处理」（与 P12/P14 同级），在 P10 闸门**之前**
--   改写 canon_code：
--     SH 000* → 'SH' + right(code, 4)   （后四位实测 SH 内 319/319 唯一，且与任何 SZ 代码零交集）
--   这样 P20 与 P22 的职责变为互斥：
--     · P20：SH 非 000 的 4,412 支股票/基金 → 裸代码；
--     · P22（前处理）：SH 的 319 支 000* 指数 → SHxxxx（含天然唯一的，形态统一）。
--
-- 影响：149 行原本为裸代码的 SH 指数（如 SH.000300 的 usc `000300`）将统一为 `SH0300`。
--   这是用户明确选择的方案 A 的直接结果，不是副作用。
--
-- 本迁移做三件事：
--   1. 规则层：P22 的 rule_type 由 MARKET_DEFAULT 改为 NORMALIZE、match_scope 改为 ALL
--      （因此不再进入「市场默认 → 兜底」的候选阶梯），并更新 rule_desc；
--   2. 引擎函数：前处理由「写死 P12/P14」改为按 rule_type='NORMALIZE' 取条目，支持
--      LITERAL / REWRITE / SH4 三种动作（SH4 = 'SH' || right(code,4)）；
--   3. 收敛校验：抽样断言 SH 指数、SZ 股票、HK 指数、期货后缀的 canon_code。
--
-- 幂等：UPDATE 带目标值判断、函数 CREATE OR REPLACE，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. P22 提为前处理
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_usc_rule
   SET rule_type   = 'NORMALIZE',
       match_scope = 'ALL',
       rule_desc   = E'把上交所 000* 指数的 usc 提前改写为 SH + 交易所代码后四位（前处理，先于 P10 闸门）。\n'
                  || E'原因：裸 000001 会被 SZ.000001（平安银行）占用；SH 内后四位实测 319/319 唯一，且与任何 SZ 代码零交集。\n'
                  || E'示例：SH.000001 → SH0001；SH.000300 → SH0300。\n'
                  || E'与 P20 的分工：P20 管 SH 非 000 的股票/基金（裸代码），P22 管 SH 的 000* 指数（SHxxxx）。',
       remark      = trim(both ' ' FROM COALESCE(remark, '') ||
                     ' | 2026-11-10 方案 A：由市场默认层提为前处理层，保证 319 支 SH 指数形态统一（需求 11072）')
 WHERE rule_no = 'P22'
   AND (rule_type <> 'NORMALIZE' OR match_scope <> 'ALL');

-- ---------------------------------------------------------------------
-- 2. 引擎函数重定义：前处理按 rule_type 驱动，支持 SH4
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION futu.f_finv_futu_usc_calc(p_env TEXT DEFAULT '*')
RETURNS TABLE (
    ftmm_symbol TEXT,
    market      TEXT,
    code        TEXT,
    instrument  TEXT,
    canon_code  TEXT,
    canon_cnt   BIGINT
) AS $$
    WITH pre_rules AS (
        SELECT i.market, i.instrument, i.code, i.code_regex, i.action,
               i.usc_literal, i.rewrite_tpl,
               ru.priority AS rule_priority, i.priority AS item_priority
          FROM futu.finv_futu_usc_rule_item i
          JOIN futu.finv_futu_usc_rule ru ON ru.rule_no = i.rule_no
         WHERE ru.flag_enable = '1'
           AND ru.rule_type = 'NORMALIZE'
           AND i.status = 'ACTIVE'
           AND i.scope_env IN ('*', p_env)
           AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
           AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
    ), pre AS (
        SELECT k.ftmm_symbol, k.market, k.code, k.instrument,
               COALESCE(m.canon, k.code) AS canon_code
          FROM futu.finv_futu_secu_key k
          LEFT JOIN LATERAL (
                SELECT CASE COALESCE(p.action, 'LITERAL')
                         WHEN 'LITERAL' THEN p.usc_literal
                         WHEN 'REWRITE' THEN regexp_replace(k.code, p.code_regex, p.rewrite_tpl, 'i')
                         WHEN 'SH4'     THEN 'SH' || right(k.code, 4)
                         ELSE k.code
                       END AS canon
                  FROM pre_rules p
                 WHERE (p.market     IS NULL OR p.market     = k.market)
                   AND (p.instrument IS NULL OR p.instrument = k.instrument)
                   AND ( (p.code IS NOT NULL AND lower(p.code) = lower(k.code))
                      OR (p.code IS NULL AND p.code_regex IS NOT NULL AND k.code ~* p.code_regex)
                      OR (p.code IS NULL AND p.code_regex IS NULL AND p.usc_literal IS NOT NULL) )
                 ORDER BY p.rule_priority, p.item_priority
                 LIMIT 1) m ON true
    ), cnt AS (
        SELECT lower(p.canon_code) AS k, count(*) AS n FROM pre p GROUP BY 1
    )
    SELECT p.ftmm_symbol, p.market, p.code, p.instrument, p.canon_code, c.n
      FROM pre p
      JOIN cnt c ON c.k = lower(p.canon_code);
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION futu.f_finv_futu_usc_calc(TEXT) IS 'USC 映射引擎：按规则条目计算每只证券的前处理代码 canon_code 与大小写不敏感计数 canon_cnt（canon_cnt=1 即 P10 闸门命中）。前处理按 rule_type=NORMALIZE 取条目，支持 LITERAL / REWRITE / SH4；p_env 为环境标识（* 表示全部环境）';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type     TEXT;
    v_scope    TEXT;
    v_sh1      TEXT;
    v_sh300    TEXT;
    v_sz1      TEXT;
    v_hk800    TEXT;
    v_hk700    TEXT;
    v_fx       TEXT;
    v_futures  TEXT;
BEGIN
    SELECT rule_type, match_scope INTO v_type, v_scope
      FROM futu.finv_futu_usc_rule WHERE rule_no = 'P22';
    IF v_type <> 'NORMALIZE' OR v_scope <> 'ALL' THEN
        RAISE EXCEPTION 'V300159 校验失败：P22 应为 NORMALIZE/ALL，实际 % / %', v_type, v_scope;
    END IF;

    SELECT canon_code INTO v_sh1   FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'SH.000001';
    SELECT canon_code INTO v_sh300 FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'SH.000300';
    SELECT canon_code INTO v_sz1   FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'SZ.000001';
    SELECT canon_code INTO v_hk800 FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'HK.800700';
    SELECT canon_code INTO v_hk700 FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'HK.00700';

    IF v_sh1 IS DISTINCT FROM 'SH0001' THEN
        RAISE EXCEPTION 'V300159 校验失败：SH.000001 的 canon_code 应为 SH0001，实际 %', v_sh1;
    END IF;
    IF v_sh300 IS DISTINCT FROM 'SH0300' THEN
        RAISE EXCEPTION 'V300159 校验失败：SH.000300 的 canon_code 应为 SH0300，实际 %', v_sh300;
    END IF;
    IF v_sz1 IS DISTINCT FROM '000001' THEN
        RAISE EXCEPTION 'V300159 校验失败：SZ.000001 的 canon_code 应保持 000001，实际 %', v_sz1;
    END IF;
    IF v_hk800 IS DISTINCT FROM 'HTI' THEN
        RAISE EXCEPTION 'V300159 校验失败：HK.800700 的 canon_code 应为 HTI，实际 %', v_hk800;
    END IF;
    IF v_hk700 IS DISTINCT FROM '00700' THEN
        RAISE EXCEPTION 'V300159 校验失败：HK.00700 的 canon_code 应保持 00700，实际 %', v_hk700;
    END IF;

    IF EXISTS (SELECT 1 FROM futu.finv_futu_secu_key WHERE market = 'FX' AND code = 'USDINDEX') THEN
        SELECT canon_code INTO v_fx FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'FX.USDINDEX';
        IF v_fx IS DISTINCT FROM 'USDIndex' THEN
            RAISE EXCEPTION 'V300159 校验失败：FX.USDINDEX 的 canon_code 应为 USDIndex，实际 %', v_fx;
        END IF;
    END IF;

    IF EXISTS (SELECT 1 FROM futu.finv_futu_secu_key WHERE market = 'HK' AND code = 'AACMAIN') THEN
        SELECT canon_code INTO v_futures FROM futu.f_finv_futu_usc_calc('*') WHERE ftmm_symbol = 'HK.AACMAIN';
        IF v_futures IS DISTINCT FROM 'AACMain' THEN
            RAISE EXCEPTION 'V300159 校验失败：HK.AACMAIN 的 canon_code 应为 AACMain，实际 %', v_futures;
        END IF;
    END IF;

    RAISE NOTICE 'V300159 就绪：P22 → NORMALIZE/ALL；SH.000001→%, SH.000300→%, SZ.000001→%', v_sh1, v_sh300, v_sz1;
END $$;

COMMIT;
