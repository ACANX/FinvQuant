-- =====================================================================
-- FinvQuant PostgreSQL V200086：删除已冻结的 dedup / dedup_frozen schema（需求 10834）
--
-- 授权：用户在会话中明确要求「把 dedup / dedup_frozen 的 schema 清理掉，以后彻底不用了」，
--       符合 DbSchemaSpec §0.3 的授权情形 1（明确点名删除）。
--
-- 背景：
--   V200080 已把 dedup 域 133,260 行数据迁入 cdeu；需求 10757 完成代码切换；
--   10759 把旧 dedup 重命名为 dedup_frozen 冻结。此后代码只读写 cdeu.*，
--   dedup_frozen 仅作观察期保留，现已确认数据完整迁移，可清理淘汰。
--
-- 备份（删除前已完成，见 DbSchemaSpec §0.7）：
--   Deploy/Backup/dedup_schemas_20261030_234253.sql.gz
--   = pg_dump -n dedup -n dedup_frozen（32 张表、32 段 COPY、约 13.4 万行、2.2 MB）
--   恢复方式：zcat <备份> | psql -U finv -d finv
--
-- 说明：dedup schema 本身在 V200080 后已不存在；这里用 IF EXISTS 兼容
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
        RAISE NOTICE 'V200086：dedup_frozen 不存在，跳过前置校验（幂等）';
        RETURN;
    END IF;

    -- 成熟度（导出判定的核心数据）
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_quote_maturity_mon;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_quote_maturity_mon;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V200086 前置校验失败：成熟度表 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 导出任务
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_job;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_job;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V200086 前置校验失败：导出任务 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 导出文件
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_export_file;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_export_file;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V200086 前置校验失败：导出文件 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    -- 证券主数据
    SELECT count(*) INTO v_src FROM dedup_frozen.finv_dedup_usc;
    SELECT count(*) INTO v_dst FROM cdeu.finv_quote_cdeu_usc;
    IF v_dst < v_src THEN
        RAISE EXCEPTION 'V200086 前置校验失败：证券主数据 cdeu=% < dedup_frozen=%', v_dst, v_src;
    END IF;

    RAISE NOTICE 'V200086 前置校验通过：cdeu 侧数据覆盖冻结副本';
END $$;

-- ---------------------------------------------------------------------
-- 2. 删除 schema —— **已推迟到 V300071 执行**
--
-- 原因（2026-10-31 在独立库 finv_migtest 做全新库全链验证时发现）：
--   V300036–V300052 等 13 个历史数据迁移仍引用 `dedup.*`（如
--   `dedup.finv_dedup_usc`），且按版本号在本迁移**之后**执行。若在此处删 schema，
--   全新库会在 V300036 直接报错。本库（生产）当时已跑过那批迁移，属增量演进库，
--   所以从未暴露。故把删除动作后移到最后一个数据迁移之后（V300071），
--   终态仍为「dedup / dedup_frozen 不存在」，与需求 10834 的验收口径一致。
--   已执行过本版本的库（含生产库）不重跑，不受影响。
-- ---------------------------------------------------------------------
DO $$
BEGIN
    RAISE NOTICE 'V200086：dedup / dedup_frozen 的删除已推迟到 V300071（需先跑完 V300036–V300052），本迁移不改 schema';
END $$;

-- ---------------------------------------------------------------------
-- 3. 后置校验：保护名单里的同名副本必须齐全（dedup 相关 schema 的最终校验在 V300071）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_count int;
BEGIN
    SELECT count(*) INTO v_count
      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE c.relname = 'finv_quote_secu_kline_min'
       AND n.nspname IN ('quote', 'export', 'cdeu');
    IF v_count <> 3 THEN
        RAISE EXCEPTION 'V200086 后置校验失败：quote/export/cdeu 三份主表应各存一份，实际 % 份', v_count;
    END IF;

    RAISE NOTICE 'V200086 完成：前置数据校验通过，schema 删除推迟到 V300071';
END $$;

COMMIT;
