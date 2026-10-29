-- =====================================================================
-- FinvQuant PostgreSQL V71：新增 sys schema（系统域）并授权
--
-- 决策（ACANX 2026-10-25）：
--   - 需求：为平台级系统对象单独建 sys schema，承载用户/角色/权限、系统参数、
--     审计留痕等与具体行情/量化业务无关的表，使业务域 schema 只放业务对象。
--   - 本迁移只建空 schema + 授权 + search_path，**不建任何表**：系统域表结构
--     由后续迁移创建（命名沿用业务前缀风格，如 sys.finv_sys_user）。
--   - 属主与授权对象取 current_user（= 应用角色，当前 finv），不硬编码角色名。
--   - search_path：sys 追加在末尾（public, quote, quant, futu, export,
--     sche, dedup, archive, sys），不改变既有 schema 的解析优先级——历史种子
--     迁移（V100xxx）不带 schema 前缀，依赖既有顺序解析，必须保持原样。
--   - 代码侧引用 sys 对象必须写 schema 前缀（sys.xxx），裸写表名会被
--     search_path 中靠前的 schema 命中同名对象。
--   - 幂等：CREATE SCHEMA IF NOT EXISTS + GRANT（重复执行无副作用）。
-- 迁移策略：单事务、失败回滚；末尾做收敛校验。
-- 前置：V37（archive schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建 schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS sys;

-- ---------------------------------------------------------------------
-- 2. 属主与权限：属主 = 应用角色；授予 USAGE（可用）+ CREATE（可建表）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl TEXT := current_user;
BEGIN
    EXECUTE format('ALTER SCHEMA sys OWNER TO %I', appl);
    EXECUTE format('GRANT USAGE, CREATE ON SCHEMA sys TO %I', appl);
    RAISE NOTICE 'V71: schema sys 属主=%，已授予 USAGE/CREATE', appl;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据库默认 search_path（新 schema 追加到末尾，不改变既有优先级）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format(
        'ALTER DATABASE %I SET search_path TO public, quote, quant, futu, export, sche, dedup, archive, sys',
        current_database()
    );
    RAISE NOTICE 'V71: 数据库 % 的 search_path 设为 public, quote, quant, futu, export, sche, dedup, archive, sys',
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
      FROM pg_namespace WHERE nspname = 'sys';
    IF missing = 0 THEN
        RAISE EXCEPTION 'V71: sys schema 未创建成功';
    END IF;

    SELECT count(*) INTO badowner
      FROM pg_namespace
     WHERE nspname = 'sys' AND pg_get_userbyid(nspowner) <> appl;
    IF badowner > 0 THEN
        RAISE EXCEPTION 'V71: sys schema 属主不是 %', appl;
    END IF;

    SELECT count(*) INTO nopriv
      FROM pg_namespace
     WHERE nspname = 'sys'
       AND NOT (has_schema_privilege(appl, nspname, 'USAGE')
            AND has_schema_privilege(appl, nspname, 'CREATE'));
    IF nopriv > 0 THEN
        RAISE EXCEPTION 'V71: sys schema 缺少 USAGE/CREATE 权限';
    END IF;

    RAISE NOTICE 'V71: 校验通过（sys 就绪，属主=%，USAGE+CREATE 齐备）', appl;
END $$;

COMMIT;
