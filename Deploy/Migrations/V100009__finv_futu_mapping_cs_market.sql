-- =====================================================================
-- FinvQuant PostgreSQL V9：富途 CS 市场映射表 finv_futu_mapping_cs_market（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - FT（富途/moomoo）行情源 CS 市场标识与交易所代码（finv_exchange.exchange_code）的字段映射表；
--   - futu_cs_market 为主键（富途行情源 CS 市场标识，如 HK / US / CN 等，具体取值以行情源为准）；
--   - finv_exchange_code 对齐 finv_exchange.exchange_code（INTEGER 1~999999），不建物理外键（项目惯例：程序层控制）；
--   - 初始数据待确认后按数据种子段 V100000+ 补充。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途 CS 市场映射表
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_cs_market (
    futu_cs_market     TEXT        PRIMARY KEY,            -- 富途行情源 CS 市场标识（如 HK / US / CN 等）
    finv_exchange_code INTEGER     NOT NULL
                        CHECK (finv_exchange_code BETWEEN 1 AND 999999),  -- 交易所代码（对齐 finv_exchange.exchange_code）
    gmt_create         TIMESTAMPTZ NOT NULL DEFAULT now(), -- 首次插入时间
    gmt_update         TIMESTAMPTZ NOT NULL DEFAULT now()  -- 最后更新时间（触发器维护）
);

-- 按交易所代码反查富途 CS 市场
CREATE INDEX idx_finv_futu_mapping_cs_market_finv
    ON finv_futu_mapping_cs_market (finv_exchange_code, futu_cs_market);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_futu_mapping_cs_market_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_cs_market
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
