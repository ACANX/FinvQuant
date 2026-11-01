-- =====================================================================
-- FinvQuant PostgreSQL V100007：基础准备——创建 schema cdeu（需求 10881 / 10882）
--
-- 一对象一脚本（需求 10882 ①）：本文件只创建 cdeu 这一个 schema；
-- 其余 schema 各自独立文件（V100002～V100009 按序排列）。
-- 用途：CDEU 工作流域（去重、导出、上传）。
-- 归属角色用 CURRENT_USER（默认即 finv），避免硬编码角色名换角色后失效。
--
-- 幂等：CREATE SCHEMA IF NOT EXISTS，重复执行 0 变更。
-- 迁移策略：单事务、失败整体回滚；末尾只收敛校验本 schema。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS cdeu AUTHORIZATION CURRENT_USER;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'cdeu') THEN
        RAISE EXCEPTION 'V100007 校验失败：schema cdeu 未创建';
    END IF;
    RAISE NOTICE 'V100007 完成：schema cdeu 就位';
END $$;

COMMIT;
