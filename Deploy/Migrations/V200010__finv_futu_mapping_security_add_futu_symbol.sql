-- =====================================================================
-- FinvQuant PostgreSQL V10：finv_futu_mapping_security 增加 futu_symbol 列
--
-- 决策（ACANX 2026-08-05）：
--   - V7 已随 PR #295 发布（可能已在部署环境执行），按迁移规范**不修改已发布迁移**，
--     新增 V10 增量迁移补充字段；
--   - futu_symbol：富途证券代码标识（字符串，如 HK.00700 / US.AAPL 等，具体取值以行情源为准）；
--   - 可空（存量行无需回填）；逻辑上为 finv_usc 之后的第二列（PG ADD COLUMN 追加到表尾，
--     列顺序不影响使用，SELECT 按名引用即可）；
--   - ADD COLUMN IF NOT EXISTS 保证幂等（重复执行不报错）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

ALTER TABLE finv_futu_mapping_security
    ADD COLUMN IF NOT EXISTS futu_symbol TEXT;

COMMIT;
