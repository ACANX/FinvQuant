-- =====================================================================
-- FinvQuant PostgreSQL V100077：新增 cdeu schema（需求 10753）
--
-- 目的：为 CDEU 工作流域预留独立 schema，先把域边界划出来，表暂不搬迁。
-- 当前 12 张 cdeu_* 表仍留在 dedup schema，本迁移只建 schema 并把 cdeu
-- 纳入数据库默认 search_path。
--
-- ⚠️ 后续迁移的约束（用户 2026-10-28 明确要求）：
--   将 cdeu_* 表从 dedup 迁入 cdeu 时，**不是简单的同名迁移**
--   （禁止 `ALTER TABLE dedup.cdeu_xxx SET SCHEMA cdeu` 式的机械搬移）。
--   届时必须重新审视并可能重构：表名去掉重复的 cdeu_ 前缀、约束/索引命名、
--   主键与外键策略、运行表与定义表的拆分粒度、历史数据兼容。
--   本迁移只做「占位」，不预设搬迁方式。
--
-- search_path 顺序：cdeu 紧随 dedup 之后、archive 之前，与域划分顺序一致。
-- 注意 public 仍在首位——未写 schema 前缀的 CRUD 会优先命中 public，
-- 因此所有 cdeu 表的读写后续必须显式写 cdeu. 前缀（见 schema_qualify_test.go）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建 schema（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'cdeu') THEN
        RAISE NOTICE 'V100077 提示：cdeu schema 已存在，跳过创建';
    ELSE
        EXECUTE 'CREATE SCHEMA cdeu AUTHORIZATION finv';
        RAISE NOTICE 'V100077 已创建 schema cdeu（属主 finv）';
    END IF;
END $$;

COMMENT ON SCHEMA cdeu IS 'CDEU 工作流域：Workflow / Phase / Process / JOB / Step 的定义与运行实例。当前 12 张 cdeu_* 表仍在 dedup，待后续迁移重新设计后迁入';

-- ---------------------------------------------------------------------
-- 2. 数据库默认 search_path 纳入 cdeu（对新连接生效，供手工 psql 与历史种子迁移兜底）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_path text;
BEGIN
    -- PG 18 的数据库级配置存放在 pg_db_role_setting（setrole = 0 表示数据库级），
    -- pg_database 已无 datconfig/setconfig 列。
    SELECT array_to_string(s.setconfig, ' ') INTO v_path
      FROM pg_db_role_setting s
     WHERE s.setdatabase = (SELECT oid FROM pg_database WHERE datname = current_database())
       AND s.setrole = 0;

    IF v_path LIKE '%cdeu%' THEN
        RAISE NOTICE 'V100077 提示：数据库 search_path 已含 cdeu，跳过设置（当前：%）', v_path;
    ELSE
        EXECUTE format(
            'ALTER DATABASE %I SET search_path TO public, quote, quant, futu, export, sche, dedup, cdeu, archive, sys',
            current_database());
        RAISE NOTICE 'V100077 数据库 % 的 search_path 已设为 public, quote, quant, futu, export, sche, dedup, cdeu, archive, sys',
            current_database();
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_owner  text;
    v_usage  boolean;
    v_create boolean;
    v_cfg    text[];
BEGIN
    -- 3.1 schema 存在且属主正确
    SELECT pg_get_userbyid(nspowner) INTO v_owner FROM pg_namespace WHERE nspname = 'cdeu';
    IF v_owner IS NULL THEN
        RAISE EXCEPTION 'V100077 收敛失败：cdeu schema 不存在';
    END IF;
    IF v_owner <> 'finv' THEN
        RAISE EXCEPTION 'V100077 收敛失败：cdeu 属主应为 finv，实际 %', v_owner;
    END IF;

    -- 3.2 应用角色具备 USAGE 与 CREATE
    SELECT has_schema_privilege('finv', 'cdeu', 'USAGE')  INTO v_usage;
    SELECT has_schema_privilege('finv', 'cdeu', 'CREATE') INTO v_create;
    IF NOT v_usage OR NOT v_create THEN
        RAISE EXCEPTION 'V100077 收敛失败：finv 对 cdeu 的 USAGE=% CREATE=%', v_usage, v_create;
    END IF;

    -- 3.3 数据库级 search_path 含 cdeu（PG 18 从 pg_db_role_setting 读取）
    SELECT s.setconfig INTO v_cfg
      FROM pg_db_role_setting s
     WHERE s.setdatabase = (SELECT oid FROM pg_database WHERE datname = current_database())
       AND s.setrole = 0;
    IF v_cfg IS NULL OR NOT EXISTS (
        SELECT 1 FROM unnest(v_cfg) c WHERE c LIKE 'search_path=%' AND c LIKE '%cdeu%'
    ) THEN
        RAISE EXCEPTION 'V100077 收敛失败：数据库 search_path 未包含 cdeu（当前 %）', v_cfg;
    END IF;

    RAISE NOTICE 'V100077 完成：schema cdeu 就位（属主 %，USAGE/CREATE 齐备），数据库 search_path 已含 cdeu', v_owner;
END $$;

COMMIT;
