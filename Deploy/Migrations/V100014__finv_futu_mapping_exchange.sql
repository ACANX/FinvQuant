-- =====================================================================
-- FinvQuant PostgreSQL V14：富途交易所映射表 finv_futu_mapping_exchange（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 富途行情源 exchange 字典（ASX / SEHK / SSE 等）的字段映射表；
--   - futu_exchange 为主键（富途 exchange 代码，如 SEHK / SSE / COMEX）；
--   - region：对应地区（如 香港 / 中国 / 美国，— 表示无地区归属）；
--   - abbr：地区简写（与 finv_region 简写体系一致，如 HK / CN / USA）；
--   - exchange_name：交易所/市场名称（如 香港交易所）；
--   - finv_exchange：finv 侧交易所标识（暂与 futu_exchange 同值，
--     后续可对齐 finv_exchange.exchange_flag 调整）；
--   - 初始数据见 V100006__finv_futu_mapping_exchange_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途交易所映射表
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_exchange (
    futu_exchange   TEXT        PRIMARY KEY,              -- 富途 exchange 代码（如 SEHK / SSE / COMEX）
    region          TEXT        NOT NULL,                 -- 对应地区（如 香港 / 中国 / 美国；— 表示无地区归属）
    abbr            TEXT        NOT NULL,                 -- 地区简写（对齐 finv_region，如 HK / CN / USA）
    exchange_name   TEXT        NOT NULL,                 -- 交易所/市场名称（如 香港交易所）
    finv_exchange   TEXT        NOT NULL,                 -- finv 侧交易所标识（暂同 futu_exchange，待对齐 finv_exchange）
    gmt_create      TIMESTAMPTZ NOT NULL DEFAULT now(),   -- 首次插入时间
    gmt_update      TIMESTAMPTZ NOT NULL DEFAULT now()    -- 最后更新时间（触发器维护）
);

-- 按 finv 侧交易所标识反查富途 exchange
CREATE INDEX idx_finv_futu_mapping_exchange_finv
    ON finv_futu_mapping_exchange (finv_exchange, futu_exchange);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_futu_mapping_exchange_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_exchange
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
