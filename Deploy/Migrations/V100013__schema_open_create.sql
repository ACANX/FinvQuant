-- =====================================================================
-- FinvQuant PostgreSQL V100013：基础准备——创建 schema open（需求 11167）
--
-- 一对象一脚本（需求 10882 ①）：本文件只创建 open 这一个 schema；
-- 其他 schema 各自独立文件（V100002～V100009、本文件按序排列）。
-- 用途：对外 OpenAPI 接入（调用方、凭证、调用审计）。
-- 归属角色用 CURRENT_USER（默认即 finv），避免硬编码角色名换角色后失效。
--
-- 为什么单独建 schema：外部调用方凭证属平台级安全对象，不能塞进业务域
-- （`quote`/`cdeu`/`futu`），也不宜与内部用户表 `sys_user` 混放在 `sys`；
-- 用户明确要求「表名以 open_ 开头，建在 open schema 内」（2026-11-15 会话）。
--
-- 幂等：CREATE SCHEMA IF NOT EXISTS，重复执行 0 变更。
-- 迁移策略：单事务、失败整体回滚；末尾只收敛校验本 schema。
-- 说明：`open` 在 PostgreSQL 里是 unreserved keyword，不加引号可直接写 `open.open_api_client`
--   （已在 2026-11-15 实测 `CREATE SCHEMA open` + `SELECT ... FROM open.<表>` 可用）。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS open AUTHORIZATION CURRENT_USER;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'open') THEN
        RAISE EXCEPTION 'V100013 校验失败：schema open 未创建';
    END IF;
    RAISE NOTICE 'V100013 完成：schema open 就位';
END $$;

COMMIT;
