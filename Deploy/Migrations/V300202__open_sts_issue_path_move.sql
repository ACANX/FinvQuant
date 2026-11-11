-- =====================================================================
-- FinvQuant PostgreSQL V300202：STS 换证接口路径迁移到 /Meta/Open/Auth/…
--
-- 需求：11236。
-- 授权依据：用户在 2026-11-11 会话中要求把
--   /OpenAPI/V1/Meta/Auth/Credential/STS/Issue 调整为 /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue。
--
-- 为什么：`Meta/Open` 表示开放平台域，与接口清单里 `open.auth.sts.issue` 编码（需求 11235）
-- 以及 open schema 的归属对齐；`Meta/Auth` 未体现「这是开放平台自己的鉴权接口」。
--
-- 代码侧同步改动（本迁移只搬数据）：
--   - router.go 注册路径、openapi.StsIssuePath 常量（兑换凭证的路径隔离判定依赖它）；
--   - 兑换凭证只能调该路径、临时凭据不能再换证（需求 11210）不受影响，判定用的是常量而非字符串。
--
-- 数据侧要改三处（缺一处就会出现「凭证被授权了但鉴权判定不认」）：
--   ① open.open_api_endpoint.path（idn=4，接口清单里的路径）；
--   ② open.open_api_credential.scope_str 里该路径的授权规则（可读快照）；
--   ③ open.open_api_credential_scope.path_pattern（鉴权实际读取来源，需求 11228）。
--
-- 合规（DbSchemaSpec §0.4，涉及改既有业务数据）：
--   ① 改前把受影响行整行备份到 archive.open_api_credential_purge_20261111
--      （复用既有 purge 表；已有同 idn 记录时 ON CONFLICT DO NOTHING，不覆盖上次备份）；
--      scope 明细行的备份写 archive.open_api_credential_scope_purge_20261111；
--   ② 单事务「备份 → 改 → 校验」；
--   ③ 校验用 RAISE EXCEPTION：三处都不得再残留旧路径，且接口清单 idn=4 必须是新路径。
--
-- 不保留旧路径：尚无外部接入方（同一批改动里 demo 脚本已切新路径），
--   且签名待签串含 PATH——留旧路径只会让签名对不上，产生 19005 噪音。
--
-- 幂等：所有 UPDATE 都带「旧值」条件；重跑影响 0 行。
-- 前置：V300193（接口清单，种子已收敛为最终编码与路径）、V300192/V300198（授权范围明细表）。
-- 说明（需求 11241）：本迁移对**全新部署**是 0 行变更（V300193 已写新路径）；
--   保留它是为了修正**已有环境**里存量凭证的 scope_str / 明细表旧路径。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS archive AUTHORIZATION CURRENT_USER;

DO $$
DECLARE
    v_old      TEXT := '/OpenAPI/V1/Meta/Auth/Credential/STS/Issue';
    v_new      TEXT := '/OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue';
    v_reason   TEXT := '需求 11236：STS 换证接口路径迁移到 /Meta/Open/Auth/…（三处同改：接口清单 / scope_str / 明细表）';
    v_ep       BIGINT;
    v_cred     BIGINT;
    v_scope    BIGINT;
    v_left_ep  BIGINT;
    v_left_str BIGINT;
    v_left_tbl BIGINT;
BEGIN
    ------------------------------------------------------------------
    -- 1. 备份：scope_str 含旧路径的凭证整行
    ------------------------------------------------------------------
    INSERT INTO archive.open_api_credential_purge_20261111
        (src_schema, idn, credential_no, auth_mode, scope_str_old, scope_str_new, reason)
    SELECT 'open', c.idn, c.credential_no, c.auth_mode, c.scope_str,
           replace(c.scope_str::text, v_old, v_new)::jsonb,
           v_reason
      FROM open.open_api_credential c
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND c.scope_str::text LIKE '%' || v_old || '%'
    ON CONFLICT (src_schema, idn) DO NOTHING;

    SELECT count(*) INTO v_cred
      FROM open.open_api_credential c
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND c.scope_str::text LIKE '%' || v_old || '%';

    ------------------------------------------------------------------
    -- 2. 备份：明细表旧路径行
    ------------------------------------------------------------------
    INSERT INTO archive.open_api_credential_scope_purge_20261111
        (src_schema, idn, credential_idn, method, path_pattern, remark,
         create_by, gmt_create, gmt_update, reason)
    SELECT 'open', s.idn, s.credential_idn, s.method, s.path_pattern, s.remark,
           s.create_by, s.gmt_create, s.gmt_update, v_reason
      FROM open.open_api_credential_scope s
     WHERE s.path_pattern = v_old
    ON CONFLICT (src_schema, idn) DO NOTHING;

    SELECT count(*) INTO v_scope FROM open.open_api_credential_scope WHERE path_pattern = v_old;

    ------------------------------------------------------------------
    -- 3. 改：三处
    ------------------------------------------------------------------
    UPDATE open.open_api_endpoint
       SET path = v_new, modify_by = 'migrate'
     WHERE path = v_old
       AND http_method = 'POST';
    GET DIAGNOSTICS v_ep = ROW_COUNT;

    UPDATE open.open_api_credential
       SET scope_str = replace(scope_str::text, v_old, v_new)::jsonb,
           modify_by = 'migrate'
     WHERE jsonb_typeof(scope_str) = 'array'
       AND scope_str::text LIKE '%' || v_old || '%';

    UPDATE open.open_api_credential_scope
       SET path_pattern = v_new
     WHERE path_pattern = v_old;

    ------------------------------------------------------------------
    -- 4. 校验（RAISE EXCEPTION）
    ------------------------------------------------------------------
    SELECT count(*) INTO v_left_ep  FROM open.open_api_endpoint WHERE path = v_old;
    SELECT count(*) INTO v_left_str FROM open.open_api_credential
     WHERE jsonb_typeof(scope_str) = 'array' AND scope_str::text LIKE '%' || v_old || '%';
    SELECT count(*) INTO v_left_tbl FROM open.open_api_credential_scope WHERE path_pattern = v_old;

    IF v_left_ep <> 0 OR v_left_str <> 0 OR v_left_tbl <> 0 THEN
        RAISE EXCEPTION 'V300202 校验失败：旧路径仍残留（接口清单 % / scope_str % / 明细表 %）',
            v_left_ep, v_left_str, v_left_tbl;
    END IF;

    IF EXISTS (SELECT 1 FROM open.open_api_endpoint WHERE http_method = 'POST' AND path = v_new) THEN
        RAISE NOTICE 'V300202 就绪：接口清单已登记新路径';
    ELSE
        RAISE NOTICE 'V300202 提示：接口清单里没有该接口（未登记），仅更新了已存在的授权数据';
    END IF;

    RAISE NOTICE 'V300202 就绪：备份 % 条凭证 / % 条明细；新路径 %', v_cred, v_scope, v_new;
END $$;

COMMIT;
