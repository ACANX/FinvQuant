-- =====================================================================
-- VeritasQuant PostgreSQL V1：历史分钟 K 线行情（FinvQuant 首版迁移）（字段级覆盖式更新）
--
-- 决策（ACANX 2026-08-04）：
--   - 历史行情存储到 PostgreSQL，主键 (ts, market_code, secu_code)；
--   - 允许修正：同键后到数据按字段覆盖旧值（INSERT ... ON CONFLICT
--     DO UPDATE），不设不可变触发器；
--   - 程序层控制：导入批次登记表 + 修正审计日志表，覆盖修正留痕可追溯。
-- 对齐参考：finv_quote_secu_kline_min (MySQL)
-- 迁移策略：与既有迁移一致，整个 V4 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 历史分钟 K 线行情主表
-- ---------------------------------------------------------------------
CREATE TABLE finv_quote_secu_kline_min (
    market_code  INTEGER        NOT NULL CHECK (market_code BETWEEN 0 AND 99999999),
    -- ↑ 证券市场数字代码（Finv 编码：1=上交所、11=美股；具体以市场字典为准）
    secu_code    TEXT           NOT NULL,                    -- 证券代码（如 518880 / NVDA）
    ts           BIGINT         NOT NULL CHECK (ts >= 0),    -- UTC 时间戳（秒）
    date         INTEGER        CHECK (date IS NULL OR (date BETWEEN 19500101 AND 21001231)), -- 交易日期 yyyymmdd
    "time"       INTEGER        CHECK ("time" IS NULL OR ("time" BETWEEN 0 AND 235959)),      -- 交易时间 hhmmss
    prev_close   NUMERIC(20,6),                              -- 前收盘价
    open         NUMERIC(20,6),                              -- 起始价
    high         NUMERIC(20,6),                              -- 区间最高价
    low          NUMERIC(20,6),                              -- 区间最低价
    close        NUMERIC(20,6),                              -- 最新价/收盘价
    paocd        NUMERIC(20,6),                              -- 当日累计均价
    volume       BIGINT         CHECK (volume IS NULL OR volume >= 0),    -- 成交量
    turnover     NUMERIC(30,8)  CHECK (turnover IS NULL OR turnover >= 0),-- 成交金额
    ext_field    TEXT,                                       -- 预留扩展字段
    remark       TEXT,                                       -- 备注
    gmt_create   TIMESTAMPTZ    NOT NULL DEFAULT now(),      -- 首次插入时间
    gmt_update   TIMESTAMPTZ    NOT NULL DEFAULT now(),      -- 最后更新时间（触发器维护）
    PRIMARY KEY (ts, market_code, secu_code)
);

-- 回放/按证券查询：secu_code + 时间范围（覆盖 MySQL 的 idx_secucode 且更有用）
CREATE INDEX idx_finv_quote_secu_secu_ts
    ON finv_quote_secu_kline_min (secu_code, ts);
-- 市场维度查询
CREATE INDEX idx_finv_quote_secu_market_ts
    ON finv_quote_secu_kline_min (market_code, ts);

-- gmt_update 自动维护（等价 MySQL 的 ON UPDATE CURRENT_TIMESTAMP(6)）
CREATE OR REPLACE FUNCTION vq_set_gmt_update()
RETURNS TRIGGER AS $$
BEGIN
    NEW.gmt_update := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update
    BEFORE UPDATE ON finv_quote_secu_kline_min
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 2. 导入批次登记表（程序层审计：谁、何时、导入/修正了什么）
-- ---------------------------------------------------------------------
CREATE TABLE finv_quote_ingest_batches (
    ingest_batch_id     TEXT        PRIMARY KEY,
    source              TEXT        NOT NULL,                 -- 数据源（如 FT）
    market_code         INTEGER     NOT NULL CHECK (market_code BETWEEN 0 AND 99999999),
    secu_code           TEXT        NOT NULL,
    data_version_id     TEXT        NOT NULL,                 -- 数据版本摘要（运行清单引用）
    file_count          INTEGER     NOT NULL DEFAULT 0 CHECK (file_count >= 0),
    record_count        BIGINT      NOT NULL DEFAULT 0 CHECK (record_count >= 0),
    upsert_mode         TEXT        NOT NULL CHECK (upsert_mode IN ('FIELD', 'ROW')),
    ts_precision        TEXT        NOT NULL,
    config_hash         TEXT        NOT NULL,
    imported_by         TEXT        NOT NULL,
    imported_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes               TEXT
);
CREATE INDEX idx_finv_quote_ingest_batches_secu
    ON finv_quote_ingest_batches (market_code, secu_code, imported_at);

-- ---------------------------------------------------------------------
-- 3. 修正审计日志（每次覆盖修正留痕：改了哪些行、为什么、前后摘要）
-- ---------------------------------------------------------------------
CREATE TABLE finv_quote_revision_log (
    revision_id      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ingest_batch_id  TEXT        NOT NULL,
    market_code      INTEGER     NOT NULL CHECK (market_code BETWEEN 0 AND 99999999),
    secu_code        TEXT        NOT NULL,
    affected_rows    BIGINT      NOT NULL CHECK (affected_rows >= 0),
    reason           TEXT        NOT NULL,
    revised_by       TEXT        NOT NULL,
    revised_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    previous_summary JSONB,      -- 修正前摘要（时间范围/行数等）
    new_summary      JSONB       -- 修正后摘要
);
CREATE INDEX idx_finv_quote_revision_log_secu
    ON finv_quote_revision_log (market_code, secu_code, revised_at);
CREATE INDEX idx_finv_quote_revision_log_batch
    ON finv_quote_revision_log (ingest_batch_id);

COMMIT;
