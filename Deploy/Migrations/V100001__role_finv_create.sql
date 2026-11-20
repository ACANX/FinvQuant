-- =====================================================================
-- FinvQuant PostgreSQL V100001：基础准备（一）——数据库角色 finv（需求 10881）
--
-- 号段规划（需求 10881）：
--   V100000～V199999：基础准备（创建角色 / 校验数据库 / 创建 schema）
--   V200000～V299999：表结构 DDL
--   V300000～V399999：数据（字典种子 / 回填 / 修正）
--
-- 本脚本职责：
--   确保连接角色 `finv` 存在。容器部署下由 PostgreSQL 初始化
--   （`POSTGRES_USER=finv`）自动创建，此处为幂等空转；裸机/自建库首次安装时
--   若执行用户具备 CREATEROLE 权限则补建，否则明确报错提示由 DBA 预建。
--
-- 安全约定：
--   **不在迁移里设置密码**（密码属部署秘密，走 `.env` / 密钥管理）；
--   本脚本只负责「角色存在性」（一对象一脚本：一个角色一个文件）；
--   各 schema 由 V100002～V100009 各自独立文件创建，权限校验见 V100010～V100012。
--
-- 幂等：`pg_roles` 存在性判断，重复执行不做任何变更。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'finv') THEN
        BEGIN
            EXECUTE 'CREATE ROLE finv LOGIN';
            RAISE NOTICE 'V100001: 已创建角色 finv（密码请由部署侧设置，迁移不落密码）';
        EXCEPTION WHEN insufficient_privilege THEN
            RAISE EXCEPTION 'V100001: 角色 finv 不存在，且当前用户 % 无 CREATEROLE 权限；请先用超级用户创建：CREATE ROLE finv LOGIN PASSWORD ''<pwd>''', current_user;
        END;
    ELSE
        RAISE NOTICE 'V100001: 角色 finv 已存在，跳过创建';
    END IF;
END $$;

-- 收敛校验：角色必须存在
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'finv') THEN
        RAISE EXCEPTION 'V100001 校验失败：角色 finv 仍不存在';
    END IF;
END $$;

COMMIT;
