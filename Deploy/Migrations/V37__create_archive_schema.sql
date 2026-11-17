-- =====================================================================
-- FinvQuant PostgreSQL V37：新增 archive schema（数据归档域）并授权
--
-- 决策（ACANX 2026-10-24）：
--   - 需求：为冷数据下沉单独建 archive schema，承载从业务域迁出的历史数据
--     （如按证券/年份归档的行情分钟线），使业务表保持可扫体积。
--   - 本迁移只建空 schema + 授权 + search_path，**不建任何表**：归档表结构
--     取决于归档粒度（按年/按证券/按批次）与是否保留原表名，由后续迁移创建。
--   - ⚠ 与仓库根目录 Archive/ 无关：Archive/ 是**文件**归档（废弃脚本/文档），
--     archive schema 是**数据库**冷数据归档域，两者不要混为一谈。
--   - ⚠ 同名表风险叠加：归档表通常会沿用业务表名（如
--     archive.finv_quote_secu_kline_min 与 quote.finv_quote_secu_kline_min、
--     export.finv_quote_secu_kline_min 三处同名）。任何读写都必须写 schema
--     前缀；裸写表名会命中 search_path 中靠前的 quote（业务表）。
--   - 属主与授权对象取 current_user（= 应用角色，当前 finv），不硬编码角色名。
--   - search_path：archive 追加在末尾（public, quote, quant, futu, export,
--     sche, dedup, archive），不改变既有 schema 的解析优先级。
--   - 幂等：CREATE SCHEMA IF NOT EXISTS + GRANT（重复执行无副作用）。
-- 迁移策略：单事务、失败回滚；末尾做收敛校验。
-- 前置：V36（dedup schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建 schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS archive;

-- ---------------------------------------------------------------------
-- 2. 属主与权限：属主 = 应用角色；授予 USAGE（可用）+ CREATE（可建表）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl TEXT := current_user;
BEGIN
    EXECUTE format('ALTER SCHEMA archive OWNER TO %I', appl);
    EXECUTE format('GRANT USAGE, CREATE ON SCHEMA archive TO %I', appl);
    RAISE NOTICE 'V37: schema archive 属主=%，已授予 USAGE/CREATE', appl;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据库默认 search_path（新 schema 追加到末尾，不改变既有优先级）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format(
        'ALTER DATABASE %I SET search_path TO public, quote, quant, futu, export, sche, dedup, archive',
        current_database()
    );
    RAISE NOTICE 'V37: 数据库 % 的 search_path 设为 public, quote, quant, futu, export, sche, dedup, archive',
        current_database();
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：schema 存在、属主为应用角色、具备 USAGE+CREATE
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl     TEXT := current_user;
    missing  INT;
    badowner INT;
    nopriv   INT;
BEGIN
    SELECT count(*) INTO missing
      FROM pg_namespace WHERE nspname = 'archive';
    IF missing = 0 THEN
        RAISE EXCEPTION 'V37: archive schema 未创建成功';
    END IF;

    SELECT count(*) INTO badowner
      FROM pg_namespace
     WHERE nspname = 'archive' AND pg_get_userbyid(nspowner) <> appl;
    IF badowner > 0 THEN
        RAISE EXCEPTION 'V37: archive schema 属主不是 %', appl;
    END IF;

    SELECT count(*) INTO nopriv
      FROM pg_namespace
     WHERE nspname = 'archive'
       AND NOT (has_schema_privilege(appl, nspname, 'USAGE')
            AND has_schema_privilege(appl, nspname, 'CREATE'));
    IF nopriv > 0 THEN
        RAISE EXCEPTION 'V37: archive schema 缺少 USAGE/CREATE 权限';
    END IF;

    RAISE NOTICE 'V37: 校验通过（archive 就绪，属主=%，USAGE+CREATE 齐备）', appl;
END $$;

COMMIT;
