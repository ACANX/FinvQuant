-- =====================================================================
-- FinvQuant 一次性维护脚本：清理 finv_quote_cdeu_usc 中「没有行情」的行
-- （需求 10920 收尾，2026-11-02）
--
-- 背景：
--   需求 10920 之前，`RefreshSecurityMetadata` 用
--   `INSERT ... SELECT FROM public.finv_security` 把证券字典整表拷进
--   `cdeu.finv_quote_cdeu_usc`，于是表里混入大量**从未采集过行情**的证券。
--   代码侧已改为：行序只由主表 `cdeu.finv_quote_secu_kline_min` 的
--   `DISTINCT usc` 决定（`DiscoverUsc`），元数据只做 `UPDATE`。
--   本脚本清理存量脏行，之后由程序按主表自动补齐。
--
-- 删除判据（唯一）：
--   该 usc 在 `cdeu.finv_quote_secu_kline_min` 中**不存在任何一行**。
--   只要主表里出现过一次，就保留（停牌/退市/连续数月无数据都不删）。
--
-- 规范合规（Docs/DevSpec/DbSchemaSpec.md §0.7）：
--   · 删除前把受影响行整行备份到 archive.finv_quote_cdeu_usc_purge_20261102，
--     备份表带 src_schema / reason / backup_at；
--   · 单事务内「备份 → 删除 → 校验」；
--   · 校验用 RAISE EXCEPTION（不是 WARNING），不满足则整体回滚。
--
-- 用法（在目标库上执行）：
--   psql "postgres://finv@127.0.0.1:5432/finv" -f Deploy/Upgrade/2611-001-PurgeUscWithoutQuote.sql
--
-- 幂等：重复执行时受影响行数为 0，脚本正常结束（备份表追加 0 行）。
-- 回滚：把备份表整行插回 cdeu.finv_quote_cdeu_usc 即可（见脚本末尾注释）。
-- =====================================================================

\set ON_ERROR_STOP on

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 备份表（首次执行创建；结构与 cdeu.finv_quote_cdeu_usc 同列 + 3 个来源列）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_quote_cdeu_usc_purge_20261102 (
    LIKE cdeu.finv_quote_cdeu_usc INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'cdeu',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, src_schema)
);

-- ---------------------------------------------------------------------
-- 2. 备份受影响行 + 3. 删除（同一判据，单事务）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    c_reason   CONSTANT text :=
        '需求 10920：ucs 表行序只应由主表 DISTINCT usc 决定；'
        '该 usc 在主表 cdeu.finv_quote_secu_kline_min 中无任何行情行，'
        '属 10920 之前从 public.finv_security 整表拷贝产生的脏行';
    v_before   bigint;
    v_backup   bigint;
    v_deleted  bigint;
    v_after    bigint;
BEGIN
    SELECT count(*) INTO v_before FROM cdeu.finv_quote_cdeu_usc;

    -- 2. 备份
    INSERT INTO archive.finv_quote_cdeu_usc_purge_20261102
        (usc, category, day_expect_rows, date_list, date_delist,
         flag_auto_discover, region, market, provider, name, timezone, currency,
         gmt_create, gmt_update,
         priority_rank, sla_days, max_wait_days, priority_gmt_update,
         src_schema, reason)
    SELECT u.usc, u.category, u.day_expect_rows, u.date_list, u.date_delist,
           u.flag_auto_discover, u.region, u.market, u.provider, u.name, u.timezone, u.currency,
           u.gmt_create, u.gmt_update,
           u.priority_rank, u.sla_days, u.max_wait_days, u.priority_gmt_update,
           'cdeu', c_reason
      FROM cdeu.finv_quote_cdeu_usc u
     WHERE NOT EXISTS (
           SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc = u.usc)
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_backup = ROW_COUNT;

    -- 3. 删除（与备份完全相同的判据）
    DELETE FROM cdeu.finv_quote_cdeu_usc u
     WHERE NOT EXISTS (
           SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc = u.usc);
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    SELECT count(*) INTO v_after FROM cdeu.finv_quote_cdeu_usc;

    -- 4. 收敛校验（失败即整事务回滚）
    IF v_deleted <> v_backup THEN
        RAISE EXCEPTION '备份行数(%) 与删除行数(%) 不一致，回滚', v_backup, v_deleted;
    END IF;
    IF v_before - v_deleted <> v_after THEN
        RAISE EXCEPTION '行数校验失败：删除前 % - 删除 % <> 删除后 %', v_before, v_deleted, v_after;
    END IF;
    IF EXISTS (
        SELECT 1 FROM cdeu.finv_quote_cdeu_usc u
         WHERE NOT EXISTS (
               SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc = u.usc)) THEN
        RAISE EXCEPTION '删除后仍存在无行情的 usc 行，回滚';
    END IF;

    RAISE NOTICE '完成：删除前 % 行，备份 % 行，删除 % 行，删除后 % 行',
        v_before, v_backup, v_deleted, v_after;
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 5. 执行后核对（可手工跑）
-- ---------------------------------------------------------------------
-- 5.1 两表行数应一致（ucs 表允许比主表多出人工预建的证券）
--   SELECT (SELECT count(*) FROM cdeu.finv_quote_cdeu_usc) AS usc_rows,
--          (SELECT count(DISTINCT usc) FROM cdeu.finv_quote_secu_kline_min) AS main_usc;
--
-- 5.2 不应再有「无行情」的 usc（人工通过 PUT /Usc 预建的除外）
--   SELECT count(*) FROM cdeu.finv_quote_cdeu_usc u
--    WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_kline_min m WHERE m.usc = u.usc);
--
-- 5.3 回滚（把备份行整行插回）：
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
--   ON CONFLICT (usc) DO NOTHING;
