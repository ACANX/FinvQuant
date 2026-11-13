-- =====================================================================
-- V300115：在 cdeu schema 下新建证券实时行情快照表
--          cdeu.finv_cdeu_secu_realtime_stock
--
-- 定位：
--   一行一个证券（主键 stock_id），采集侧按快照整体覆盖更新，不保留历史；
--   历史序列看 cdeu.finv_quote_secu_kline_min（分钟线）与
--   cdeu.finv_quote_secu_kline_day（日线），本表只承载「最新一张快照」。
--
-- 命名说明：
--   · cdeu 域常规前缀为 finv_quote_cdeu_，本表沿用用户给定的
--     finv_cdeu_secu_realtime_stock（与 V300114 的 finv_quote_secu_kline_day
--     同属刻意例外），已在 Docs/DevSpec/DbSchemaSpec.md §1 登记，不得擅自改名；
--   · trade_trunover / trade_volumn 按用户给定 DDL 保留上游拼写，实际含义为
--     turnover（成交额）/ volume（成交量），口径写进列注释，避免下游按拼写误读。
--
-- 结构口径：
--   · 严格贴合给定 DDL：15 列、主键 stock_id；
--   · 不新增 gmt_create / gmt_update、不建触发器、不加二级索引；
--   · 金额与股数沿用 NUMERIC(24, 6)；
--   · dt_update 为 Unix 毫秒时间戳（不是 PG timestamptz，也不是 yyyyMMdd）。
--
-- 幂等：表 / 约束 / 注释均可重复执行；末尾校验列清单、主键名与主键列、
--   列注释数量，任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- cdeu schema 由 V100007 建立；这里保留 IF NOT EXISTS 以便单独重放本脚本。
CREATE SCHEMA IF NOT EXISTS cdeu;

CREATE TABLE IF NOT EXISTS cdeu.finv_cdeu_secu_realtime_stock (
    usc                      TEXT,
    stock_id                 INTEGER      NOT NULL,
    name                     TEXT,
    stock_code               TEXT,
    market_type              TEXT,
    market_label             TEXT,
    instrument_type          TEXT,
    currency                 TEXT,
    market_value             NUMERIC(24, 6),
    total_shares             NUMERIC(24, 6),
    circulation_total_shares NUMERIC(24, 6),
    trade_trunover           NUMERIC(24, 6),
    trade_volumn             NUMERIC(24, 6),
    circulation_market_value NUMERIC(24, 6),
    dt_update                BIGINT,
    CONSTRAINT finv_cdeu_secu_realtime_stock_pk PRIMARY KEY (stock_id)
);

-- ---------------------------------------------------------------------
-- 列注释：口径写进注释，避免下游各自理解
-- ---------------------------------------------------------------------
COMMENT ON TABLE cdeu.finv_cdeu_secu_realtime_stock
    IS '证券实时行情快照（一行一证券，主键 stock_id，按快照覆盖更新，不保留历史）';

COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.usc
    IS '证券唯一编码（Finv 内部统一编码，关联 public.finv_security.usc）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.stock_id
    IS '证券编号（源系统证券 ID，本表主键，一证券一行）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.name
    IS '证券名称（市场本地简称）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.stock_code
    IS '证券代码（行情代码，不含市场前缀）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.market_type
    IS '市场类型（源系统市场分类编码）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.market_label
    IS '市场标签（市场显示名称）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.instrument_type
    IS '品种类型（股票/基金/债券等）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.currency
    IS '货币类型（ISO 4217 货币代码标准 ISO 4217:2015，三位字母代码，如 CNY=人民币、USD=美元、HKD=港币）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.market_value
    IS '总市值（= 最新价 × 总股本）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.total_shares
    IS '总股本（股）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.circulation_total_shares
    IS '流通股本（股）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.trade_trunover
    IS '成交额（当日累计；列名沿用上游拼写 trunover，含义为 turnover）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.trade_volumn
    IS '成交量（当日累计；列名沿用上游拼写 volumn，含义为 volume）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.circulation_market_value
    IS '流通市值（= 最新价 × 流通股本）';
COMMENT ON COLUMN cdeu.finv_cdeu_secu_realtime_stock.dt_update
    IS '快照更新时间（Unix 毫秒时间戳）';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols text;
    v_want CONSTANT text := 'usc,stock_id,name,stock_code,market_type,market_label,'
                            'instrument_type,currency,market_value,total_shares,'
                            'circulation_total_shares,trade_trunover,trade_volumn,'
                            'circulation_market_value,dt_update';
    v_pk   text;
    v_pk_name text;
    v_cmt  int;
    v_tbl_cmt text;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_secu_realtime_stock';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300115 列结构不符：期望 %，实际 %', v_want, COALESCE(v_cols, '<表不存在>');
    END IF;

    SELECT c.conname, string_agg(a.attname, ',' ORDER BY k.ord)
      INTO v_pk_name, v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND c.contype = 'p'
     GROUP BY c.conname;
    IF v_pk IS DISTINCT FROM 'stock_id' THEN
        RAISE EXCEPTION 'V300115 主键列不符：期望 stock_id，实际 %', COALESCE(v_pk, '<无主键>');
    END IF;
    IF v_pk_name IS DISTINCT FROM 'finv_cdeu_secu_realtime_stock_pk' THEN
        RAISE EXCEPTION 'V300115 主键约束名不符：期望 finv_cdeu_secu_realtime_stock_pk，实际 %',
            COALESCE(v_pk_name, '<无主键>');
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_secu_realtime_stock'
       AND d.objsubid > 0;
    IF v_cmt <> 15 THEN
        RAISE EXCEPTION 'V300115 列注释数量不符：期望 15，实际 %', v_cmt;
    END IF;

    SELECT obj_description('cdeu.finv_cdeu_secu_realtime_stock'::regclass, 'pg_class')
      INTO v_tbl_cmt;
    IF v_tbl_cmt IS NULL OR v_tbl_cmt = '' THEN
        RAISE EXCEPTION 'V300115 缺少表注释';
    END IF;

    RAISE NOTICE 'V300115 校验通过：15 列 / 主键(stock_id) / 列注释齐全';
END $$;

COMMIT;
