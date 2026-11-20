-- =====================================================================
-- FinvQuant PostgreSQL V300146：cdeu.finv_quote_secu_indicator 补齐 131 列中文注释（需求 11053）
--
-- 背景：V300144 建表时只给表与 8 个关键列写了注释，其余 123 列在 \d 里是空的，排查口径不便。
-- 来源：列中文说明取自上游采集脚本
--   `FinvQuantDesign` 之外的设计仓：CodeSnippet/TypeScript/FtmmSecuMetaFetch/sql/schema.pg.sql
--   （`COMMENT ON COLUMN "finv_quote_secu_meta"` 段落，由该仓 spec/fields.json 生成）；
--   本表缺失的 14 个身份/标记列不在本表，无需注释。
--
-- 本表专有的 10 列用更贴合本地语义的措辞覆盖上游说明（usc / ftmm_symbol / ts / secu_url /
--   source_channel / dt_collect / dt_create / dt_update / count_update / json_unmapped），
--   其中 usc 明确「上游为空时由同步用 ftmm_symbol 占位」、dt_update 明确「由触发器刷新」。
--
-- 幂等：COMMENT ON 本身可重复执行（覆盖同值），重跑无副作用。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；
--   末尾校验「131 列全部有非空注释」且逐列注释长度 > 0。
-- =====================================================================

BEGIN;

COMMENT ON TABLE cdeu.finv_quote_secu_indicator IS
    '证券指标表（列结构与上游 Supabase public.finv_quote_secu_meta 同构，131 列，每列均带中文注释）：'
    '一行=一个证券的最新指标快照，主键 usc；只保最新、不建历史表；'
    '由「证券参数同步 → 证券指标数据同步」分批增量写入（需求 11047/11048/11049/11053）';

-- ---------------------------------------------------------------------
-- 131 列中文注释（列序与建表一致）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.usc IS '证券统一编码（主键；上游为 NULL/空时由同步用 ftmm_symbol 占位）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ftmm_symbol IS '富途符号 {市场}.{代码}（如 US.AAPL；冗余，便于与 futu.finv_futu_secu_key 按符号关联）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.name IS '中文名称';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.lot_size IS '每手股数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.spread_code IS '点差/精度档位';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ex_type IS '交易所/证券类型枚举';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_accuracy IS '价格小数位';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume_precision IS '成交量 / 委托量精度';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.order_volume_precision IS '成交量 / 委托量精度';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price IS '最新价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_price IS '涨跌额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_ratio IS '涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_direct IS '涨跌方向 `up/down/flat`';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_open IS '今开';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_highest IS '今日最高';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_lowest IS '今日最低';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_last_close IS '昨收';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_nominal IS '名义最新价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.hight_direct IS '最高/开盘/最低相对昨收的方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.open_direct IS '最高/开盘/最低相对昨收的方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.low_direct IS '最高/开盘/最低相对昨收的方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume IS '成交量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.turnover IS '成交额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ratio_volume IS '量比';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ratio_turnover IS '换手率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.amplitude_price IS '振幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_average IS '均价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_speed_price IS '涨速';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ratio_bid_ask IS '委比';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.total_shares IS '总股本';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.total_market_cap IS '总市值';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.outstanding_shares IS '流通股本';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.outstanding_market_cap IS '流通市值';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_highest_week52 IS '52 周最高';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_lowest_week52 IS '52 周最低';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_highest_history IS '历史最高';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_lowest_history IS '历史最低';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_bid IS '买一 / 卖一价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_ask IS '买一 / 卖一价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume_bid IS '买一 / 卖一量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume_ask IS '买一 / 卖一量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume_sold IS '内盘 / 外盘';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.volume_bought IS '内盘 / 外盘';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_limit_up IS '涨停价 / 跌停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.price_limit_down IS '涨停价 / 跌停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.kcb_static_volume IS '静态统计成交量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.kcb_static_turnover IS '静态统计成交额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pe_ttm IS '市盈率 TTM';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pe_lyr IS '市盈率 LYR';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pb_ratio IS '市净率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.eps_ttm IS '每股收益 TTM';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.eps_lyr IS '每股收益 LYR';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ps_ttm IS '市销率 TTM / LYR';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ps_lyr IS '市销率 TTM / LYR';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dividend IS '每股股息';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dividend_ratio IS '股息率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dividend_lfy IS '上一财年股息';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dividend_lfy_ratio IS '上一财年股息率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ts IS '行情时间戳（epoch 毫秒，上游原值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.market_status IS '交易状态码';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.market_status_text IS '交易状态文本';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.delay_time IS '行情延迟秒数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.delay_seconds IS '行情延迟秒数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ts_exchange_dt IS '交易所数据时间';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ts_server_send_to_client IS '服务端下发时间';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ts_server_recv_from_exchange IS '服务端收到交易所数据时间';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_status IS '盘前状态码';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_exchange_time IS '盘前行情时间';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_price IS '盘前价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_change IS '盘前涨跌额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_change_ratio IS '盘前涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.before_open_price_direct IS '盘前涨跌方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_dividend_yield IS '股息率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_aum IS '基金资产规模';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_outstanding_units IS '基金总份额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_net_asset_value IS '基金净值';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_premium IS '溢价率';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_asset_class IS '资产类别枚举';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_asset_class_sc IS '资产类别简体名';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_asset_class_tc IS '资产类别繁体名';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.trust_asset_class_en IS '资产类别英文名';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_last_settlement_price IS '上次结算价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_hold_vol IS '持仓量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_diff_hold_vol IS '持仓变化';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_main_flag IS '是否主力合约标记';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_distance_due_date IS '距到期天数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_price_limit_up IS '期货涨停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.future_price_limit_down IS '期货跌停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_stock_id IS '标的证券内部 ID';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_stock_code IS '标的代码';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_name IS '标的名称';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_market_label IS '标的所在市场';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_instrument_type IS '标的品种类型';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_price IS '标的现价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_change IS '标的涨跌额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_change_ratio IS '标的涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_price_direct IS '标的涨跌方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.underlying_price_nominal IS '标的名义价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pod_text IS '升贴水文本';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pod_value IS '升贴水数值';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.pod_direct IS '升贴水方向';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.hp_multiplier IS '合约乘数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.last_trade_date IS '最后交易日';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.timezone IS '交易所时区';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.index_count_all IS '成分股总数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.index_count_raise IS '上涨家数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.index_count_fall IS '下跌家数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.index_count_equal IS '平盘家数';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_price_highest IS '24h 最高价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_price_lowest IS '24h 最低价';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_volume IS '24h 成交量';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_turnover IS '24h 成交额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_price_change IS '24h 涨跌额';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stat_h24_ratio_price_change IS '24h 涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.difference IS '报价量纲基数（待确认）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.bid_price_lowest IS '可成交下边界（待确认）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ask_price_highest IS '可成交上边界（待确认）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d005 IS '5 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d010 IS '10 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d020 IS '20 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d060 IS '60 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d120 IS '120 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_d250 IS '250 日涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.change_year IS '年初至今涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.secu_url IS '来源 URL（moomoo 证券页；同步到 futu.finv_futu_secu_key.url，需求 11050）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.source_channel IS '来源通道（如 http_requests）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_collect IS '上游采集时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_create IS '上游首次采集时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_update IS '最后更新时间（可空；由触发器 finv_quote_secu_indicator_set_dt_update 在 UPDATE 时刷新）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.count_update IS '上游累计更新次数（原样同步）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.json_unmapped IS '上游未登记字段的原始 JSON（排查字段口径变化用）';

-- ---------------------------------------------------------------------
-- 收敛校验：131 列必须全部有非空注释
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total    int;
    v_commented int;
    v_missing  text;
BEGIN
    SELECT count(*) INTO v_total FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_indicator';

    SELECT count(*), string_agg(c.column_name, ', ' ORDER BY c.ordinal_position)
      INTO v_commented, v_missing
      FROM information_schema.columns c
     WHERE c.table_schema = 'cdeu' AND c.table_name = 'finv_quote_secu_indicator'
       AND COALESCE(col_description('cdeu.finv_quote_secu_indicator'::regclass, c.ordinal_position), '') <> '';

    IF v_commented <> 131 OR v_total <> 131 THEN
        RAISE EXCEPTION 'V300146 注释未补齐：有注释 % / 总列 %（期望均为 131）', v_commented, v_total;
    END IF;

    RAISE NOTICE 'V300146 就绪：cdeu.finv_quote_secu_indicator 全部 % 列已有中文注释', v_commented;
END $$;

COMMIT;
