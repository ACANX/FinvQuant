-- =====================================================================
-- FinvQuant PostgreSQL V300170：清理无键表依据的重复 stock_id（需求 11087）
--
-- 背景：
--   V300169 收尾后，cdeu.finv_quote_secu_indicator 仍有 503 组「无键表依据」的重复 stock_id
--   （每组 2 行、共 1,006 行）。这些证券不在 futu.finv_futu_secu_key 里（不在当前 73,212
--   采集清单内），因此 V300168 的对齐链路覆盖不到，但按 stock_id join 时仍会出双份。
--
-- 判定依据（本迁移的仲裁顺序）：
--   ① 首选 futu.finv_futu_quote_collect.usc（同 stock_id）：该表在 V300168/V300169 已对齐到
--      键表，是这三张表里最可信的口径；实测 502/503 组恰好有一行与之相同，保留该行、删另一行。
--   ② 组内没有任何一行与 collect 一致时（实测 1 组：stock_id 72000026 欧元指数，collect=EURindex，
--      指标表分别是 EURINDEX / FX.EURindex）：保留 dt_update 最新的那一行，并把 usc 改成 collect 的
--      值（改名后若撞主键则整次中止，不猜）。
--   ③ collect 里没有该 stock_id 的组：整组保持现状（实测 0 组），只在归档里标注。
--
-- 归档：删除行与改名行都先写入 archive.finv_quote_secu_indicator_usc_align_20261110
--       （V300168 授权建的表），reason 区分「删除 / 改名」。行内容与 V300169 一致，均为改前快照。
--
-- 幂等：全部按当前状态重算；重跑时待删/待改为 0，校验仍通过。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 重复组与仲裁结果
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_dup_groups ON COMMIT DROP AS
SELECT i.stock_id
  FROM cdeu.finv_quote_secu_indicator i
 WHERE i.stock_id IS NOT NULL
 GROUP BY i.stock_id
HAVING count(*) > 1;

-- collect 侧按 stock_id 归并：只有「同一 stock_id 的 collect.usc 唯一」时才作为依据
-- （该表有 70 组重复 stock_id，其中若 usc 不一致则不猜，整组跳过）
CREATE TEMP TABLE tmp_collect_usc ON COMMIT DROP AS
SELECT stock_id,
       CASE WHEN count(DISTINCT usc) = 1 THEN min(usc) END AS usc
  FROM futu.finv_futu_quote_collect
 WHERE stock_id IS NOT NULL
 GROUP BY stock_id;

CREATE TEMP TABLE tmp_dup_rows ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, i.stock_id, i.usc, i.ftmm_symbol, i.dt_update,
       c.usc AS collect_usc,
       row_number() OVER (
         PARTITION BY i.stock_id
         ORDER BY (i.usc = c.usc) DESC NULLS LAST, i.dt_update DESC NULLS LAST, i.usc
       ) AS keep_rank
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_dup_groups g ON g.stock_id = i.stock_id
  LEFT JOIN tmp_collect_usc c ON c.stock_id = i.stock_id;

-- 保留行（每组 1 行）：优先与 collect 一致，其次 dt_update 最新
CREATE TEMP TABLE tmp_keep ON COMMIT DROP AS
SELECT r.*, (r.usc IS DISTINCT FROM r.collect_usc) AS need_rename
  FROM tmp_dup_rows r
 WHERE r.keep_rank = 1 AND r.collect_usc IS NOT NULL;

-- 待删行：除保留行以外，且该组有 collect 依据（无依据的组整组不动）
CREATE TEMP TABLE tmp_delete ON COMMIT DROP AS
SELECT r.row_ctid, r.usc, r.stock_id, r.ftmm_symbol
  FROM tmp_dup_rows r
 WHERE r.keep_rank > 1
   AND r.collect_usc IS NOT NULL;

-- ---------------------------------------------------------------------
-- ② 归档（删除行 + 改名行，均为改前快照；缺一行快照就中止）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT d.usc, '（删除：无键表依据的重复 stock_id）', d.stock_id, d.ftmm_symbol, to_jsonb(i),
       '需求 11087 删除：无键表依据的重复 stock_id（按 quote_collect 仲裁）'
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_delete d ON d.row_ctid = i.ctid
ON CONFLICT (usc_from) DO NOTHING;

INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT k.usc, k.collect_usc, k.stock_id, k.ftmm_symbol, to_jsonb(i),
       '需求 11087 改名：组内无 collect 同值行，保留最新行并改为 collect 的 usc'
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_keep k ON k.row_ctid = i.ctid
 WHERE k.need_rename
ON CONFLICT (usc_from) DO NOTHING;

DO $$
DECLARE
    v_missing INT;
BEGIN
    SELECT count(*) INTO v_missing
      FROM tmp_delete d
     WHERE NOT EXISTS (SELECT 1 FROM archive.finv_quote_secu_indicator_usc_align_20261110 a
                        WHERE a.usc_from = d.usc);
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V300170 中止：待删行缺少归档快照（% 行）', v_missing;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- ③ 改名（仅 need_rename 的组）与删除
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_secu_indicator i
   SET usc = k.collect_usc
  FROM tmp_keep k
 WHERE i.ctid = k.row_ctid
   AND k.need_rename;

DELETE FROM cdeu.finv_quote_secu_indicator i USING tmp_delete d WHERE i.ctid = d.row_ctid;

-- ---------------------------------------------------------------------
-- ④ 收敛校验（任一不满足即抛异常 → 整个事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_dup_groups   BIGINT;
    v_mismatch     BIGINT;
    v_rows         BIGINT;
    v_deleted      BIGINT;
    v_renamed      BIGINT;
    v_key_diff     BIGINT;
BEGIN
    SELECT count(*) INTO v_dup_groups FROM (
      SELECT stock_id FROM cdeu.finv_quote_secu_indicator
       WHERE stock_id IS NOT NULL GROUP BY 1 HAVING count(*) > 1) t;

    -- 本次处理过的组（原重复组且有 collect 依据）现在必须只剩 1 行且 usc = collect.usc；
    -- 其余单行证券与 collect 的历史差异（74 行量级）不在本次范围，不做断言。
    SELECT count(*) INTO v_mismatch
      FROM cdeu.finv_quote_secu_indicator i
      JOIN tmp_collect_usc c ON c.stock_id = i.stock_id
     WHERE c.usc IS NOT NULL
       AND i.usc <> c.usc
       AND i.stock_id IN (SELECT stock_id FROM tmp_dup_groups);

    SELECT count(*) INTO v_key_diff
      FROM cdeu.finv_quote_secu_indicator i
      JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
     WHERE i.usc <> k.usc;

    SELECT count(*) INTO v_rows FROM cdeu.finv_quote_secu_indicator;
    SELECT count(*) INTO v_deleted FROM tmp_delete;
    SELECT count(*) INTO v_renamed FROM tmp_keep WHERE need_rename;

    IF v_dup_groups <> 0 THEN
        RAISE EXCEPTION 'V300170 未收敛：仍有 % 组重复 stock_id', v_dup_groups;
    END IF;
    IF v_mismatch <> 0 THEN
        RAISE EXCEPTION 'V300170 未收敛：% 行 usc 与 quote_collect 不一致', v_mismatch;
    END IF;
    IF v_key_diff <> 0 THEN
        RAISE EXCEPTION 'V300170 回归：% 行与键表不一致（V300168 的对齐被破坏）', v_key_diff;
    END IF;

    RAISE NOTICE 'V300170 完成：删除 % 行、改名 % 行，指标表剩 % 行；重复 stock_id 0 组、与 quote_collect 差异 0、与键表差异 0',
        v_deleted, v_renamed, v_rows;
END $$;

COMMIT;
