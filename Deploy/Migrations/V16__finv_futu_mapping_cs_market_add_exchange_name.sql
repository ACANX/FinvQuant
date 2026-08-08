-- =====================================================================
-- FinvQuant PostgreSQL V16：finv_futu_mapping_cs_market 增加 exchange_name 列并放宽 finv_exchange_code 约束
--
-- 决策（ACANX 2026-08-05）：
--   - V9 已发布（PR #295 合并），按迁移规范不修改已发布迁移，新增 V16 增量迁移；
--   - exchange_name：交易所/市场名称（如 香港证券市场 / 美国证券市场），TEXT 可空，
--     存量行由 V100008 种子填充；
--   - finv_exchange_code 约束由 1~999999 放宽为 -1~999999：富途 cs_market 无对应
--     finv_exchange 交易所时，以 -1 作为缺省填充（ACANX 决策）；
--   - ADD COLUMN / DROP+ADD 约束成对执行，幂等（重复执行不报错）；
--   - 初始数据见 V100008__finv_futu_mapping_cs_market_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

ALTER TABLE finv_futu_mapping_cs_market
    ADD COLUMN IF NOT EXISTS exchange_name TEXT;

ALTER TABLE finv_futu_mapping_cs_market
    DROP CONSTRAINT IF EXISTS finv_futu_mapping_cs_market_finv_exchange_code_check;

ALTER TABLE finv_futu_mapping_cs_market
    ADD CONSTRAINT finv_futu_mapping_cs_market_finv_exchange_code_check
    CHECK (finv_exchange_code BETWEEN -1 AND 999999);

COMMIT;
