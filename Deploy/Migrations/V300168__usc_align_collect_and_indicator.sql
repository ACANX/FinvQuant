-- =====================================================================
-- FinvQuant PostgreSQL V300168：usc 一次性对齐（需求 11085）
--
-- 背景：
--   USC 映射规则（需求 11072）把 usc 统一写进 futu.finv_futu_usc_map 与
--   futu.finv_futu_secu_key，但下游两张表当年按旧形态落库，一直没跟上：
--     · futu.finv_futu_quote_collect  —— 3,771 行 usc 与键表不一致（其中 3,689 行仍是
--       「usc = ftmm_symbol」的占位形态，如 CC.AAVE 而键表已是 AAVE）
--     · cdeu.finv_quote_secu_indicator —— 4,089 行不一致（按 stock_id 关联；市场前缀 3,432、
--       前导点 322、其它 335，如 .BFLY 而键表是 IDX.BFLY）
--   本迁移按「futu.finv_futu_secu_key 为唯一权威」一次性对齐这两张表。
--
-- 授权依据（用户 2026-11-10 会话明确点名）：
--   · 新建 archive.finv_quote_collect_usc_align_20261110
--   · 新建 archive.finv_quote_secu_indicator_usc_align_20261110
--   · 无键表依据的孤儿行（collect 192 行、indicator 1,333 行）与「目标 usc 已被占用」的行
--     （collect 89 行）一律保持现状，只归档留痕、不改数据
--
-- 关键约束：
--   · 两表主键都是 usc，且存在「链式改名」（CC.ADAEUR → ADAEUR，而 ADAEUR 被
--     CC.ADAEUR.FTUS 占着，后者应改 ADAEUR.FTUS），因此两阶段：
--     先把待改行改成 'TMP§' || 旧值（旧值即主键、必然唯一），再落到目标值。
--   · 物理行用 ctid 标识：quote_collect 存在「同一 ftmm_symbol 两行、stock_id 不同」
--     （实测 75 组 / 150 行），只按 ftmm_symbol 比对会漏判占用。
--   · 改前整行快照进归档表（DbSchemaSpec §0.7），失败/回滚后可从 row_json 反向恢复。
--   · 只改两张表的 usc，不动其它列、不动行数。
--
-- 幂等：变更集每次都从 key 表现算，重跑时为空 → 0 行归档、0 行更新、校验通过。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 归档表（本次对齐专用，保留可追溯）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_quote_collect_usc_align_20261110 (
    idn         BIGSERIAL PRIMARY KEY,
    usc_from    TEXT        NOT NULL,
    usc_to      TEXT        NOT NULL,
    stock_id    BIGINT,
    ftmm_symbol TEXT,
    row_json    JSONB       NOT NULL,
    reason      TEXT        NOT NULL DEFAULT '需求 11085 usc 一次性对齐',
    dt_snapshot TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_finv_quote_collect_usc_align UNIQUE (usc_from, stock_id)
);

COMMENT ON TABLE  archive.finv_quote_collect_usc_align_20261110 IS 'futu.finv_futu_quote_collect 的 usc 一次性对齐归档（需求 11085），改前整行快照';
COMMENT ON COLUMN archive.finv_quote_collect_usc_align_20261110.usc_from    IS '改前 usc（旧形态）';
COMMENT ON COLUMN archive.finv_quote_collect_usc_align_20261110.usc_to      IS '改后 usc（来自 futu.finv_futu_secu_key）';
COMMENT ON COLUMN archive.finv_quote_collect_usc_align_20261110.row_json    IS '改前整行快照';
COMMENT ON COLUMN archive.finv_quote_collect_usc_align_20261110.reason      IS '处理结论（对齐 / 跳过原因）';
COMMENT ON COLUMN archive.finv_quote_collect_usc_align_20261110.dt_snapshot IS '归档时间';

CREATE TABLE IF NOT EXISTS archive.finv_quote_secu_indicator_usc_align_20261110 (
    idn         BIGSERIAL PRIMARY KEY,
    usc_from    TEXT        NOT NULL,
    usc_to      TEXT        NOT NULL,
    stock_id    BIGINT,
    ftmm_symbol TEXT,
    row_json    JSONB       NOT NULL,
    reason      TEXT        NOT NULL DEFAULT '需求 11085 usc 一次性对齐',
    dt_snapshot TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_finv_quote_secu_indicator_usc_align UNIQUE (usc_from)
);

COMMENT ON TABLE  archive.finv_quote_secu_indicator_usc_align_20261110 IS 'cdeu.finv_quote_secu_indicator 的 usc 一次性对齐归档（需求 11085），改前整行快照';
COMMENT ON COLUMN archive.finv_quote_secu_indicator_usc_align_20261110.usc_from    IS '改前 usc（旧形态）';
COMMENT ON COLUMN archive.finv_quote_secu_indicator_usc_align_20261110.usc_to      IS '改后 usc（来自 futu.finv_futu_secu_key）';
COMMENT ON COLUMN archive.finv_quote_secu_indicator_usc_align_20261110.row_json    IS '改前整行快照';
COMMENT ON COLUMN archive.finv_quote_secu_indicator_usc_align_20261110.reason      IS '处理结论（对齐 / 跳过原因）';
COMMENT ON COLUMN archive.finv_quote_secu_indicator_usc_align_20261110.dt_snapshot IS '归档时间';

-- ---------------------------------------------------------------------
-- ② 变更集（临时表，事务结束自动删除）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_collect_diff ON COMMIT DROP AS
SELECT c.ctid AS row_ctid, c.usc AS usc_from, k.usc AS usc_to, c.stock_id, c.ftmm_symbol
  FROM futu.finv_futu_quote_collect c
  JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
 WHERE c.usc <> k.usc;

-- 可执行集：排除「目标 usc 已被一张不移动的行占着」的 89 行
--   ① 75 行：同一 ftmm_symbol 的重复行（同证券两条记录、stock_id 不同），其中一条已等于目标值；
--   ② 14 行：另一证券或大小写变体已占住该 usc（如 CC.LTC 与 US.LTC 都要 LTC）——映射冲突，另案处理。
--   · 同一目标 usc 只允许一行落地（DISTINCT ON usc_to，按 ctid 稳定取第一个），
--     其余同目标行保持现状、归档标注（多出来的都是重复记录，删除需另案授权）。
CREATE TEMP TABLE tmp_collect_align ON COMMIT DROP AS
SELECT DISTINCT ON (usc_to) *
  FROM (
        SELECT d.*
          FROM tmp_collect_diff d
         WHERE NOT EXISTS (
               SELECT 1 FROM futu.finv_futu_quote_collect o
                WHERE o.usc = d.usc_to
                  AND o.ctid <> d.row_ctid
                  AND o.ctid NOT IN (SELECT row_ctid FROM tmp_collect_diff))
       ) x
 ORDER BY usc_to, row_ctid;

CREATE TEMP TABLE tmp_indicator_diff ON COMMIT DROP AS
SELECT i.ctid AS row_ctid, i.usc AS usc_from, k.usc AS usc_to, i.stock_id, i.ftmm_symbol
  FROM cdeu.finv_quote_secu_indicator i
  JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
 WHERE i.usc <> k.usc;

-- 指标表有 4,148 组重复 stock_id（同一证券存在「旧形态 usc」与「新形态 usc」两行），
-- 因此同一个目标 usc 可能被多行争用：DISTINCT ON (usc_to) 只让一行落地，其余跳过归档。
CREATE TEMP TABLE tmp_indicator_align ON COMMIT DROP AS
SELECT DISTINCT ON (usc_to) *
  FROM (
        SELECT d.*
          FROM tmp_indicator_diff d
         WHERE NOT EXISTS (
               SELECT 1 FROM cdeu.finv_quote_secu_indicator o
                WHERE o.usc = d.usc_to
                  AND o.ctid <> d.row_ctid
                  AND o.ctid NOT IN (SELECT row_ctid FROM tmp_indicator_diff))
       ) x
 ORDER BY usc_to, row_ctid;

-- ---------------------------------------------------------------------
-- ③ 归档（改前整行快照；重复执行不重复归档）
-- ---------------------------------------------------------------------
INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json)
SELECT t.usc_from, t.usc_to, t.stock_id, t.ftmm_symbol, to_jsonb(c)
  FROM futu.finv_futu_quote_collect c
  JOIN tmp_collect_align t ON t.row_ctid = c.ctid
ON CONFLICT (usc_from, stock_id) DO NOTHING;

-- 跳过的行也留痕（不改数据，只记录为什么没改）
INSERT INTO archive.finv_quote_collect_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT d.usc_from, d.usc_to, d.stock_id, d.ftmm_symbol, to_jsonb(c),
       '需求 11085 跳过：目标 usc 已被非变更行占用（保持现状，另案处理）'
  FROM futu.finv_futu_quote_collect c
  JOIN tmp_collect_diff d ON d.row_ctid = c.ctid
 WHERE NOT EXISTS (SELECT 1 FROM tmp_collect_align a WHERE a.row_ctid = d.row_ctid)
ON CONFLICT (usc_from, stock_id) DO NOTHING;

INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json)
SELECT t.usc_from, t.usc_to, t.stock_id, t.ftmm_symbol, to_jsonb(i)
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_indicator_align t ON t.row_ctid = i.ctid
ON CONFLICT (usc_from) DO NOTHING;

INSERT INTO archive.finv_quote_secu_indicator_usc_align_20261110
       (usc_from, usc_to, stock_id, ftmm_symbol, row_json, reason)
SELECT d.usc_from, d.usc_to, d.stock_id, d.ftmm_symbol, to_jsonb(i),
       '需求 11085 跳过：目标 usc 已被非变更行占用（保持现状，另案处理）'
  FROM cdeu.finv_quote_secu_indicator i
  JOIN tmp_indicator_diff d ON d.row_ctid = i.ctid
 WHERE NOT EXISTS (SELECT 1 FROM tmp_indicator_align a WHERE a.row_ctid = d.row_ctid)
ON CONFLICT (usc_from) DO NOTHING;

-- ---------------------------------------------------------------------
-- ④ 两阶段改名：先挪到临时值（断开链式占用），再落到目标值
--    'TMP§' 前缀不会出现在真实代码里，末尾校验断言 0 残留
-- ---------------------------------------------------------------------
UPDATE futu.finv_futu_quote_collect c
   SET usc = 'TMP§' || c.usc
 WHERE c.usc IN (SELECT usc_from FROM tmp_collect_align);

UPDATE futu.finv_futu_quote_collect c
   SET usc = t.usc_to
  FROM tmp_collect_align t
 WHERE c.usc = 'TMP§' || t.usc_from;

UPDATE cdeu.finv_quote_secu_indicator i
   SET usc = 'TMP§' || i.usc
 WHERE i.usc IN (SELECT usc_from FROM tmp_indicator_align);

UPDATE cdeu.finv_quote_secu_indicator i
   SET usc = t.usc_to
  FROM tmp_indicator_align t
 WHERE i.usc = 'TMP§' || t.usc_from;

-- ---------------------------------------------------------------------
-- ⑤ 收敛校验（任一不满足即抛异常 → 整个事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_collect_diff   BIGINT;
    v_indicator_diff BIGINT;
    v_tmp_left       BIGINT;
    v_collect_rows   BIGINT;
    v_indicator_rows BIGINT;
    v_arch_collect   BIGINT;
    v_arch_indicator BIGINT;
    v_collect_skip   BIGINT;
    v_indicator_skip BIGINT;
    v_align_collect  BIGINT;
    v_align_indicator BIGINT;
BEGIN
    SELECT count(*) INTO v_collect_diff
      FROM futu.finv_futu_quote_collect c
      JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = c.ftmm_symbol
     WHERE c.usc <> k.usc;

    SELECT count(*) INTO v_indicator_diff
      FROM cdeu.finv_quote_secu_indicator i
      JOIN futu.finv_futu_secu_key k ON k.stock_id = i.stock_id
     WHERE i.usc <> k.usc;

    SELECT (SELECT count(*) FROM futu.finv_futu_quote_collect WHERE usc LIKE 'TMP§%')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_indicator WHERE usc LIKE 'TMP§%')
      INTO v_tmp_left;

    SELECT count(*) INTO v_collect_rows    FROM futu.finv_futu_quote_collect;
    SELECT count(*) INTO v_indicator_rows  FROM cdeu.finv_quote_secu_indicator;
    SELECT count(*) INTO v_arch_collect    FROM archive.finv_quote_collect_usc_align_20261110;
    SELECT count(*) INTO v_arch_indicator  FROM archive.finv_quote_secu_indicator_usc_align_20261110;

    -- 主动跳过的行数（目标被占）——剩余差异必须恰好等于这些行
    SELECT count(*) INTO v_collect_skip
      FROM tmp_collect_diff d
     WHERE NOT EXISTS (SELECT 1 FROM tmp_collect_align a WHERE a.row_ctid = d.row_ctid);
    SELECT count(*) INTO v_indicator_skip
      FROM tmp_indicator_diff d
     WHERE NOT EXISTS (SELECT 1 FROM tmp_indicator_align a WHERE a.row_ctid = d.row_ctid);

    SELECT count(*) INTO v_align_collect   FROM tmp_collect_align;
    SELECT count(*) INTO v_align_indicator FROM tmp_indicator_align;

    IF v_collect_diff <> v_collect_skip OR v_indicator_diff <> v_indicator_skip THEN
        RAISE EXCEPTION 'V300168 对齐未收敛：quote_collect 剩余 % 行（跳过 % 行）、indicator 剩余 % 行（跳过 % 行）',
            v_collect_diff, v_collect_skip, v_indicator_diff, v_indicator_skip;
    END IF;
    IF v_tmp_left <> 0 THEN
        RAISE EXCEPTION 'V300168 临时值残留 % 行（两阶段改名异常）', v_tmp_left;
    END IF;
    IF v_collect_rows <> (SELECT count(*) FROM futu.finv_futu_quote_collect)
       OR v_indicator_rows <> (SELECT count(*) FROM cdeu.finv_quote_secu_indicator) THEN
        RAISE EXCEPTION 'V300168 行数发生变化（只应改主键值，不应增删行）';
    END IF;

    RAISE NOTICE 'V300168 完成：quote_collect 共 % 行（对齐 % 行，跳过 % 行，归档累计 % 行）、indicator 共 % 行（对齐 % 行，跳过 % 行，归档累计 % 行）；临时值残留 0',
        v_collect_rows, v_align_collect, v_collect_skip, v_arch_collect,
        v_indicator_rows, v_align_indicator, v_indicator_skip, v_arch_indicator;
END $$;

COMMIT;
