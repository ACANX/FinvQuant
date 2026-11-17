-- =====================================================================
-- FinvQuant PostgreSQL V20：finv_market 增加 exchange_code 交易所代码字段
--
-- 决策（ACANX 2026-08-05）：
--   - finv_market 在 en_security_type（允许证券类型）之后需要 exchange_code
--     （所属交易所代码，对齐 finv_exchange.exchange_code）字段；
--   - 类型 INTEGER，缺省值 0（0=未指定/待维护；有值后与 finv_exchange 字典对应）；
--   - 用途：市场归属交易所的显式表达，供前端「市场代码」下拉选项
--     （基于 finv_market 查询）与证券字典 market_code 维护使用；
--   - ADD COLUMN IF NOT EXISTS 幂等，重复执行不报错；
--   - 与 V17/V18/V19 风格一致：单事务、失败回滚；
--   - 注意：ALTER TABLE ADD COLUMN 物理上追加到表尾，结构体/文档/查询
--     中按语义放在 en_security_type 之后展示。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 交易市场表：所属交易所代码（INTEGER，缺省 0）
-- ---------------------------------------------------------------------
ALTER TABLE finv_market
    ADD COLUMN IF NOT EXISTS exchange_code INTEGER NOT NULL DEFAULT 0;

COMMIT;
