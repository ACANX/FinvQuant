-- =====================================================================
-- V300102：本地独有的 6 只证券发放给 Prod（需求 10930，清单 M11）
--
-- 背景：
--   LocalDocker 的主表有 AMAT / MRVL / NOW / PLTR / SMCI / XRP 的行情，
--   由 DiscoverUsc 自动登记进 cdeu.finv_quote_cdeu_usc（flag_auto_discover='t'）；
--   Prod 主表还没有这些证券的行情，所以登记表里没有它们。
--   业务方确认固化为 SQL 向 Prod 发放，避免换机后丢失登记结果。
--
-- 数据来源：LocalDocker 现网值（元数据由 V300096 的 SQL 发放与 DiscoverUsc 自动补全）。
--
-- 安全：ON CONFLICT DO NOTHING，已有记录一律不覆盖；重复执行影响 0 行。
-- =====================================================================

BEGIN;

INSERT INTO cdeu.finv_quote_cdeu_usc
    (usc, category, day_expect_rows, date_list, flag_auto_discover,
     region, market, provider, name, timezone, currency, priority_rank, sla_days, max_wait_days)
VALUES
    ('AMAT', 'Stock',  390,  DATE '1972-10-12', true, 'US', 'NSDQ', 'FTMM',
     'Applied Materials, Inc.', 'America/New_York', 'USD', 100, 5, 7),
    ('MRVL', 'Stock',  390,  DATE '2000-06-27', true, 'US', 'NSDQ', 'FTMM',
     'Marvell Technology, Inc.', 'America/New_York', 'USD', 100, 5, 7),
    ('NOW',  'Stock',  390,  DATE '2012-06-29', true, 'US', 'NYSE', 'FTMM',
     'ServiceNow, Inc.', 'America/New_York', 'USD', 100, 5, 7),
    ('PLTR', 'Stock',  390,  DATE '2020-09-30', true, 'US', 'NSDQ', 'FTMM',
     'Palantir Technologies Inc.', 'America/New_York', 'USD', 100, 5, 7),
    ('SMCI', 'Stock',  390,  DATE '2007-03-29', true, 'US', 'NSDQ', 'FTMM',
     'Super Micro Computer, Inc.', 'America/New_York', 'USD', 100, 5, 7),
    ('XRP',  'Crypto', 1440, DATE '2026-10-23', true, 'CC', 'FTC',  'FTMM',
     'XRP', 'UTC', 'USD', 100, 5, 7)
ON CONFLICT (usc) DO NOTHING;

DO $$
DECLARE
    v_total bigint;
BEGIN
    SELECT count(*) INTO v_total
      FROM cdeu.finv_quote_cdeu_usc
     WHERE usc IN ('AMAT','MRVL','NOW','PLTR','SMCI','XRP');
    RAISE NOTICE '完成：6 只证券在登记表中现存 % 行（应为 6）', v_total;
    IF v_total <> 6 THEN
        RAISE EXCEPTION '登记表应有 6 行，实际 %，回滚', v_total;
    END IF;
END $$;

COMMIT;

-- 执行后核对
-- SELECT usc, category, region, market, name, timezone, currency,
--        day_expect_rows, date_list, flag_auto_discover
--   FROM cdeu.finv_quote_cdeu_usc
--  WHERE usc IN ('AMAT','MRVL','NOW','PLTR','SMCI','XRP') ORDER BY usc;
