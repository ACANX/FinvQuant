-- =====================================================================
-- FinvQuant PostgreSQL V300172：恢复 V300171 误置空的 stock_id（需求 11088 修正）
--
-- 缺陷：
--   V300171 的「indicator stock_id 对齐」按 usc 关联键表回填 `stock_id = key.stock_id`，
--   但键表对这批证券的 stock_id 本身就是 NULL（尚未解析），于是 826 行原本**正确**的
--   stock_id（实测 826/826 都能在 futu.finv_futu_quote_collect.stock_id 里找到对应富途记录）
--   被写成了 NULL——属于数据回退。
--
-- 修正：
--   从归档 archive.finv_quote_secu_indicator_usc_align_20261110（V300171 改前快照）把
--   这批行的 stock_id 恢复回来；只恢复「当前为 NULL 且归档有旧值」的行，不动其它行。
--   今后对齐口径应为 `stock_id = COALESCE(key.stock_id, 原值)`，不得用 NULL 覆盖既有值。
--
-- 幂等：恢复完再跑为 0 行；末尾校验断言「这 826 行的 stock_id 均非 NULL」。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

CREATE TEMP TABLE tmp_restore ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, a.usc_from, a.stock_id AS old_stock
  FROM cdeu.finv_quote_secu_indicator i
  JOIN archive.finv_quote_secu_indicator_usc_align_20261110 a ON a.usc_from = i.usc
 WHERE a.reason LIKE '需求 11088 stock_id 对齐%'
   AND a.stock_id IS NOT NULL
   AND i.stock_id IS NULL;

-- 恢复前确认不会造成重复 stock_id
DO $$
DECLARE
    v_collision BIGINT;
BEGIN
    SELECT count(*) INTO v_collision
      FROM tmp_restore r
     WHERE EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_indicator x
                    WHERE x.stock_id = r.old_stock AND x.ctid <> r.row_ctid);
    IF v_collision <> 0 THEN
        RAISE EXCEPTION 'V300172 中止：恢复后会产生 % 行重复 stock_id', v_collision;
    END IF;
END $$;

UPDATE cdeu.finv_quote_secu_indicator i
   SET stock_id = r.old_stock
  FROM tmp_restore r
 WHERE i.ctid = r.row_ctid;

DO $$
DECLARE
    v_restored BIGINT;
    v_left     BIGINT;
    v_dup      BIGINT;
    v_key_diff BIGINT;
BEGIN
    SELECT count(*) INTO v_restored FROM tmp_restore;
    SELECT count(*) INTO v_left
      FROM cdeu.finv_quote_secu_indicator i
      JOIN archive.finv_quote_secu_indicator_usc_align_20261110 a ON a.usc_from = i.usc
     WHERE a.reason LIKE '需求 11088 stock_id 对齐%'
       AND a.stock_id IS NOT NULL
       AND i.stock_id IS NULL;
    SELECT count(*) INTO v_dup FROM (
      SELECT stock_id FROM cdeu.finv_quote_secu_indicator
       WHERE stock_id IS NOT NULL GROUP BY 1 HAVING count(*) > 1) t;
    SELECT count(*) INTO v_key_diff
      FROM cdeu.finv_quote_secu_indicator i
      JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
     WHERE i.usc <> k.usc;

    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300172 未收敛：仍有 % 行 stock_id 为 NULL', v_left;
    END IF;
    IF v_dup <> 0 THEN
        RAISE EXCEPTION 'V300172 引入 % 组重复 stock_id', v_dup;
    END IF;
    IF v_key_diff <> 0 THEN
        RAISE EXCEPTION 'V300172 回归：与键表差异 % 行', v_key_diff;
    END IF;

    RAISE NOTICE 'V300172 完成：恢复 % 行 stock_id；重复 stock_id 0 组、与键表差异 0', v_restored;
END $$;

COMMIT;
