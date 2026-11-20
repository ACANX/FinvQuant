-- =====================================================================
-- V300120：cdeu.finv_cdeu_secu_realtime_stock 增加 page（来源页码）
--
-- 背景：该表存的是「按市值排行接口分页拉取」得到的证券最新行情快照。
--   新增 page 记录该证券这张快照来自排行榜的第几页（从 1 开始），
--   供后续按页定位、手动刷新对应证券的快照。
--
-- 列序说明：PostgreSQL 的 ADD COLUMN 只能追加到表尾，故落在第 26 列。
-- 取值说明：历史 13045 行无法追溯页码，保持 NULL；不设 NOT NULL / DEFAULT。
--   约束 page_chk 只拦 0 与负数（页码无 0 与负语义），NULL 合法。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + 约束存在性判断；
--   末尾校验列位置/类型/约束/注释数量，不符回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS page INTEGER;

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.page
    IS '来源页码：该证券最新快照由市值排行接口分页拉取所得时的页码（从 1 开始）；NULL 表示未知';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conname = 'finv_cdeu_secu_realtime_stock_page_chk'
           AND conrelid = 'cdeu.finv_cdeu_secu_realtime_stock'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
            ADD CONSTRAINT finv_cdeu_secu_realtime_stock_page_chk
            CHECK (page IS NULL OR page > 0);
    END IF;
END $$;

DO $$
DECLARE
    v_type text;
    v_pos  int;
    v_cmt  int;
    v_ck   int;
BEGIN
    SELECT data_type, ordinal_position INTO v_type, v_pos
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'page';
    IF v_type IS DISTINCT FROM 'integer' THEN
        RAISE EXCEPTION 'V300120 page 类型不符：期望 integer，实际 %', COALESCE(v_type, '<列不存在>');
    END IF;
    IF v_pos <> 26 THEN
        RAISE EXCEPTION 'V300120 page 列位置不符：期望第 26 列，实际第 % 列', v_pos;
    END IF;

    SELECT count(*) INTO v_ck
      FROM pg_constraint
     WHERE conname = 'finv_cdeu_secu_realtime_stock_page_chk'
       AND conrelid = 'cdeu.finv_cdeu_secu_realtime_stock'::regclass;
    IF v_ck <> 1 THEN
        RAISE EXCEPTION 'V300120 page_chk 约束不符：期望 1 个，实际 %', v_ck;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 26 THEN
        RAISE EXCEPTION 'V300120 列注释数量不符：期望 26（原 25 + 新增 1），实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300120 校验通过：page 第 26 列（integer，允许 NULL，> 0 或 NULL），注释 26';
END $$;

COMMIT;
