-- =====================================================================
-- FinvQuant PostgreSQL V300171：清理孤立行（需求 11088）
--
-- V300170 收尾后，两张表还剩「无键表依据」的行（stock_id / ftmm_symbol 不在
-- futu.finv_futu_secu_key 内）。逐条查证后分两类处理——**能修的先修，真无用的才删**：
--
--  ① indicator 826 行：usc 本身就在键表里，只是 stock_id / ftmm_symbol 是旧值
--     （例如 usc=000498 的行挂着 stock_id=43035574306418，键表里 000498 的 stock_id 是别的值）。
--     按 usc 关联键表回填 stock_id + ftmm_symbol → 不再是无依据行，且不丢任何一条证券。
--  ② quote_collect 64 行：usc 已在键表里，但 ftmm_symbol 大小写/市场前缀与键表不一致
--     （HK.CNCmain vs HK.CNCMAIN、HK.BK1191 vs BK.BK1191、FX.USDindex vs FX.USDINDEX）。
--     按 usc 关联键表回填 market_label + futu_symbol（生成列 ftmm_symbol 随之对齐）→ 不再是无依据行。
--  ③ 真无用的孤立行删除（实测 4 行，均无任何下游引用）：
--     · quote_collect：US.SMCF（键表无此证券，finv_security / 实时 / K 线 / 指标表均无）
--     · indicator：USDINDEX（正确写法是 USDIndex，键表无 FX 指数）、.DJUSNS、000117
--  ④ 保留并留痕（有下游引用，删了会让既有页面缺属性）：
--     · quote_collect 126 行：`public.finv_security` 有记录（110 行还有实时行情）
--     · indicator 1 行（EURindex）：`public.finv_security` 在用
--     这些行只写归档标注，不改不删，等人工确认是否连下游一起处理。
--
-- 归档：改动行与删除行都先写 archive.finv_quote_*_usc_align_20261110（V300168 授权建的表），
--       reason 区分「stock_id 对齐 / 符号对齐 / 删除」。
-- 幂等：全部按当前状态重算；重跑时待改/待删为 0。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 待改集
-- ---------------------------------------------------------------------
-- indicator：usc 在键表里但 stock_id 不在（旧 stock_id / 旧 ftmm_symbol）
CREATE TEMP TABLE tmp_ind_fix ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, i.usc, i.stock_id AS old_stock, i.ftmm_symbol AS old_symbol,
       k.stock_id AS new_stock, k.ftmm_symbol AS new_symbol
  FROM cdeu.finv_quote_secu_indicator i
  JOIN futu.finv_futu_secu_key k ON k.usc = i.usc
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k1 WHERE k1.stock_id = i.stock_id);

-- collect：usc 在键表里但 ftmm_symbol 不在（大小写 / 市场前缀不一致）
CREATE TEMP TABLE tmp_col_fix ON COMMIT DROP AS
SELECT c.ctid AS row_ctid, c.usc, c.ftmm_symbol AS old_symbol, k.ftmm_symbol AS new_symbol,
       split_part(k.ftmm_symbol, '.', 1) AS new_label,
       substring(k.ftmm_symbol FROM strpos(k.ftmm_symbol, '.') + 1) AS new_futu_symbol
  FROM futu.finv_futu_quote_collect c
  JOIN futu.finv_futu_secu_key k ON k.usc = c.usc
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k1 WHERE k1.ftmm_symbol = c.ftmm_symbol);

-- ---------------------------------------------------------------------
-- ② 待删集（无键表依据 + 无任何下游引用）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_col_del ON COMMIT DROP AS
SELECT c.ctid AS row_ctid, c.usc, c.ftmm_symbol, c.stock_id
  FROM futu.finv_futu_quote_collect c
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = c.usc)
   AND NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = c.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_cdeu_secu_realtime_stock r WHERE r.usc = c.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc u WHERE u.usc = c.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_indicator i WHERE i.usc = c.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_day d WHERE d.usc = c.usc);

CREATE TEMP TABLE tmp_ind_del ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, i.usc, i.stock_id, i.ftmm_symbol
  FROM cdeu.finv_quote_secu_indicator i
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.stock_id = i.stock_id)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = i.usc)
   AND NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = i.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_cdeu_secu_realtime_stock r WHERE r.usc = i.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc u WHERE u.usc = i.usc)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect c WHERE c.usc = i.usc)
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_day d WHERE d.usc = i.usc);

-- ---------------------------------------------------------------------
-- ③ 归档（改动行 + 删除行，均为改前快照）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT f.usc, f.usc, f.old_stock, f.old_symbol, to_jsonb(i),
       '需求 11088 stock_id 对齐（按 usc 关联键表回填 stock_id / ftmm_symbol）'
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_ind_fix f ON f.row_ctid = i.ctid
ON CONFLICT (usc_from) DO NOTHING;

INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT d.usc, '（删除：无键表依据且无下游引用）', d.stock_id, d.ftmm_symbol, to_jsonb(i),
       '需求 11088 删除：无键表依据且无任何下游引用的孤立行'
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_ind_del d ON d.row_ctid = i.ctid
ON CONFLICT (usc_from) DO NOTHING;

INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT f.usc, f.usc, c.stock_id, f.old_symbol, to_jsonb(c),
       '需求 11088 符号对齐（按 usc 关联键表回填 market_label / futu_symbol）'
  FROM futu.finv_futu_quote_collect c
  JOIN tmp_col_fix f ON f.row_ctid = c.ctid
ON CONFLICT (usc_from, stock_id) DO NOTHING;

INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT d.usc, '（删除：无键表依据且无下游引用）', d.stock_id, d.ftmm_symbol, to_jsonb(c),
       '需求 11088 删除：无键表依据且无任何下游引用的孤立行'
  FROM futu.finv_futu_quote_collect c
  JOIN tmp_col_del d ON d.row_ctid = c.ctid
ON CONFLICT (usc_from, stock_id) DO NOTHING;

-- 保留行也留痕（不改不删，只记录为什么留着）
INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT c.usc, '（保留：无键表依据但 finv_security / 实时行情 在用）', c.stock_id, c.ftmm_symbol, to_jsonb(c),
       '需求 11088 保留：无键表依据但有下游引用（待人工确认）'
  FROM futu.finv_futu_quote_collect c
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = c.usc)
   AND EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = c.usc)
ON CONFLICT (usc_from, stock_id) DO NOTHING;

-- ---------------------------------------------------------------------
-- ④ 执行：先改、后删
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_secu_indicator i
   SET stock_id = f.new_stock,
       ftmm_symbol = f.new_symbol
  FROM tmp_ind_fix f
 WHERE i.ctid = f.row_ctid;

UPDATE futu.finv_futu_quote_collect c
   SET market_label = f.new_label,
       futu_symbol = f.new_futu_symbol
  FROM tmp_col_fix f
 WHERE c.ctid = f.row_ctid;

DELETE FROM cdeu.finv_quote_secu_indicator i USING tmp_ind_del d WHERE i.ctid = d.row_ctid;
DELETE FROM futu.finv_futu_quote_collect c USING tmp_col_del d WHERE c.ctid = d.row_ctid;

-- ---------------------------------------------------------------------
-- ⑤ 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_ind_orphan  BIGINT;
    v_col_orphan  BIGINT;
    v_ind_dup     BIGINT;
    v_col_dup     BIGINT;
    v_key_diff    BIGINT;
    v_collect_diff BIGINT;
    v_fix_ind     BIGINT;
    v_fix_col     BIGINT;
    v_del_ind     BIGINT;
    v_del_col     BIGINT;
BEGIN
    SELECT count(*) INTO v_fix_ind FROM tmp_ind_fix;
    SELECT count(*) INTO v_fix_col FROM tmp_col_fix;
    SELECT count(*) INTO v_del_ind FROM tmp_ind_del;
    SELECT count(*) INTO v_del_col FROM tmp_col_del;

    -- 整改后剩余的无依据行：只允许「有下游引用而保留」的那些
    SELECT count(*) INTO v_ind_orphan
      FROM cdeu.finv_quote_secu_indicator i
     WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.stock_id = i.stock_id)
       AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = i.usc);
    SELECT count(*) INTO v_col_orphan
      FROM futu.finv_futu_quote_collect c
     WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
       AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = c.usc);

    IF v_ind_orphan > 1 OR v_col_orphan > 126 THEN
        RAISE EXCEPTION 'V300171 未收敛：indicator 剩 % 行、quote_collect 剩 % 行（应分别 ≤ 1 / ≤ 126）',
            v_ind_orphan, v_col_orphan;
    END IF;

    SELECT count(*) INTO v_ind_dup FROM (
      SELECT stock_id FROM cdeu.finv_quote_secu_indicator
       WHERE stock_id IS NOT NULL GROUP BY 1 HAVING count(*) > 1) t;
    SELECT count(*) INTO v_col_dup FROM (
      SELECT ftmm_symbol FROM futu.finv_futu_quote_collect GROUP BY 1 HAVING count(*) > 1) t;
    IF v_ind_dup <> 0 OR v_col_dup <> 0 THEN
        RAISE EXCEPTION 'V300171 引入重复：indicator % 组 / quote_collect % 组', v_ind_dup, v_col_dup;
    END IF;

    SELECT count(*) INTO v_key_diff
      FROM cdeu.finv_quote_secu_indicator i
      JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
     WHERE i.usc <> k.usc;
    SELECT count(*) INTO v_collect_diff
      FROM futu.finv_futu_quote_collect c
      JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
     WHERE c.usc <> k.usc;
    IF v_key_diff <> 0 OR v_collect_diff <> 0 THEN
        RAISE EXCEPTION 'V300171 回归：与键表差异 indicator % 行 / quote_collect % 行', v_key_diff, v_collect_diff;
    END IF;

    RAISE NOTICE 'V300171 完成：indicator 对齐 % 行、删除 % 行（剩无依据 % 行）；quote_collect 对齐 % 行、删除 % 行（剩无依据 % 行）；与键表差异 0',
        v_fix_ind, v_del_ind, v_ind_orphan, v_fix_col, v_del_col, v_col_orphan;
END $$;

COMMIT;
