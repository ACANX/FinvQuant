-- =====================================================================
-- FinvQuant PostgreSQL V100010：基础准备——校验数据库（需求 10881 / 10882）
--
-- 一对象一脚本（需求 10882 ①）：本文件只校验「当前连接的数据库」这一项；
-- 角色权限见 V100011，schema 权限见 V100012。
--
-- 为什么只校验、不创建数据库：
--   迁移器（`internal/database/migrate.go`）把每个脚本放在**单个事务**里执行，
--   而 PostgreSQL 的 `CREATE DATABASE` 不允许在事务块内运行
--   （`CREATE DATABASE cannot run inside a transaction block`）；
--   且迁移只能发生在**已连上的库**里（库不存在根本连不上）。
--   因此建库由部署侧负责：
--     · 容器：`POSTGRES_DB=finv`（docker-compose / `.env`）
--     · 裸机：安装脚本 `createdb` 或 DBA 手工创建
--
-- 迁移策略：单事务、只做只读校验，失败即 `RAISE EXCEPTION` 回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_db text := current_database();
BEGIN
    -- 库名：正式库为 finv；测试库（如 finv_migtest）允许 finv* 前缀
    IF v_db NOT LIKE 'finv%' THEN
        RAISE EXCEPTION 'V100010 校验失败：当前数据库为 %（预期 finv 或 finv 前缀的测试库）；CREATE DATABASE 不能在迁移事务内执行，请由部署侧创建后重试', v_db;
    END IF;

    RAISE NOTICE 'V100010 完成：数据库 % 校验通过', v_db;
END $$;

COMMIT;
