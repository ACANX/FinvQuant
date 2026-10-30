-- =====================================================================
-- FinvQuant PostgreSQL V100083：新建富途原始属性镜像表 futu.finv_futu_quote_collect
--
-- 决策（ACANX 2026-10-30）：
--   - 背景：上游 Supabase 视图 finv_quote_collect_futu_view 中的「富途侧原始属性」
--     （stockId / marketType / marketCode / instrumentType / subInstrumentType /
--     reqSection / quoteMarket / typeSymbol）此前没有任何本地表承载，
--     finv_security 只保留 Finv 语义列；行情链路需要按富途原始编码反查 usc
--     （如按 stockId 或 (market_code, futu_symbol)），故单独建镜像表；
--   - 归属：表名以 finv_futu_ 开头，按 DbSchemaSpec §3 属「外部源映射域」，
--     放在 futu schema（与 V100033 迁入的 5 张 finv_futu_mapping_* 同域）；
--   - 主键 usc：与上游视图一致（上游 usc 全局唯一），便于与 finv_security 对位；
--   - 唯一约束 (market_code, futu_symbol)：富途侧「市场 + 代码」唯一，
--     实测上游 17,138 条无重复，作为第二业务键；
--   - 原始类型：stock_id 用 BIGINT（上游最大 148739012639708）；
--     market_type / market_code / instrument_type / sub_instrument_type 上游为
--     数字字符串，落 INTEGER（实测 7 次/5 类/33 类/19 类取值，无脏值）；
--   - req_section 可空（上游 17,138 条中仅 2 条有值，其余 NULL），
--     其余业务列 NOT NULL；flag_enable 沿用 CHAR(1) DEFAULT '0'（V100018 口径），
--     remark 用 TEXT NOT NULL DEFAULT ''（V100082 口径）；
--   - gmt_update 由既有 public.vq_set_gmt_update() 触发器维护；
--   - 数据初始化与增量同步由仓库外工具 FinvQuantDesign/Scripts/SecuSync/
--     BuildFutuQuoteCollect.py 生成幂等 SQL（ON CONFLICT (usc) DO UPDATE），
--     不写入迁移文件，避免每次上游更新都新增迁移。
-- 迁移策略：单事务、失败整体回滚；幂等（IF NOT EXISTS / 先删后建触发器）；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表（幂等）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_futu_quote_collect (
    usc                 TEXT        PRIMARY KEY,                        -- 统一证券代码（与 finv_security.usc 对位）
    stock_id            BIGINT      NOT NULL,                           -- 富途证券内部 ID（stockId）
    futu_symbol         TEXT        NOT NULL,                           -- 富途侧证券代码（原始形态，如 00700 / GCmain / .BFLY）
    market_type         INTEGER     NOT NULL,                           -- 富途市场类型（1=港股 2=美股 4=A股 11=外汇 17=加密）
    market_code         INTEGER     NOT NULL,                           -- 富途市场代码（1/2/3/10/30/31/35/120/360…，非 finv_market.market_code）
    instrument_type     INTEGER     NOT NULL,                           -- 富途品种类型（3=股票 4=ETF 6=指数 9=期货 10=外汇 11=加密 7=板块）
    sub_instrument_type INTEGER     NOT NULL,                           -- 富途品种子类型（3002/4002/6001/10001…）
    req_section         TEXT,                                           -- 富途行情订阅段（通常为空）
    quote_market        TEXT        NOT NULL,                           -- 富途行情市场标识（SZ/SH/HK/US…）
    type_symbol         TEXT        NOT NULL,                           -- 富途证券类型标识（stock/etfs/index/currency/crypto/futures/sectors）
    flag_enable         CHAR(1)     NOT NULL DEFAULT '0',               -- 启用标志（'0'=禁用，'1'=启用）
    remark              TEXT        NOT NULL DEFAULT '',                -- 备注（批次标记，空串=无备注）
    gmt_create          TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 首次插入时间
    gmt_update          TIMESTAMPTZ NOT NULL DEFAULT now()              -- 最后更新时间（触发器维护）
);

-- ---------------------------------------------------------------------
-- 2. 表与列注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE futu.finv_futu_quote_collect IS
    '富途原始属性镜像表：上游 finv_quote_collect_futu_view 的富途侧编码（stockId / marketType / marketCode / instrumentType 等），用于按富途原始标识反查 usc';

COMMENT ON COLUMN futu.finv_futu_quote_collect.usc IS '统一证券代码，主键，与 finv_security.usc 对位';
COMMENT ON COLUMN futu.finv_futu_quote_collect.stock_id IS '富途证券内部 ID（stockId，全局唯一）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.futu_symbol IS '富途侧证券代码（原始形态，未做 finv 规范化）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.market_type IS '富途市场类型（1=港股 2=美股 4=A股 11=外汇 17=加密）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.market_code IS '富途市场代码（非 finv_market.market_code；换算见 finv_futu_mapping_market_code）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.instrument_type IS '富途品种类型（3=股票 4=ETF 6=指数 9=期货 10=外汇 11=加密 7=板块）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.sub_instrument_type IS '富途品种子类型（如 3002 / 4002 / 6001 / 10001）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.req_section IS '富途行情订阅段，通常为空';
COMMENT ON COLUMN futu.finv_futu_quote_collect.quote_market IS '富途行情市场标识（SZ / SH / HK / US …）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.type_symbol IS '富途证券类型标识（stock / etfs / index / currency / crypto / futures / sectors）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.flag_enable IS '启用标志（''0''=禁用，''1''=启用）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.remark IS '备注（批次标记，空字符串表示无备注）';
COMMENT ON COLUMN futu.finv_futu_quote_collect.gmt_create IS '首次插入时间';
COMMENT ON COLUMN futu.finv_futu_quote_collect.gmt_update IS '最后更新时间（触发器维护）';

-- ---------------------------------------------------------------------
-- 3. 索引与约束
-- ---------------------------------------------------------------------
-- 富途「市场 + 代码」唯一（第二业务键，便于按富途标识定位）
CREATE UNIQUE INDEX IF NOT EXISTS uk_finv_futu_quote_collect_market_symbol
    ON futu.finv_futu_quote_collect (market_code, futu_symbol);

-- 按富途 stockId 反查 usc
CREATE INDEX IF NOT EXISTS idx_finv_futu_quote_collect_stock_id
    ON futu.finv_futu_quote_collect (stock_id);

-- 按富途证券类型 + 启用状态筛选
CREATE INDEX IF NOT EXISTS idx_finv_futu_quote_collect_type
    ON futu.finv_futu_quote_collect (type_symbol, flag_enable);

-- gmt_update 自动维护（复用 public 的触发器函数）
DROP TRIGGER IF EXISTS trg_finv_futu_quote_collect_gmt_update ON futu.finv_futu_quote_collect;
CREATE TRIGGER trg_finv_futu_quote_collect_gmt_update
    BEFORE UPDATE ON futu.finv_futu_quote_collect
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 4. 收敛校验（列/类型/可空/默认值/主键/唯一索引/触发器）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_expected CONSTANT TEXT[][] := ARRAY[
        ['usc',                 'text',        'NO',  NULL],
        ['stock_id',            'bigint',      'NO',  NULL],
        ['futu_symbol',         'text',        'NO',  NULL],
        ['market_type',         'integer',     'NO',  NULL],
        ['market_code',         'integer',     'NO',  NULL],
        ['instrument_type',     'integer',     'NO',  NULL],
        ['sub_instrument_type', 'integer',     'NO',  NULL],
        ['req_section',         'text',        'YES', NULL],
        ['quote_market',        'text',        'NO',  NULL],
        ['type_symbol',         'text',        'NO',  NULL],
        ['flag_enable',         'character',   'NO',  'bpchar'],
        ['remark',              'text',        'NO',  'text'],
        ['gmt_create',          'timestamp with time zone', 'NO', 'timestamptz'],
        ['gmt_update',          'timestamp with time zone', 'NO', 'timestamptz']
    ];
    v_row    TEXT[];
    v_type   TEXT;
    v_null   TEXT;
    v_def    TEXT;
    v_cnt    BIGINT;
BEGIN
    -- 4.1 列存在性与类型
    FOREACH v_row SLICE 1 IN ARRAY v_expected LOOP
        SELECT c.data_type, c.is_nullable, c.column_default
          INTO v_type, v_null, v_def
          FROM information_schema.columns AS c
         WHERE c.table_schema = 'futu'
           AND c.table_name   = 'finv_futu_quote_collect'
           AND c.column_name  = v_row[1];

        IF v_type IS NULL THEN
            RAISE EXCEPTION 'V100083 收敛失败：futu.finv_futu_quote_collect.% 未创建', v_row[1];
        END IF;
        IF v_type <> v_row[2] THEN
            RAISE EXCEPTION 'V100083 收敛失败：% 类型为 %，预期 %', v_row[1], v_type, v_row[2];
        END IF;
        IF v_null <> v_row[3] THEN
            RAISE EXCEPTION 'V100083 收敛失败：% 可空性为 %，预期 %', v_row[1], v_null, v_row[3];
        END IF;
        IF v_row[4] = 'bpchar' AND (v_def IS NULL OR position('bpchar' in v_def) = 0) THEN
            RAISE EXCEPTION 'V100083 收敛失败：flag_enable 默认值异常（%）', COALESCE(v_def, '(NULL)');
        END IF;
        IF v_row[4] = 'text' AND (v_def IS NULL OR replace(v_def, chr(39), '') <> '::text') THEN
            RAISE EXCEPTION 'V100083 收敛失败：% 默认值异常（%）', v_row[1], COALESCE(v_def, '(NULL)');
        END IF;
        IF v_row[4] = 'timestamptz' AND (v_def IS NULL OR position('now()' in v_def) = 0) THEN
            RAISE EXCEPTION 'V100083 收敛失败：% 默认值应为 now()（%）', v_row[1], COALESCE(v_def, '(NULL)');
        END IF;
    END LOOP;

    -- 4.2 列总数
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_futu_quote_collect';
    IF v_cnt <> 14 THEN
        RAISE EXCEPTION 'V100083 收敛失败：列数为 %，预期 14', v_cnt;
    END IF;

    -- 4.3 主键
    SELECT count(*) INTO v_cnt
      FROM pg_constraint c
      JOIN pg_class t     ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_futu_quote_collect' AND c.contype = 'p';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100083 收敛失败：主键数量为 %，预期 1', v_cnt;
    END IF;

    -- 4.4 (market_code, futu_symbol) 唯一索引
    SELECT count(*) INTO v_cnt
      FROM pg_indexes
     WHERE schemaname = 'futu'
       AND tablename  = 'finv_futu_quote_collect'
       AND indexname  = 'uk_finv_futu_quote_collect_market_symbol';
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100083 收敛失败：唯一索引 uk_finv_futu_quote_collect_market_symbol 缺失';
    END IF;

    -- 4.5 gmt_update 触发器
    SELECT count(*) INTO v_cnt
      FROM pg_trigger tg
      JOIN pg_class t     ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_futu_quote_collect'
       AND tg.tgname = 'trg_finv_futu_quote_collect_gmt_update' AND NOT tg.tgisinternal;
    IF v_cnt <> 1 THEN
        RAISE EXCEPTION 'V100083 收敛失败：触发器 trg_finv_futu_quote_collect_gmt_update 缺失';
    END IF;

    RAISE NOTICE 'V100083 完成：futu.finv_futu_quote_collect 就绪（14 列 / 主键 usc / 唯一 (market_code, futu_symbol) / gmt_update 触发器）';
END $$;

COMMIT;
