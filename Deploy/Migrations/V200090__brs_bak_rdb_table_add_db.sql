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
--
-- 【全新库保护（需求 11022）】本脚本版本号（200090）早于建表脚本 V300060，而
--   brs.brs_bak_rdb_table 是在 V300060 才 CREATE 的 —— 全新空库按版本号顺序执行时，
--   这里表还不存在，原实现会直接报 42P01 让整条迁移链失败。
--   因此本脚本改为「表不存在就整体跳过并打印 NOTICE」，等 V300060 建表后，
--   再由 V300126__brs_bak_rdb_table_ensure_db.sql 用同一套语义把 db 列/唯一约束收敛到位
--   （存量库早已是该状态，两者均为幂等空转）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_cols  text;
    v_empty bigint;
    v_uq    boolean;
BEGIN
    -- 全新库：建表脚本 V300060 还没跑，直接跳过（由 V300126 收敛）
    IF to_regclass('brs.brs_bak_rdb_table') IS NULL THEN
        RAISE NOTICE 'V200090 跳过：brs.brs_bak_rdb_table 尚未创建（全新库由 V300060 建表、V300126 补 db 列）';
        RETURN;
    END IF;

    -- 1) 新增 db 列
    ALTER TABLE brs.brs_bak_rdb_table
        ADD COLUMN IF NOT EXISTS db varchar(64) NOT NULL DEFAULT 'finv';

    EXECUTE 'COMMENT ON COLUMN brs.brs_bak_rdb_table.db IS ''数据库名（登记归属库；本次备份执行仍连当前库）''';

    -- 2) 唯一约束改为包含 db：同一数据库内 schema + 表唯一
    --    （原约束 (rdb_type, schema_name, table_name) 会让不同库的同名表无法登记）
    SELECT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'uq_brs_bak_rdb_table'
           AND conrelid = 'brs.brs_bak_rdb_table'::regclass
           AND pg_get_constraintdef(oid) LIKE '%(rdb_type, db, schema_name, table_name)%'
    ) INTO v_uq;

    IF NOT v_uq THEN
        ALTER TABLE brs.brs_bak_rdb_table DROP CONSTRAINT IF EXISTS uq_brs_bak_rdb_table;
        ALTER TABLE brs.brs_bak_rdb_table
            ADD CONSTRAINT uq_brs_bak_rdb_table UNIQUE (rdb_type, db, schema_name, table_name);
    END IF;

    -- 3) 收敛校验：列存在、无空 db、约束列齐全
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
