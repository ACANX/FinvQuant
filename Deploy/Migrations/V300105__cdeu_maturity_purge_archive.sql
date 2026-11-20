-- =====================================================================
-- V300105：成熟度表迁移备份表（需求 10936）
--
-- 背景：
--   USC 补丁（需求 10931）把主表行情从旧 usc 迁到新 usc 后，成熟度表
--   cdeu.finv_quote_cdeu_quote_maturity_mon 仍留着旧 usc 的月度统计，
--   导致「历史月份存量数据清点」页面继续显示已迁移的旧 usc。
--
--   patch 执行时会把这些成熟度记录合并到新 usc 并删除旧记录，
--   按 DbSchemaSpec §0.7 先整行备份到本表。
-- =====================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102 (
    LIKE cdeu.finv_quote_cdeu_quote_maturity_mon INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'cdeu',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, trade_date, src_schema)
);

COMMENT ON TABLE archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102 IS '成熟度表 usc 迁移前的整行备份（需求 10936）';
COMMENT ON COLUMN archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102.src_schema IS '来源 schema';
COMMENT ON COLUMN archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102.reason IS '备份原因';
COMMENT ON COLUMN archive.finv_quote_cdeu_quote_maturity_mon_purge_20261102.backup_at IS '备份时间';

COMMIT;
