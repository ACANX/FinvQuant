-- =====================================================================
-- FinvQuant PostgreSQL V100012：基础准备——校验 schema 权限（需求 10881 / 10882）
--
-- 一对象一脚本（需求 10882 ①）：本文件只校验「连接角色对 8 个基础 schema 的
-- USAGE + CREATE 权限」这一项；数据库名见 V100010，角色对 public 的权限见 V100011。
--
-- 清单：quote / quant / futu / sys / sche / cdeu / archive（V100002～V100008 创建）
--       + brs（V100009 创建，备份恢复域）。
-- 这些 schema 由 `AUTHORIZATION CURRENT_USER` 创建，正常情况下连接角色即属主；
-- 本脚本用于发现「换角色连接」「属主被人为改动」等部署事故。
--
-- 迁移策略：单事务、只做只读校验，失败即 `RAISE EXCEPTION` 回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_user text := current_user;
    v_bad  text;
BEGIN
    SELECT string_agg(name || '(' || missing_priv || ')', ', ' ORDER BY name) INTO v_bad
      FROM (
            SELECT v.name,
                   concat_ws(',',
                             CASE WHEN NOT has_schema_privilege(v_user, v.name, 'USAGE')  THEN 'USAGE'  END,
                             CASE WHEN NOT has_schema_privilege(v_user, v.name, 'CREATE') THEN 'CREATE' END) AS missing_priv
              FROM (VALUES ('quote'), ('quant'), ('futu'), ('sys'),
                           ('sche'), ('cdeu'), ('archive'), ('brs')) AS v(name)
           ) AS p
     WHERE p.missing_priv <> '';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V100012 校验失败：角色 % 缺少 schema 权限：%', v_user, v_bad;
    END IF;

    RAISE NOTICE 'V100012 完成：角色 % 对 8 个基础 schema 权限校验通过', v_user;
END $$;

COMMIT;
