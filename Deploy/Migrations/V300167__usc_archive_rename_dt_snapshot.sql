-- =====================================================================
-- FinvQuant PostgreSQL V300167：archive.finv_usc_reuse_snapshot 时间列改 dt_ 前缀（需求 11083）
--
-- 要求（用户 2026-11-10）：`snapshot_at` → `dt_snapshot`，与 V300166 的
--   `dt_retire` / `dt_compute` / `dt_first_assign` 保持同一口径。
--
-- 说明：只改列名，类型（timestamptz）、默认值（now()）、注释与数据全部保留。
--   代码侧归档语句不显式写该列（走默认值），因此无需改 SQL；仅文档与注释同步。
--
-- 幂等：仅当旧列存在、新列不存在时改名，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_renamed INT := 0;
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='archive' AND table_name='finv_usc_reuse_snapshot' AND column_name='snapshot_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='archive' AND table_name='finv_usc_reuse_snapshot' AND column_name='dt_snapshot') THEN
        ALTER TABLE archive.finv_usc_reuse_snapshot RENAME COLUMN snapshot_at TO dt_snapshot;
        v_renamed := 1;
    END IF;

    RAISE NOTICE 'V300167：本次改名 % 列', v_renamed;
END $$;

COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.dt_snapshot IS '归档时间（usc 复用发生时写入）';

DO $$
DECLARE
    v_old  INT;
    v_new  INT;
    v_rows BIGINT;
BEGIN
    SELECT count(*) INTO v_old FROM information_schema.columns
     WHERE table_schema='archive' AND table_name='finv_usc_reuse_snapshot' AND column_name='snapshot_at';
    SELECT count(*) INTO v_new FROM information_schema.columns
     WHERE table_schema='archive' AND table_name='finv_usc_reuse_snapshot' AND column_name='dt_snapshot';
    SELECT count(*) INTO v_rows FROM archive.finv_usc_reuse_snapshot;

    IF v_old <> 0 THEN
        RAISE EXCEPTION 'V300167 校验失败：snapshot_at 仍存在';
    END IF;
    IF v_new <> 1 THEN
        RAISE EXCEPTION 'V300167 校验失败：dt_snapshot 不存在';
    END IF;

    RAISE NOTICE 'V300167 完成：snapshot_at → dt_snapshot（现有 % 行数据保留）', v_rows;
END $$;

COMMIT;
