-- =====================================================================
-- V200058：cdeu 分钟行情成交量 volume 改为浮点（numeric(20,6)）
--
-- 背景（需求 10791）：
--   旧定义 volume 为 bigint（V100039 重建表时设定），无法保存小数成交量
--   （加密货币、基金份额等场景需要小数）。cdeu.finv_quote_secu_kline_min
--   是运行期唯一写入/查询的分钟行情主表，改为 numeric(20,6) 与其它价格列
--   精度口径一致，同时保留原 CHECK（volume >= 0）。
--
-- 说明：
--   · 只调整 cdeu 表；public / quote / dedup / export 的同名表不动；
--   · 幂等：已是 numeric 则跳过；
--   · bigint → numeric 为无损转换；
--   · 引用 volume 的 CHECK 约束按实际名称动态查找后重建。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_type text;
    r      record;
BEGIN
    SELECT c.data_type
      INTO v_type
      FROM information_schema.columns c
     WHERE c.table_schema = 'cdeu'
       AND c.table_name   = 'finv_quote_secu_kline_min'
       AND c.column_name  = 'volume';

    IF v_type IS NULL THEN
        RAISE EXCEPTION 'cdeu.finv_quote_secu_kline_min.volume 不存在，终止';
    END IF;

    IF v_type = 'numeric' THEN
        RAISE NOTICE 'V200058: volume 已是 numeric，跳过';
        RETURN;
    END IF;

    -- 删除引用 volume 的 CHECK 约束（约束名可能随建表方式不同）
    FOR r IN
        SELECT conname
          FROM pg_constraint
         WHERE conrelid = 'cdeu.finv_quote_secu_kline_min'::regclass
           AND contype  = 'c'
           AND pg_get_constraintdef(oid) ILIKE '%volume%'
    LOOP
        EXECUTE format('ALTER TABLE cdeu.finv_quote_secu_kline_min DROP CONSTRAINT %I', r.conname);
    END LOOP;

    ALTER TABLE cdeu.finv_quote_secu_kline_min
        ALTER COLUMN volume TYPE numeric(20,6) USING volume::numeric(20,6);

    ALTER TABLE cdeu.finv_quote_secu_kline_min
        ADD CONSTRAINT finv_quote_secu_kline_min_volume_check
        CHECK (volume IS NULL OR volume >= 0);

    RAISE NOTICE 'V200058: volume 已改为 numeric(20,6)';
END $$;

COMMIT;
