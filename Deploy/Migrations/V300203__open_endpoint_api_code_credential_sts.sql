-- =====================================================================
-- FinvQuant PostgreSQL V300203：STS 换证接口编码改为 open.auth.credential.sts.issue
--
-- 需求：11237。
-- 授权依据：用户在 2026-11-11 会话中要求把 `open.auth.sts.issue` 改为
--   `open.auth.credential.sts.issue`。
--
-- 理由：与其它接口编码保持「业务域.模块.资源.动作」的层级——
--   行情类：finv.quote.cdeu.min.pageuni / finv.quote.cdeu.realtime.ftstockupsert
--   鉴权类：open.auth.credential.sts.issue（open 域 → auth 模块 → credential 资源 → sts.issue 动作）
--   原编码缺「资源」这一段，与页面/表名（open_api_credential）对不上。
--
-- 安全性：审计与授权范围都用 `(http_method, path)` 关联本表，`api_code` 只是人读标识，
-- 改名后历史审计随 join 自动显示新编码，无需回刷数据。
--
-- 幂等：只在「当前值仍是旧编码」时更新；重跑影响 0 行。
-- 收敛校验：断言 idn=4 已是新编码（脚本自己能保证的结构性事实）。
-- 前置：V300193、V300201（改名为 open.auth.sts.issue）。
-- =====================================================================

BEGIN;

UPDATE open.open_api_endpoint
   SET api_code  = 'open.auth.credential.sts.issue',
       modify_by = 'migrate'
 WHERE idn = 4
   AND api_code = 'open.auth.sts.issue';

DO $$
DECLARE
    v_code TEXT;
BEGIN
    SELECT api_code INTO v_code FROM open.open_api_endpoint WHERE idn = 4;
    IF v_code IS NULL THEN
        RAISE NOTICE 'V300203 提示：idn=4 的接口不存在（本库未登记该接口），本次不做修改';
        RETURN;
    END IF;
    IF v_code <> 'open.auth.credential.sts.issue' THEN
        RAISE EXCEPTION 'V300203 校验失败：idn=4 的 api_code 应为 open.auth.credential.sts.issue，实际 %', v_code;
    END IF;

    RAISE NOTICE 'V300203 就绪：idn=4 的接口编码为 %', v_code;
    RAISE NOTICE 'V300203 提示：当前清单编码 %',
        (SELECT string_agg(api_code, ', ' ORDER BY idn) FROM open.open_api_endpoint);
END $$;

COMMIT;
