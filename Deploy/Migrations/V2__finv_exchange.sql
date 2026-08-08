-- =====================================================================
-- FinvQuant PostgreSQL V2：交易所/市场字典表 finv_exchange（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 单一事实来源：交易所/市场编码字典，覆盖证券/期货/期权/场外/外汇市场；
--   - exchange_code 为主键（市场数字代码，如 10=中国证券市场全体、11=上交所、31=纳斯达克）；
--   - ft_list_exchange_code 为预留映射列：后续按 FT 行情源列表编码映射补充，当前留空；
--   - 初始数据见 V100000__finv_exchange_seed.sql（数据种子段，确保在所有表结构脚本之后执行）。
-- 迁移策略：与既有迁移一致，整个 V2 在一个事务内执行；失败自动回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 交易所/市场字典表
-- ---------------------------------------------------------------------
CREATE TABLE finv_exchange (
    exchange_code          INTEGER     PRIMARY KEY
                           CHECK (exchange_code BETWEEN 1 AND 999999),
    exchange_flag          TEXT        NOT NULL,                 -- 交易所标志（如 CN / SH / NSDQ / FX）
    exchange_abbr          TEXT        NOT NULL,                 -- 交易所英文缩写（如 SSE / SZSE / HKEX）
    exchange_name          TEXT        NOT NULL,                 -- 交易所英文全称（如 Shanghai Stock Exchange）
    exchange_abbr_cn       TEXT        NOT NULL,                 -- 交易所中文缩写/名称（如 上交所 / 纳斯达克）
    en_market_type         TEXT        NOT NULL,                 -- 市场类型（证券/期货/黄金及贵金属/场外/期权/外汇）
    region                 TEXT        NOT NULL,                 -- 地区编码（如 CN / HK / USA / JP）
    base_currency          TEXT        NOT NULL,                 -- 基础计价货币（如 CNY / USD / HKD）
    ft_list_exchange_code  TEXT,                                 -- FT 行情源列表交易所编码（映射预留，暂空）
    gmt_create             TIMESTAMPTZ NOT NULL DEFAULT now(),   -- 首次插入时间
    gmt_update             TIMESTAMPTZ NOT NULL DEFAULT now(),   -- 最后更新时间（触发器维护）
    CONSTRAINT uq_finv_exchange_flag UNIQUE (exchange_flag)
);

-- 按地区查询
CREATE INDEX idx_finv_exchange_region
    ON finv_exchange (region, exchange_code);

-- 按市场类型查询
CREATE INDEX idx_finv_exchange_market_type
    ON finv_exchange (en_market_type, exchange_code);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_exchange_gmt_update
    BEFORE UPDATE ON finv_exchange
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
