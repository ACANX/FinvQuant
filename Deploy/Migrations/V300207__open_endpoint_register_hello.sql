-- =====================================================================
-- FinvQuant PostgreSQL V300207：接口清单登记握手接口（ClientHello / ServerHello）
--
-- 需求：11242。
-- 授权依据：用户在 2026-11-11 会话中要求新增
--   GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello 用于客户端与服务端的 HTTP 连通性握手，
--   参照 TLS 握手思路：客户端发 ClientHello（先完成认证）→ 服务端回 ServerHello。
--
-- 编码：open.region.az.server.hello（与 path 段一一对应：Meta/Region/AZ/Server/Hello）
--   catalog = open.region.az.server（编码最后一个点之前，由 V300204 生成列自动算出）
--   func    = Hello                 （path 最后一个斜杠之后，大驼峰，由 V300204 生成列算出）
--   alias   = open.region.az.server.Hello（由 V300206 生成列算出）
--   —— 三列都是生成列，INSERT 时**不得显式列出**（写了 PostgreSQL 直接报错）。
--
-- 本接口的特殊性（写进 remark，页面 ⓘ 可见）：
--   认证通过即放行，**不校验凭证授权范围**（middleware 的 scopeFreePaths 豁免）；
--   但凭证类型隔离照旧——STS 兑换凭证仍只能调 STS/Issue，不能握手。
--
-- 幂等：ON CONFLICT DO NOTHING；重跑影响 0 行。
-- 收敛校验：断言该编码已登记、且 catalog / func / alias 派生正确。
-- 前置：V300193（表与种子）、V300204（catalog/func）、V300206（alias）。
-- =====================================================================

BEGIN;

INSERT INTO open.open_api_endpoint
    (api_code, http_method, path, api_name, api_group, flag_write, flag_public, flag_enable,
     remark, create_by)
VALUES
    ('open.region.az.server.hello', 'GET', '/OpenAPI/V1/Meta/Region/AZ/Server/Hello',
     '握手连通性测试（ServerHello）', '握手', '0', '1', '1',
     'ClientHello→ServerHello 连通性握手（需求 11242）：只验认证与连通性、无业务副作用；'
     || '豁免授权范围校验（凭证 scope 无需包含本接口），但凭证类型隔离照旧——STS 兑换凭证仍只能调 STS/Issue',
     'migrate')
ON CONFLICT DO NOTHING;

DO $$
DECLARE
    v_cnt    BIGINT;
    v_bad    BIGINT;
    v_cat    TEXT;
    v_func   TEXT;
    v_alias  TEXT;
BEGIN
    SELECT count(*) INTO v_cnt FROM open.open_api_endpoint
     WHERE api_code = 'open.region.az.server.hello';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V300207 校验失败：握手接口未登记（找到 % 条）', v_cnt;
    END IF;

    SELECT catalog, func, alias INTO v_cat, v_func, v_alias
      FROM open.open_api_endpoint WHERE api_code = 'open.region.az.server.hello';

    IF v_cat <> 'open.region.az.server' OR v_func <> 'Hello'
       OR v_alias <> 'open.region.az.server.Hello' THEN
        RAISE EXCEPTION 'V300207 校验失败：派生列不符合预期（catalog=%, func=%, alias=%）',
            v_cat, v_func, v_alias;
    END IF;

    RAISE NOTICE 'V300207 就绪：握手接口已登记 %（catalog=% / func=% / alias=%）',
        'GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello', v_cat, v_func, v_alias;
    RAISE NOTICE 'V300207 提示：当前接口清单共 % 条', (SELECT count(*) FROM open.open_api_endpoint);
END $$;

COMMIT;
