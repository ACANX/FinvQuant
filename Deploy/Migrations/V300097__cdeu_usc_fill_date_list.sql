-- =====================================================================
-- V300097：补全 cdeu.finv_quote_cdeu_usc.date_list（上市/上线日期）
-- （需求 10925）
--
-- 背景：
--   该列的唯一来源是 `RefreshSecurityMetadata` 从 `public.finv_security.init_date`
--   取值。需求 10922 那批「上游证券字典未收录、但行情确在采」的证券因此永久为空
--   （本机 495 行中有 10 行为空：AMAT/MRVL/NOW/PLTR/SMCI、XRP、899050、
--   SH000680/SH000688/SH000888）。
--
-- 本迁移两步（只补空值，不覆盖人工维护或上游已给的日期）：
--   1. 上市日期确定的股票按显式清单发放（美股 5 只，公开市场 IPO/直接上市日）；
--   2. 其余仍为空的用「主表最早行情交易日」兜底——语义是"本系统可追溯到的最早
--      交易日"，而不是杜撰一个上市日；指数、加密货币等没有统一"上市日"概念，
--      与其编造不如用这个可核对的值。
--
-- 幂等：全部 `WHERE date_list IS NULL`，重复执行影响 0 行。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 显式清单：美股 5 只的上市日
-- ---------------------------------------------------------------------
WITH seed(usc, date_list) AS (
    VALUES
        ('AMAT', DATE '1972-10-12'),   -- Applied Materials，纳斯达克 IPO
        ('MRVL', DATE '2000-06-27'),   -- Marvell Technology，纳斯达克 IPO
        ('NOW',  DATE '2012-06-29'),   -- ServiceNow，纽交所 IPO
        ('PLTR', DATE '2020-09-30'),   -- Palantir，纽交所直接上市
        ('SMCI', DATE '2007-03-29')    -- Super Micro Computer，纳斯达克 IPO
)
UPDATE cdeu.finv_quote_cdeu_usc u
   SET date_list  = s.date_list,
       gmt_update = now()
  FROM seed s
 WHERE u.usc = s.usc
   AND u.date_list IS NULL;

-- ---------------------------------------------------------------------
-- 2. 兜底：仍为空的取主表最早行情交易日
--
-- 覆盖 XRP（加密货币无统一上市日）、899050 / SH0006xx / SH000888（指数）等。
-- 主表 date 为空的行（异常数据）不参与，避免退化成 NULL。
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_usc u
   SET date_list  = m.first_date,
       gmt_update = now()
  FROM (
        -- 主表 date 是 integer（yyyyMMdd），需转 date；越界值（脏数据）不参与
        SELECT usc,
               CASE WHEN min(date) BETWEEN 19500101 AND 21001231
                    THEN to_date(min(date)::text, 'YYYYMMDD')
               END AS first_date
          FROM cdeu.finv_quote_secu_kline_min
         WHERE date IS NOT NULL
         GROUP BY usc
       ) m
 WHERE u.usc = m.usc
   AND u.date_list IS NULL
   AND m.first_date IS NOT NULL;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total bigint;
    v_null  bigint;
BEGIN
    SELECT count(*), count(*) FILTER (WHERE date_list IS NULL)
      INTO v_total, v_null
      FROM cdeu.finv_quote_cdeu_usc;
    RAISE NOTICE 'V300097 收敛校验：共 % 行，date_list 仍为空 % 行', v_total, v_null;
END $$;

COMMIT;
