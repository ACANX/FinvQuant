-- =====================================================================
-- FinvQuant PostgreSQL V8：富途市场代码映射表 finv_futu_mapping_market_code（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - FT（富途/moomoo）行情源市场代码与交易市场代码（finv_market.market_code）的字段映射表；
--   - futu_market_code 为主键（富途行情源市场代码，如 1 / 10 / 11 / 30 / 70 / 120 / 360，
--     值域由行情源决定，不设 CHECK 限制）；
--   - finv_market_code 对齐 finv_market.market_code（INTEGER 1~999999），不建物理外键（项目惯例：程序层控制）；
--   - 初始数据待确认后按数据种子段 V100000+ 补充。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途市场代码映射表
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_market_code (
    futu_market_code  INTEGER     PRIMARY KEY,             -- 富途行情源市场代码（如 1 / 10 / 11 / 30 / 70 / 120 / 360）
    finv_market_code  INTEGER     NOT NULL
                      CHECK (finv_market_code BETWEEN 1 AND 999999),  -- 交易市场代码（对齐 finv_market.market_code）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now()   -- 最后更新时间（触发器维护）
);

-- 按交易市场代码反查富途市场代码
CREATE INDEX idx_finv_futu_mapping_market_code_finv
    ON finv_futu_mapping_market_code (finv_market_code, futu_market_code);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_futu_mapping_market_code_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_market_code
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
