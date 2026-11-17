-- =====================================================================
-- FinvQuant PostgreSQL V34：新增 export / sche 两个 schema 并授权
--
-- 决策（ACANX 2026-10-24）：
--   - 需求：为系统融合新增两个业务域 schema
--       export —— 数据导出相关对象（导出任务、导出产物元数据等）
--       sche   —— 定时任务调度相关对象（调度定义、执行记录等）
--     命名沿用需求原文的短名（与 quote / quant / futu 风格一致）；
--     如需改为 sched/schedule，用 ALTER SCHEMA sche RENAME TO <新名> 并同步
--     本迁移之后的 search_path 与 DbSchemaSpec.md。
--   - 本迁移只建空 schema + 授权 + search_path，**不建任何表**：表结构由
--     后续业务迁移按需创建（届时表名需带 schema 限定，见 DbSchemaSpec §3）。
--   - 属主与授权对象取 current_user（即应用连接角色，当前为 finv），而非硬编码
--     名字 —— 库名/角色名变更（如 2026-10-24 的 finvquant→finv）后无需再改本迁移。
--   - search_path 把两个新 schema 追加在**末尾**，不改变既有 4 个 schema 的
--     解析优先级；历史种子迁移（V100xxx）不带 schema 前缀，优先级必须保持稳定。
--   - 幂等：CREATE SCHEMA IF NOT EXISTS + GRANT（重复执行无副作用）。
-- 迁移策略：单事务、失败回滚；末尾做收敛校验。
-- 前置：V33（futu schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建 schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS export;
CREATE SCHEMA IF NOT EXISTS sche;

-- ---------------------------------------------------------------------
-- 2. 属主与权限：属主 = 应用角色；授予 USAGE（可用）+ CREATE（可建表）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    sch  TEXT;
    appl TEXT := current_user;
BEGIN
    FOREACH sch IN ARRAY ARRAY['export', 'sche'] LOOP
        EXECUTE format('ALTER SCHEMA %I OWNER TO %I', sch, appl);
        EXECUTE format('GRANT USAGE, CREATE ON SCHEMA %I TO %I', sch, appl);
        RAISE NOTICE 'V34: schema % 属主=%，已授予 USAGE/CREATE', sch, appl;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据库默认 search_path（新 schema 追加到末尾，不改变既有优先级）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format(
        'ALTER DATABASE %I SET search_path TO public, quote, quant, futu, export, sche',
        current_database()
    );
    RAISE NOTICE 'V34: 数据库 % 的 search_path 设为 public, quote, quant, futu, export, sche',
        current_database();
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：两个 schema 存在、属主为应用角色、具备 USAGE+CREATE
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl     TEXT := current_user;
    missing  INT;
    badowner INT;
    nopriv   INT;
BEGIN
    SELECT count(*) INTO missing
      FROM (VALUES ('export'), ('sche')) AS v(n)
     WHERE NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = v.n);

    SELECT count(*) INTO badowner
      FROM pg_namespace
     WHERE nspname IN ('export', 'sche')
       AND pg_get_userbyid(nspowner) <> appl;

    SELECT count(*) INTO nopriv
      FROM pg_namespace
     WHERE nspname IN ('export', 'sche')
       AND NOT (has_schema_privilege(appl, nspname, 'USAGE')
            AND has_schema_privilege(appl, nspname, 'CREATE'));

    IF missing > 0 THEN
        RAISE EXCEPTION 'V34: 有 % 个 schema 未创建成功', missing;
    END IF;
    IF badowner > 0 THEN
        RAISE EXCEPTION 'V34: 有 % 个 schema 属主不是 %', badowner, appl;
    END IF;
    IF nopriv > 0 THEN
        RAISE EXCEPTION 'V34: 有 % 个 schema 缺少 USAGE/CREATE 权限', nopriv;
    END IF;

    RAISE NOTICE 'V34: 校验通过（export / sche 就绪，属主=%，USAGE+CREATE 齐备）', appl;
END $$;

COMMIT;
