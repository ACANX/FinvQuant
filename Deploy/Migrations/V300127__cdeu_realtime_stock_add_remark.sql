-- =====================================================================
-- V300127：cdeu.finv_cdeu_secu_realtime_stock 增加 remark（备注）列（需求 11028）
--
-- 背景：快照表缺少自由备注列，人工核查/临时标注（如某行的币种为何为空、
--   汇率取值依据、采集异常说明）只能记在库外。本迁移补上 remark，供写入方
--   按需填写；不参与任何计算与排行。
--
-- 口径：
--   · 类型 TEXT、可空：NULL / 空串都表示「没有备注」，语义与其它表 remark 一致；
--   · 只承载人工/采集端说明，不参与换算、过滤与排序；
--   · 追加在原 31 列末尾（ADD COLUMN 顺序即列序），列名与其它表统一用 remark。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + COMMENT 可重复执行；
--   末尾校验列存在/类型/列序/列总数/注释，任一不符整事务回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS remark TEXT;

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.remark IS
    '备注：人工标注或采集端说明（如币种缺失原因、汇率取值依据、异常说明）；不参与换算/过滤/排序，NULL 或空串表示无备注';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type  text;
    v_order text;
    v_cnt   int;
    v_cmt   int;
BEGIN
    SELECT data_type INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'remark';
    IF v_type IS DISTINCT FROM 'text' THEN
        RAISE EXCEPTION 'V300127 remark 列缺失或类型不符：期望 text，实际 %', COALESCE(v_type, '<缺失>');
    END IF;

    -- 列序：remark 必须是最后一列（第 32 列）
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_order
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND ordinal_position > 31;
    IF v_order IS DISTINCT FROM 'remark' THEN
        RAISE EXCEPTION 'V300127 列序不符：第 32 列应为 remark，实际 %', COALESCE(v_order, '<空>');
    END IF;

    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock';
    IF v_cnt <> 32 THEN
        RAISE EXCEPTION 'V300127 列总数不符：期望 32，实际 %', v_cnt;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.objsubid
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND a.attname = 'remark';
    IF v_cmt <> 1 THEN
        RAISE EXCEPTION 'V300127 remark 列注释缺失';
    END IF;
END $$;

COMMIT;
