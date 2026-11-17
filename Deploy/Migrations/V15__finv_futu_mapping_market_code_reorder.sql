-- =====================================================================
-- FinvQuant PostgreSQL V15：finv_futu_mapping_market_code 重建表（列顺序调整）
--
-- 决策（ACANX 2026-08-05）：
--   - V8 已发布（PR #295 合并），其建表列顺序为 futu_market_code / finv_market_code；
--   - ACANX 要求字段顺序为 futu_market_code / market_name / exchange / finv_market_code，
--     PG 的 ADD COLUMN 只能追加到表尾，无法调整既有列顺序，故采用【重建表】方式：
--     建新表（目标列顺序）→ 迁移数据 → 删旧表 → 重命名 → 重建索引与触发器；
--   - market_name：市场名称（如 港股主板 / 美股指数）；exchange：对应交易所（如 SEHK / US / SSE）；
--   - 新装与已部署环境均执行本迁移，最终列顺序一致；数据（如有）完整迁移保留；
--   - 初始数据见 V100007__finv_futu_mapping_market_code_seed.sql（数据种子段）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 新表（目标列顺序）
-- ---------------------------------------------------------------------
CREATE TABLE finv_futu_mapping_market_code_new (
    futu_market_code  INTEGER     PRIMARY KEY,             -- 富途行情源市场代码（如 1 / 10 / 30 / 70 / 360）
    market_name       TEXT,                                -- 市场名称（如 港股主板 / 美股指数）
    exchange          TEXT,                                -- 对应交易所（如 SEHK / US / SSE；无交易所 N/A）
    finv_market_code  INTEGER     NOT NULL
                      CHECK (finv_market_code BETWEEN 1 AND 999999),  -- 交易市场代码（对齐 finv_market.market_code）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now()   -- 最后更新时间（触发器维护）
);

-- ---------------------------------------------------------------------
-- 2. 迁移数据（旧表 V8 无 market_name / exchange 列，占位 NULL；
--    该两列数据由 V100007 种子填充）
-- ---------------------------------------------------------------------
INSERT INTO finv_futu_mapping_market_code_new
    (futu_market_code, market_name, exchange, finv_market_code, gmt_create, gmt_update)
SELECT futu_market_code, NULL, NULL, finv_market_code, gmt_create, gmt_update
FROM finv_futu_mapping_market_code;

-- ---------------------------------------------------------------------
-- 3. 删除旧表（连带旧索引/触发器）
-- ---------------------------------------------------------------------
DROP TABLE finv_futu_mapping_market_code;

-- ---------------------------------------------------------------------
-- 4. 重命名新表
-- ---------------------------------------------------------------------
ALTER TABLE finv_futu_mapping_market_code_new
    RENAME TO finv_futu_mapping_market_code;

-- ---------------------------------------------------------------------
-- 5. 重建索引
-- ---------------------------------------------------------------------
CREATE INDEX idx_finv_futu_mapping_market_code_finv
    ON finv_futu_mapping_market_code (finv_market_code, futu_market_code);

-- ---------------------------------------------------------------------
-- 6. 重建触发器（gmt_update 自动维护）
-- ---------------------------------------------------------------------
CREATE TRIGGER trg_finv_futu_mapping_market_code_gmt_update
    BEFORE UPDATE ON finv_futu_mapping_market_code
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
