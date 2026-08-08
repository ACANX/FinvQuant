-- =====================================================================
-- FinvQuant PostgreSQL V11：finv_region 允许 idx=0（约束放宽）
--
-- 决策（ACANX 2026-08-05）：
--   - finv_region.idx 的 CHECK 由 1~999999 放宽为 0~999999，
--     以与富途 region 源数据一一对应（region=0 未知/默认）；
--   - 约束名 finv_region_idx_check（PG 内联 CHECK 默认命名），
--     DROP IF EXISTS + ADD 成对执行，重复执行幂等；
--   - 初始数据见 V100004__finv_region_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

ALTER TABLE finv_region
    DROP CONSTRAINT IF EXISTS finv_region_idx_check;

ALTER TABLE finv_region
    ADD CONSTRAINT finv_region_idx_check
    CHECK (idx BETWEEN 0 AND 999999);

COMMIT;
