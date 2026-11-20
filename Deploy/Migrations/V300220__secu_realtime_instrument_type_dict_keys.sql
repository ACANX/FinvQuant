-- =====================================================================
-- V300220：把 `cdeu.finv_quote_secu_realtime.instrument_type` 的文本写法
--          统一为富途品种类型字典键值（需求 11302）
--
-- 背景：
--   · 字段规范见 `futu.finv_futu_quote_collect.instrument_type` 的列注释：
--     3=股票 4=ETF 6=指数 7=板块 9=期货 10=外汇 11=加密（INTEGER 键值）。
--   · BDFS 采集（百度股市通 `bsestock` / `cnetf`）历史实现把 `stock` / `etf`
--     直接写进了这张 TEXT 表，导致同一语义在库内有两套写法
--     （FTMM 写 `3` / `4`，BDFS 写 `stock` / `etf`），品种筛选与下游关联都要额外兼容。
--   · 本次把文本写法改回字典键值：`stock` → `3`、`etf` → `4`。
--     `12`（富途返回的股票同义值）属另一类问题（FTMM 原值保真），本迁移**不改**。
--
-- 范围：只改 `cdeu.finv_quote_secu_realtime` 一张表的两类文本值；
--       不改代码读取侧的兼容分支（归一表达式仍兼容历史文本，见
--       `realtime_stock.go` 的 realtimeStockInstrumentTypeExpr）。
--
-- 备份：受影响行整行备份到 archive.finv_quote_secu_realtime_itfix_20261118。
-- 幂等：备份用 IF NOT EXISTS，UPDATE 带 lower(btrim(...)) 条件，重复执行影响 0 行。
-- 回滚：用备份表的 instrument_type 原值按 usc 回写即可。
-- =====================================================================

BEGIN;

-- 1) 备份受影响行（只备份仍是文本写法的行，重复执行不追加）
CREATE TABLE IF NOT EXISTS archive.finv_quote_secu_realtime_itfix_20261118 AS
SELECT * FROM cdeu.finv_quote_secu_realtime
 WHERE lower(btrim(instrument_type)) IN ('stock', 'etf');

-- 2) 归一为字典键值
UPDATE cdeu.finv_quote_secu_realtime
   SET instrument_type = '4'
 WHERE lower(btrim(instrument_type)) = 'etf';

UPDATE cdeu.finv_quote_secu_realtime
   SET instrument_type = '3'
 WHERE lower(btrim(instrument_type)) = 'stock';

-- 3) 列注释补上字典口径，避免后续再写文本
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.instrument_type
    IS '品种类型（富途品种类型键值：3=股票 4=ETF 6=指数 7=板块 9=期货 10=外汇 11=加密；TEXT 存储，禁止写 stock/etf 等文本）';

-- 4) 校验：不允许残留非数字键
DO $$
DECLARE
    bad bigint;
BEGIN
    SELECT count(*) INTO bad
      FROM cdeu.finv_quote_secu_realtime
     WHERE instrument_type IS NOT NULL
       AND btrim(instrument_type) !~ '^[0-9]+$';
    IF bad > 0 THEN
        RAISE EXCEPTION 'V300220 校验失败：仍有 % 行 instrument_type 不是数字键值', bad;
    END IF;
END $$;

COMMIT;

-- 5) 执行结果概览（便于迁移日志核对）
SELECT instrument_type, count(*)
  FROM cdeu.finv_quote_secu_realtime
 GROUP BY 1
 ORDER BY 2 DESC;
