-- =====================================================================
-- FinvQuant PostgreSQL V300198：授权范围明细表与 scope_str 全量对账
--
-- 需求：11228（明细表接入读写，鉴权改为读明细表）。
-- 授权依据：用户在 2026-11-11 会话中要求「写路径改成改 scope_str 的同时写表、
--   selectCredentialSQL 加 LATERAL 聚合、PurgeExpired 级联删规则行」。
--
-- 为什么必须先对账：
--   本迁移之前，明细表只是 V300192 的一次性快照，之后新签发的凭证在表里没有行，
--   而「0 行」在鉴权语义里等于**不限接口（全部放行）**。
--   若直接切成读表，那些「scope_str 有限制、表里没行」的凭证会被放大成全部接口。
--   因此先双向拉齐：
--     ① 补漏：`scope_str` 有、表里没有的规则 → 插入；
--     ② 删多余：表里有、`scope_str` 里已经没有的规则 → 删除（删前整行备份）。
--   拉齐后两侧一致，读取以明细表为准、`scope_str` 作为可读快照保留。
--
-- 合规（DbSchemaSpec §0.4，第二步涉及删除）：
--   - 删前把受影响行整行备份到 archive.open_api_credential_scope_purge_20261111
--     （复用 V300195 建的表，含 reason / backup_at / src_schema，幂等 ON CONFLICT DO NOTHING）；
--   - 单事务「备份 → 插入 → 删除 → 校验」；
--   - 校验只断言脚本自己能保证的事实：**补漏后两侧条目集合必须完全一致**
--     （用 EXCEPT 双向差集各为 0 条）；不一致即 RAISE EXCEPTION。
--
-- 幂等：补漏 ON CONFLICT DO NOTHING、多余行为空时不删；重跑 0 变更。
-- 前置：V300192（明细表）、V300195（备份表）、V300183（凭证表）。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS archive AUTHORIZATION CURRENT_USER;

DO $$
DECLARE
    v_ins    BIGINT;
    v_del    BIGINT;
    v_left_a BIGINT;
    v_left_b BIGINT;
    v_bad    BIGINT;
    v_reason TEXT := '需求 11228：明细表与 scope_str 对账时移除的多余规则（scope_str 里已不存在）';
BEGIN
    -- -----------------------------------------------------------------
    -- 0. 先看 scope_str 里有多少「不合规」条目：这些**无法**结构化，也不该被静默放大
    --    （非对象元素 / path 不以 / 开头 / method 越界），只提示不处理。
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_bad
      FROM open.open_api_credential c
      CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS r
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND NOT (
              jsonb_typeof(r) = 'object'
          AND NULLIF(btrim(r->>'path'), '') IS NOT NULL
          AND btrim(r->>'path') LIKE '/%'
          AND length(btrim(r->>'path')) > 1
          AND COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*')
              IN ('*','GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')
       );
    IF v_bad > 0 THEN
        RAISE NOTICE 'V300198 注意：scope_str 里有 % 条不合规条目无法结构化，鉴权侧会因其不匹配而自然不生效；请人工核对', v_bad;
    END IF;

    -- -----------------------------------------------------------------
    -- 1. 补漏：把 scope_str 里合规、但明细表缺失的规则插入
    -- -----------------------------------------------------------------
    WITH src AS (
        SELECT DISTINCT
               c.idn                                                AS credential_idn,
               COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*') AS method,
               btrim(r->>'path')                                    AS path_pattern
          FROM open.open_api_credential c
          CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS r
         WHERE jsonb_typeof(c.scope_str) = 'array'
           AND jsonb_typeof(r) = 'object'
           AND NULLIF(btrim(r->>'path'), '') IS NOT NULL
           AND btrim(r->>'path') LIKE '/%'
           AND length(btrim(r->>'path')) > 1
           AND COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*')
               IN ('*','GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')
    )
    INSERT INTO open.open_api_credential_scope (credential_idn, method, path_pattern, create_by, remark)
    SELECT s.credential_idn, s.method, s.path_pattern, 'migrate', 'V300198 对账补录'
      FROM src s
    ON CONFLICT (credential_idn, method, path_pattern) DO NOTHING;
    GET DIAGNOSTICS v_ins = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 2. 删多余：明细表有、scope_str 已没有的规则（先备份整行）
    -- -----------------------------------------------------------------
    WITH extra AS (
        SELECT s.idn, s.credential_idn, s.method, s.path_pattern
          FROM open.open_api_credential_scope s
          JOIN open.open_api_credential c ON c.idn = s.credential_idn
         WHERE NOT EXISTS (
                 SELECT 1
                   FROM jsonb_array_elements(c.scope_str) AS r
                  WHERE jsonb_typeof(c.scope_str) = 'array'
                    AND jsonb_typeof(r) = 'object'
                    AND COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*') = s.method
                    AND btrim(r->>'path') = s.path_pattern
               )
    ),
    backup AS (
        INSERT INTO archive.open_api_credential_scope_purge_20261111
            (src_schema, idn, credential_idn, method, path_pattern, remark,
             create_by, gmt_create, gmt_update, reason)
        SELECT 'open', s.idn, s.credential_idn, s.method, s.path_pattern, s.remark,
               s.create_by, s.gmt_create, s.gmt_update, v_reason
          FROM open.open_api_credential_scope s
         WHERE s.idn IN (SELECT idn FROM extra)
        ON CONFLICT (src_schema, idn) DO NOTHING
        RETURNING idn
    )
    DELETE FROM open.open_api_credential_scope
     WHERE idn IN (SELECT idn FROM extra);
    GET DIAGNOSTICS v_del = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. 校验：两侧集合必须完全一致（双向差集都为 0）
    -- -----------------------------------------------------------------
    WITH src AS (
        SELECT DISTINCT c.idn AS credential_idn,
               COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*') AS method,
               btrim(r->>'path') AS path_pattern
          FROM open.open_api_credential c
          CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS r
         WHERE jsonb_typeof(c.scope_str) = 'array'
           AND jsonb_typeof(r) = 'object'
           AND NULLIF(btrim(r->>'path'), '') IS NOT NULL
           AND btrim(r->>'path') LIKE '/%'
           AND length(btrim(r->>'path')) > 1
           AND COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*')
               IN ('*','GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')
    )
    SELECT count(*) INTO v_left_a
      FROM (SELECT credential_idn, method, path_pattern FROM src
            EXCEPT
            SELECT credential_idn, method, path_pattern FROM open.open_api_credential_scope) d;

    WITH src AS (
        SELECT DISTINCT c.idn AS credential_idn,
               COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*') AS method,
               btrim(r->>'path') AS path_pattern
          FROM open.open_api_credential c
          CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS r
         WHERE jsonb_typeof(c.scope_str) = 'array'
           AND jsonb_typeof(r) = 'object'
           AND NULLIF(btrim(r->>'path'), '') IS NOT NULL
           AND btrim(r->>'path') LIKE '/%'
           AND length(btrim(r->>'path')) > 1
           AND COALESCE(NULLIF(btrim(upper(r->>'method')), ''), '*')
               IN ('*','GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')
    )
    SELECT count(*) INTO v_left_b
      FROM (SELECT credential_idn, method, path_pattern FROM open.open_api_credential_scope
            EXCEPT
            SELECT credential_idn, method, path_pattern FROM src) d;

    IF v_left_a <> 0 OR v_left_b <> 0 THEN
        RAISE EXCEPTION 'V300198 校验失败：对账后两侧仍不一致（scope_str 有表没有 % 条 / 表有 scope_str 没有 % 条）',
            v_left_a, v_left_b;
    END IF;

    RAISE NOTICE 'V300198 就绪：补录 % 条、移除 % 条；明细表与 scope_str 已完全一致（共 % 条规则）',
        v_ins, v_del, (SELECT count(*) FROM open.open_api_credential_scope);
    RAISE NOTICE 'V300198 提醒：此后鉴权以 open_api_credential_scope 为准，scope_str 仅作可读快照，改授权范围须两处同写';
END $$;

COMMIT;
