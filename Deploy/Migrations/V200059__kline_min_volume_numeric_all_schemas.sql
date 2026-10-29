-- =====================================================================
-- V200059：quote / export / dedup 同名分钟行情表 volume 统一为 numeric(20,6)
--
-- 背景（需求 10792）：
--   V200058 已把 cdeu.finv_quote_secu_kline_min.volume 改为 numeric(20,6)。
--   导入（MVsv → quote）、导出补丁中转（cdeu → export）、去重旧副本（dedup）
--   仍为 bigint，小数成交量会在这些链路被截断或报错。本迁移把三张同名表
--   统一为 numeric(20,6)，与 cdeu 口径一致。
--
-- 说明：
--   · 只处理 quote / export / dedup 三张同名表；public 表不动；
--   · 幂等：已是 numeric 或表不存在时跳过；
--   · bigint → numeric 无损；
--   · 引用 volume 的 CHECK 约束按实际名称动态删除后重建。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    t      record;
    v_type text;
    r      record;
BEGIN
    FOR t IN
        SELECT *
          FROM (VALUES
                ('quote',  'finv_quote_secu_kline_min'),
                ('export', 'finv_quote_secu_kline_min'),
                ('dedup',  'finv_quote_secu_kline_min')
               ) AS x(sch, tbl)
    LOOP
        SELECT c.data_type
          INTO v_type
          FROM information_schema.columns c
         WHERE c.table_schema = t.sch
           AND c.table_name   = t.tbl
           AND c.column_name  = 'volume';

        IF v_type IS NULL THEN
            RAISE NOTICE 'V200059: %.% 不存在或没有 volume，跳过', t.sch, t.tbl;
            CONTINUE;
        END IF;

        IF v_type = 'numeric' THEN
            RAISE NOTICE 'V200059: %.%.volume 已是 numeric，跳过', t.sch, t.tbl;
            CONTINUE;
        END IF;

        FOR r IN
            SELECT conname
              FROM pg_constraint
             WHERE conrelid = format('%I.%I', t.sch, t.tbl)::regclass
               AND contype  = 'c'
               AND pg_get_constraintdef(oid) ILIKE '%volume%'
        LOOP
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT %I', t.sch, t.tbl, r.conname);
        END LOOP;

        EXECUTE format(
            'ALTER TABLE %I.%I ALTER COLUMN volume TYPE numeric(20,6) USING volume::numeric(20,6)',
            t.sch, t.tbl);

        EXECUTE format(
            'ALTER TABLE %I.%I ADD CONSTRAINT %I CHECK (volume IS NULL OR volume >= 0)',
            t.sch, t.tbl, t.tbl || '_volume_check');

        RAISE NOTICE 'V200059: %.%.volume → numeric(20,6)', t.sch, t.tbl;
    END LOOP;
END $$;

COMMIT;
