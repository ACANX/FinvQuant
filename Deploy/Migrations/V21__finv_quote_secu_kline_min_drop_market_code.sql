-- =====================================================================
-- FinvQuant PostgreSQL V21：finv_quote_secu_kline_min 移除 market_code 字段
--
-- 决策（ACANX 2026-08-06）：
--   - 主表不再冗余存储 market_code：市场信息统一由 finv_security 字典
--     （finv_security.market_code，V19 新增）关联获取；
--   - 主键由 (ts, market_code, secu_code) 变更为 (ts, secu_code)：
--     secu_code 实际为统一证券代码（usc，全局唯一），同一 ts 下不会
--     存在多个市场同代码，主键收窄语义成立；
--   - 索引 idx_finv_quote_secu_market_ts (market_code, ts) 随列删除；
--     保留 idx_finv_quote_secu_secu_ts (secu_code, ts)（不含该列）；
--   - 审计表（finv_quote_ingest_batches / finv_quote_revision_log）的
--     market_code 字段保留：它们是导入留痕/审计表，保留来源市场信息
--     不影响主表关联语义；
--   - 迁移顺序：先删依赖 market_code 的索引，再删主键约束，
--     再删列，最后按新主键重建；单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 删除依赖 market_code 的索引（幂等）
-- ---------------------------------------------------------------------
DROP INDEX IF EXISTS idx_finv_quote_secu_market_ts;

-- ---------------------------------------------------------------------
-- 2. 删除原主键（隐含依赖 market_code 列）
-- ---------------------------------------------------------------------
ALTER TABLE finv_quote_secu_kline_min
    DROP CONSTRAINT IF EXISTS finv_quote_secu_kline_min_pkey;

-- ---------------------------------------------------------------------
-- 3. 删除 market_code 列
-- ---------------------------------------------------------------------
ALTER TABLE finv_quote_secu_kline_min
    DROP COLUMN IF EXISTS market_code;

-- ---------------------------------------------------------------------
-- 4. 按新主键 (ts, secu_code) 重建
-- ---------------------------------------------------------------------
ALTER TABLE finv_quote_secu_kline_min
    ADD CONSTRAINT finv_quote_secu_kline_min_pkey PRIMARY KEY (ts, secu_code);

COMMIT;
