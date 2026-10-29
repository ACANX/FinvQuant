-- =====================================================================
-- FinvQuant PostgreSQL V5：证券代码表 finv_security（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 统一证券代码表：将各交易所/市场的源证券代码映射到统一证券代码（usc）；
--   - usc 为主键（统一证券代码，全局唯一）；
--   - exchange_code 对齐 finv_exchange 字典（INTEGER），currency_type 对齐
--     finv_currency 字典（TEXT），便于关联换算；均不建物理外键（项目惯例：程序层控制）；
--   - MySQL 字段 `currency` / `timezone` 与 JSON 键 `currency_type` / `time_zone`
--     不一致，PG 保留 MySQL 字段命名（currency_type / timezone）；
--   - init_date 默认 20000000（yyyyMMdd，未上市/未知时占位）；
--   - 初始数据见 V100002__finv_security_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 证券代码表
-- ---------------------------------------------------------------------
CREATE TABLE finv_security (
    usc               TEXT        PRIMARY KEY,             -- 统一证券代码（全局唯一，如 GCMain / HSI / 000985）
    exchange_code     INTEGER     NOT NULL
                      CHECK (exchange_code BETWEEN 1 AND 999999),  -- 交易所代码（对齐 finv_exchange.exchange_code）
    security_type     TEXT        NOT NULL,                -- 证券类型（如 Futures / StockIndex / Stock / ETF）
    security_code     TEXT        NOT NULL,                -- 源证券代码（交易所原始代码）
    security_name     TEXT        NOT NULL,                -- 源证券名称
    security_name_cn  TEXT        NOT NULL,                -- 证券名称（中文）
    security_name_full TEXT,                               -- 证券名称（全称，可为空）
    currency_type     TEXT        NOT NULL,                -- 交易计价基础货币（对齐 finv_currency.currency_type）
    init_date         INTEGER     NOT NULL DEFAULT 20000000
                      CHECK (init_date BETWEEN 19500101 AND 21001231),  -- 首次上市交易日期 yyyymmdd
    timezone         TEXT,                                -- 时区（如 -04:00 / +08:00）
    tz                TEXT,                                -- 时区标识（如 America/New_York / Asia/Shanghai）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 最后更新时间（触发器维护）
    CONSTRAINT uq_finv_security_exchange_code UNIQUE (exchange_code, security_code)  -- 同一交易所内源代码唯一
);

-- 按交易所查询
CREATE INDEX idx_finv_security_exchange
    ON finv_security (exchange_code, usc);

-- 按证券类型查询
CREATE INDEX idx_finv_security_type
    ON finv_security (security_type, usc);

-- 按货币查询
CREATE INDEX idx_finv_security_currency
    ON finv_security (currency_type, usc);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_security_gmt_update
    BEFORE UPDATE ON finv_security
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
