-- =====================================================================
-- FinvQuant PostgreSQL V100009：基础准备——创建 schema brs（需求 10881 / 10882）
--
-- 一对象一脚本（需求 10882 ①）：本文件只创建 brs（备份恢复域）这一个 schema；
-- 其余 schema 各自独立文件（V100002～V100008）。
--
-- 归位说明：brs 原由数据段 `V300060__brs_backup_table.sql` 自建
--   `CREATE SCHEMA IF NOT EXISTS brs`；需求 10882 将其归位到基础准备段。
--   `V300060` 里的同名语句保留（历史迁移不回改，且幂等），对全新库不再承担
--   建 schema 的职责。
-- 归属角色用 CURRENT_USER（默认即 finv），避免硬编码角色名换角色后失效。
--
-- 幂等：CREATE SCHEMA IF NOT EXISTS，重复执行 0 变更。
-- 迁移策略：单事务、失败整体回滚；末尾只收敛校验本 schema。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS brs AUTHORIZATION CURRENT_USER;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'brs') THEN
        RAISE EXCEPTION 'V100009 校验失败：schema brs 未创建';
    END IF;
    RAISE NOTICE 'V100009 完成：schema brs 就位';
END $$;

COMMIT;
