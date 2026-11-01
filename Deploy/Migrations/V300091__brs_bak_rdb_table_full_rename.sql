-- =====================================================================
-- V300091：小表全量备份登记表改名 brs_bak_rdb_table → brs_bak_rdb_table_full
--
-- 背景：需求 10905 新增了大表增量登记表 `brs_bak_rdb_table_incr`，
--   原 `brs_bak_rdb_table` 语义是「小表整表全量备份」，改名 `_full` 后
--   两张表并列、职责一眼可辨：`_full`（全量）/ `_incr`（增量）。
--
-- 幂等：旧表存在且新表不存在时才改名；约束/触发器/序列按旧名匹配后改名，
--   重复执行不会失败（找不到旧名就跳过）。
--
-- 说明：V300060（建表）、V300061/V200090/V300083/V300086（加列与种子）等
--   历史迁移按「不回改」原则保持原样，由本迁移完成最终命名；
--   全新库从零执行时，先按旧名建表并灌种子，最后改名为 `_full`。
-- =====================================================================

BEGIN;

-- 1) 表改名（旧表存在、新表不存在时）
DO $$
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table') IS NOT NULL
       AND to_regclass('brs.brs_bak_rdb_table_full') IS NULL THEN
        ALTER TABLE brs.brs_bak_rdb_table RENAME TO brs_bak_rdb_table_full;
        RAISE NOTICE 'V300091：brs_bak_rdb_table → brs_bak_rdb_table_full 已完成';
    ELSE
        RAISE NOTICE 'V300091：无需改名（旧表不存在或新表已存在）';
    END IF;
END $$;

-- 2) 约束改名（主键 / 唯一 / CHECK / NOT NULL）
DO $$
DECLARE
    r record;
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_full') IS NULL THEN
        RAISE NOTICE 'V300091：表不存在，跳过约束改名';
        RETURN;
    END IF;
    FOR r IN
        SELECT conname
          FROM pg_constraint
         WHERE conrelid = 'brs.brs_bak_rdb_table_full'::regclass
           AND conname LIKE '%brs_bak_rdb_table%'
           AND conname NOT LIKE '%brs_bak_rdb_table_full%'
    LOOP
        EXECUTE format('ALTER TABLE brs.brs_bak_rdb_table_full RENAME CONSTRAINT %I TO %I',
                       r.conname, replace(r.conname, 'brs_bak_rdb_table', 'brs_bak_rdb_table_full'));
        RAISE NOTICE 'V300091：约束 % → %', r.conname,
                     replace(r.conname, 'brs_bak_rdb_table', 'brs_bak_rdb_table_full');
    END LOOP;
END $$;

-- 3) 触发器改名
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgrelid = 'brs.brs_bak_rdb_table_full'::regclass
           AND tgname = 'trg_brs_bak_rdb_table_gmt_update'
    ) THEN
        ALTER TRIGGER trg_brs_bak_rdb_table_gmt_update ON brs.brs_bak_rdb_table_full
            RENAME TO trg_brs_bak_rdb_table_full_gmt_update;
        RAISE NOTICE 'V300091：触发器已改名为 trg_brs_bak_rdb_table_full_gmt_update';
    END IF;
EXCEPTION
    WHEN undefined_table THEN
        RAISE NOTICE 'V300091：表不存在，跳过触发器改名';
END $$;

-- 4) identity 序列改名
DO $$
DECLARE
    v_seq text;
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_full') IS NULL THEN
        RETURN;
    END IF;
    v_seq := pg_get_serial_sequence('brs.brs_bak_rdb_table_full', 'idn');
    IF v_seq IS NOT NULL AND v_seq LIKE '%brs_bak_rdb_table_idn_seq' THEN
        EXECUTE format('ALTER SEQUENCE %s RENAME TO %I', v_seq, 'brs_bak_rdb_table_full_idn_seq');
        RAISE NOTICE 'V300091：序列已改名为 brs_bak_rdb_table_full_idn_seq';
    END IF;
END $$;

-- 5) 表注释与新名对齐
COMMENT ON TABLE brs.brs_bak_rdb_table_full IS
    '小表全量备份登记：一行 = 一张要整表 pg_dump 的表（BRS）';

COMMIT;
