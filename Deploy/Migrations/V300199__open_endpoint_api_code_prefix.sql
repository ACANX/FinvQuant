-- =====================================================================
-- FinvQuant PostgreSQL V300199：行情类接口的 api_code 加 `finv.quote.` 前缀
--
-- 需求：11233。
-- 授权依据：用户在 2026-11-11 会话中要求「OpenAPI 接口清单中 idn=1/2/3 的记录，接口编码加上前缀 finv.quote.」。
--
-- 背景：接口清单（V300193）首批种子用了 `cdeu.…` 前缀，只体现模块不体现业务域。
-- 加上 `finv.quote.` 后与库表命名（`finv_quote_*`）、菜单层级（行情 → 收剔导传ETL）对齐：
--   idn=1  cdeu.min.pageuni            → finv.quote.cdeu.min.pageuni
--   idn=2  cdeu.min.upsert             → finv.quote.cdeu.min.upsert
--   idn=3  cdeu.realtime.stockupsert   → finv.quote.cdeu.realtime.stockupsert
--   idn=4  auth.sts.issue              保持不变（属鉴权域，不是行情）
--
-- 为什么改编码是安全的：审计报表与授权范围都用 `(http_method, path)` 关联本表，
-- `api_code` 只是人读的稳定标识（重命名路径时它不变）；本次改的是编码本身，
-- 历史审计记录会随 join 自动显示新编码，无需回刷任何数据。
--
-- 幂等：只在「当前还没有 finv.quote. 前缀」时更新；重跑影响 0 行。
-- 收敛校验：只断言目标行的 api_code 已带前缀（脚本自己能保证），其它行只 NOTICE。
-- 前置：V300193（接口清单表与种子）。
-- =====================================================================

BEGIN;

UPDATE open.open_api_endpoint
   SET api_code   = 'finv.quote.' || api_code,
       modify_by  = 'migrate'
 WHERE idn IN (1, 2, 3)
   AND api_code NOT LIKE 'finv.quote.%';

DO $$
DECLARE
    v_ok    BIGINT;
    v_total BIGINT;
BEGIN
    SELECT count(*) INTO v_ok
      FROM open.open_api_endpoint
     WHERE idn IN (1, 2, 3) AND api_code LIKE 'finv.quote.%';
    IF v_ok <> 3 THEN
        RAISE EXCEPTION 'V300199 校验失败：idn=1/2/3 的 api_code 应全部带 finv.quote. 前缀，实际 % 条', v_ok;
    END IF;

    SELECT count(*) INTO v_total FROM open.open_api_endpoint;
    RAISE NOTICE 'V300199 就绪：行情类接口编码已加 finv.quote. 前缀（清单共 % 条）', v_total;

    -- 其它接口（鉴权域等）不在此次调整范围，仅提示当前取值便于人工核对
    RAISE NOTICE 'V300199 提示：未加前缀的接口编码为 %',
        (SELECT string_agg(api_code, ', ' ORDER BY idn)
           FROM open.open_api_endpoint WHERE api_code NOT LIKE 'finv.quote.%');
END $$;

COMMIT;
