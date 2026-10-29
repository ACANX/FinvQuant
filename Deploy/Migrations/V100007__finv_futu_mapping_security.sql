-- =====================================================================
-- FinvQuant PostgreSQL V7：富途证券代码映射表 finv_futu_mapping_security（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - FT（富途/moomoo）行情源证券内部 ID 与统一证券代码（usc）的字段映射表；
--   - futu_stock_id 为主键（富途证券内部 ID，全局唯一，如 "70000294" / "50616191183396"）；
--   - finv_usc 对齐 finv_security.usc（统一证券代码，TEXT），不建物理外键（项目惯例：程序层控制）；
--   - 初始数据待确认后按数据种子段 V100000+ 补充。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途证券代码映射表
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_security (
    futu_stock_id  TEXT        PRIMARY KEY,               -- 富途证券内部 ID（moomoo stockId，如 70000294 / 50616191183396）
    finv_usc       TEXT        NOT NULL,                  -- 统一证券代码（对齐 finv_security.usc）
    gmt_create     TIMESTAMPTZ NOT NULL DEFAULT now(),    -- 首次插入时间
    gmt_update     TIMESTAMPTZ NOT NULL DEFAULT now()     -- 最后更新时间（触发器维护）
);

-- 按统一证券代码反查富途证券 ID
CREATE INDEX idx_finv_futu_mapping_security_usc
    ON finv_futu_mapping_security (finv_usc, futu_stock_id);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_futu_mapping_security_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_security
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
