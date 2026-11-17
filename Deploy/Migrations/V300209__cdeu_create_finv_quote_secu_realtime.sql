-- =====================================================================
-- V300209：在 cdeu schema 下新建跨 provider 的证券实时行情快照表
--          cdeu.finv_quote_secu_realtime
--
-- 定位（需求 11261）：
--   一行一个证券（主键 usc），按快照整体覆盖更新，不保留历史；
--   与 cdeu.finv_cdeu_secu_realtime_stock（单 provider 原始快照，主键 stock_id）
--   并存，**不得合并、不得互相改写**（DbSchemaSpec §0 红线）：
--   · finv_cdeu_secu_realtime_stock：源系统原样落库，主键 stock_id，无 provider 维度；
--   · finv_quote_secu_realtime：跨 provider 归一表，主键 usc，带 provider 区分来源。
--
-- 结构来源：
--   列集合与列序参考 cdeu.finv_cdeu_secu_realtime_stock（V300115 建表 +
--   V300117～V300128 逐步加列，共 34 列），本次变更三处：
--   1. 主键由 stock_id 改为 usc（usc NOT NULL）；
--   2. stock_id 由 BIGINT（V300116）改为 TEXT（可空，NULL = 源系统未分配）；
--   3. 表尾新增 provider：TEXT NOT NULL DEFAULT 'FTMM'（数据提供方）。
--   其余 31 列名 / 类型 / 口径与参考表完全一致（含 page_chk 约束与 gmt_update 触发器）。
--
-- 命名说明：
--   · 按用户明确要求落在 cdeu schema（与参考表同 schema），表名沿用
--     finv_quote_secu_ 前缀，属 DbSchemaSpec §1 登记的刻意例外，不得按
--     cdeu 域常规前缀 finv_quote_cdeu_ 改名；
--   · 表名 / 列名与参考表保持可对照，便于人工比对数仓两侧数据。
--
-- 幂等：CREATE TABLE / 约束 / 触发器 / 注释均可重复执行；末尾校验列清单、
--   列类型、主键名与主键列、page_chk、provider 默认值、注释数量与触发器，
--   任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- cdeu schema 由 V100007 建立；这里保留 IF NOT EXISTS 以便单独重放本脚本。
CREATE SCHEMA IF NOT EXISTS cdeu;

CREATE TABLE IF NOT EXISTS cdeu.finv_quote_secu_realtime (
    usc                      TEXT         NOT NULL,
    stock_id                 TEXT,
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
    price_nominal            NUMERIC(20, 6),
    change_ratio             NUMERIC(20, 6),
    trust_aum                NUMERIC(24, 6),
    trade_change_ratio       NUMERIC(20, 6),
    price_amplitude          NUMERIC(20, 6),
    volumn_ratio             NUMERIC(20, 6),
    buysell_ratio            NUMERIC(20, 6),
    change_price             NUMERIC(20, 6),
    price_direct             CHAR(1),
    leverage                 NUMERIC(20, 6),
    page                     INTEGER,
    rmb_exchange_rate        NUMERIC(24, 10),
    rmb_market_value         NUMERIC(24, 6),
    rmb_circulation_market_value NUMERIC(24, 6),
    rmb_trade_trunover       NUMERIC(24, 6),
    rmb_price_nominal        NUMERIC(20, 6),
    remark                   TEXT,
    gmt_create               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    gmt_update               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    provider                 TEXT         NOT NULL DEFAULT 'FTMM',
    CONSTRAINT finv_quote_secu_realtime_pk PRIMARY KEY (usc)
);

-- ---------------------------------------------------------------------
-- page 约束：页码无 0 与负语义，NULL 合法（与参考表 V300120 同口径）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_secu_realtime_page_chk'
           AND conrelid = 'cdeu.finv_quote_secu_realtime'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_quote_secu_realtime
            ADD CONSTRAINT finv_quote_secu_realtime_page_chk
            CHECK (page IS NULL OR page > 0);
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- gmt_update 自动维护（函数 public.vq_set_gmt_update 由 V200001 建立）
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_secu_realtime_gmt_update
    ON cdeu.finv_quote_secu_realtime;
CREATE TRIGGER trg_finv_quote_secu_realtime_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_secu_realtime
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 注释：口径逐列写明，避免下游按参考表拼写各自理解
-- ---------------------------------------------------------------------
COMMENT ON TABLE cdeu.finv_quote_secu_realtime
    IS '证券实时行情快照（跨 provider，一行一证券，主键 usc，按快照覆盖更新，不保留历史；与 finv_cdeu_secu_realtime_stock 并存、不得合并）';

COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.usc
    IS '证券唯一编码（Finv 内部统一编码，关联 public.finv_security.usc；本表主键，一证券一行）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.stock_id
    IS '证券编号（源系统证券 ID，字符串类型，可空；NULL = 源系统尚未分配）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.name
    IS '证券名称（市场本地简称）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.stock_code
    IS '证券代码（行情代码，不含市场前缀）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.market_type
    IS '市场类型（源系统市场分类编码）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.market_label
    IS '市场标签（市场显示名称）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.instrument_type
    IS '品种类型（股票/基金/债券等）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.currency
    IS '货币类型（ISO 4217 货币代码标准 ISO 4217:2015，三位字母代码，如 CNY=人民币、USD=美元、HKD=港币）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.market_value
    IS '总市值（= 最新价 × 总股本）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.total_shares
    IS '总股本（股）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.circulation_total_shares
    IS '流通股本（股）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.trade_trunover
    IS '成交额（当日累计；列名沿用上游拼写 trunover，含义为 turnover）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.trade_volumn
    IS '成交量（当日累计；列名沿用上游拼写 volumn，含义为 volume）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.circulation_market_value
    IS '流通市值（= 最新价 × 流通股本）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.dt_update
    IS '快照更新时间（Unix 毫秒时间戳）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.price_nominal
    IS '最新价（名义价，未复权）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.change_ratio
    IS '涨跌幅（相对昨收，小数比例）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.trust_aum
    IS '信托资产管理规模（仅基金类有值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.trade_change_ratio
    IS '成交额变动比率（相对基准期）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.price_amplitude
    IS '振幅（最高价与最低价之差相对昨收的比例）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.volumn_ratio
    IS '量比（当日成交量与过去若干日均量之比）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.buysell_ratio
    IS '委比（买卖盘强弱比例）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.change_price
    IS '涨跌额（相对昨收的价格变动）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.price_direct
    IS '价格变动方向符号（+ 涨 / - 跌 / 0 平）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.leverage
    IS '杠杆倍数（仅杠杆类品种有值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.page
    IS '来源页码（列表采集时的分页页码，INTEGER；NULL 表示未知）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.rmb_exchange_rate
    IS '人民币汇率（换算为人民币所用的汇率，1 单位本币折合人民币）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.rmb_market_value
    IS '总市值（人民币计价）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.rmb_circulation_market_value
    IS '流通市值（人民币计价）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.rmb_trade_trunover
    IS '成交额（人民币计价）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.rmb_price_nominal
    IS '最新价（人民币计价）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.remark
    IS '备注';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.gmt_create
    IS '创建时间（数据库写入时刻）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.gmt_update
    IS '更新时间（由触发器 trg_finv_quote_secu_realtime_gmt_update 维护）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime.provider
    IS '数据提供方（快照来源标识，如 FTMM；默认 FTMM）';

-- ---------------------------------------------------------------------
-- 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols text;
    v_want CONSTANT text := 'usc,stock_id,name,stock_code,market_type,market_label,'
                            'instrument_type,currency,market_value,total_shares,'
                            'circulation_total_shares,trade_trunover,trade_volumn,'
                            'circulation_market_value,dt_update,price_nominal,change_ratio,'
                            'trust_aum,trade_change_ratio,price_amplitude,volumn_ratio,'
                            'buysell_ratio,change_price,price_direct,leverage,page,'
                            'rmb_exchange_rate,rmb_market_value,rmb_circulation_market_value,'
                            'rmb_trade_trunover,rmb_price_nominal,remark,gmt_create,gmt_update,'
                            'provider';
    v_pk       text;
    v_pk_name  text;
    v_types    text;
    v_cmt      int;
    v_tbl_cmt  text;
    v_prov     text;
    v_prov_def text;
    v_ck       int;
    v_trg      int;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_realtime';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300209 列结构不符：期望 %，实际 %', v_want, COALESCE(v_cols, '<表不存在>');
    END IF;

    -- usc / stock_id 类型与可空性
    SELECT string_agg(column_name || ':' || data_type || ':' ||
                      CASE WHEN is_nullable = 'YES' THEN 'null' ELSE 'notnull' END, ','
                      ORDER BY ordinal_position)
      INTO v_types
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_realtime'
       AND column_name IN ('usc', 'stock_id');
    IF v_types IS DISTINCT FROM 'usc:text:notnull,stock_id:text:null' THEN
        RAISE EXCEPTION 'V300209 usc/stock_id 类型不符：期望 usc:text:notnull,stock_id:text:null，实际 %',
            COALESCE(v_types, '<无>');
    END IF;

    SELECT c.conname, string_agg(a.attname, ',' ORDER BY k.ord)
      INTO v_pk_name, v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_realtime'
       AND c.contype = 'p'
     GROUP BY c.conname;
    IF v_pk IS DISTINCT FROM 'usc' THEN
        RAISE EXCEPTION 'V300209 主键列不符：期望 usc，实际 %', COALESCE(v_pk, '<无主键>');
    END IF;
    IF v_pk_name IS DISTINCT FROM 'finv_quote_secu_realtime_pk' THEN
        RAISE EXCEPTION 'V300209 主键约束名不符：期望 finv_quote_secu_realtime_pk，实际 %',
            COALESCE(v_pk_name, '<无主键>');
    END IF;

    SELECT count(*) INTO v_ck
      FROM pg_constraint
     WHERE conname = 'finv_quote_secu_realtime_page_chk'
       AND conrelid = 'cdeu.finv_quote_secu_realtime'::regclass;
    IF v_ck <> 1 THEN
        RAISE EXCEPTION 'V300209 page_chk 约束不符：期望 1 个，实际 %', v_ck;
    END IF;

    SELECT is_nullable, column_default
      INTO v_prov, v_prov_def
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_realtime'
       AND column_name = 'provider';
    IF v_prov IS DISTINCT FROM 'NO' THEN
        RAISE EXCEPTION 'V300209 provider 可空性不符：期望 NOT NULL，实际 %', COALESCE(v_prov, '<无列>');
    END IF;
    IF v_prov_def IS NULL OR v_prov_def NOT LIKE '%FTMM%' THEN
        RAISE EXCEPTION 'V300209 provider 默认值不符：期望 FTMM，实际 %', COALESCE(v_prov_def, '<无默认值>');
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger
     WHERE tgname = 'trg_finv_quote_secu_realtime_gmt_update'
       AND tgrelid = 'cdeu.finv_quote_secu_realtime'::regclass
       AND NOT tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300209 gmt_update 触发器不符：期望 1 个，实际 %', v_trg;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_realtime'
       AND d.objsubid > 0;
    IF v_cmt <> 35 THEN
        RAISE EXCEPTION 'V300209 列注释数量不符：期望 35，实际 %', v_cmt;
    END IF;

    SELECT obj_description('cdeu.finv_quote_secu_realtime'::regclass, 'pg_class')
      INTO v_tbl_cmt;
    IF v_tbl_cmt IS NULL OR v_tbl_cmt = '' THEN
        RAISE EXCEPTION 'V300209 缺少表注释';
    END IF;

    RAISE NOTICE 'V300209 校验通过：35 列 / 主键(usc) / stock_id TEXT / provider 默认 FTMM / 约束与触发器齐全';
END $$;

COMMIT;
