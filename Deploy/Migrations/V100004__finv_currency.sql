-- =====================================================================
-- FinvQuant PostgreSQL V4：货币字典表 finv_currency（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 货币类型/名称/最新兑换人民币汇率字典，供市场与行情计价换算使用；
--   - currency_type 为主键（货币类型，如 CNY / USD / HKD）；
--   - 对齐参考：finv_currency (MySQL)；MySQL 字段 `exchange rate_cny`（含空格）
--     按 PG snake_case 规范命名为 `exchange_rate_cny`；
--   - 汇率使用 NUMERIC(20,8) 对齐 MySQL double(20,8) 精度；
--   - 初始数据见 V100001__finv_currency_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 货币字典表
-- ---------------------------------------------------------------------
CREATE TABLE finv_currency (
    currency_type     TEXT        PRIMARY KEY,             -- 货币类型（如 CNY / USD / HKD）
    currency_name     TEXT        NOT NULL,                -- 货币名称（如 人民币 / 美元 / 港币）
    exchange_rate_cny NUMERIC(20,8) NOT NULL
                      CHECK (exchange_rate_cny >= 0),      -- 最新兑换人民币汇率（1 单位本币 = N 人民币）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now()   -- 最后更新时间（触发器维护）
);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_currency_gmt_update
    BEFORE UPDATE ON finv_currency
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
