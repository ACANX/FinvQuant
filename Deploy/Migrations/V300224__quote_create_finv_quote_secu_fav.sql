-- =====================================================================
-- V300224：新建 quote.finv_quote_secu_fav 证券收藏表（需求 11312）
--
-- 用途：用户在「元数据管理 → 规范证券信息维护」页点「收藏」时，把该证券的 usc
--       记录到本表；后续由其它功能使用（当前仅收藏/取消收藏与列表回显）。
--
-- 设计：
--   · 主键 usc（一只证券最多一行收藏）；不存快照信息，需要时回查 public.finv_security；
--   · remark 预留人工备注（与主表口径一致：TEXT NOT NULL DEFAULT ''）；
--   · gmt_create / gmt_update 由触发器 public.vq_set_gmt_update() 维护（V200001 建立）；
--   · 表名与 schema 由用户指定（quote.），属 DbSchemaSpec §1 登记范围。
--
-- 幂等：CREATE TABLE / 触发器 / 注释 / 备份登记均可重复执行。
-- 回滚：DROP TABLE quote.finv_quote_secu_fav;（无外键引用）
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS quote;

-- ---------------------------------------------------------------------
-- 1. 建表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quote.finv_quote_secu_fav (
    usc        TEXT        NOT NULL,
    remark     TEXT        NOT NULL DEFAULT '',
    gmt_create TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_secu_fav_pkey PRIMARY KEY (usc)
);

COMMENT ON TABLE quote.finv_quote_secu_fav IS
    '证券收藏表（需求 11312）：一行 = 用户收藏的一只证券，主键 usc';
COMMENT ON COLUMN quote.finv_quote_secu_fav.usc IS
    '证券统一编码（对齐 public.finv_security.usc）';
COMMENT ON COLUMN quote.finv_quote_secu_fav.remark IS
    '备注（人工标注；空字符串表示无备注）';
COMMENT ON COLUMN quote.finv_quote_secu_fav.gmt_create IS
    '创建时间（默认 now()）';
COMMENT ON COLUMN quote.finv_quote_secu_fav.gmt_update IS
    '更新时间（由触发器 public.vq_set_gmt_update() 在 UPDATE 时刷新）';

-- ---------------------------------------------------------------------
-- 2. gmt_update 自动维护
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_secu_fav_gmt_update ON quote.finv_quote_secu_fav;
CREATE TRIGGER trg_finv_quote_secu_fav_gmt_update
    BEFORE UPDATE ON quote.finv_quote_secu_fav
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 3. BRS 全量备份登记（动态取最小空闲 priority，避免唯一约束吞行）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_priority int;
    v_inserted int;
BEGIN
    IF to_regclass('quote.finv_quote_secu_fav') IS NULL THEN
        RAISE EXCEPTION 'V300224 建表失败：quote.finv_quote_secu_fav 不存在';
    END IF;

    SELECT gs.slot INTO v_priority
      FROM generate_series(10, 1000, 10) AS gs(slot)
     WHERE NOT EXISTS (SELECT 1 FROM brs.brs_bak_rdb_table_full b WHERE b.priority = gs.slot)
     ORDER BY gs.slot ASC
     LIMIT 1;

    IF v_priority IS NULL THEN
        RAISE EXCEPTION 'V300224 备份登记失败：1..1000 内没有空闲 priority';
    END IF;

    INSERT INTO brs.brs_bak_rdb_table_full
        (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
         bak_method, compression, flag_upload, creator, remark)
    VALUES ('postgresql', 'finv', 'quote', 'finv_quote_secu_fav', '1', v_priority, 'both',
            'pg_dump', 'zstd', '1', 'migration', '证券收藏表（需求 11312）')
    ON CONFLICT DO NOTHING;

    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RAISE NOTICE 'V300224 备份登记完成：priority=%（本次写入 % 行）', v_priority, v_inserted;
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols int;
    v_pk   int;
    v_trg  int;
    v_bak  int;
BEGIN
    SELECT count(*) INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'quote' AND table_name = 'finv_quote_secu_fav';
    IF v_cols <> 4 THEN
        RAISE EXCEPTION 'V300224 校验失败：列数 = %（期望 4）', v_cols;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conrelid = 'quote.finv_quote_secu_fav'::regclass AND contype = 'p';
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V300224 校验失败：主键数量 = %（期望 1）', v_pk;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger
     WHERE tgrelid = 'quote.finv_quote_secu_fav'::regclass
       AND tgname = 'trg_finv_quote_secu_fav_gmt_update'
       AND NOT tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300224 校验失败：gmt_update 触发器缺失';
    END IF;

    SELECT count(*) INTO v_bak
      FROM brs.brs_bak_rdb_table_full
     WHERE schema_name = 'quote' AND table_name = 'finv_quote_secu_fav';
    IF v_bak <> 1 THEN
        RAISE EXCEPTION 'V300224 校验失败：备份登记行数 = %（期望 1）', v_bak;
    END IF;

    RAISE NOTICE 'V300224 完成：quote.finv_quote_secu_fav（4 列 / 主键 usc / 触发器 / 备份登记）';
END $$;

COMMIT;
