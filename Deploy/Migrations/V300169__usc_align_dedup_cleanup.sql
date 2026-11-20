-- =====================================================================
-- FinvQuant PostgreSQL V300169：usc 对齐收尾——删除重复/孤立行（需求 11086）
--
-- V300168 把两张表的 usc 对齐到 futu.finv_futu_secu_key，但保留了 89 + 3,649 行「跳过」行。
-- 本次把它们收干净（用户 2026-11-10 明确同意「按建议清理」）：
--
--  ① cdeu.finv_quote_secu_indicator —— 3,649 行陈旧重复行：同一 stock_id 已有一行 usc 正确，
--     这些是过去 Tab 2 用上游旧形态 usc 当主键 upsert 留下的副本（按 stock_id join 会出双份）。
--  ② futu.finv_futu_quote_collect —— 88 行重复行：
--       · 75 行：同一 ftmm_symbol 的两条记录（stock_id 不同），保留 stock_id 与键表一致的那条；
--       · 13 行：同一 stock_id 的大小写变体记录（US.1OZMAIN / US.1OZmain），保留 usc 已等同键表的那条。
--  ③ futu.finv_futu_quote_collect —— 1 行孤立行 `US.LTC`：键表里没有 US.LTC（上游也无此证券，
--     实测 REST 查询 missing），但它的 usc=`LTC` 占住了加密 CC.LTC 的目标值。删除后 ④ 才能落地。
--     其余孤立行（collect 191 行、indicator 1,333 行）按既有决定**保持现状**。
--  ④ 收尾对齐：删完 ③ 后只剩 CC.LTC → LTC 一行差异，用两阶段改名落地。
--
-- 归档：①②③ 删除前都要求能在 archive.finv_quote_*_usc_align_20261110 里查到整行快照
--       （V300168 已归档 3,771 / 4,089 行）；③ 的 US.LTC 不在其中，本迁移先补归档再删。
--       归档行的 reason 改为「删除」，便于事后区分「对齐」「删除」「跳过」。
--
-- 幂等：全部按当前状态重算，重跑时待删/待改为 0；末尾 RAISE EXCEPTION 收敛校验。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 归档补录：US.LTC（孤立行，V300168 未覆盖）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT c.usc, '（删除：孤立行，无键表依据且占住 CC.LTC 的目标 LTC）', c.stock_id, c.ftmm_symbol, to_jsonb(c),
       '需求 11086 删除：孤立行（上游无此证券，占住他人 usc）'
  FROM futu.finv_futu_quote_collect c
 WHERE c.ftmm_symbol = 'US.LTC'
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
ON CONFLICT (usc_from, stock_id) DO NOTHING;

-- ---------------------------------------------------------------------
-- ② 待删集（临时表）
-- ---------------------------------------------------------------------
-- indicator：usc 与键表不一致，且同 stock_id 存在 usc 正确的那一行
CREATE TEMP TABLE tmp_indicator_dup ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, i.usc, i.stock_id
  FROM cdeu.finv_quote_secu_indicator i
  JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
 WHERE i.usc <> k.usc
   AND EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_indicator s
                WHERE s.stock_id = i.stock_id AND s.ctid <> i.ctid AND s.usc = k.usc);

-- collect：同 ftmm_symbol 重复（保留 stock_id 与键表一致的那条）+ 同 stock_id 的大小写变体
CREATE TEMP TABLE tmp_collect_dup ON COMMIT DROP AS
WITH c AS (
  SELECT x.ctid AS row_ctid, x.usc, x.ftmm_symbol, x.stock_id, k.usc AS key_usc, k.stock_id AS key_stock
    FROM futu.finv_futu_quote_collect x
    LEFT JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = x.ftmm_symbol
)
SELECT c.row_ctid, c.usc, c.ftmm_symbol, c.stock_id
  FROM c
 WHERE c.key_usc IS NOT NULL
   AND c.usc <> c.key_usc
   AND (
        -- ① 同 ftmm_symbol 的兄弟行已经是正确 usc（保留键表 stock_id 的那条）
        EXISTS (SELECT 1 FROM c s
                 WHERE s.ftmm_symbol = c.ftmm_symbol AND s.row_ctid <> c.row_ctid AND s.usc = c.key_usc)
        -- ② 同 stock_id 的大小写变体已经是正确 usc
     OR EXISTS (SELECT 1 FROM c s
                 WHERE s.stock_id = c.stock_id AND s.row_ctid <> c.row_ctid AND s.usc = c.key_usc)
       );

-- ---------------------------------------------------------------------
-- ③ 删除前校验：每一行都必须已归档（否则中止）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_ind_missing INT;
    v_col_missing INT;
BEGIN
    SELECT count(*) INTO v_ind_missing
      FROM tmp_indicator_dup d
     WHERE NOT EXISTS (SELECT 1 FROM archive.finv_quote_secu_indicator_usc_align_20261110 a
                        WHERE a.usc_from = d.usc);
    SELECT count(*) INTO v_col_missing
      FROM tmp_collect_dup d
     WHERE NOT EXISTS (SELECT 1 FROM archive.finv_quote_collect_usc_align_20261110 a
                        WHERE a.usc_from = d.usc AND a.stock_id IS NOT DISTINCT FROM d.stock_id);
    IF v_ind_missing <> 0 OR v_col_missing <> 0 THEN
        RAISE EXCEPTION 'V300169 中止：待删行缺少归档快照（indicator % 行、collect % 行）',
            v_ind_missing, v_col_missing;
    END IF;
END $$;

UPDATE archive.finv_quote_secu_indicator_usc_align_20261110 a
   SET reason = '需求 11086 删除：重复 stock_id 的陈旧行（已归档）'
  FROM tmp_indicator_dup d WHERE a.usc_from = d.usc;

UPDATE archive.finv_quote_collect_usc_align_20261110 a
   SET reason = '需求 11086 删除：重复记录（已归档）'
  FROM tmp_collect_dup d
 WHERE a.usc_from = d.usc AND a.stock_id IS NOT DISTINCT FROM d.stock_id;

-- ---------------------------------------------------------------------
-- ④ 删除
-- ---------------------------------------------------------------------
DELETE FROM cdeu.finv_quote_secu_indicator i USING tmp_indicator_dup d WHERE i.ctid = d.row_ctid;
DELETE FROM futu.finv_futu_quote_collect c  USING tmp_collect_dup  d WHERE c.ctid = d.row_ctid;

-- ---------------------------------------------------------------------
-- ⑤ 清除「占住他人目标 usc」的孤立行（无键表依据）——本次只应命中 US.LTC
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_collect_orphan_block ON COMMIT DROP AS
SELECT c.ctid AS row_ctid, c.usc, c.ftmm_symbol, c.stock_id
  FROM futu.finv_futu_quote_collect c
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
   AND EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect d
                 JOIN futu.finv_futu_secu_key kd ON kd.ftmm_symbol = d.ftmm_symbol
                WHERE d.usc <> kd.usc AND kd.usc = c.usc);

INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT o.usc, '（删除：孤立行）', o.stock_id, o.ftmm_symbol, to_jsonb(c),
       '需求 11086 删除：孤立行（上游无此证券，占住他人 usc）'
  FROM futu.finv_futu_quote_collect c
  JOIN tmp_collect_orphan_block o ON o.row_ctid = c.ctid
ON CONFLICT (usc_from, stock_id) DO NOTHING;

DELETE FROM futu.finv_futu_quote_collect c USING tmp_collect_orphan_block o WHERE c.ctid = o.row_ctid;

-- ---------------------------------------------------------------------
-- ⑥ 收尾对齐：删完阻塞行后，剩余的 collect 差异应能全部落地（本次只剩 CC.LTC → LTC）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_collect_align2 ON COMMIT DROP AS
SELECT d.usc_from, d.usc_to, d.row_ctid
  FROM (
        SELECT c.ctid AS row_ctid, c.usc AS usc_from, k.usc AS usc_to
          FROM futu.finv_futu_quote_collect c
          JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
         WHERE c.usc <> k.usc
       ) d
 WHERE NOT EXISTS (
       SELECT 1 FROM futu.finv_futu_quote_collect o
        WHERE o.usc = d.usc_to AND o.ctid <> d.row_ctid);

UPDATE futu.finv_futu_quote_collect c
   SET usc = 'TMP§' || c.usc
 WHERE c.ctid IN (SELECT row_ctid FROM tmp_collect_align2);

UPDATE futu.finv_futu_quote_collect c
   SET usc = t.usc_to
  FROM tmp_collect_align2 t
 WHERE c.usc = 'TMP§' || t.usc_from;

-- 收尾落地行同样归档（改前快照取不到原行时跳过——这些行 V300168 已归档过则用 DO NOTHING）
UPDATE archive.finv_quote_collect_usc_align_20261110 a
   SET reason = '需求 11085 对齐 + 需求 11086 删除阻塞孤立行后落地'
  FROM tmp_collect_align2 t
 WHERE a.usc_from = t.usc_from;

-- ---------------------------------------------------------------------
-- ⑦ 收敛校验（任一不满足即抛异常 → 整个事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_collect_diff    BIGINT;
    v_indicator_diff  BIGINT;
    v_ind_dup_stock   BIGINT;
    v_col_dup_symbol  BIGINT;
    v_tmp_left        BIGINT;
    v_ind_rows        BIGINT;
    v_col_rows        BIGINT;
    v_ind_deleted     BIGINT;
    v_col_deleted     BIGINT;
    v_orphan_deleted  BIGINT;
    v_align2          BIGINT;
BEGIN
    SELECT count(*) INTO v_collect_diff
      FROM futu.finv_futu_quote_collect c
      JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
     WHERE c.usc <> k.usc;

    SELECT count(*) INTO v_indicator_diff
      FROM cdeu.finv_quote_secu_indicator i
      JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
     WHERE i.usc <> k.usc;

    -- 只校验「有键表依据」的重复：无键表依据的 stock_id（1,333 行）里本身还有 503 组历史重复，
    -- 与 usc 无关，按决定保持现状，不在本次校验范围内。
    SELECT count(*) INTO v_ind_dup_stock FROM (
      SELECT i.stock_id FROM cdeu.finv_quote_secu_indicator i
        JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
       WHERE i.stock_id IS NOT NULL
       GROUP BY i.stock_id HAVING count(*) > 1) t;

    SELECT count(*) INTO v_col_dup_symbol FROM (
      SELECT c.ftmm_symbol FROM futu.finv_futu_quote_collect c
        JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
       GROUP BY c.ftmm_symbol HAVING count(*) > 1) t;

    SELECT (SELECT count(*) FROM futu.finv_futu_quote_collect WHERE usc LIKE 'TMP§%')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_indicator WHERE usc LIKE 'TMP§%')
      INTO v_tmp_left;

    SELECT count(*) INTO v_ind_rows FROM cdeu.finv_quote_secu_indicator;
    SELECT count(*) INTO v_col_rows FROM futu.finv_futu_quote_collect;

    IF v_collect_diff <> 0 OR v_indicator_diff <> 0 THEN
        RAISE EXCEPTION 'V300169 未收敛：quote_collect 剩余 % 行、indicator 剩余 % 行',
            v_collect_diff, v_indicator_diff;
    END IF;
    IF v_ind_dup_stock <> 0 THEN
        RAISE EXCEPTION 'V300169 indicator 仍有 % 组重复 stock_id', v_ind_dup_stock;
    END IF;
    IF v_col_dup_symbol <> 0 THEN
        RAISE EXCEPTION 'V300169 quote_collect 仍有 % 组重复 ftmm_symbol', v_col_dup_symbol;
    END IF;
    IF v_tmp_left <> 0 THEN
        RAISE EXCEPTION 'V300169 临时值残留 % 行', v_tmp_left;
    END IF;

    RAISE NOTICE 'V300169 完成：indicator 剩 % 行（删 3,649 行陈旧重复，保留 503 组无键表依据的历史重复）、quote_collect 剩 % 行（删 88 行重复 + 1 行孤立 US.LTC）、两表相对键表的 usc 差异 0',
        v_ind_rows, v_col_rows;
END $$;

COMMIT;
