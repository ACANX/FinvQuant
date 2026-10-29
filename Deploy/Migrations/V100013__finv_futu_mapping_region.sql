-- =====================================================================
-- FinvQuant PostgreSQL V13：富途区域映射表 finv_futu_mapping_region（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 富途行情源 region 字典（0~24）与 finv_region.idx 的字段映射表；
--   - futu_region 为主键（富途 region 值，如 0~24）；
--   - abbr / name：区域简写与中文名称（冗余展示，对齐 finv_region.region / name）；
--   - finv_region 对齐 finv_region.idx（INTEGER 0~999999，V11 起允许 0），
--     不建物理外键（项目惯例：程序层控制）；
--   - 初始数据见 V100005__finv_futu_mapping_region_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 富途区域映射表
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_region (
    futu_region   INTEGER     PRIMARY KEY,               -- 富途 region 值（如 0~24）
    abbr          TEXT        NOT NULL,                  -- 区域简写（对齐 finv_region.region，如 CN / HK / USA）
    name          TEXT        NOT NULL,                  -- 区域中文名称（对齐 finv_region.name，如 中国大陆 / 香港）
    finv_region   INTEGER     NOT NULL
                  CHECK (finv_region BETWEEN 0 AND 999999),  -- finv 区域序号（对齐 finv_region.idx）
    gmt_create    TIMESTAMPTZ NOT NULL DEFAULT now(),    -- 首次插入时间
    gmt_update    TIMESTAMPTZ NOT NULL DEFAULT now()     -- 最后更新时间（触发器维护）
);

-- 按 finv 区域序号反查富途 region
CREATE INDEX idx_finv_futu_mapping_region_finv
    ON finv_futu_mapping_region (finv_region, futu_region);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_futu_mapping_region_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_region
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
