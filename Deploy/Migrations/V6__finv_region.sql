-- =====================================================================
-- FinvQuant PostgreSQL V6：区域字典表 finv_region（表结构）
--
-- 决策（ACANX 2026-08-05）：
--   - 区域代码字典：idx 为数字序号，region 为区域简写（如 CN / HK / USA / JP）；
--   - idx 为主键；region 唯一；
--   - 本表暂无可确认的初始数据，待后续补充（届时按数据种子段 V100000+ 新增种子脚本）；
--   - 关联参考：finv_exchange.region（交易所所属区域）可对齐本表 region。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 区域字典表
-- ---------------------------------------------------------------------
CREATE TABLE finv_region (
    idx         INTEGER     PRIMARY KEY
                CHECK (idx BETWEEN 1 AND 999999),          -- 区域序号（数字）
    region      TEXT        NOT NULL,                      -- 区域简写（如 CN / HK / USA / JP）
    gmt_create  TIMESTAMPTZ NOT NULL DEFAULT now(),        -- 首次插入时间
    gmt_update  TIMESTAMPTZ NOT NULL DEFAULT now(),        -- 最后更新时间（触发器维护）
    CONSTRAINT uq_finv_region_region UNIQUE (region)
);

-- gmt_update 自动维护（复用 V1 定义的 vq_set_gmt_update 触发器函数）
CREATE TRIGGER trg_finv_region_gmt_update
    BEFORE UPDATE ON finv_region
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
