-- =====================================================================
-- FinvQuant PostgreSQL V100003：基础准备（三）——数据库与权限校验（需求 10881）
--
-- 号段规划见 V100001 头部说明：本脚本属「V100000～V199999 基础准备」段。
--
-- 为什么只校验、不创建数据库：
--   迁移器（`internal/database/migrate.go`）把每个脚本放在**单个事务**里执行，
--   而 PostgreSQL 的 `CREATE DATABASE` 不允许在事务块内运行
--   （`CREATE DATABASE cannot run inside a transaction block`）。
--   因此建库由部署侧负责：
--     · 容器：`POSTGRES_DB=finv`（docker-compose / `.env`）
--     · 裸机：安装脚本 `createdb` 或 DBA 手工创建
--   本脚本校验「当前连接的库名 + 角色 + schema 权限」，不符时给出明确指引。
--
-- 迁移策略：单事务、只做只读校验，失败即 `RAISE EXCEPTION` 回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_db      text := current_database();
    v_user    text := current_user;
    v_bad     text;
BEGIN
    -- 1. 库名：正式库为 finv；测试库（如 finv_migtest）允许 finv* 前缀
    IF v_db NOT LIKE 'finv%' THEN
        RAISE EXCEPTION 'V100003 校验失败：当前数据库为 %（预期 finv 或 finv 前缀的测试库）；CREATE DATABASE 不能在迁移事务内执行，请由部署侧创建后重试', v_db;
    END IF;

    -- 2. 连接角色可写（对 public 有 CREATE 权限，DDL 段才有意义）
    IF NOT has_schema_privilege(v_user, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V100003 校验失败：角色 % 对 schema public 无 CREATE 权限，无法执行后续 DDL 迁移', v_user;
    END IF;

    -- 3. 基础 schema 可用（V100002 已创建；这里再核一次 USAGE+CREATE）
    SELECT string_agg(name || '(' || missing_priv || ')', ', ' ORDER BY name) INTO v_bad
      FROM (
            SELECT v.name,
                   concat_ws(',',
                             CASE WHEN NOT has_schema_privilege(v_user, v.name, 'USAGE')  THEN 'USAGE'  END,
                             CASE WHEN NOT has_schema_privilege(v_user, v.name, 'CREATE') THEN 'CREATE' END) AS missing_priv
              FROM (VALUES ('quote'), ('quant'), ('futu'), ('sys'),
                           ('sche'), ('cdeu'), ('archive')) AS v(name)
           ) AS p
     WHERE p.missing_priv <> '';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V100003 校验失败：角色 % 缺少 schema 权限：%', v_user, v_bad;
    END IF;

    RAISE NOTICE 'V100003 完成：数据库 % / 角色 % / schema 权限校验通过', v_db, v_user;
END $$;

COMMIT;
