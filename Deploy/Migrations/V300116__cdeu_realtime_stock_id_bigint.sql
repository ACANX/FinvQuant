-- =====================================================================
-- V300116：cdeu.finv_cdeu_secu_realtime_stock.stock_id 由 INTEGER 扩为 BIGINT
--
-- 背景（设计口径修正）：
--   V300115 按给定 DDL 把 stock_id 建成 INTEGER（PostgreSQL int4，上限
--   2147483647）。但本表主键就是上游富途的 stockId，实测最大值
--   148739012639708（64 位），13045 行里 9561 行装不下，写入被拒绝。
--   本表一行一证券、主键即源系统证券编号，必须用 BIGINT（8 字节）。
--
-- 影响：仅放宽 stock_id 列宽（int4 → int8 是值域扩大，不改语义、不动数据、
--   不动表名/归属/其它列）。主键索引随列类型自动重建。
--
-- 幂等：已是 bigint 时跳过 ALTER；末尾校验列类型、主键、注释数量，不符回滚。
-- =====================================================================

BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = 'cdeu'
           AND table_name = 'finv_cdeu_secu_realtime_stock'
           AND column_name = 'stock_id'
           AND data_type = 'integer'
    ) THEN
        ALTER TABLE cdeu.finv_cdeu_secu_realtime_stock
            ALTER COLUMN stock_id TYPE BIGINT;
        RAISE NOTICE 'V300116 stock_id 已由 integer 扩为 bigint';
    ELSE
        RAISE NOTICE 'V300116 stock_id 已是 bigint，跳过';
    END IF;
END $$;

-- 注释随列语义同步（64 位源系统编号）
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.stock_id
    IS '证券编号（源系统证券 ID，BIGINT 64 位；本表主键，一证券一行）';

DO $$
DECLARE
    v_type text;
    v_pk   text;
    v_cmt  int;
BEGIN
    SELECT data_type
      INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'cdeu'
       AND table_name = 'finv_cdeu_secu_realtime_stock'
       AND column_name = 'stock_id';
    IF v_type IS DISTINCT FROM 'bigint' THEN
        RAISE EXCEPTION 'V300116 stock_id 类型不符：期望 bigint，实际 %', COALESCE(v_type, '<列不存在>');
    END IF;

    SELECT string_agg(a.attname, ',' ORDER BY k.ord)
      INTO v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND c.contype = 'p';
    IF v_pk IS DISTINCT FROM 'stock_id' THEN
        RAISE EXCEPTION 'V300116 主键列不符：期望 stock_id，实际 %', COALESCE(v_pk, '<无主键>');
    END IF;

    SELECT count(*)
      INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 15 THEN
        RAISE EXCEPTION 'V300116 列注释数量不符：期望 15，实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300116 校验通过：stock_id=bigint / 主键(stock_id) / 列注释 15';
END $$;

COMMIT;
