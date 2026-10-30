-- =====================================================================
-- FinvQuant PostgreSQL V100086：删除已冻结的 dedup / dedup_frozen schema（需求 10834）
--
-- 授权：用户在会话中明确要求「把 dedup / dedup_frozen 的 schema 清理掉，以后彻底不用了」，
--       符合 DbSchemaSpec §0.3 的授权情形 1（明确点名删除）。
--
-- 背景：
--   V100080 已把 dedup 域 133,260 行数据迁入 cdeu；需求 10757 完成代码切换；
--   10759 把旧 dedup 重命名为 dedup_frozen 冻结。此后代码只读写 cdeu.*，
--   dedup_frozen 仅作观察期保留，现已确认数据完整迁移，可清理淘汰。
--
-- 备份（删除前已完成，见 DbSchemaSpec §0.7）：
--   Deploy/Backup/dedup_schemas_20261030_234253.sql.gz
--   = pg_dump -n dedup -n dedup_frozen（32 张表、32 段 COPY、约 13.4 万行、2.2 MB）
--   恢复方式：zcat <备份> | psql -U finv -d finv
--
-- 说明：dedup schema 本身在 V100080 后已不存在；这里用 IF EXISTS 兼容
--       「某次集成测试曾以 CREATE SCHEMA IF NOT EXISTS dedup 误复活」的情况
--       （该误复活已在需求 10834 中修正测试代码，改为建在 cdeu）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：cdeu 侧数据必须不少于冻结副本（防止删掉未迁移的数据）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_src bigint;
    v_dst bigint;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'dedup_frozen') THEN
        RAISE NOTICE 'V100086：dedup_frozen 不存在，跳过前置校验（幂等）';
        RETURN;
    END IF;

    -- 成熟度（导出判定的核心数据）
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_quote_maturity_mon;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_quote_maturity_mon;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V100086 前置校验失败：成熟度表 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 导出任务
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_job;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_job;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V100086 前置校验失败：导出任务 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 导出文件
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_file;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_file;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V100086 前置校验失败：导出文件 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 证券主数据
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_usc;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_usc;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V100086 前置校验失败：证券主数据 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    RAISE NOTICE 'V100086 前置校验通过：cdeu 侧数据覆盖冻结副本';
END $$;

-- ---------------------------------------------------------------------
-- 2. 删除 schema（CASCADE 会一并清理表、序列、索引、约束）
-- ---------------------------------------------------------------------
DROP SCHEMA IF EXISTS dedup CASCADE;
DROP SCHEMA IF EXISTS dedup_frozen CASCADE;

-- ---------------------------------------------------------------------
-- 3. 后置校验：schema 必须不存在，且 cdeu / quote / export 三份同名表仍在
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_count int;
BEGIN
    SELECT count(*) INTO v_count FROM pg_namespace WHERE nspname IN ('dedup', 'dedup_frozen');
    IF v_count <> 0 THEN
        RAISE EXCEPTION 'V100086 后置校验失败：仍有 % 个 dedup 相关 schema', v_count;
    END IF;

    -- 保护名单里的其余副本不得受影响（DbSchemaSpec §0.2）
    SELECT count(*) INTO v_count
      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE c.relname = 'finv_quote_secu_kline_min'
       AND n.nspname IN ('quote', 'export', 'cdeu');
    IF v_count <> 3 THEN
        RAISE EXCEPTION 'V100086 后置校验失败：quote/export/cdeu 三份主表应各存一份，实际 % 份', v_count;
    END IF;

    RAISE NOTICE 'V100086 完成：dedup / dedup_frozen 已删除，quote/export/cdeu 主表保留';
END $$;

COMMIT;
