-- =====================================================================
-- FinvQuant PostgreSQL V300208：接口清单登记采集端指标回传接口
--
-- 需求：11248。
-- 授权依据：用户在 2026-11-12 会话中明确要求新增
--   POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert，
--   供 Cloudflare Worker 采集端把一只证券的完整指标回传，服务端按 usc 关联
--   Upsert 到 cdeu.finv_quote_secu_indicator / futu.finv_futu_quote_collect，
--   并把 futu.finv_futu_secu_key.status 置 DONE（后续采集跳过该证券）。
--
-- 编码：finv.quote.cdeu.realtime.ftsecuindicatorupsert
--   catalog = finv.quote.cdeu.realtime（编码最后一个点之前，V300204 生成列自动算出）
--   func    = FTSecuIndicatorUpsert    （path 最后一个斜杠之后，大驼峰，V300204 生成列算出）
--   alias   = finv.quote.cdeu.realtime.FTSecuIndicatorUpsert（V300206 生成列算出）
--   —— 三列都是生成列，INSERT 时**不得显式列出**（写了 PostgreSQL 直接报错）。
--
-- 写接口：flag_write='1'，静态 Key 调用必须带查询参数 `_`（毫秒时间戳，±60 秒），
--   与 Min/Upsert、FTStockUpsert 同口径。
--
-- 注意：本表是「接口目录 + 元数据」，**不是鉴权白名单**。本迁移只登记元数据，
--   不授予任何凭证的授权范围；要在页面给具体凭证勾上该接口后调用方才可用。
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
    ('finv.quote.cdeu.realtime.ftsecuindicatorupsert', 'POST',
     '/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert',
     '采集端证券指标回传', '实时行情', '1', '1', '1',
     'router.go openGroup.POST（deps.ExportArchive 非空时注册）；写接口，静态 Key 调用必须带查询参数 `_`。'
     || '一次事务写三张表：cdeu.finv_quote_secu_indicator（指标）+ futu.finv_futu_quote_collect（采集参数）'
     || '+ 把 futu.finv_futu_secu_key.status 置 DONE（采集端后续跳过）。'
     || 'usc 解析顺序：key 表 → indicator.ftmm_symbol → collect.ftmm_symbol → ftmm_symbol 兜底（需求 11248）',
     'migrate')
ON CONFLICT DO NOTHING;

DO $$
DECLARE
    v_cnt    BIGINT;
    v_cat    TEXT;
    v_func   TEXT;
    v_alias  TEXT;
    v_write  CHAR(1);
BEGIN
    SELECT count(*) INTO v_cnt FROM open.open_api_endpoint
     WHERE api_code = 'finv.quote.cdeu.realtime.ftsecuindicatorupsert';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V300208 校验失败：采集端指标回传接口未登记（找到 % 条）', v_cnt;
    END IF;

    SELECT catalog, func, alias, flag_write INTO v_cat, v_func, v_alias, v_write
      FROM open.open_api_endpoint
     WHERE api_code = 'finv.quote.cdeu.realtime.ftsecuindicatorupsert';

    IF v_cat <> 'finv.quote.cdeu.realtime' OR v_func <> 'FTSecuIndicatorUpsert'
       OR v_alias <> 'finv.quote.cdeu.realtime.FTSecuIndicatorUpsert' THEN
        RAISE EXCEPTION 'V300208 校验失败：派生列不符合预期（catalog=%, func=%, alias=%）',
            v_cat, v_func, v_alias;
    END IF;

    IF v_write <> '1' THEN
        RAISE EXCEPTION 'V300208 校验失败：flag_write 应为 1（写接口），实际 %', v_write;
    END IF;

    RAISE NOTICE 'V300208 就绪：采集端指标回传接口已登记 %（catalog=% / func=% / alias=%）',
        'POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert',
        v_cat, v_func, v_alias;
    RAISE NOTICE 'V300208 提示：当前接口清单共 % 条', (SELECT count(*) FROM open.open_api_endpoint);
END $$;

COMMIT;
