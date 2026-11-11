-- =====================================================================
-- FinvQuant PostgreSQL V300200：实时行情写入接口的 api_code 改为 ftstockupsert
--
-- 需求：11234。
-- 授权依据：用户在 2026-11-11 会话中要求把
--   `finv.quote.cdeu.realtime.stockupsert` 改为 `finv.quote.cdeu.realtime.ftstockupsert`。
--
-- 理由：该接口的路径是 `/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`
-- （FT = 富途 Futu 数据源），编码少了 `ft` 与路径对不上，人工核对时容易误认为另一条接口。
-- 编码改为与路径末段一致：`finv.quote.cdeu.realtime.ftstockupsert`（38 字符，列宽 48 内）。
--
-- 安全性：审计与授权范围都用 `(http_method, path)` 关联本表，`api_code` 只是人读标识，
-- 改名后历史审计随 join 自动显示新编码，无需回刷数据。
--
-- 幂等：只在「当前值仍是旧编码」时更新；重跑影响 0 行。
-- 收敛校验：断言 idn=3 已是新编码（脚本自己能保证的结构性事实）。
-- 前置：V300193（接口清单）、V300199（加 finv.quote. 前缀）。
-- =====================================================================

BEGIN;

UPDATE open.open_api_endpoint
   SET api_code  = 'finv.quote.cdeu.realtime.ftstockupsert',
       modify_by = 'migrate'
 WHERE idn = 3
   AND api_code = 'finv.quote.cdeu.realtime.stockupsert';

DO $$
DECLARE
    v_code TEXT;
BEGIN
    SELECT api_code INTO v_code FROM open.open_api_endpoint WHERE idn = 3;
    IF v_code IS NULL THEN
        RAISE NOTICE 'V300200 提示：idn=3 的接口不存在（本库未登记该接口），本次不做修改';
        RETURN;
    END IF;
    IF v_code <> 'finv.quote.cdeu.realtime.ftstockupsert' THEN
        RAISE EXCEPTION 'V300200 校验失败：idn=3 的 api_code 应为 finv.quote.cdeu.realtime.ftstockupsert，实际 %', v_code;
    END IF;

    RAISE NOTICE 'V300200 就绪：idn=3 的接口编码为 %', v_code;
    RAISE NOTICE 'V300200 提示：当前清单编码 %',
        (SELECT string_agg(api_code, ', ' ORDER BY idn) FROM open.open_api_endpoint);
END $$;

COMMIT;
