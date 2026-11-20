-- =====================================================================
-- FinvQuant PostgreSQL V100011：基础准备——校验角色权限（需求 10881 / 10882）
--
-- 一对象一脚本（需求 10882 ①）：本文件只校验「连接角色对 public schema 的
-- 建表权限」这一项；数据库名见 V100010，schema 权限见 V100012。
--
-- 为何单独校验 public：
--   后续 DDL 段（V2xxxxx）相当一部分迁移在 public 建表 / 建函数，
--   角色对 public 没有 CREATE 权限时，迁移会在中途失败且难以定位。
--
-- 迁移策略：单事务、只做只读校验，失败即 `RAISE EXCEPTION` 回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_user text := current_user;
BEGIN
    IF NOT has_schema_privilege(v_user, 'public', 'CREATE') THEN
        RAISE EXCEPTION 'V100011 校验失败：角色 % 对 schema public 无 CREATE 权限，无法执行后续 DDL 迁移', v_user;
    END IF;

    RAISE NOTICE 'V100011 完成：角色 % 对 schema public 有 CREATE 权限', v_user;
END $$;

COMMIT;
