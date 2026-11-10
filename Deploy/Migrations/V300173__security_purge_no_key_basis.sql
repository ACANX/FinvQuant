-- =====================================================================
-- FinvQuant PostgreSQL V300173：删除无键表依据的证券（需求 11089）
--
-- 用户 2026-11-10 明确要求：「剩下的 127 行（126 + EURindex）在 public.finv_security 清理掉」。
--
-- 范围（实测）：
--   · 126 个 usc：在 futu.finv_futu_quote_collect 里无键表依据（ftmm_symbol / usc 都对不上
--     futu.finv_futu_secu_key），且 public.finv_security 里有同名记录；
--   · 其中 `EURindex`（FX 欧元指数）同时是 cdeu.finv_quote_secu_indicator 的最后 1 行无依据行。
--
-- 动作：
--   ① 归档 public.finv_security 的这 126 行（写入既有 archive.finv_security_purge_20261110，
--      该表列结构与 finv_security 对齐并带 src_table / reason / backup_at，未新建表）；
--   ② 删除 public.finv_security 这 126 行；
--   ③ 删除 futu.finv_futu_quote_collect 的 126 行、cdeu.finv_quote_secu_indicator 的 1 行
--      （这两批行在 V300171 已整行归档，这里把归档 reason 改成「删除」，不重复写快照）。
--
-- 未处理（在报告里列出，等确认）：cdeu.finv_cdeu_secu_realtime_stock 110 行、
--   cdeu.finv_quote_cdeu_usc 5 行仍引用这些 usc；K 线表 0 行引用。
--
-- 幂等：全部按当前状态重算；重跑时待删为 0。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 目标集合
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_target ON COMMIT DROP AS
SELECT c.usc FROM futu.finv_futu_quote_collect c
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = c.usc)
   AND EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = c.usc)
UNION
SELECT i.usc FROM cdeu.finv_quote_secu_indicator i
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.stock_id = i.stock_id)
   AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = i.usc)
   AND EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = i.usc);

-- ---------------------------------------------------------------------
-- ② 归档 finv_security（既有归档表，列与源表同构）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_security_purge_20261110
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, init_date, timezone, tz, gmt_create, gmt_update,
        flag_enable, market_code, type_secu_sub, provider, remark, src_table, reason, backup_at)
SELECT s.usc, s.exchange_code, s.security_type, s.security_code, s.security_name, s.security_name_cn,
       s.security_name_full, s.currency_type, s.init_date, s.timezone, s.tz, s.gmt_create, s.gmt_update,
       s.flag_enable, s.market_code, s.type_secu_sub, s.provider, s.remark,
       'public.finv_security',
       '需求 11089 删除：无键表依据（futu.finv_futu_secu_key 无该证券），清理后待重建',
       now()
  FROM public.finv_security s
  JOIN tmp_target t ON t.usc = s.usc;

-- ---------------------------------------------------------------------
-- ③ 归档 reason 改标注（这两批行已在 V300171 归档过整行快照）
-- ---------------------------------------------------------------------
UPDATE archive.finv_quote_collect_usc_align_20261110 a
   SET reason = '需求 11089 删除：无键表依据，随 finv_security 一起清理'
 WHERE a.usc_from IN (SELECT usc FROM tmp_target)
   AND a.reason LIKE '需求 11088 保留%';

UPDATE archive.finv_quote_secu_indicator_usc_align_20261110 a
   SET reason = '需求 11089 删除：无键表依据，随 finv_security 一起清理'
 WHERE a.usc_from IN (SELECT usc FROM tmp_target)
   AND a.reason LIKE '需求 11088 保留%';

DO $$
DECLARE
    v_missing INT;
BEGIN
    SELECT count(*) INTO v_missing
      FROM tmp_target t
     WHERE NOT EXISTS (SELECT 1 FROM archive.finv_security_purge_20261110 a
                        WHERE a.usc = t.usc AND a.reason LIKE '需求 11089%');
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V300173 中止：% 个 usc 缺少 finv_security 归档快照', v_missing;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- ④ 删除
-- ---------------------------------------------------------------------
DELETE FROM public.finv_security s USING tmp_target t WHERE s.usc = t.usc;
DELETE FROM futu.finv_futu_quote_collect c USING tmp_target t WHERE c.usc = t.usc;
DELETE FROM cdeu.finv_quote_secu_indicator i USING tmp_target t WHERE i.usc = t.usc;

-- ---------------------------------------------------------------------
-- ⑤ 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_security_left BIGINT;
    v_col_orphan    BIGINT;
    v_ind_orphan    BIGINT;
    v_security_rows BIGINT;
    v_collect_rows  BIGINT;
    v_ind_rows      BIGINT;
    v_key_diff      BIGINT;
    v_collect_diff  BIGINT;
BEGIN
    SELECT count(*) INTO v_security_left
      FROM public.finv_security s JOIN tmp_target t ON t.usc = s.usc;
    IF v_security_left <> 0 THEN
        RAISE EXCEPTION 'V300173 未收敛：finv_security 仍残留 % 行', v_security_left;
    END IF;

    SELECT count(*) INTO v_col_orphan
      FROM futu.finv_futu_quote_collect c
     WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.ftmm_symbol = c.ftmm_symbol)
       AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = c.usc);
    SELECT count(*) INTO v_ind_orphan
      FROM cdeu.finv_quote_secu_indicator i
     WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k WHERE k.stock_id = i.stock_id)
       AND NOT EXISTS (SELECT 1 FROM futu.finv_futu_secu_key k2 WHERE k2.usc = i.usc);
    IF v_col_orphan <> 0 OR v_ind_orphan <> 0 THEN
        RAISE EXCEPTION 'V300173 未收敛：quote_collect 剩 % 行、indicator 剩 % 行无依据行',
            v_col_orphan, v_ind_orphan;
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
        RAISE EXCEPTION 'V300173 回归：与键表差异 indicator % 行 / quote_collect % 行', v_key_diff, v_collect_diff;
    END IF;

    SELECT count(*) INTO v_security_rows FROM public.finv_security;
    SELECT count(*) INTO v_collect_rows  FROM futu.finv_futu_quote_collect;
    SELECT count(*) INTO v_ind_rows      FROM cdeu.finv_quote_secu_indicator;

    RAISE NOTICE 'V300173 完成：finv_security 剩 % 行、quote_collect 剩 % 行、indicator 剩 % 行；两表无依据行 0、与键表差异 0',
        v_security_rows, v_collect_rows, v_ind_rows;
END $$;

COMMIT;
