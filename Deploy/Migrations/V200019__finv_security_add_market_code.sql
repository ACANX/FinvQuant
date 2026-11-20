-- =====================================================================
-- FinvQuant PostgreSQL V19：finv_security 增加 market_code 市场代码字段
--
-- 决策（ACANX 2026-08-05）：
--   - finv_security 在 exchange_code（交易所代码）之后需要 market_code
--     （交易市场代码，对齐 finv_market.market_code）字段；
--   - 类型 INTEGER，缺省值 0（0=未指定/待维护；有值后与 finv_market 字典对应）；
--   - 用途：历史行情导入双策略中「选中证券自动带出市场代码」、
--     「按文件头代码自动匹配补全」可直接取该字段，无需再依赖
--     exchange_code 推导；也用于 finv_quote_secu_kline_min.market_code 关联；
--   - ADD COLUMN IF NOT EXISTS 幂等，重复执行不报错；
--   - 与 V17/V18 风格一致：单事务、失败回滚；
--   - 注意：ALTER TABLE ADD COLUMN 物理上追加到表尾，结构体/文档/查询
--     中按语义放在 exchange_code 之后展示。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 证券代码表：市场代码（INTEGER，缺省 0）
-- ---------------------------------------------------------------------
ALTER TABLE finv_security
    ADD COLUMN IF NOT EXISTS market_code INTEGER NOT NULL DEFAULT 0;

COMMIT;
