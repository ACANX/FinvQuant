-- =====================================================================
-- V300114：在 cdeu schema 下新建证券日 K 线表 finv_quote_secu_kline_day
--
-- 背景：
--   已有分钟线 cdeu.finv_quote_secu_kline_min（一天多行，主键 ts+usc）。
--   日线在此之前没有落库表，回测/统计只能临时从分钟线聚合。
--   本迁移先建在 cdeu schema（与去重、存量清点链路同库），等字段稳定后
--   再按同一结构扩展到 quote schema（届时另起迁移，不移动本表）。
--
-- 与分钟线表的关键差异：
--   · 一天一行 → 主键 (date, usc)；ts 保留为普通列（日 K 线时间戳，
--     通常取当日收盘时刻的 UTC 秒），不参与主键，否则同一天写两次
--     不同 ts 会产生重复日线。
--   · 去掉 time（HHmmss 对日线无意义）。
--   · 新增日线常用量：trade_count / turnover_rate / amplitude /
--     open_interest / hist_volatility / implied_volatility / pe_ratio /
--     adj_factor / suspend_flag / limit_up / limit_down。
--
-- 写入策略：与分钟线一致——允许修正，同键后到数据按字段覆盖
--   （INSERT ... ON CONFLICT (date, usc) DO UPDATE），不设不可变触发器。
--   gmt_update 由 public.vq_set_gmt_update() 触发器维护。
--
-- 幂等：重复执行影响 0 行；表/索引/触发器/注释均 IF NOT EXISTS 或重建。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 日 K 线主表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_secu_kline_day (
    usc                TEXT           NOT NULL,
    date               INTEGER        NOT NULL
                       CHECK (date BETWEEN 19500101 AND 21001231),
    ts                 BIGINT         CHECK (ts IS NULL OR ts >= 0),
    open               NUMERIC(20,6),
    high               NUMERIC(20,6),
    low                NUMERIC(20,6),
    close              NUMERIC(20,6),
    last_close         NUMERIC(20,6),
    change_price       NUMERIC(20,6),
    change_ratio       NUMERIC(20,6),
    vwap               NUMERIC(20,6),
    volume             NUMERIC(20,6)  CHECK (volume IS NULL OR volume >= 0),
    turnover           NUMERIC(30,8)  CHECK (turnover IS NULL OR turnover >= 0),
    trade_count        BIGINT         CHECK (trade_count IS NULL OR trade_count >= 0),
    turnover_rate      NUMERIC(20,6),
    amplitude          NUMERIC(20,6),
    open_interest      NUMERIC(20,6),
    hist_volatility    NUMERIC(20,8),
    implied_volatility NUMERIC(20,8),
    pe_ratio           NUMERIC(20,6),
    adj_factor         NUMERIC(20,8),
    suspend_flag       SMALLINT       CHECK (suspend_flag IS NULL OR suspend_flag IN (0, 1)),
    limit_up           NUMERIC(20,6),
    limit_down         NUMERIC(20,6),
    ext_field          TEXT,
    remark             TEXT,
    provider           VARCHAR(255),
    gmt_create         TIMESTAMPTZ    NOT NULL DEFAULT now(),
    gmt_update         TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_secu_kline_day_pkey PRIMARY KEY (date, usc)
);

-- 回测按「单证券 + 时间范围」拉全历史：最关键的一条索引。
CREATE INDEX IF NOT EXISTS idx_finv_quote_secu_kline_day_usc_date
    ON cdeu.finv_quote_secu_kline_day (usc, date);

-- gmt_update 自动维护（复用分钟线表的同一函数，避免重建）
DROP TRIGGER IF EXISTS trg_finv_quote_secu_kline_day_gmt_update
    ON cdeu.finv_quote_secu_kline_day;
CREATE TRIGGER trg_finv_quote_secu_kline_day_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_secu_kline_day
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 2. 列注释（口径写进注释，避免下游各自理解）
-- ---------------------------------------------------------------------
COMMENT ON TABLE cdeu.finv_quote_secu_kline_day IS '证券日K线行情（一天一行，主键 date+usc；与分钟线同字段风格）';

COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.usc IS '证券唯一编码';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.date IS '交易日期（yyyyMMdd）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.ts IS '日K线时间戳（UTC 秒，通常为当日收盘时刻；不参与主键）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.open IS '开盘价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.high IS '最高价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.low IS '最低价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.close IS '收盘价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.last_close IS '前收盘价（与 open 独立，不可作为开盘价替代）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.change_price IS '涨跌额';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.change_ratio IS '涨跌幅';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.vwap IS '当日成交量加权平均价（= turnover / volume）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.volume IS '成交量';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.turnover IS '成交额';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.trade_count IS '成交笔数';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.turnover_rate IS '换手率（%）；无流通股本数据时为 NULL';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.amplitude IS '振幅（%）= (high - low) / last_close * 100';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.open_interest IS '持仓量（期货/期权未平仓合约量；单位随品种，现货为空）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.hist_volatility IS '历史波动率（小数表示，0.18523456 = 18.52%；默认 20 个交易日窗口、按 252 交易日年化）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.implied_volatility IS '隐含波动率（小数表示；仅期权等有隐含波动率的品种非空）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.pe_ratio IS '市盈率（TTM，可为负；亏损时保留负值，静态 PE 不写入本列）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.adj_factor IS '复权因子（后复权；无复权数据填 1）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.suspend_flag IS '停牌标记：0=正常，1=停牌';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.limit_up IS '涨停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.limit_down IS '跌停价';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.ext_field IS '扩展字段（预留，存附加信息）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.remark IS '备注';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.provider IS '数据来源（行情供应商）';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.gmt_create IS '创建时间';
COMMENT ON COLUMN cdeu.finv_quote_secu_kline_day.gmt_update IS '更新时间（触发器维护）';

-- ---------------------------------------------------------------------
-- 3. 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols   text;
    v_want   CONSTANT text := 'usc,date,ts,open,high,low,close,last_close,change_price,'
                              'change_ratio,vwap,volume,turnover,trade_count,turnover_rate,'
                              'amplitude,open_interest,hist_volatility,implied_volatility,'
                              'pe_ratio,adj_factor,suspend_flag,limit_up,limit_down,'
                              'ext_field,remark,provider,gmt_create,gmt_update';
    v_pk     text;
    v_idx    int;
    v_trg    int;
    v_cmt    int;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_kline_day';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300114 列结构不符：期望 %，实际 %', v_want, COALESCE(v_cols, '<表不存在>');
    END IF;

    SELECT string_agg(a.attname, ',' ORDER BY k.ord)
      INTO v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_kline_day'
       AND c.contype = 'p';
    IF v_pk IS DISTINCT FROM 'date,usc' THEN
        RAISE EXCEPTION 'V300114 主键不符：期望 date,usc，实际 %', COALESCE(v_pk, '<无主键>');
    END IF;

    SELECT count(*) INTO v_idx
      FROM pg_indexes
     WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_kline_day'
       AND indexname = 'idx_finv_quote_secu_kline_day_usc_date';
    IF v_idx <> 1 THEN
        RAISE EXCEPTION 'V300114 缺少索引 idx_finv_quote_secu_kline_day_usc_date';
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_kline_day'
       AND tg.tgname = 'trg_finv_quote_secu_kline_day_gmt_update'
       AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300114 缺少 gmt_update 触发器';
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_kline_day'
       AND d.objsubid > 0;
    IF v_cmt <> 29 THEN
        RAISE EXCEPTION 'V300114 列注释数量不符：期望 29，实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300114 校验通过：29 列 / 主键(date,usc) / 索引 / 触发器 / 列注释齐全';
END $$;

COMMIT;
