-- =====================================================================
-- V300090：大表增量备份登记表改名 brs_bak_rdb_table_slice → brs_bak_rdb_table_incr
--
-- 背景：`_slice` 读起来像「切片动作」，`_incr` 才能直接表达「增量备份」的语义；
--   表名与需求 10905 的口径保持一致（小表全量 `brs_bak_rdb_table` /
--   大表增量 `brs_bak_rdb_table_incr`）。
--
-- 幂等：旧表存在且新表不存在时才改名；约束/触发器/序列按旧名匹配后改名，
--   重复执行不会失败（找不到旧名就跳过）。
--
-- 说明：V300088（建表）与 V300089（压缩口径）按历史迁移「不回改」原则保持原样，
--   由本迁移完成最终命名。
-- =====================================================================

BEGIN;

-- 1) 表改名（旧表存在、新表不存在时）
DO $$
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_slice') IS NOT NULL
       AND to_regclass('brs.brs_bak_rdb_table_incr') IS NULL THEN
        ALTER TABLE brs.brs_bak_rdb_table_slice RENAME TO brs_bak_rdb_table_incr;
        RAISE NOTICE 'V300090：brs_bak_rdb_table_slice → brs_bak_rdb_table_incr 已完成';
    ELSE
        RAISE NOTICE 'V300090：无需改名（旧表不存在或新表已存在）';
    END IF;
END $$;

-- 2) 约束改名（主键 / 唯一 / CHECK）
DO $$
DECLARE
    r record;
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_incr') IS NULL THEN
        RAISE NOTICE 'V300090：表不存在，跳过约束改名';
        RETURN;
    END IF;
    FOR r IN
        SELECT conname
          FROM pg_constraint
         WHERE conrelid = 'brs.brs_bak_rdb_table_incr'::regclass
           AND conname LIKE '%brs_bak_rdb_table_slice%'
    LOOP
        EXECUTE format('ALTER TABLE brs.brs_bak_rdb_table_incr RENAME CONSTRAINT %I TO %I',
                       r.conname, replace(r.conname, 'brs_bak_rdb_table_slice', 'brs_bak_rdb_table_incr'));
        RAISE NOTICE 'V300090：约束 % → %', r.conname,
                     replace(r.conname, 'brs_bak_rdb_table_slice', 'brs_bak_rdb_table_incr');
    END LOOP;
END $$;

-- 3) 触发器改名
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgrelid = 'brs.brs_bak_rdb_table_incr'::regclass
           AND tgname = 'trg_brs_bak_rdb_table_slice_gmt_update'
    ) THEN
        ALTER TRIGGER trg_brs_bak_rdb_table_slice_gmt_update ON brs.brs_bak_rdb_table_incr
            RENAME TO trg_brs_bak_rdb_table_incr_gmt_update;
        RAISE NOTICE 'V300090：触发器已改名为 trg_brs_bak_rdb_table_incr_gmt_update';
    END IF;
EXCEPTION
    WHEN undefined_table THEN
        RAISE NOTICE 'V300090：表不存在，跳过触发器改名';
END $$;

-- 4) identity 序列改名（随表名一起变得直观）
DO $$
DECLARE
    v_seq text;
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_incr') IS NULL THEN
        RETURN;
    END IF;
    v_seq := pg_get_serial_sequence('brs.brs_bak_rdb_table_incr', 'idn');
    IF v_seq IS NOT NULL AND v_seq LIKE '%brs_bak_rdb_table_slice%' THEN
        EXECUTE format('ALTER SEQUENCE %s RENAME TO %I', v_seq, 'brs_bak_rdb_table_incr_idn_seq');
        RAISE NOTICE 'V300090：序列已改名为 brs_bak_rdb_table_incr_idn_seq';
    END IF;
END $$;

-- 5) 表注释与新名对齐
COMMENT ON TABLE brs.brs_bak_rdb_table_incr IS
    '大表增量（分片）备份登记：按分片维度切分导出，同一分片键覆盖（BRS）';

COMMIT;
