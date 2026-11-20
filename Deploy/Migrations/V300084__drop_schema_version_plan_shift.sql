-- =====================================================================
-- V300084：退役号段平移脚本，删除一次性 marker 表（需求 10895）
--
-- 背景：`Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql` 是需求 10881 的一次性
--   存量库过渡脚本（`public.schema_version.version` +100000），用
--   `public.schema_version_plan_shift` 作防重跑 marker。
--
--   用户确认（2026-11-01，选项 B）：所有存量库已完成平移，脚本与 marker 表一并退役：
--   · 删除 `Deploy/Upgrade/2611-001-MigrationSegmentsShift.sql`（不再随包分发）；
--   · 删除 marker 表 `public.schema_version_plan_shift`；
--   · 同步删除 `brs.brs_bak_rdb_table` 中对它的备份登记（表不存在后登记会备份失败）。
--
-- 前置假设（执行前须成立）：目标库的 `public.schema_version` 已是三段新号段
--   （`min(version::bigint) >= 100001`），即平移早已完成。本迁移会在校验段验证这一点。
--
-- 授权：删除表属 DbSchemaSpec §0 红线操作，本次由用户**明确点名**（需求 10895）。
-- 幂等：DROP TABLE IF EXISTS + DELETE 幂等。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：号段平移必须已完成（旧号段 <100000 的记录不应存在）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_legacy bigint;
BEGIN
    SELECT count(*) INTO v_legacy
      FROM public.schema_version
     WHERE version ~ '^[0-9]+$'
       AND version::bigint < 100000;

    IF v_legacy > 0 THEN
        RAISE EXCEPTION 'V300084 前置校验失败：public.schema_version 仍有 % 条旧号段记录（<100000），请先完成号段平移', v_legacy;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 删除备份登记（表即将不存在，保留登记会导致每日备份报错）
-- ---------------------------------------------------------------------
DELETE FROM brs.brs_bak_rdb_table
 WHERE schema_name = 'public'
   AND table_name = 'schema_version_plan_shift';

-- ---------------------------------------------------------------------
-- 3. 删除一次性 marker 表
-- ---------------------------------------------------------------------
DROP TABLE IF EXISTS public.schema_version_plan_shift;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：表与登记均已不存在
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_tables
         WHERE schemaname = 'public' AND tablename = 'schema_version_plan_shift'
    ) THEN
        RAISE EXCEPTION 'V300084 校验失败：public.schema_version_plan_shift 仍存在';
    END IF;

    IF EXISTS (
        SELECT 1 FROM brs.brs_bak_rdb_table
         WHERE schema_name = 'public' AND table_name = 'schema_version_plan_shift'
    ) THEN
        RAISE EXCEPTION 'V300084 校验失败：备份登记中仍有 schema_version_plan_shift';
    END IF;
END $$;

COMMIT;
