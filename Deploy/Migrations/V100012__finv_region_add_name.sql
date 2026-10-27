-- =====================================================================
-- FinvQuant PostgreSQL V12：finv_region 增加 name 列（区域中文名称）
--
-- 决策（ACANX 2026-08-05）：
--   - 新增 name 列：区域中文名称（如 中国大陆 / 香港 / 美国），与 region 简写互补；
--   - TEXT 可空（存量行由 V100004 种子 ON CONFLICT DO UPDATE 回填，见种子脚本）；
--   - ADD COLUMN IF NOT EXISTS 保证幂等（重复执行不报错）；
--   - 初始数据见 V100004__finv_region_seed.sql（数据种子段，含 name 回填）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

ALTER TABLE finv_region
    ADD COLUMN IF NOT EXISTS name TEXT;

COMMIT;
