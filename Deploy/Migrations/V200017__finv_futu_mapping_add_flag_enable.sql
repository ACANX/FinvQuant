-- =====================================================================
-- FinvQuant PostgreSQL V17：富途行情源映射表统一增加 flag_enable 启用标志
--
-- 决策（ACANX 2026-08-05）：
--   - 5 张富途映射表（security / exchange / market_code / region / cs_market）
--     统一增加 flag_enable 启用标志字段；
--   - 类型 CHAR(1)，默认值字符 '0'（0=禁用，1=启用；程序层按需 UPDATE 切换）；
--   - ADD COLUMN IF NOT EXISTS 幂等，重复执行不报错；
--   - 与既有迁移一致：单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途证券代码映射表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_security
    ADD COLUMN IF NOT EXISTS flag_enable CHAR(1) NOT NULL DEFAULT '0';

-- ---------------------------------------------------------------------
-- 2. 富途交易所映射表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_exchange
    ADD COLUMN IF NOT EXISTS flag_enable CHAR(1) NOT NULL DEFAULT '0';

-- ---------------------------------------------------------------------
-- 3. 富途市场代码映射表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_market_code
    ADD COLUMN IF NOT EXISTS flag_enable CHAR(1) NOT NULL DEFAULT '0';

-- ---------------------------------------------------------------------
-- 4. 富途区域映射表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_region
    ADD COLUMN IF NOT EXISTS flag_enable CHAR(1) NOT NULL DEFAULT '0';

-- ---------------------------------------------------------------------
-- 5. 富途 CS 市场映射表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_cs_market
    ADD COLUMN IF NOT EXISTS flag_enable CHAR(1) NOT NULL DEFAULT '0';

COMMIT;
