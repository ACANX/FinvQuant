-- =====================================================================
-- FinvQuant 一次性维护脚本：删除导入测试残留证券 IT001 / IT002 / IT003
-- （需求 10923，2026-11-02）
--
-- 背景：
--   这三只 usc 是行情导入联调时插进去的占位数据：
--     · 各只有 1 行，ts 完全相同（1792373400），date / time 全为空；
--     · provider 分别为 'IT' 与空串；
--     · 证券字典（public.finv_security）、富途镜像（futu.finv_futu_quote_collect /
--       finv_futu_mapping_security）、成熟度表里都没有它们的记录。
--   由于没有任何可依据的来源，无法推导 category/region/market（而 region/market
--   会拼进归档目录 {region}_{market}，编造比空着更危险），故整条删除。
--
-- 规范合规（Docs/DevSpec/DbSchemaSpec.md §0.7）：
--   · 删除前把受影响行整行备份到 archive 下的 purge 表，带 src_schema / reason / backup_at；
--   · 单事务内「备份 → 删除 → 校验」；
--   · 校验用 RAISE EXCEPTION，不满足则整体回滚。
--
-- 用法（在目标库上执行）：
--   psql "postgres://finv@127.0.0.1:5432/finv" -f Deploy/Upgrade/2611-002-PurgeTestUsc.sql
--
-- 幂等：删除后受影响行数为 0，重复执行正常结束。
-- 回滚：把 purge 表里的行整行插回各自原表即可（见脚本末尾注释）。
-- =====================================================================

\set ON_ERROR_STOP on

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 备份表（两张：主表行情行 + usc 主数据行）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_quote_secu_kline_min_purge_20261102 (
    LIKE cdeu.finv_quote_secu_kline_min INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'cdeu',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ts, usc, src_schema)
);

-- usc 主数据的 purge 表由 2611-001 创建；这里按需补建（结构一致）。
CREATE TABLE IF NOT EXISTS archive.finv_quote_cdeu_usc_purge_20261102 (
    LIKE cdeu.finv_quote_cdeu_usc INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'cdeu',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, src_schema)
);

-- ---------------------------------------------------------------------
-- 2. 备份 → 3. 删除（同一判据：usc 属于测试残留清单）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    c_reason  CONSTANT text :=
        '需求 10923：导入联调残留（单行、date/time 为空、ts 相同 1792373400、'
        '证券字典与富途镜像均无记录），无来源可推导元数据，整条删除';
    v_quote_before  bigint;
    v_quote_backup  bigint;
    v_quote_deleted bigint;
    v_usc_before    bigint;
    v_usc_backup    bigint;
    v_usc_deleted   bigint;
BEGIN
    SELECT count(*) INTO v_quote_before
      FROM cdeu.finv_quote_secu_kline_min
     WHERE usc IN ('IT001','IT002','IT003');

    INSERT INTO archive.finv_quote_secu_kline_min_purge_20261102
        (usc, ts, date, time, open, high, low, close, vwap, volume, turnover,
         change_price, change_ratio, ext_field, last_close, remark, provider,
         gmt_create, gmt_update, src_schema, reason)
    SELECT m.usc, m.ts, m.date, m.time, m.open, m.high, m.low, m.close, m.vwap,
           m.volume, m.turnover, m.change_price, m.change_ratio, m.ext_field,
           m.last_close, m.remark, m.provider, m.gmt_create, m.gmt_update,
           'cdeu', c_reason
      FROM cdeu.finv_quote_secu_kline_min m
     WHERE m.usc IN ('IT001','IT002','IT003')
    ON CONFLICT (ts, usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_quote_backup = ROW_COUNT;

    DELETE FROM cdeu.finv_quote_secu_kline_min
     WHERE usc IN ('IT001','IT002','IT003');
    GET DIAGNOSTICS v_quote_deleted = ROW_COUNT;

    SELECT count(*) INTO v_usc_before
      FROM cdeu.finv_quote_cdeu_usc
     WHERE usc IN ('IT001','IT002','IT003');

    INSERT INTO archive.finv_quote_cdeu_usc_purge_20261102
        (usc, category, day_expect_rows, date_list, date_delist,
         flag_auto_discover, region, market, provider, name, timezone, currency,
         gmt_create, gmt_update,
         priority_rank, sla_days, max_wait_days, priority_gmt_update,
         src_schema, reason)
    SELECT u.usc, u.category, u.day_expect_rows, u.date_list, u.date_delist,
           u.flag_auto_discover, u.region, u.market, u.provider, u.name,
           u.timezone, u.currency, u.gmt_create, u.gmt_update,
           u.priority_rank, u.sla_days, u.max_wait_days, u.priority_gmt_update,
           'cdeu', c_reason
      FROM cdeu.finv_quote_cdeu_usc u
     WHERE u.usc IN ('IT001','IT002','IT003')
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_usc_backup = ROW_COUNT;

    DELETE FROM cdeu.finv_quote_cdeu_usc
     WHERE usc IN ('IT001','IT002','IT003');
    GET DIAGNOSTICS v_usc_deleted = ROW_COUNT;

    -- 4. 收敛校验（失败即整事务回滚）
    IF v_quote_deleted <> v_quote_backup THEN
        RAISE EXCEPTION '行情表备份(%) 与删除(%) 行数不一致，回滚', v_quote_backup, v_quote_deleted;
    END IF;
    IF v_usc_deleted <> v_usc_backup THEN
        RAISE EXCEPTION 'usc 表备份(%) 与删除(%) 行数不一致，回滚', v_usc_backup, v_usc_deleted;
    END IF;
    IF EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_min WHERE usc IN ('IT001','IT002','IT003'))
       OR EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('IT001','IT002','IT003')) THEN
        RAISE EXCEPTION '删除后仍能查到 IT001~IT003，回滚';
    END IF;

    RAISE NOTICE '完成：行情表 删除前 % 行 / 备份 % / 删除 %；usc 表 删除前 % 行 / 备份 % / 删除 %',
        v_quote_before, v_quote_backup, v_quote_deleted,
        v_usc_before, v_usc_backup, v_usc_deleted;
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 5. 执行后核对（可手工跑）
-- ---------------------------------------------------------------------
-- 5.1 两张表都不应再出现这三只 usc
--   SELECT count(*) FROM cdeu.finv_quote_secu_kline_min WHERE usc IN ('IT001','IT002','IT003');
--   SELECT count(*) FROM cdeu.finv_quote_cdeu_usc         WHERE usc IN ('IT001','IT002','IT003');
--
-- 5.2 usc 表应回到「与主表 DISTINCT usc 一致」
--   SELECT (SELECT count(*) FROM cdeu.finv_quote_cdeu_usc) AS usc_rows,
--          (SELECT count(DISTINCT usc) FROM cdeu.finv_quote_secu_kline_min WHERE usc <> '') AS main_usc;
--
-- 5.3 回滚（把备份行整行插回原表）：
--   INSERT INTO cdeu.finv_quote_secu_kline_min
--       (usc, ts, date, time, open, high, low, close, vwap, volume, turnover,
--        change_price, change_ratio, ext_field, last_close, remark, provider,
--        gmt_create, gmt_update)
--   SELECT usc, ts, date, time, open, high, low, close, vwap, volume, turnover,
--          change_price, change_ratio, ext_field, last_close, remark, provider,
--          gmt_create, gmt_update
--     FROM archive.finv_quote_secu_kline_min_purge_20261102
--   ON CONFLICT (ts, usc) DO NOTHING;
--
--   INSERT INTO cdeu.finv_quote_cdeu_usc
--       (usc, category, day_expect_rows, date_list, date_delist,
--        flag_auto_discover, region, market, provider, name, timezone, currency,
--        gmt_create, gmt_update,
--        priority_rank, sla_days, max_wait_days, priority_gmt_update)
--   SELECT usc, category, day_expect_rows, date_list, date_delist,
--          flag_auto_discover, region, market, provider, name, timezone, currency,
--          gmt_create, gmt_update,
--          priority_rank, sla_days, max_wait_days, priority_gmt_update
--     FROM archive.finv_quote_cdeu_usc_purge_20261102
--    WHERE reason LIKE '需求 10923%'
--   ON CONFLICT (usc) DO NOTHING;
