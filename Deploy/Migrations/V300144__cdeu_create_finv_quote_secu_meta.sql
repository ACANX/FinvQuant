-- =====================================================================
-- FinvQuant PostgreSQL V300144：cdeu schema 新建证券指标表 finv_quote_secu_meta（需求 11048）
--
-- 来源：用户于 2026-11-09 提供的上游（Supabase public.finv_quote_secu_meta）建表脚本，
--   本迁移在 **cdeu schema** 建同构表，作为「证券指标数据同步」（需求 11047）的本地落点。
--   列名 / 类型 / 可空性 / 主键 / 索引 / 触发器名与上游脚本一一对应，便于逐列映射与差异核对。
--
-- 与上游脚本的差异（均为本仓库约定，不改语义）：
--   1. 表放 `cdeu.` schema（上游是 `public.`）：本地 `public` 域不放这张表，避免与
--      `futu.` / `cdeu.` 的同名表混淆（DbSchemaSpec §0：同名表是刻意的隔离设计）；
--   2. 触发器函数建在 `cdeu` schema（同名 `finv_quote_secu_meta_set_dt_update`），
--      不复用 `public.vq_set_gmt_update()`——后者写的是 `gmt_update` 列，本表列名是 `dt_update`；
--   3. 全部 DDL 幂等（CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS /
--      触发器先查 `pg_trigger` 再建），空库与已部署库都可重复执行；
--   4. `dt_collect` / `dt_create` / `dt_update` 按上游脚本保持**可空、无默认值**
--      （值由同步写入上游时间戳），仅由触发器在 UPDATE 时刷新 `dt_update`；
--   5. 补表注释与关键列注释（上游脚本未带注释）。
--
-- 表语义：一行 = 一个证券的最新指标快照（主键 `usc`；`ftmm_symbol` 冗余，便于与
--   `futu.finv_futu_secu_key` 按富途符号关联）；只保最新、不建历史表；
--   `json_unmapped` 保存上游未映射字段的原始 JSON。
--
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾收敛校验
--   （列数 131、numeric(30,10) 列数 80、主键 usc、两个索引、触发器及触发器行为自检）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 触发器函数：UPDATE 时刷新 dt_update（幂等：CREATE OR REPLACE）
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION cdeu.finv_quote_secu_meta_set_dt_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.dt_update := now();
    RETURN NEW;
END;
$$;

COMMENT ON FUNCTION cdeu.finv_quote_secu_meta_set_dt_update() IS
    'cdeu.finv_quote_secu_meta 的 BEFORE UPDATE 触发器函数：把 dt_update 刷新为 now()（对齐上游 public.finv_quote_secu_meta）';

-- ---------------------------------------------------------------------
-- 2. 建表（131 列，主键 usc）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_secu_meta (
    usc text not null,
    ftmm_symbol text not null,
    name text null,
    lot_size integer null,
    spread_code text null,
    ex_type smallint null,
    price_accuracy smallint null,
    volume_precision smallint null,
    order_volume_precision smallint null,
    price numeric(30, 10) null,
    change_price numeric(30, 10) null,
    change_ratio numeric(30, 10) null,
    price_direct text null,
    price_open numeric(30, 10) null,
    price_highest numeric(30, 10) null,
    price_lowest numeric(30, 10) null,
    price_last_close numeric(30, 10) null,
    price_nominal numeric(30, 10) null,
    hight_direct text null,
    open_direct text null,
    low_direct text null,
    volume numeric(30, 10) null,
    turnover numeric(30, 10) null,
    ratio_volume numeric(30, 10) null,
    ratio_turnover numeric(30, 10) null,
    amplitude_price numeric(30, 10) null,
    price_average numeric(30, 10) null,
    change_speed_price numeric(30, 10) null,
    ratio_bid_ask numeric(30, 10) null,
    total_shares numeric(30, 10) null,
    total_market_cap numeric(30, 10) null,
    outstanding_shares numeric(30, 10) null,
    outstanding_market_cap numeric(30, 10) null,
    price_highest_week52 numeric(30, 10) null,
    price_lowest_week52 numeric(30, 10) null,
    price_highest_history numeric(30, 10) null,
    price_lowest_history numeric(30, 10) null,
    price_bid numeric(30, 10) null,
    price_ask numeric(30, 10) null,
    volume_bid numeric(30, 10) null,
    volume_ask numeric(30, 10) null,
    volume_sold numeric(30, 10) null,
    volume_bought numeric(30, 10) null,
    price_limit_up numeric(30, 10) null,
    price_limit_down numeric(30, 10) null,
    kcb_static_volume numeric(30, 10) null,
    kcb_static_turnover numeric(30, 10) null,
    pe_ttm numeric(30, 10) null,
    pe_lyr numeric(30, 10) null,
    pb_ratio numeric(30, 10) null,
    eps_ttm numeric(30, 10) null,
    eps_lyr numeric(30, 10) null,
    ps_ttm numeric(30, 10) null,
    ps_lyr numeric(30, 10) null,
    dividend numeric(30, 10) null,
    dividend_ratio numeric(30, 10) null,
    dividend_lfy numeric(30, 10) null,
    dividend_lfy_ratio numeric(30, 10) null,
    ts bigint null,
    market_status smallint null,
    market_status_text text null,
    delay_time smallint null,
    delay_seconds smallint null,
    ts_exchange_dt bigint null,
    ts_server_send_to_client bigint null,
    ts_server_recv_from_exchange bigint null,
    before_open_status smallint null,
    before_open_exchange_time bigint null,
    before_open_price numeric(30, 10) null,
    before_open_change numeric(30, 10) null,
    before_open_change_ratio numeric(30, 10) null,
    before_open_price_direct numeric(30, 10) null,
    trust_dividend_yield numeric(30, 10) null,
    trust_aum numeric(30, 10) null,
    trust_outstanding_units numeric(30, 10) null,
    trust_net_asset_value numeric(30, 10) null,
    trust_premium numeric(30, 10) null,
    trust_asset_class smallint null,
    trust_asset_class_sc text null,
    trust_asset_class_tc text null,
    trust_asset_class_en text null,
    future_last_settlement_price numeric(30, 10) null,
    future_hold_vol numeric(30, 10) null,
    future_diff_hold_vol numeric(30, 10) null,
    future_main_flag smallint null,
    future_distance_due_date smallint null,
    future_price_limit_up numeric(30, 10) null,
    future_price_limit_down numeric(30, 10) null,
    underlying_stock_id text null,
    underlying_stock_code text null,
    underlying_name text null,
    underlying_market_label text null,
    underlying_instrument_type smallint null,
    underlying_price numeric(30, 10) null,
    underlying_change numeric(30, 10) null,
    underlying_change_ratio numeric(30, 10) null,
    underlying_price_direct text null,
    underlying_price_nominal numeric(30, 10) null,
    pod_text text null,
    pod_value numeric(30, 10) null,
    pod_direct text null,
    hp_multiplier integer null,
    last_trade_date text null,
    timezone text null,
    index_count_all integer null,
    index_count_raise integer null,
    index_count_fall integer null,
    index_count_equal integer null,
    stat_h24_price_highest numeric(30, 10) null,
    stat_h24_price_lowest numeric(30, 10) null,
    stat_h24_volume numeric(30, 10) null,
    stat_h24_turnover numeric(30, 10) null,
    stat_h24_price_change numeric(30, 10) null,
    stat_h24_ratio_price_change numeric(30, 10) null,
    difference numeric(30, 10) null,
    bid_price_lowest numeric(30, 10) null,
    ask_price_highest numeric(30, 10) null,
    change_d005 numeric(30, 10) null,
    change_d010 numeric(30, 10) null,
    change_d020 numeric(30, 10) null,
    change_d060 numeric(30, 10) null,
    change_d120 numeric(30, 10) null,
    change_d250 numeric(30, 10) null,
    change_year numeric(30, 10) null,
    secu_url text null,
    source_channel text null,
    dt_collect timestamp with time zone null,
    dt_create timestamp with time zone null,
    dt_update timestamp with time zone null,
    count_update integer null,
    json_unmapped text null,
    CONSTRAINT finv_quote_secu_meta_pkey PRIMARY KEY (usc)
) TABLESPACE pg_default;

COMMENT ON TABLE cdeu.finv_quote_secu_meta IS
    '证券指标表（对齐上游 Supabase public.finv_quote_secu_meta，131 列）：一行=一个证券的最新指标快照，主键 usc；'
    '只保最新、不建历史表；由「证券参数同步 → 证券指标数据同步」分批增量写入（需求 11047/11048）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.usc IS '证券统一编码（主键，同步对齐键）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.ftmm_symbol IS '富途 ftmm_symbol（如 US.AAPL；冗余，便于与 futu.finv_futu_secu_key 关联）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.ts IS '行情时间戳（epoch，上游原值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.dt_collect IS '上游采集时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.dt_create IS '上游创建时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.dt_update IS '最后更新时间（可空；由触发器 finv_quote_secu_meta_set_dt_update 在 UPDATE 时刷新）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.count_update IS '上游累计更新次数（原样同步）';
COMMENT ON COLUMN cdeu.finv_quote_secu_meta.json_unmapped IS '上游未映射字段的原始 JSON（排查字段口径变化用）';

-- ---------------------------------------------------------------------
-- 3. 索引（幂等）
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_finv_quote_secu_meta_ts
    ON cdeu.finv_quote_secu_meta USING btree (ts) TABLESPACE pg_default;

CREATE INDEX IF NOT EXISTS idx_finv_quote_secu_meta_dt_update
    ON cdeu.finv_quote_secu_meta USING btree (dt_update) TABLESPACE pg_default;

-- ---------------------------------------------------------------------
-- 4. 触发器（PG 不支持 CREATE TRIGGER IF NOT EXISTS：先查 pg_trigger 再建）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM pg_trigger tg
          JOIN pg_class t ON t.oid = tg.tgrelid
          JOIN pg_namespace n ON n.oid = t.relnamespace
         WHERE n.nspname = 'cdeu'
           AND t.relname = 'finv_quote_secu_meta'
           AND tg.tgname = 'finv_quote_secu_meta_set_dt_update'
           AND NOT tg.tgisinternal
    ) THEN
        CREATE TRIGGER finv_quote_secu_meta_set_dt_update
            BEFORE UPDATE ON cdeu.finv_quote_secu_meta
            FOR EACH ROW
            EXECUTE FUNCTION cdeu.finv_quote_secu_meta_set_dt_update();
        RAISE NOTICE 'V300144 触发器 finv_quote_secu_meta_set_dt_update 已创建';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 5. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols    int;
    v_numeric int;
    v_pk      int;
    v_pk_cols text;
    v_idx     int;
    v_trg     int;
    v_before  timestamptz;
    v_after   timestamptz;
BEGIN
    SELECT count(*) INTO v_cols FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_meta';
    IF v_cols <> 131 THEN
        RAISE EXCEPTION 'V300144 列数不符：期望 131，实际 %', v_cols;
    END IF;

    SELECT count(*) INTO v_numeric FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_meta'
       AND data_type = 'numeric' AND numeric_precision = 30 AND numeric_scale = 10;
    IF v_numeric <> 80 THEN
        RAISE EXCEPTION 'V300144 numeric(30,10) 列数不符：期望 80，实际 %', v_numeric;
    END IF;

    SELECT count(*), string_agg(a.attname, ',')
      INTO v_pk, v_pk_cols
      FROM pg_constraint c
      JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
     WHERE c.conrelid = 'cdeu.finv_quote_secu_meta'::regclass AND c.contype = 'p';
    IF v_pk <> 1 OR v_pk_cols <> 'usc' THEN
        RAISE EXCEPTION 'V300144 主键不符：约束数=% 列=%', v_pk, v_pk_cols;
    END IF;

    SELECT count(*) INTO v_idx FROM pg_indexes
     WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_meta'
       AND indexname IN ('idx_finv_quote_secu_meta_ts', 'idx_finv_quote_secu_meta_dt_update');
    IF v_idx <> 2 THEN
        RAISE EXCEPTION 'V300144 索引缺失：期望 2，实际 %', v_idx;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_meta'
       AND tg.tgname = 'finv_quote_secu_meta_set_dt_update' AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300144 触发器缺失（实际 %）', v_trg;
    END IF;

    -- 触发器行为自检：插临时行 → UPDATE → dt_update 必须被刷新，dt_create 保持不变
    INSERT INTO cdeu.finv_quote_secu_meta (usc, ftmm_symbol, name, dt_create, dt_update)
    VALUES ('__V300144_SELFCHECK__', 'US.SELFCHECK', '自检临时行', now() - interval '1 day', now() - interval '1 day');
    SELECT dt_create INTO v_before FROM cdeu.finv_quote_secu_meta WHERE usc = '__V300144_SELFCHECK__';
    UPDATE cdeu.finv_quote_secu_meta SET name = '自检临时行2' WHERE usc = '__V300144_SELFCHECK__';
    SELECT dt_update INTO v_after FROM cdeu.finv_quote_secu_meta WHERE usc = '__V300144_SELFCHECK__';
    DELETE FROM cdeu.finv_quote_secu_meta WHERE usc = '__V300144_SELFCHECK__';
    IF v_after IS NULL OR v_after <= now() - interval '1 hour' THEN
        RAISE EXCEPTION 'V300144 触发器未刷新 dt_update（值=%）', v_after;
    END IF;

    RAISE NOTICE 'V300144 就绪：cdeu.finv_quote_secu_meta 131 列 / 主键(usc) / 2 索引 / 触发器；自检通过（dt_create 保持 %）', v_before;
END $$;

COMMIT;
