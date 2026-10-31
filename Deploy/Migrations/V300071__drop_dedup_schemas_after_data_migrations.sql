-- =====================================================================
-- FinvQuant PostgreSQL V300071：删除已冻结的 dedup / dedup_frozen schema
--   （本迁移承接 V200086 的删除动作；原因见 V200086 头部与下方说明）
--
-- 为什么不在 V200086 里删：
--   2026-10-31 在独立库 `finv_migtest` 做「全新库全链迁移」验证时发现，
--   V300036–V300052 共 13 个历史数据迁移仍引用 `dedup.*`（如
--   `dedup.finv_dedup_usc`、`dedup.cdeu_process_run`），而它们的版本号在
--   V200086 之后。V200086 一旦先删 schema，全新库会在 V300036 直接失败。
--   生产库是增量演进库（那批迁移早已执行），所以此前从未暴露。
--   故把删除动作后移到最后一个数据迁移之后（本迁移），终态不变：
--   `dedup` / `dedup_frozen` 均不存在。
--
-- 授权与备份（与原 V200086 一致）：
--   需求 10834：用户在会话中明确要求清理 dedup / dedup_frozen（DbSchemaSpec §0.3 情形 1）；
--   删除前备份：Deploy/Backup/dedup_schemas_20261030_234253.sql.gz
--   （pg_dump -n dedup -n dedup_frozen，32 张表 / 约 13.4 万行 / 2.2 MB；
--    恢复：zcat <备份> | psql -U finv -d finv）。
--
-- 幂等：DROP SCHEMA IF EXISTS ... CASCADE；已删除的库上重跑影响 0 对象。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾收敛校验（schema 必须不存在、
--           quote/cdeu 两份同名主表必须保留），不符整体回滚。
-- 前置：V300036–V300052（历史 dedup 数据迁移）与 V200086（前置数据校验）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置数据校验（仅当 dedup_frozen 仍存在时执行；生产库已删则跳过）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_src bigint;
    v_dst bigint;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'dedup_frozen') THEN
        RAISE NOTICE 'V300071：dedup_frozen 不存在（已删除或从未创建），跳过前置数据校验';
        RETURN;
    END IF;

    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_quote_maturity_mon;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_quote_maturity_mon;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V300071 前置校验失败：成熟度表 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_job;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_job;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V300071 前置校验失败：导出任务 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_file;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_file;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V300071 前置校验失败：导出文件 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_usc;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_usc;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V300071 前置校验失败：证券主数据 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    RAISE NOTICE 'V300071 前置校验通过：cdeu 侧数据覆盖冻结副本';
END $$;

-- ---------------------------------------------------------------------
-- 2. 删除 schema（CASCADE 一并清理表、序列、索引、约束）
-- ---------------------------------------------------------------------
DROP SCHEMA IF EXISTS dedup CASCADE;
DROP SCHEMA IF EXISTS dedup_frozen CASCADE;

-- ---------------------------------------------------------------------
-- 3. 收敛校验：schema 必须不存在；保护名单里的同名列主表必须保留
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_count int;
BEGIN
    SELECT count(*) INTO v_count FROM pg_namespace WHERE nspname IN ('dedup', 'dedup_frozen');
    IF v_count <> 0 THEN
        RAISE EXCEPTION 'V300071 收敛失败：仍有 % 个 dedup 相关 schema', v_count;
    END IF;

    SELECT count(*) INTO v_count
      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE c.relname = 'finv_quote_secu_kline_min'
       AND n.nspname IN ('quote', 'cdeu');
    IF v_count <> 2 THEN
        RAISE EXCEPTION 'V300071 收敛失败：quote/cdeu 两份主表应各存一份，实际 % 份', v_count;
    END IF;

    RAISE NOTICE 'V300071 完成：dedup / dedup_frozen 已删除，quote / cdeu 主表保留';
END $$;

COMMIT;
