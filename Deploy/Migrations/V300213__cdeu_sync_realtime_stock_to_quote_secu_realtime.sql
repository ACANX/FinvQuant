-- =====================================================================
-- V300213：存量数据迁移（阶段一）——cdeu.finv_cdeu_secu_realtime_stock
--           → cdeu.finv_quote_secu_realtime，源表只读、不清空
--
-- 背景（需求 11269）：
--   单 provider 原始快照表 finv_cdeu_secu_realtime_stock（主键 stock_id）与
--   跨 provider 归一快照表 finv_quote_secu_realtime（主键 usc）在阶段一并存；
--   本迁移把前者存量增量复制到后者，**不做 DELETE/TRUNCATE**，
--   业务代码切换留到下一阶段（切换前两张表由本函数保持同步）。
--
-- 字段映射：
--   · 34 列中 32 列同名直传；另两处转换：
--       stock_id  BIGINT → TEXT（目标表口径）
--       price_direct  源表 "+ / - / 0" → 目标表 "+ / - / ="（0 改 =）
--   · provider 固定写 'FTMM'（源表为富途/FTMM 采集链路，无该列）
--   · gmt_create / gmt_update 不进列清单，由目标表默认值与触发器维护
--
-- 增量与幂等：
--   · 复制逻辑封装为 cdeu.f_finv_quote_secu_realtime_sync(p_limit)
--     —— 按 stock_id 水位分批（默认 5000 行/批），可被后续业务代码定时重复调用；
--   · 冲突策略：ON CONFLICT (usc) DO UPDATE，且仅当
--       目标行 provider = 'FTMM' 且源 dt_update 更新时才覆盖
--     —— ① 重复执行不产生额外写入；② **不覆盖 BDFS 写入的行**（两 provider 不互相踩）；
--   · 源表 usc 为空的行跳过（无法定位主键），函数返回跳过数供日志/巡检使用。
--
-- 生产自动执行：本文件随发行包发布，服务启动时按 schema_version 顺序应用；
--   末尾调用一次函数完成存量迁移并校验「源表 usc 已全部在目标表可查」。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 增量同步函数（可重复调用；阶段二业务代码的入口）
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION cdeu.f_finv_quote_secu_realtime_sync(p_limit int DEFAULT 5000)
RETURNS TABLE (synced bigint, skipped_no_usc bigint, batches int)
LANGUAGE plpgsql
AS $$
DECLARE
    v_last    bigint := -9223372036854775808;
    v_rows    int;
    v_next    bigint;
    v_total   bigint := 0;
    v_batches int := 0;
    v_skip    bigint;
BEGIN
    IF p_limit IS NULL OR p_limit <= 0 THEN
        p_limit := 5000;
    END IF;

    -- 源表缺 usc 的行无法定位主键，统计后跳过
    SELECT count(*) INTO v_skip
      FROM cdeu.finv_cdeu_secu_realtime_stock
     WHERE usc IS NULL OR usc = '';

    LOOP
        WITH batch AS (
            SELECT s.*
              FROM cdeu.finv_cdeu_secu_realtime_stock s
             WHERE s.usc IS NOT NULL
               AND s.usc <> ''
               AND s.stock_id > v_last
             ORDER BY s.stock_id
             LIMIT p_limit
        ), done AS (
            INSERT INTO cdeu.finv_quote_secu_realtime (
                usc, stock_id, name, stock_code, market_type, market_label,
                instrument_type, currency, market_value, total_shares,
                circulation_total_shares, trade_trunover, trade_volumn,
                circulation_market_value, dt_update, price_nominal, change_ratio,
                trust_aum, trade_change_ratio, price_amplitude, volumn_ratio,
                buysell_ratio, change_price, price_direct, leverage, page,
                rmb_exchange_rate, rmb_market_value, rmb_circulation_market_value,
                rmb_trade_trunover, rmb_price_nominal, remark, provider
            )
            SELECT b.usc,
                   b.stock_id::text,
                   b.name, b.stock_code, b.market_type, b.market_label,
                   b.instrument_type, b.currency, b.market_value, b.total_shares,
                   b.circulation_total_shares, b.trade_trunover, b.trade_volumn,
                   b.circulation_market_value, b.dt_update, b.price_nominal, b.change_ratio,
                   b.trust_aum, b.trade_change_ratio, b.price_amplitude, b.volumn_ratio,
                   b.buysell_ratio, b.change_price,
                   CASE WHEN b.price_direct = '0' THEN '=' ELSE b.price_direct END,
                   b.leverage, b.page,
                   b.rmb_exchange_rate, b.rmb_market_value, b.rmb_circulation_market_value,
                   b.rmb_trade_trunover, b.rmb_price_nominal, b.remark,
                   'FTMM'
              FROM batch b
            ON CONFLICT (usc) DO UPDATE SET
                stock_id                 = EXCLUDED.stock_id,
                name                     = EXCLUDED.name,
                stock_code               = EXCLUDED.stock_code,
                market_type              = EXCLUDED.market_type,
                market_label             = EXCLUDED.market_label,
                instrument_type          = EXCLUDED.instrument_type,
                currency                 = EXCLUDED.currency,
                market_value             = EXCLUDED.market_value,
                total_shares             = EXCLUDED.total_shares,
                circulation_total_shares = EXCLUDED.circulation_total_shares,
                trade_trunover           = EXCLUDED.trade_trunover,
                trade_volumn             = EXCLUDED.trade_volumn,
                circulation_market_value = EXCLUDED.circulation_market_value,
                dt_update                = EXCLUDED.dt_update,
                price_nominal            = EXCLUDED.price_nominal,
                change_ratio             = EXCLUDED.change_ratio,
                trust_aum                = EXCLUDED.trust_aum,
                trade_change_ratio       = EXCLUDED.trade_change_ratio,
                price_amplitude          = EXCLUDED.price_amplitude,
                volumn_ratio             = EXCLUDED.volumn_ratio,
                buysell_ratio            = EXCLUDED.buysell_ratio,
                change_price             = EXCLUDED.change_price,
                price_direct             = EXCLUDED.price_direct,
                leverage                 = EXCLUDED.leverage,
                page                     = EXCLUDED.page,
                rmb_exchange_rate        = EXCLUDED.rmb_exchange_rate,
                rmb_market_value         = EXCLUDED.rmb_market_value,
                rmb_circulation_market_value = EXCLUDED.rmb_circulation_market_value,
                rmb_trade_trunover       = EXCLUDED.rmb_trade_trunover,
                rmb_price_nominal        = EXCLUDED.rmb_price_nominal,
                remark                   = EXCLUDED.remark
            -- 保护其它 provider 的行；同 provider 只在源快照更新时覆盖
            WHERE cdeu.finv_quote_secu_realtime.provider = 'FTMM'
              AND (cdeu.finv_quote_secu_realtime.dt_update IS NULL
                   OR cdeu.finv_quote_secu_realtime.dt_update < EXCLUDED.dt_update)
            RETURNING 1
        )
        SELECT (SELECT count(*) FROM done), (SELECT max(stock_id) FROM batch)
          INTO v_rows, v_next;

        EXIT WHEN v_rows = 0 AND v_next IS NULL;

        v_total   := v_total + v_rows;
        v_batches := v_batches + 1;
        v_last    := COALESCE(v_next, v_last);

        EXIT WHEN v_next IS NULL;   -- 已到源表末尾
    END LOOP;

    synced         := v_total;
    skipped_no_usc := v_skip;
    batches        := v_batches;
    RETURN NEXT;
END;
$$;

COMMENT ON FUNCTION cdeu.f_finv_quote_secu_realtime_sync(int)
    IS '把 cdeu.finv_cdeu_secu_realtime_stock 的存量/增量快照复制到 cdeu.finv_quote_secu_realtime（provider=FTMM，源表只读；按 stock_id 水位分批，不覆盖其它 provider 的行）';

-- ---------------------------------------------------------------------
-- 2. 执行一次存量迁移（幂等：重复执行只同步变更新行）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_synced bigint;
    v_skip   bigint;
    v_batch  int;
    v_miss   bigint;
BEGIN
    SELECT f.synced, f.skipped_no_usc, f.batches
      INTO v_synced, v_skip, v_batch
      FROM cdeu.f_finv_quote_secu_realtime_sync(5000) f;
    RAISE NOTICE 'V300213 同步完成：写入 % 行，% 批次，源表缺 usc 跳过 % 行', v_synced, v_batch, v_skip;

    -- 校验：源表所有有 usc 的行，在目标表都能查到（同 provider 覆盖或已被其它 provider 占用）
    SELECT count(*) INTO v_miss
      FROM cdeu.finv_cdeu_secu_realtime_stock s
     WHERE s.usc IS NOT NULL AND s.usc <> ''
       AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_realtime t WHERE t.usc = s.usc);
    IF v_miss > 0 THEN
        RAISE EXCEPTION 'V300213 校验失败：仍有 % 行源数据未出现在 cdeu.finv_quote_secu_realtime', v_miss;
    END IF;

    RAISE NOTICE 'V300213 校验通过：源表存量已全部可在目标表查到（源表未做任何删除）';
END $$;

COMMIT;
