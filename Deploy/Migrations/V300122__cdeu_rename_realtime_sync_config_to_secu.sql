-- =====================================================================
-- V300122：cdeu.finv_cdeu_realtime_sync_config 更名为
--          cdeu.finv_cdeu_secu_realtime_sync_config
--
-- 授权来源：用户在会话中明确点名要求改名（DbSchemaSpec §0 唯一授权口径）。
-- 只改表名与约束名，不改列、不改数据、不删不改业务行。
--
-- 幂等：旧表存在且新表不存在时才 RENAME；约束改名按「旧名在、新名不在」判断；
--   末尾校验新表存在 / 旧表不存在 / 列数 / 行数 / 约束名 / 注释数，不符回滚。
-- =====================================================================

BEGIN;

DO $$
BEGIN
    IF to_regclass('cdeu.finv_cdeu_realtime_sync_config') IS NOT NULL
       AND to_regclass('cdeu.finv_cdeu_secu_realtime_sync_config') IS NULL THEN
        ALTER TABLE cdeu.finv_cdeu_realtime_sync_config
            RENAME TO finv_cdeu_secu_realtime_sync_config;
    END IF;
END $$;

-- 约束名同步（RENAME TABLE 不会自动改约束名）
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_cdeu_realtime_sync_config_pk'
           AND conrelid = 'cdeu.finv_cdeu_secu_realtime_sync_config'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_cdeu_secu_realtime_sync_config
            RENAME CONSTRAINT finv_cdeu_realtime_sync_config_pk
                         TO finv_cdeu_secu_realtime_sync_config_pk;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_cdeu_realtime_sync_config_enable_chk'
           AND conrelid = 'cdeu.finv_cdeu_secu_realtime_sync_config'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_cdeu_secu_realtime_sync_config
            RENAME CONSTRAINT finv_cdeu_realtime_sync_config_enable_chk
                         TO finv_cdeu_secu_realtime_sync_config_enable_chk;
    END IF;
END $$;

DO $$
DECLARE
    v_new    regclass;
    v_old    regclass;
    v_cols   int;
    v_rows   int;
    v_cmt    int;
    v_pk     int;
    v_chk    int;
BEGIN
    v_new := to_regclass('cdeu.finv_cdeu_secu_realtime_sync_config');
    v_old := to_regclass('cdeu.finv_cdeu_realtime_sync_config');

    IF v_new IS NULL THEN
        RAISE EXCEPTION 'V300122 改名失败：cdeu.finv_cdeu_secu_realtime_sync_config 不存在';
    END IF;
    IF v_old IS NOT NULL THEN
        RAISE EXCEPTION 'V300122 改名失败：旧表 cdeu.finv_cdeu_realtime_sync_config 仍存在';
    END IF;

    SELECT count(*) INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_sync_config';
    IF v_cols <> 9 THEN
        RAISE EXCEPTION 'V300122 列数不符：期望 9，实际 %', v_cols;
    END IF;

    SELECT count(*) INTO v_rows FROM cdeu.finv_cdeu_secu_realtime_sync_config;
    IF v_rows < 3 THEN
        RAISE EXCEPTION 'V300122 配置行数不足：期望 ≥ 3，实际 %', v_rows;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conname = 'finv_cdeu_secu_realtime_sync_config_pk'
       AND conrelid = v_new;
    SELECT count(*) INTO v_chk
      FROM pg_constraint
     WHERE conname = 'finv_cdeu_secu_realtime_sync_config_enable_chk'
       AND conrelid = v_new;
    IF v_pk <> 1 OR v_chk <> 1 THEN
        RAISE EXCEPTION 'V300122 约束名不符：pk=% chk=%', v_pk, v_chk;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_sync_config';
    IF v_cmt <> 10 THEN
        RAISE EXCEPTION 'V300122 注释数不符：期望 10（表 + 9 列），实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300122 校验通过：新表 9 列 / 配置行 % / 约束名与注释已同步', v_rows;
END $$;

COMMIT;
