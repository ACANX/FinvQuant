-- =====================================================================
-- FinvQuant PostgreSQL V40：分钟行情主表补充日期范围索引
--
-- 背景（ACANX 2026-10-24）：
--   - 去重行情查询/统计页面默认按「证券 + 最近一个月交易日期」过滤；
--   - 原索引仅 (usc, ts)，日期过滤需沿 ts 索引扫描历史全部行，2.8M 行下
--     单次分页查询可达 8~17 秒并触发前端/网关超时；
--   - 补充 (usc, date, ts) 与 (date, usc, ts) 两个复合索引，覆盖：
--       * 单证券 + 日期范围 + ts 排序分页；
--       * 多证券/全市场按日期范围过滤与按日聚合。
--   - 幂等：CREATE INDEX IF NOT EXISTS；单事务。
-- 前置：V38/V39（表已在 dedup，列顺序已调整）。
-- =====================================================================

BEGIN;

CREATE INDEX IF NOT EXISTS idx_finv_quote_usc_date_ts
    ON dedup.finv_quote_secu_kline_min (usc, date, ts);

CREATE INDEX IF NOT EXISTS idx_finv_quote_date_usc_ts
    ON dedup.finv_quote_secu_kline_min (date, usc, ts);

COMMIT;
