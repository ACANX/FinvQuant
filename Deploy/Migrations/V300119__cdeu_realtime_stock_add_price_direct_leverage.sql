-- =====================================================================
-- V300119：cdeu.finv_cdeu_secu_realtime_stock 增加 price_direct（涨跌方向）
--          与 leverage（杠杆）
--
-- 背景：上游 secu_realtime_quote 的 price_direct 是 'up'/'down'/'flat' 文本，
--   leverage 是数值文本（可为负，约 1.0–9.999 量级，10659 行为空）。
--   为便于统一处理，price_direct 存「符号」而非英文词：up→'+'、down→'-'、flat→'0'；
--   leverage 直接存数值。
--
-- 列序说明：PostgreSQL 的 ADD COLUMN 只能追加到表尾，故落在第 24、25 列。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；末尾校验列位置/类型与注释数量，不符回滚。
-- =====================================================================

BEGIN;

ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
    ADD COLUMN IF NOT EXISTS price_direct CHAR(1),
    ADD COLUMN IF NOT EXISTS leverage     NUMERIC(20,6);

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.price_direct
    IS '涨跌方向（符号值：+ = 涨（源 up）、- = 跌（源 down）、0 = 平（源 flat））';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.leverage
    IS '杠杆（数值，可为负；源值为数字文本，空值不写入）';

DO $$
DECLARE
    v_direct_type text;
    v_direct_pos  int;
    v_lev_type    text;
    v_lev_pos     int;
    v_cmt         int;
BEGIN
    SELECT data_type, ordinal_position INTO v_direct_type, v_direct_pos
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'price_direct';
    IF v_direct_type IS DISTINCT FROM 'character' THEN
        RAISE EXCEPTION 'V300119 price_direct 类型不符：期望 char(1)，实际 %', COALESCE(v_direct_type, '<列不存在>');
    END IF;
    IF v_direct_pos <> 24 THEN
        RAISE EXCEPTION 'V300119 price_direct 列位置不符：期望第 24 列，实际第 % 列', v_direct_pos;
    END IF;

    SELECT data_type, ordinal_position INTO v_lev_type, v_lev_pos
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'leverage';
    IF v_lev_type IS DISTINCT FROM 'numeric' THEN
        RAISE EXCEPTION 'V300119 leverage 类型不符：期望 numeric，实际 %', COALESCE(v_lev_type, '<列不存在>');
    END IF;
    IF v_lev_pos <> 25 THEN
        RAISE EXCEPTION 'V300119 leverage 列位置不符：期望第 25 列，实际第 % 列', v_lev_pos;
    END IF;

    SELECT count(*)
      INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 25 THEN
        RAISE EXCEPTION 'V300119 列注释数量不符：期望 25（原 23 + 新增 2），实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300119 校验通过：price_direct 第 24 列（char(1)）/ leverage 第 25 列（numeric）/ 注释 25';
END $$;

COMMIT;
