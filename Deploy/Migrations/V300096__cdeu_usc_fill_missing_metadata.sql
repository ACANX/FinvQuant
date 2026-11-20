-- =====================================================================
-- V300096：补全 cdeu.finv_quote_cdeu_usc 中缺失的证券元数据（需求 10922）
--
-- 背景：
--   需求 10920/10921 把该表的行序改回「只由主表 cdeu.finv_quote_secu_kline_min
--   发现」，并用一次性脚本清掉了从 public.finv_security 拷来的脏行。清理后暴露出
--   两类字段缺口（本机 498 行中）：
--
--   A. region=CC 的加密货币（20 行）：timezone、day_expect_rows 为空。
--      原因：BackfillUscDefaults 的时区推导只覆盖 CN/HK/US/FX/GLOBAL，没有 CC；
--            模板表 finv_quote_cdeu_usc_day_incr_row_tpl 也没有 CC 区域的行。
--
--   B. 上游字典未收录的证券（13 行）：category/region/market/name/timezone/
--      currency/day_expect_rows 全空。原因：public.finv_security、futu 镜像表
--      （finv_futu_quote_collect / finv_futu_mapping_security）都没有这些 usc——
--      行情已采集但证券字典未收录，程序无从推导，只能显式发放。
--
-- 本迁移只做「数据发放」，不改任何代码逻辑，四步：
--   1. 模板表补 CC 区域两行（加密货币 7×24 = 1440）；
--   2. region=CC 的行补 timezone=UTC、day_expect_rows=1440；
--   3. 按显式清单补齐上游未收录证券的完整元数据（美股 5 只 + XRP + 3 个 A 股指数）；
--   4. name 仍为空的用 usc 兜底（不编造名称），避免页面出现空白单元格。
--
-- 幂等：全部为 COALESCE / ON CONFLICT 更新，重复执行影响 0 行且不覆盖人工值。
-- 人工值保护：所有列都用 COALESCE(原值, 新值)，只有空值才补。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 模板表：补 CC（加密货币）区域
--
-- region=CC / market=FTC 是本库加密货币的实际取值（futu quote_market=CC），
-- category=Crypto；7×24 交易 → 每天 1440 根分钟线。
-- ---------------------------------------------------------------------
INSERT INTO cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
    (region, market, category, day_expect_rows, flag_enable, remark)
VALUES
    ('CC', 'FTC', 'Crypto', 1440, '1', '加密货币 7×24 分钟线（富途 quote_market=CC、market=FTC）；V300096 补'),
    ('CC', '',    '',       1440, '1', '区域级兜底：CC 区域（market/category 未维护时）；V300096 补')
ON CONFLICT (region, market, category) DO UPDATE SET
    day_expect_rows = EXCLUDED.day_expect_rows,
    flag_enable     = EXCLUDED.flag_enable,
    remark          = EXCLUDED.remark,
    gmt_update      = now();

-- ---------------------------------------------------------------------
-- 2. region=CC：timezone / day_expect_rows
--
-- 加密货币 7×24 无休市，时区取 UTC（与 FX / GLOBAL 同口径）；
-- day_expect_rows 直接按第 1 步的模板值补，不依赖另一次 BackfillUscDefaults。
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_usc
   SET timezone        = COALESCE(timezone, 'UTC'),
       day_expect_rows = COALESCE(day_expect_rows, 1440),
       gmt_update      = now()
 WHERE upper(COALESCE(region, '')) = 'CC'
   AND (timezone IS NULL OR day_expect_rows IS NULL);

-- ---------------------------------------------------------------------
-- 3. 显式发放：上游字典未收录、但行情确实在采的证券
--
-- 这些 usc 在 public.finv_security / futu 镜像表里都查不到，程序无法推导市场与
-- 名称，只能在此登记。取值口径与库内既有数据一致：
--   美股：region=US、market=NSDQ/NYSE、category=Stock、America/New_York、USD、390；
--   加密：region=CC、market=FTC、category=Crypto、UTC、USD、1440；
--   A 股指数：region=CN、market=SH/BJ、category=StockIndex、Asia/Shanghai、CNY、241。
-- 只补空值，不覆盖人工维护过的值（COALESCE）。
-- ---------------------------------------------------------------------
WITH seed(usc, category, region, market, name, timezone, currency, day_expect_rows) AS (
    VALUES
        ('AMAT',     'Stock',      'US', 'NSDQ', 'Applied Materials, Inc.',        'America/New_York', 'USD', 390),
        ('MRVL',     'Stock',      'US', 'NSDQ', 'Marvell Technology, Inc.',        'America/New_York', 'USD', 390),
        ('NOW',      'Stock',      'US', 'NYSE', 'ServiceNow, Inc.',                'America/New_York', 'USD', 390),
        ('PLTR',     'Stock',      'US', 'NSDQ', 'Palantir Technologies Inc.',      'America/New_York', 'USD', 390),
        ('SMCI',     'Stock',      'US', 'NSDQ', 'Super Micro Computer, Inc.',      'America/New_York', 'USD', 390),
        ('XRP',      'Crypto',     'CC', 'FTC',  'XRP',                            'UTC',              'USD', 1440),
        ('899050',   'StockIndex', 'CN', 'BJ',   '北证50成份指数',                  'Asia/Shanghai',    'CNY', 241),
        ('SH000688', 'StockIndex', 'CN', 'SH',   '科创50指数',                      'Asia/Shanghai',    'CNY', 241),
        ('SH000888', 'StockIndex', 'CN', 'SH',   '上证综合指数',                    'Asia/Shanghai',    'CNY', 241)
)
UPDATE cdeu.finv_quote_cdeu_usc u
   SET category        = COALESCE(u.category, s.category),
       region          = COALESCE(u.region, s.region),
       market          = COALESCE(u.market, s.market),
       name            = COALESCE(u.name, s.name),
       timezone        = COALESCE(u.timezone, s.timezone),
       currency        = COALESCE(u.currency, s.currency),
       day_expect_rows = COALESCE(u.day_expect_rows, s.day_expect_rows),
       gmt_update      = now()
  FROM seed s
 WHERE u.usc = s.usc
   AND (u.category IS NULL OR u.region IS NULL OR u.market IS NULL OR u.name IS NULL
     OR u.timezone IS NULL OR u.currency IS NULL OR u.day_expect_rows IS NULL);

-- ---------------------------------------------------------------------
-- 4. name 兜底：仍为空的用 usc 本身
--
-- 不为无来源的证券编造名称（例如 SH000680 这类历史自定义指数、IT001~003 这类
-- provider=IT 的测试残留），但也不让页面出现空单元格。人工维护后用 PUT /Usc
-- 覆盖即可（本步只补空值，不会回写）。
-- ---------------------------------------------------------------------
UPDATE cdeu.finv_quote_cdeu_usc
   SET name       = usc,
       gmt_update = now()
 WHERE name IS NULL;

-- ---------------------------------------------------------------------
-- 5. 收敛校验：输出补全后仍缺的字段行数（不阻断升级）
--
-- 预期结果：category/region/market/name/timezone/currency/day_expect_rows 全为 0，
-- 唯一可能残留的是 provider（上游字典未收录、行情侧也没带 provider 的证券）。
-- 若出现非 0，说明还有证券缺来源，需按 JOB 日志的「仍缺 …；样本=…」人工维护。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total      bigint;
    v_no_cat     bigint;
    v_no_region  bigint;
    v_no_market  bigint;
    v_no_name    bigint;
    v_no_tz      bigint;
    v_no_ccy     bigint;
    v_no_expect  bigint;
    v_no_prov    bigint;
BEGIN
    SELECT count(*),
           count(*) FILTER (WHERE category IS NULL),
           count(*) FILTER (WHERE region IS NULL),
           count(*) FILTER (WHERE market IS NULL),
           count(*) FILTER (WHERE name IS NULL),
           count(*) FILTER (WHERE timezone IS NULL),
           count(*) FILTER (WHERE currency IS NULL),
           count(*) FILTER (WHERE day_expect_rows IS NULL),
           count(*) FILTER (WHERE provider IS NULL)
      INTO v_total, v_no_cat, v_no_region, v_no_market, v_no_name,
           v_no_tz, v_no_ccy, v_no_expect, v_no_prov
      FROM cdeu.finv_quote_cdeu_usc;

    RAISE NOTICE 'V300096 收敛校验：共 % 行；缺 category=% region=% market=% name=% timezone=% currency=% day_expect_rows=% provider=%',
        v_total, v_no_cat, v_no_region, v_no_market, v_no_name, v_no_tz, v_no_ccy, v_no_expect, v_no_prov;
END $$;

COMMIT;
