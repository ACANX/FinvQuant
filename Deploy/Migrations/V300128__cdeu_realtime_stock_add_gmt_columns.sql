-- =====================================================================
-- V300128：cdeu.finv_cdeu_secu_realtime_stock 增加 gmt_create / gmt_update（需求 11029）
--
-- 背景：快照表此前只有上游的 dt_update（行情快照时间），没有库侧的创建/更新时间，
--   无法判断「这一行是什么时候入库的、最近一次被覆盖是什么时候」。本迁移补齐审计时间：
--   · gmt_create —— 记录入库时刻（新增行由 DEFAULT now() 取当前时间，此后不再变化）；
--   · gmt_update —— 记录最近一次更新时间，由触发器 public.vq_set_gmt_update() 自动维护
--     （与 finv_fxcer / finv_security 等表同一套机制，复用既有函数，不新建函数）。
--
-- 说明：ADD COLUMN 时对已存在的行会用 DEFAULT now() 回填——历史行的真实创建时间无从得知，
--   这里统一记为「本迁移执行时刻」，属已知取舍；gmt_update 之后的每次 UPDATE 都会刷新。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + 触发器先 DROP IF EXISTS 再建 + COMMENT 可重复执行；
--   末尾校验列存在/类型/默认值/触发器/列注释，任一不符整事务回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS gmt_create TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS gmt_update TIMESTAMPTZ NOT NULL DEFAULT now();

-- gmt_update 自动维护（复用 public.vq_set_gmt_update，避免重建函数）
DROP TRIGGER IF EXISTS trg_finv_cdeu_secu_realtime_stock_gmt_update
    ON cdeu.finv_cdeu_secu_realtime_stock;
CREATE TRIGGER trg_finv_cdeu_secu_realtime_stock_gmt_update
    BEFORE UPDATE ON cdeu.finv_cdeu_secu_realtime_stock
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.gmt_create IS
    '记录创建时间（入库时刻，DEFAULT now()，此后不再变化；V300128 之前的存量行为迁移执行时刻）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.gmt_update IS
    '记录更新时间（由触发器 public.vq_set_gmt_update() 在每次 UPDATE 前自动置为 now()）';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt  int;
    v_bad  text;
    v_trg  int;
    v_cmt  int;
BEGIN
    -- 1) 两列存在、类型与默认值正确
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name IN ('gmt_create', 'gmt_update')
       AND data_type = 'timestamp with time zone'
       AND is_nullable = 'NO'
       AND column_default = 'now()';
    IF v_cnt <> 2 THEN
        RAISE EXCEPTION 'V300128 gmt_create / gmt_update 列缺失或类型/默认值不符：命中 % 列（期望 2）', v_cnt;
    END IF;

    -- 2) 列序：追加在第 33、34 列
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position) INTO v_bad
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND ordinal_position > 32;
    IF v_bad IS DISTINCT FROM 'gmt_create,gmt_update' THEN
        RAISE EXCEPTION 'V300128 列序不符：第 33~34 列应为 gmt_create,gmt_update，实际 %', COALESCE(v_bad, '<空>');
    END IF;

    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock';
    IF v_cnt <> 34 THEN
        RAISE EXCEPTION 'V300128 列总数不符：期望 34，实际 %', v_cnt;
    END IF;

    -- 3) 触发器存在且非内部触发器
    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND tg.tgname = 'trg_finv_cdeu_secu_realtime_stock_gmt_update'
       AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300128 缺少 gmt_update 触发器';
    END IF;

    -- 4) 列注释齐全
    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.objsubid
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND a.attname IN ('gmt_create', 'gmt_update');
    IF v_cmt <> 2 THEN
        RAISE EXCEPTION 'V300128 列注释数不符：期望 2，实际 %', v_cmt;
    END IF;
END $$;

COMMIT;
