-- =====================================================================
-- FinvQuant PostgreSQL V300201：STS 换证接口的 api_code 改为 open.auth.sts.issue
--
-- 需求：11235。
-- 授权依据：用户在 2026-11-11 会话中要求把 `auth.sts.issue` 改为 `open.auth.sts.issue`。
--
-- 理由：接口清单里编码按「业务域.…」分段，`open` 对应开放平台域（与 open schema 一致），
-- 与上一轮行情域的 `finv.quote.` 前缀形成同一套口径；只写 `auth.` 没有域前缀，看起来像顶级域。
--
-- 安全性：审计与授权范围都用 `(http_method, path)` 关联本表，`api_code` 只是人读标识，
-- 改名后历史审计随 join 自动显示新编码，无需回刷数据。
--
-- 幂等：只在「当前值仍是旧编码」时更新；重跑影响 0 行。
-- 收敛校验：断言 idn=4 已是新编码（脚本自己能保证的结构性事实）。
-- 前置：V300193（接口清单）、V300199/V300200（行情域编码调整）。
-- =====================================================================

BEGIN;

UPDATE open.open_api_endpoint
   SET api_code  = 'open.auth.sts.issue',
       modify_by = 'migrate'
 WHERE idn = 4
   AND api_code = 'auth.sts.issue';

DO $$
DECLARE
    v_code TEXT;
BEGIN
    SELECT api_code INTO v_code FROM open.open_api_endpoint WHERE idn = 4;
    IF v_code IS NULL THEN
        RAISE NOTICE 'V300201 提示：idn=4 的接口不存在（本库未登记该接口），本次不做修改';
        RETURN;
    END IF;
    IF v_code <> 'open.auth.sts.issue' THEN
        RAISE EXCEPTION 'V300201 校验失败：idn=4 的 api_code 应为 open.auth.sts.issue，实际 %', v_code;
    END IF;

    RAISE NOTICE 'V300201 就绪：idn=4 的接口编码为 %', v_code;
    RAISE NOTICE 'V300201 提示：当前清单编码 %',
        (SELECT string_agg(api_code, ', ' ORDER BY idn) FROM open.open_api_endpoint);
END $$;

COMMIT;
