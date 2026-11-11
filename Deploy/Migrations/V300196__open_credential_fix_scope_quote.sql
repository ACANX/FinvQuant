-- =====================================================================
-- FinvQuant PostgreSQL V300196：修复凭证 idn=20 的 scope_str 脏路径（尾部多余双引号）
--
-- 需求：11226（数据修复）。
-- 授权依据：用户在 2026-11-11 会话中列出「凭证 idn=20 的 GET 规则：那条带 " 的路径永远匹配不上」
--   并批准做一次数据修复。
--
-- 问题（由 V300194 的对账暴露，V300194 只清了镜像表、没动源数据）：
--   open.open_api_credential 中 idn=20（CRED-20-…）的 scope_str 有一条：
--       {"path": "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/*\"", "method": "GET"}
--   路径尾部多了一个双引号。鉴权侧 InScope 只认「精确相等 / 尾部 `/*` / 尾部 `/`」三种，
--   末尾是 `"` 时三者都不成立 → 这条规则**永远匹配不上**，该凭证实际只有 POST 权限。
--
-- 修法（保守，不改变授权语义）：
--   去掉末尾多余的 `"`，恢复成用户原本想要的通配 `GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/*`。
--   **不**改成精确路径 PageUni，也不扩大/缩小作者原意——只修「转义多写一个引号」这个笔误。
--   若业务上希望收窄成 PageUni，请另行提出（那是权限变更，不是数据修复）。
--
-- 合规（DbSchemaSpec §0.4）：
--   ① 修前整行快照备份到 archive.open_api_credential_purge_20261111（复用 V300195 的表，
--      主键 (src_schema, idn) 已有 idn=20 的记录则跳过，避免覆盖上一次备份）；
--   ② 单事务「备份 → 修改 → 校验」；
--   ③ 校验用 RAISE EXCEPTION；找不到目标行时只 NOTICE 不动（幂等）。
--
-- 幂等：找不到「含尾部引号的路径」时直接返回，重跑 0 变更。
-- 前置：V300183（凭证表）、V300195（备份表）、V300192（scope 镜像表）。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS archive AUTHORIZATION CURRENT_USER;

DO $$
DECLARE
    v_cred_idn   BIGINT;
    v_bad_path   TEXT;
    v_good_path  TEXT;
    v_new_scope  JSONB;
    v_left       BIGINT;
    v_reason     TEXT := '需求 11226：修复 scope_str 里尾部多一个双引号的路径（原本永远匹配不上，等于该凭证丢了 GET 权限）';
BEGIN
    -- 1. 定位（只处理「路径以双引号结尾」这一种脏形态，别的脏形态不动、只提示）
    SELECT c.idn, elem->>'path'
      INTO v_cred_idn, v_bad_path
      FROM open.open_api_credential c
      CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS elem
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND jsonb_typeof(elem) = 'object'
       AND btrim(elem->>'path') LIKE '%"'
     ORDER BY c.idn
     LIMIT 1;

    IF v_cred_idn IS NULL THEN
        RAISE NOTICE 'V300196：未发现「路径尾部带双引号」的授权规则（已是修复后的状态），本次不做修改';
        RETURN;
    END IF;

    v_good_path := rtrim(btrim(v_bad_path), '"');
    IF v_good_path = '' OR v_good_path !~ '^/OpenAPI/V1/' THEN
        RAISE EXCEPTION 'V300196 中止：凭证 idn=% 的修复目标路径非法（原值 %，去引号后 %）',
            v_cred_idn, v_bad_path, v_good_path;
    END IF;

    -- 2. 修前整行快照（已有备份则跳过，不覆盖上一次）
    INSERT INTO archive.open_api_credential_purge_20261111
        (src_schema, idn, credential_no, auth_mode, scope_str_old, scope_str_new, reason)
    -- 注意：jsonb_set 的数组下标**从 0 开始**，而 WITH ORDINALITY 从 1 开始，
    -- 直接用 ord 会错位（V300196 初版就踩了这个坑，由 V300197 修正历史备份行）。
    -- 这里改用与下面 UPDATE 完全相同的 CASE 重写表达式，彻底不依赖下标。
    SELECT 'open', c.idn, c.credential_no, c.auth_mode, c.scope_str,
           (SELECT jsonb_agg(
                       CASE
                           WHEN jsonb_typeof(elem) = 'object' AND btrim(elem->>'path') = v_bad_path
                               THEN jsonb_set(elem, '{path}', to_jsonb(v_good_path), false)
                           ELSE elem
                       END
                       ORDER BY ord)
              FROM jsonb_array_elements(c.scope_str) WITH ORDINALITY AS t(elem, ord)),
           v_reason
      FROM open.open_api_credential c
     WHERE c.idn = v_cred_idn
    ON CONFLICT (src_schema, idn) DO NOTHING;

    -- 3. 逐条修正：只把该路径的尾部引号去掉，其余元素原样保留（含顺序）
    SELECT jsonb_agg(
               CASE
                   WHEN jsonb_typeof(elem) = 'object' AND btrim(elem->>'path') = v_bad_path
                       THEN jsonb_set(elem, '{path}', to_jsonb(v_good_path), false)
                   ELSE elem
               END
               ORDER BY ord)
      INTO v_new_scope
      FROM open.open_api_credential c
      CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) WITH ORDINALITY AS t(elem, ord)
     WHERE c.idn = v_cred_idn;

    IF v_new_scope IS NULL THEN
        RAISE EXCEPTION 'V300196 中止：重写 scope_str 失败（凭证 idn=% 聚合结果为空）', v_cred_idn;
    END IF;

    UPDATE open.open_api_credential
       SET scope_str = v_new_scope,
           modify_by = 'migrate'
     WHERE idn = v_cred_idn;

    -- 4. 校验：不得再有「路径尾部带双引号」的规则
    SELECT count(*) INTO v_left
      FROM open.open_api_credential c
      CROSS JOIN LATERAL jsonb_array_elements(c.scope_str) AS elem
     WHERE jsonb_typeof(c.scope_str) = 'array'
       AND jsonb_typeof(elem) = 'object'
       AND btrim(elem->>'path') LIKE '%"';
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300196 校验失败：仍残留 % 条「路径尾部带双引号」的授权规则', v_left;
    END IF;

    RAISE NOTICE 'V300196 就绪：凭证 idn=% 的规则已由 % 修正为 %', v_cred_idn, v_bad_path, v_good_path;
    RAISE NOTICE 'V300196 提醒：该凭证的 SK 密文此前已不可解密，仍需在「OpenAPI 凭证」页点一次「轮换」才能正常验签';
END $$;

COMMIT;
