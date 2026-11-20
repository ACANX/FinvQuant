-- =====================================================================
-- V300218：证券列表采集配置补 ETF 两行（需求 11279）
--
-- 背景：页面「行情快照同步 → ETF」Tab 此前是占位（PLACEHOLDER_SYMBOLS），
--   没有配置行所以不可采集。现接入 moomoo get-etf-list：
--
--     港股 ETF：/quote-api/quote-v2/get-etf-list?routeType=1&plateId=900075&rankType=6&leverage=&page=0&pageSize=50
--     美股 ETF：/quote-api/quote-v2/get-etf-list?routeType=1&plateId=10002799&rankType=6&leverage=&page=0&pageSize=50
--
--   市场由 **plateId** 决定（该接口没有 marketType 参数），映射写在代码里：
--   internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go 的 etfPlateIDByMarket；
--   本迁移只负责让「市场 + 品种」这两行配置存在，采集器据此逐市场拉取。
--
-- 落库：与股票列表完全共用一条链路 —— 逐页 upsert 到
--   cdeu.finv_quote_secu_realtime（主键 usc，provider 默认 FTMM），
--   条目类型与股票同构（get-etf-list 返回的 instrumentType 为 4），
--   因此 usc 解析、币种补齐、待补键表、进度回写全部复用，无需新表。
--
-- count_plan 留 NULL：按接口 pagination.total 全量采集（每个市场多少条由站点决定）。
-- 幂等：INSERT ... ON CONFLICT (market_type, instrument_symbol) DO NOTHING；
--   末尾校验两行存在且启用，不符整事务回滚。
-- =====================================================================

BEGIN;

INSERT INTO cdeu.finv_cdeu_secu_realtime_sync_config
    (instrument_symbol, market_type, count_plan, remark)
VALUES
    ('etf', '1', NULL,
     'HK 港股 ETF（marketType=1）：moomoo get-etf-list?routeType=1&plateId=900075&rankType=6&leverage=&page=0&pageSize=50，'
     '按接口 total 全量采集；结果写 cdeu.finv_quote_secu_realtime（provider=FTMM、instrument_type=4），采集后回写 count_exists / total_available。'),
    ('etf', '2', NULL,
     'US 美股 ETF（marketType=2）：moomoo get-etf-list?routeType=1&plateId=10002799&rankType=6&leverage=&page=0&pageSize=50，'
     '按接口 total 全量采集；结果写 cdeu.finv_quote_secu_realtime（provider=FTMM、instrument_type=4），采集后回写 count_exists / total_available。')
ON CONFLICT (market_type, instrument_symbol) DO NOTHING;

DO $$
DECLARE
    v_rows   int;
    v_market text;
    v_off    int;
BEGIN
    SELECT count(*) INTO v_rows
      FROM cdeu.finv_cdeu_secu_realtime_sync_config
     WHERE instrument_symbol = 'etf';
    IF v_rows <> 2 THEN
        RAISE EXCEPTION 'V300218 ETF 配置行数不符：期望 2（HK/US），实际 %', v_rows;
    END IF;

    SELECT string_agg(market_type, ',' ORDER BY market_type) INTO v_market
      FROM cdeu.finv_cdeu_secu_realtime_sync_config
     WHERE instrument_symbol = 'etf' AND flag_enable = '1';
    IF v_market IS DISTINCT FROM '1,2' THEN
        RAISE EXCEPTION 'V300218 ETF 配置行市场/启用状态不符：期望启用 1,2，实际 %', COALESCE(v_market, '（无）');
    END IF;

    SELECT count(*) INTO v_off
      FROM cdeu.finv_quote_secu_realtime
     WHERE instrument_type = '4';  -- 仅作提示，不参与校验（表可能为空）
    RAISE NOTICE 'V300218 校验通过：ETF 配置 2 行（HK plateId=900075 / US plateId=10002799）；'
                 '当前 cdeu.finv_quote_secu_realtime 中 instrument_type=4 的既有 ETF 行数=%', v_off;
END $$;

COMMIT;
