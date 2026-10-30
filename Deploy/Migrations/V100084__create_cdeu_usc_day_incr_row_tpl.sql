-- =====================================================================
-- FinvQuant PostgreSQL V100084：新建日增量行数模板表 cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
--
-- 决策（ACANX 2026-10-30，需求 10820）：
--   - 背景：cdeu.finv_quote_cdeu_usc.day_expect_rows 是成熟度判定的校准值，
--     为空时该证券的日数据长期停在 PARTIAL、永不进入导出候选；自动发现
--     （DiscoverUsc / RefreshSecurityMetadata）只写 usc 与 region/market 等派生列，
--     不给 day_expect_rows，导致流程「一开始就跑不起来」，必须人工逐个证券补。
--   - 方案：新增「按 region + market + category 预置的日增量行数模板」，
--     检查时若证券的 day_expect_rows 为空则按模板补全；已配置（非空）一律不覆盖，
--     保证运维改过的值不会被自动流程冲掉。
--   - 匹配键三列均可为**空字符串表示通配**：
--       region=''、market=''、category='' 三者全空即全局兜底行；
--       匹配时按「特异性」降序取第一条启用的模板
--       （region+market+category > region+market > region > market > 全通配）。
--   - category 对标 cdeu.finv_quote_cdeu_usc.category（如 Stock / ETF / StockIndex /
--     Futures / Forex；历史数据存在大小写不一致，匹配按 upper() 归一化）。
--   - day_expect_rows 语义与 cdeu.finv_quote_cdeu_usc.day_expect_rows 完全一致
--     （每日预期条数，A 股 241、加密货币 1440、美股 390、期货/外汇 1380），
--     必须为正整数：<=0 会让成熟度判定失去意义，故加 CHECK (>0)。
--   - 优先级 / SLA / 最大等待三列**不进本表**（用户 2026-10-30 明确）：
--     自动发现时的默认值固定按既有口径 100 / 5 / 7 补齐。
--   - gmt_update 由既有 public.vq_set_gmt_update() 触发器维护（同 V100083）。
--   - 模板初始数据由 V200065 写入（ON CONFLICT DO NOTHING，不覆盖运维已调整的值）。
-- 迁移策略：单事务、失败整体回滚；幂等（IF NOT EXISTS / 先删后建触发器）；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表（幂等）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_cdeu_usc_day_incr_row_tpl (
    region            TEXT        NOT NULL DEFAULT '',                  -- 区域（对应 usc.region，''=通配）
    market            TEXT        NOT NULL DEFAULT '',                  -- 市场符号（对应 usc.market，''=通配）
    category          TEXT        NOT NULL DEFAULT '',                  -- 证券分类（对应 usc.category，''=通配）
    day_expect_rows   INTEGER     NOT NULL,                             -- 每日预期条数（须 > 0）
    flag_enable       CHAR(1)     NOT NULL DEFAULT '1',                 -- 启用标志（'0'=停用不参与匹配，'1'=启用）
    remark            TEXT        NOT NULL DEFAULT '',                  -- 备注（取值依据，空串=无备注）
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),               -- 首次插入时间
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now(),               -- 最后更新时间（触发器维护）
    CONSTRAINT finv_quote_cdeu_usc_day_incr_row_tpl_pkey PRIMARY KEY (region, market, category),
    CONSTRAINT finv_quote_cdeu_usc_day_incr_row_tpl_rows_check CHECK (day_expect_rows > 0),
    CONSTRAINT finv_quote_cdeu_usc_day_incr_row_tpl_flag_check CHECK (flag_enable IN ('0', '1'))
);

-- ---------------------------------------------------------------------
-- 2. 表与列注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE cdeu.finv_quote_cdeu_usc_day_incr_row_tpl IS
    '日增量行数模板：按 region + market + category 预置 day_expect_rows，usc 主数据该列缺失时由自动流程按此补全（已配置的非空值不覆盖）';

COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.region IS '区域，对应 cdeu.finv_quote_cdeu_usc.region；空字符串表示通配（匹配任意区域）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.market IS '市场符号，对应 cdeu.finv_quote_cdeu_usc.market（取 finv_market.market_symbol，如 SH / SZ / BJ / HK / NSDQ / CFD）；空字符串表示通配';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.category IS '证券分类，对应 cdeu.finv_quote_cdeu_usc.category（如 Stock / ETF / StockIndex / Futures / Forex）；空字符串表示通配；匹配时按 upper() 归一化';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.day_expect_rows IS '每日预期条数（与 cdeu.finv_quote_cdeu_usc.day_expect_rows 同义）：A 股 241、美股 390、期货/外汇 1380、加密货币 1440';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.flag_enable IS '启用标志（''0''=停用不参与匹配，''1''=启用）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.remark IS '备注（取值依据与口径说明，空字符串表示无备注）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.gmt_create IS '首次插入时间';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.gmt_update IS '最后更新时间（触发器维护）';

-- ---------------------------------------------------------------------
-- 3. gmt_update 自动维护（复用 public 的触发器函数）
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_cdeu_usc_day_incr_row_tpl_gmt_update ON cdeu.finv_quote_cdeu_usc_day_incr_row_tpl;
CREATE TRIGGER trg_finv_quote_cdeu_usc_day_incr_row_tpl_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 4. 收敛校验（列/类型/可空/默认值/主键/CHECK/触发器）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_expected CONSTANT TEXT[][] := ARRAY[
        ['region',          'text',   'NO',  'text'],
        ['market',          'text',   'NO',  'text'],
        ['category',        'text',   'NO',  'text'],
        ['day_expect_rows', 'integer','NO',  NULL],
        ['flag_enable',     'character','NO','bpchar'],
        ['remark',          'text',   'NO',  'text'],
        ['gmt_create',      'timestamp with time zone', 'NO', 'timestamptz'],
        ['gmt_update',      'timestamp with time zone', 'NO', 'timestamptz']
    ];
    v_row  TEXT[];
    v_type TEXT;
    v_null TEXT;
    v_def  TEXT;
    v_cnt  BIGINT;
BEGIN
    -- 4.1 列存在性、类型、可空性与默认值
    FOREACH v_row SLICE 1 IN ARRAY v_expected LOOP
        SELECT c.data_type, c.is_nullable, c.column_default
          INTO v_type, v_null, v_def
          FROM information_schema.columns AS c
         WHERE c.table_schema = 'cdeu'
           AND c.table_name   = 'finv_quote_cdeu_usc_day_incr_row_tpl'
           AND c.column_name  = v_row[1];

        IF v_type IS NULL THEN
            RAISE EXCEPTION 'V100084 收敛失败：cdeu.finv_quote_cdeu_usc_day_incr_row_tpl.% 未创建', v_row[1];
        END IF;
        IF v_type <> v_row[2] THEN
            RAISE EXCEPTION 'V100084 收敛失败：% 类型为 %，预期 %', v_row[1], v_type, v_row[2];
        END IF;
        IF v_null <> v_row[3] THEN
            RAISE EXCEPTION 'V100084 收敛失败：% 可空性为 %，预期 %', v_row[1], v_null, v_row[3];
        END IF;
        IF v_row[4] = 'bpchar' AND (v_def IS NULL OR position('bpchar' in v_def) = 0) THEN
            RAISE EXCEPTION 'V100084 收敛失败：flag_enable 默认值异常（%）', COALESCE(v_def, '(NULL)');
        END IF;
        IF v_row[4] = 'text' AND (v_def IS NULL OR replace(v_def, chr(39), '') <> '::text') THEN
            RAISE EXCEPTION 'V100084 收敛失败：% 默认值异常（%）', v_row[1], COALESCE(v_def, '(NULL)');
        END IF;
        IF v_row[4] = 'timestamptz' AND (v_def IS NULL OR position('now()' in v_def) = 0) THEN
            RAISE EXCEPTION 'V100084 收敛失败：% 默认值应为 now()（%）', v_row[1], COALESCE(v_def, '(NULL)');
        END IF;
    END LOOP;

    -- 4.2 列总数
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_cdeu_usc_day_incr_row_tpl';
    IF v_cnt <> 8 THEN
        RAISE EXCEPTION 'V100084 收敛失败：列数为 %，预期 8', v_cnt;
    END IF;

    -- 4.3 主键 (region, market, category)
    SELECT count(*) INTO v_cnt
      FROM pg_constraint c
      JOIN pg_class t     ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu'
       AND t.relname = 'finv_quote_cdeu_usc_day_incr_row_tpl'
       AND c.contype = 'p';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100084 收敛失败：主键数量为 %，预期 1', v_cnt;
    END IF;

    -- 4.4 day_expect_rows > 0 检查约束
    SELECT count(*) INTO v_cnt
      FROM pg_constraint c
      JOIN pg_class t     ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu'
       AND t.relname = 'finv_quote_cdeu_usc_day_incr_row_tpl'
       AND c.conname = 'finv_quote_cdeu_usc_day_incr_row_tpl_rows_check'
       AND c.contype = 'c';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100084 收敛失败：检查约束 finv_quote_cdeu_usc_day_incr_row_tpl_rows_check 缺失';
    END IF;

    -- 4.5 gmt_update 触发器
    SELECT count(*) INTO v_cnt
      FROM pg_trigger tg
      JOIN pg_class t     ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu'
       AND t.relname = 'finv_quote_cdeu_usc_day_incr_row_tpl'
       AND tg.tgname = 'trg_finv_quote_cdeu_usc_day_incr_row_tpl_gmt_update'
       AND NOT tg.tgisinternal;
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100084 收敛失败：触发器 trg_finv_quote_cdeu_usc_day_incr_row_tpl_gmt_update 缺失';
    END IF;

    RAISE NOTICE 'V100084 完成：cdeu.finv_quote_cdeu_usc_day_incr_row_tpl 就绪（8 列 / 主键 (region, market, category) / day_expect_rows > 0 / gmt_update 触发器）';
END $$;

COMMIT;
