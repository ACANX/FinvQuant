-- =====================================================================
-- FinvQuant PostgreSQL V300195：清理 OpenAPI 授权范围里的 DELETE 规则（16 条）
--
-- 需求：11222（清理无效 DELETE 授权）。
-- 授权依据：用户在 2026-11-11 会话中明确要求「清掉这 16 条授权」。
--   受影响范围已核验：16 条规则分布在 16 个凭证行上，**全部属于演示调用方 `openapi-demo`**
--   （`DEMO-AKSK` + `DEMO-STS` + 14 条 STS 临时凭据），无真实接入方受影响。
--
-- 为什么清：
--   平台对外 OpenAPI **不提供删除能力**——3 条 DELETE 路由注册的是显式拒绝（需求 11180），
--   鉴权通过后固定返回 405。因此 `method='DELETE'` 的授权永远只是「能走到 405」，
--   既没有实际能力，又会在授权范围页面/报表里显示成「已授权删除」，属误导性脏数据。
--
-- 清理范围（两处同口径，缺一不可）：
--   ① `open.open_api_credential_scope` 删掉 `method='DELETE'` 的行（V300192 建的结构化表）；
--   ② `open.open_api_credential.scope_str` 移除 `method='DELETE'` 的数组元素（**鉴权真正读的是它**）。
--   只清 ① 等于没清，只清 ② 会让两处漂移。
--
-- 删除数据的合规要求（DbSchemaSpec §0.4）：
--   ① 删除前整行备份到 `archive.*_purge_20261111`（含 `reason` / `backup_at` / 来源标识 `src_schema`）；
--   ② 单事务内「备份 → 修改/删除 → 校验」；
--   ③ 校验用 `RAISE EXCEPTION`（备份为空即为失败，杜绝"没备份就删"）。
--
-- 幂等：备份 `ON CONFLICT DO NOTHING`、UPDATE/DELETE 带完整条件；重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V300192（scope 表）、V300183（凭证表）、schema archive（V200037）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. schema 兜底（V200037 已建；此处保证单独执行本文件也不失败）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS archive AUTHORIZATION CURRENT_USER;

-- ---------------------------------------------------------------------
-- 1. 备份表（整行 + reason + backup_at；主键含来源标识，避免与同名表互相覆盖）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.open_api_credential_scope_purge_20261111 (
    src_schema     VARCHAR(16)  NOT NULL DEFAULT 'open',
    idn            BIGINT       NOT NULL,
    credential_idn BIGINT       NOT NULL,
    method         VARCHAR(8)   NOT NULL,
    path_pattern   VARCHAR(256) NOT NULL,
    remark         VARCHAR(500) NULL,
    create_by      VARCHAR(32)  NULL,
    gmt_create     TIMESTAMPTZ  NULL,
    gmt_update     TIMESTAMPTZ  NULL,
    reason         VARCHAR(500) NULL,
    backup_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_open_api_credential_scope_purge PRIMARY KEY (src_schema, idn)
);

COMMENT ON TABLE archive.open_api_credential_scope_purge_20261111 IS
    'V300195 删除前整行备份：open.open_api_credential_scope 中 method=DELETE 的 16 条规则（需求 11222）';

CREATE TABLE IF NOT EXISTS archive.open_api_credential_purge_20261111 (
    src_schema    VARCHAR(16) NOT NULL DEFAULT 'open',
    idn           BIGINT      NOT NULL,
    credential_no VARCHAR(48) NOT NULL,
    auth_mode     VARCHAR(16) NOT NULL,
    scope_str_old JSONB       NULL,
    scope_str_new JSONB       NULL,
    reason        VARCHAR(500) NULL,
    backup_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pk_open_api_credential_purge PRIMARY KEY (src_schema, idn)
);

COMMENT ON TABLE archive.open_api_credential_purge_20261111 IS
    'V300195 改前快照：open.open_api_credential 中 scope_str 被移除 DELETE 条目的凭证行（需求 11222）';

-- ---------------------------------------------------------------------
-- 2. 单事务：备份 → 修改 scope_str → 删除 scope 行 → 校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_scope_rows BIGINT;
    v_cred_rows  BIGINT;
    v_left_scope BIGINT;
    v_left_str   BIGINT;
    v_reason     TEXT := '需求 11222：DELETE 授权无效（平台对外 OpenAPI 不提供删除能力，鉴权通过后固定 405），按用户 2026-11-11 授权清理';
BEGIN
    -- 2.1 备份 scope 表里的 DELETE 行
    INSERT INTO archive.open_api_credential_scope_purge_20261111
        (src_schema, idn, credential_idn, method, path_pattern, remark,
         create_by, gmt_create, gmt_update, reason)
    SELECT 'open', idn, credential_idn, method, path_pattern, remark,
           create_by, gmt_create, gmt_update, v_reason
      FROM open.open_api_credential_scope
     WHERE method = 'DELETE'
    ON CONFLICT (src_schema, idn) DO NOTHING;

    -- 2.2 备份「scope_str 含 DELETE 条目」的凭证整行（改前/改后都留）
    INSERT INTO archive.open_api_credential_purge_20261111
        (src_schema, idn, credential_no, auth_mode, scope_str_old, scope_str_new, reason)
    SELECT 'open', c.idn, c.credential_no, c.auth_mode, c.scope_str,
           COALESCE((
               SELECT jsonb_agg(elem ORDER BY ord)
                 FROM jsonb_array_elements(c.scope_str) WITH ORDINALITY AS t(elem, ord)
                WHERE upper(COALESCE(elem->>'method', '')) <> 'DELETE'
           ), '[]'::jsonb),
           v_reason
      FROM open.open_api_credential c
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND EXISTS (
           SELECT 1 FROM jsonb_array_elements(c.scope_str) e
            WHERE upper(COALESCE(e->>'method', '')) = 'DELETE'
       )
    ON CONFLICT (src_schema, idn) DO NOTHING;

    SELECT count(*) INTO v_scope_rows
      FROM archive.open_api_credential_scope_purge_20261111
     WHERE src_schema = 'open';
    SELECT count(*) INTO v_cred_rows
      FROM archive.open_api_credential_purge_20261111
     WHERE src_schema = 'open';

    -- 2.3 【硬门槛】备份为空说明前面没抓到数据，此时绝不能继续删
    IF v_scope_rows = 0 AND v_cred_rows = 0 THEN
        RAISE NOTICE 'V300195：未发现 DELETE 授权（已是清理后的状态），本次不做修改';
        RETURN;
    END IF;
    IF v_scope_rows = 0 THEN
        RAISE EXCEPTION 'V300195 中止：scope 表备份为 0 行但凭证表有 % 行待改，数据不一致，请人工核对', v_cred_rows;
    END IF;

    -- 2.4 从 scope_str 移除 DELETE 条目（鉴权真正读的地方）
    UPDATE open.open_api_credential c
       SET scope_str = COALESCE((
               SELECT jsonb_agg(elem ORDER BY ord)
                 FROM jsonb_array_elements(c.scope_str) WITH ORDINALITY AS t(elem, ord)
                WHERE upper(COALESCE(elem->>'method', '')) <> 'DELETE'
           ), '[]'::jsonb),
           modify_by = 'migrate'
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND EXISTS (
           SELECT 1 FROM jsonb_array_elements(c.scope_str) e
            WHERE upper(COALESCE(e->>'method', '')) = 'DELETE'
       );

    -- 2.5 删除 scope 表里的 DELETE 行
    DELETE FROM open.open_api_credential_scope WHERE method = 'DELETE';

    -- 2.6 校验（RAISE EXCEPTION，不得降级为 WARNING）
    SELECT count(*) INTO v_left_scope FROM open.open_api_credential_scope WHERE method = 'DELETE';
    IF v_left_scope <> 0 THEN
        RAISE EXCEPTION 'V300195 校验失败：open_api_credential_scope 仍残留 % 条 DELETE 规则', v_left_scope;
    END IF;

    SELECT count(*) INTO v_left_str
      FROM open.open_api_credential c
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND EXISTS (
           SELECT 1 FROM jsonb_array_elements(c.scope_str) e
            WHERE upper(COALESCE(e->>'method', '')) = 'DELETE'
       );
    IF v_left_str <> 0 THEN
        RAISE EXCEPTION 'V300195 校验失败：仍残留 % 条凭证的 scope_str 含 DELETE 条目', v_left_str;
    END IF;

    RAISE NOTICE 'V300195 就绪：已清理 scope 表 % 条 DELETE 规则、% 条凭证的 scope_str 条目；备份在 archive.open_api_credential_scope_purge_20261111 / archive.open_api_credential_purge_20261111',
        v_scope_rows, v_cred_rows;
END $$;

COMMIT;
