-- =====================================================================
-- FinvQuant PostgreSQL V300194：纠正 V300192 回填进 open_api_credential_scope 的脏路径
--
-- 需求：11219（回填口径修正，承接 V300192）。
-- 授权依据：用户在 2026-11-11 会话中批准「先建表，回填数据」；本迁移只在**同一批新建的表**内
--   删除明显不可能匹配的脏行，不触碰 open_api_credential.scope_str（源数据保持原样）。
--
-- 发现的问题：
--   凭证 idn=20（cred-20-20261111-0924）的 scope_str 里有一条
--       {"path": "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/*\"", "method": "GET"}
--   路径尾部多了一个双引号。V300192 的回填条件只校验了「以 / 开头、长度 > 1」，
--   于是把它当成合法规则复制进了新表（表里表现为 `/OpenAPI/V1/Meta/Finv/Quote/Cdeu/*"`）。
--
--   这类形态（含 `*` 但不以 `/*` 结尾）**永远匹配不上任何请求**：
--   鉴权侧 InScope 只认「精确相等 / 尾部 `/*` 前缀 / 尾部 `/` 目录」三种，末尾是 `"` 时三者都不成立。
--   放进清单表只会误导「这个凭证能查全部 CDEU 接口」，因此不应回填。
--
-- 本迁移做两件事：
--   ① 删除本表里「含 `*` 但不以 `/*` 结尾」的行（仅限 V300192 回填行，`create_by='migrate'`，
--      不误删后续人工录入的规则）；
--   ② 收敛校验：断言表内不再存在这类行（脚本自己能保证的结构性事实）。
--   源数据 scope_str 不动——是否修凭证 20 的授权范围属数据修复，需用户单独授权。
--
-- 幂等：DELETE 带完整条件，重跑影响 0 行；校验只数该形态行数。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V300192。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_bad   BIGINT;
    v_left  BIGINT;
BEGIN
    -- 1. 删除本不该回填的脏行（只动 migrate 回填的行）
    SELECT count(*) INTO v_bad
      FROM open.open_api_credential_scope
     WHERE path_pattern LIKE '%*%'
       AND path_pattern NOT LIKE '%/*'
       AND COALESCE(create_by, '') = 'migrate';
    RAISE NOTICE 'V300194：待清理的脏路径规则 % 条', v_bad;

    DELETE FROM open.open_api_credential_scope
     WHERE path_pattern LIKE '%*%'
       AND path_pattern NOT LIKE '%/*'
       AND COALESCE(create_by, '') = 'migrate';

    -- 2. 收敛校验：该形态不得残留（无论 create_by，因为这种规则本身不可能生效）
    SELECT count(*) INTO v_left
      FROM open.open_api_credential_scope
     WHERE path_pattern LIKE '%*%'
       AND path_pattern NOT LIKE '%/*';
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300194 校验失败：仍残留 % 条「含 * 但不以 /* 结尾」的路径规则', v_left;
    END IF;

    RAISE NOTICE 'V300194 就绪：脏路径已清理；生效规则 % 条',
        (SELECT count(*) FROM open.open_api_credential_scope);
    RAISE NOTICE 'V300194 提醒：凭证 idn=20 的 scope_str 源数据未改（GET 那条永远匹配不上），如需修正请单独授权数据修复';
END $$;

COMMIT;
