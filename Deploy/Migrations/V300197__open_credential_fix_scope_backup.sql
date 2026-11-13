-- =====================================================================
-- FinvQuant PostgreSQL V300197：修正 V300196 写错的备份列 scope_str_new
--
-- 需求：11226（数据修复，承接 V300196）。
-- 授权依据：用户在 2026-11-11 会话中批准修复凭证 idn=20 的 scope_str；
--   本迁移只改 **V300196 自己刚写的备份行**（archive.open_api_credential_purge_20261111），
--   不碰任何业务表的实际数据。
--
-- 问题：V300196 初版生成备份列 scope_str_new 时用了
--   jsonb_set(scope_str, ARRAY[idx::text,'path'], ...)，其中 idx 来自 WITH ORDINALITY（1 起），
--   而 jsonb_set 的数组下标**从 0 起**，导致下标错位、create_missing=false 下静默不改值，
--   备份里 scope_str_new 与 scope_str_old 完全相同（实际业务表已被正确修复）。
--   追溯时会误以为"改了等于没改"，故此处按正确表达式重算一遍。
--
-- 幂等：只处理「old 里有尾部带引号的路径 且 new 与之相同」的行；重跑 0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT）。
-- 前置：V300196（备份行）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_fixed  BIGINT;
    v_left   BIGINT;
    v_reason TEXT := '需求 11226：按正确表达式重算备份列 scope_str_new（V300196 初版下标错位）';
BEGIN
    WITH target AS (
        SELECT p.src_schema, p.idn, p.scope_str_old
          FROM archive.open_api_credential_purge_20261111 p
         WHERE p.scope_str_old IS NOT NULL
           AND p.scope_str_old::text LIKE '%\"}%'
           AND p.scope_str_new IS NOT DISTINCT FROM p.scope_str_old
    )
    UPDATE archive.open_api_credential_purge_20261111 p
       SET scope_str_new = (
               SELECT jsonb_agg(
                           CASE
                               WHEN jsonb_typeof(elem) = 'object' AND btrim(elem->>'path') LIKE '%"'
                                   THEN jsonb_set(elem, '{path}', to_jsonb(rtrim(btrim(elem->>'path'), '"')), false)
                               ELSE elem
                           END
                           ORDER BY ord)
                 FROM jsonb_array_elements(t.scope_str_old) WITH ORDINALITY AS x(elem, ord)
           ),
           reason = COALESCE(p.reason, '') || '｜' || v_reason
      FROM target t
     WHERE p.src_schema = t.src_schema AND p.idn = t.idn;

    GET DIAGNOSTICS v_fixed = ROW_COUNT;

    -- 校验：不得再有「old 带尾部引号 且 new 与 old 相同」的行
    SELECT count(*) INTO v_left
      FROM archive.open_api_credential_purge_20261111 p
     WHERE p.scope_str_old IS NOT NULL
       AND p.scope_str_old::text LIKE '%\"}%'
       AND p.scope_str_new IS NOT DISTINCT FROM p.scope_str_old;
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300197 校验失败：仍残留 % 条 new 与 old 相同的备份行', v_left;
    END IF;

    RAISE NOTICE 'V300197 就绪：修正备份行 % 条', v_fixed;
END $$;

COMMIT;
