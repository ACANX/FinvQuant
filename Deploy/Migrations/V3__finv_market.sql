-- =====================================================================
-- FinvQuant PostgreSQL V3：交易市场表 finv_market（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 交易所下属交易市场代码表（如 上交所股票/基金/债券等细分市场），与 finv_exchange 字典互补；
--   - market_code 为主键（市场数字代码）；market_flag 唯一（市场标识）；
--   - en_security_type 记录该市场允许的证券类型；
--   - 本表暂无可确认的初始数据，待后续补充（届时按数据种子段 V100000+ 新增种子脚本）；
--   - 对齐参考：finv_market (MySQL)；迁移策略与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 交易市场表
-- ---------------------------------------------------------------------
CREATE TABLE finv_market (
    market_code       INTEGER     PRIMARY KEY
                      CHECK (market_code BETWEEN 1 AND 999999),
    market_flag       TEXT        NOT NULL,                 -- 市场标识（如 SH_A / SZ_STAR 等，编码待定）
    market_abbr       TEXT        NOT NULL,                 -- 交易所简码
    market_name       TEXT        NOT NULL,                 -- 交易所名称
    en_security_type  TEXT        NOT NULL,                 -- 允许证券类型（如 STOCK / FUND / BOND 等）
    base_currency     TEXT        NOT NULL,                 -- 基础计价货币（如 CNY / USD / HKD）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),   -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now(),   -- 最后更新时间（触发器维护）
    CONSTRAINT uq_finv_market_flag UNIQUE (market_flag)
);

-- 按证券类型查询
CREATE INDEX idx_finv_market_security_type
    ON finv_market (en_security_type, market_code);

-- 按计价货币查询
CREATE INDEX idx_finv_market_base_currency
    ON finv_market (base_currency, market_code);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_market_gmt_update
    BEFORE UPDATE ON finv_market
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
