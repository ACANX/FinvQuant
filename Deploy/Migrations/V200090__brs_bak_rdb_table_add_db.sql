-- =====================================================================
-- V200090：brs.brs_bak_rdb_table 增加 db（数据库名）列（需求 10889）
--
-- 背景：备份登记原先只有 schema_name + table_name，隐含「当前连接库」。
--   跨库/多库场景需要显式登记数据库名，故新增 db 列。
--
-- 变更：
--   1) 新增 db varchar(64) NOT NULL DEFAULT 'finv'（存量 12 行种子由默认值回填）；
--   2) 唯一约束由 (rdb_type, schema_name, table_name) 改为 (rdb_type, db, schema_name, table_name)；
--   3) 本次仅登记字段，备份执行链（LoadTables / pg_dump）仍连当前库，不按 db 分库执行。
--
-- 幂等：ADD COLUMN IF NOT EXISTS + 约束按 pg_constraint 存在性判断。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 新增 db 列
-- ---------------------------------------------------------------------
ALTER TABLE brs.brs_bak_rdb_table
    ADD COLUMN IF NOT EXISTS db varchar(64) NOT NULL DEFAULT 'finv';

COMMENT ON COLUMN brs.brs_bak_rdb_table.db IS '数据库名（登记归属库；本次备份执行仍连当前库）';

-- ---------------------------------------------------------------------
-- 2. 唯一约束改为包含 db：同一数据库内 schema + 表唯一
--    （原约束 (rdb_type, schema_name, table_name) 会让不同库的同名表无法登记）
-- ---------------------------------------------------------------------
ALTER TABLE brs.brs_bak_rdb_table
    DROP CONSTRAINT IF EXISTS uq_brs_bak_rdb_table;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'uq_brs_bak_rdb_table'
           AND conrelid = 'brs.brs_bak_rdb_table'::regclass
    ) THEN
        ALTER TABLE brs.brs_bak_rdb_table
            ADD CONSTRAINT uq_brs_bak_rdb_table UNIQUE (rdb_type, db, schema_name, table_name);
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 收敛校验：列存在、约束列齐全、无空 db
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols text;
    v_empty bigint;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'brs'
       AND table_name = 'brs_bak_rdb_table'
       AND column_name = 'db';

    IF v_cols IS NULL THEN
        RAISE EXCEPTION 'V200090 校验失败：brs.brs_bak_rdb_table.db 列不存在';
    END IF;

    SELECT count(*) INTO v_empty FROM brs.brs_bak_rdb_table WHERE db IS NULL OR db = '';
    IF v_empty > 0 THEN
        RAISE EXCEPTION 'V200090 校验失败：存在 % 行 db 为空', v_empty;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'uq_brs_bak_rdb_table'
           AND conrelid = 'brs.brs_bak_rdb_table'::regclass
           AND pg_get_constraintdef(oid) LIKE '%(rdb_type, db, schema_name, table_name)%'
    ) THEN
        RAISE EXCEPTION 'V200090 校验失败：唯一约束 uq_brs_bak_rdb_table 未包含 (rdb_type, db, schema_name, table_name)';
    END IF;
END $$;

COMMIT;
