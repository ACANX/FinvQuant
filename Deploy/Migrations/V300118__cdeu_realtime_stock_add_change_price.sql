-- =====================================================================
-- V300118：cdeu.finv_cdeu_secu_realtime_stock 增加 change_price（涨跌额）
--
-- 背景：上游 secu_realtime_quote 的 change 列是带正负号的文本（'+0.750' / '-1.240'），
--   无法直接统计排序；本迁移增加数值列 change_price，字符串解析在写入侧完成。
--   列名与日K表 finv_quote_secu_kline_day.change_price（涨跌额）同义。
--
-- 列序说明：PostgreSQL 的 ADD COLUMN 只能追加到表尾（不支持指定中间位置），
--   因此本列落在第 23 位（原 22 列之后）；如需插到中间得重建整表，不做。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；末尾校验列位置/类型与注释数量，不符回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS change_price NUMERIC(20,6);

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.change_price
    IS '涨跌额（源值 "+0.750" 去正号后写入；与日K表 change_price 同义）';

DO $$
DECLARE
    v_type text;
    v_pos  int;
    v_cmt  int;
BEGIN
    SELECT data_type, ordinal_position
      INTO v_type, v_pos
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'change_price';
    IF v_type IS DISTINCT FROM 'numeric' THEN
        RAISE EXCEPTION 'V300118 change_price 类型不符：期望 numeric，实际 %', COALESCE(v_type, '<列不存在>');
    END IF;
    IF v_pos <> 23 THEN
        RAISE EXCEPTION 'V300118 change_price 列位置不符：期望第 23 列（追加在表尾），实际第 % 列', v_pos;
    END IF;

    SELECT count(*)
      INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 23 THEN
        RAISE EXCEPTION 'V300118 列注释数量不符：期望 23（原 22 + 新增 1），实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300118 校验通过：change_price 为 numeric 第 23 列，列注释 23';
END $$;

COMMIT;
