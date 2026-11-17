-- =====================================================================
-- V300216：期货列表采集——新建结果表 + 采集配置表（需求 11273/11274）
--
-- 背景（用户 2026-11-20 决策）：
--   moomoo 接口 `/quote-api/quote-v2/get-futures-list` 按「市场 + 分类」分页返回期货合约，
--   现要把 futuresType 1~8 全部采集落到**一张独立的新表**，口径对齐「行情快照同步」里
--   处理股票数据的方式（读配置表 → 分页拉取 → 逐页 upsert → 回写统计）。
--
-- 两张新表：
--   1. cdeu.finv_quote_realtime_future
--      结果表，一行一合约（**主键单列 stock_id**，用户决策）；只保最新快照，不建历史表。
--   2. cdeu.finv_quote_cdeu_future_sync_config
--      采集配置表，一行=一个「市场 + 期货分类」，采前读、采后回写 count_exists / total_available。
--
-- 关键口径（逐条都有明确来源，勿按其它快照表的习惯"纠正"）：
--   · 主键单列 stock_id（源 stockId，全局唯一）。**刻意不把 futures_type 放进主键**：
--     同一合约若同时出现在多个期货分类里只保留一行，futures_type 由最后采集的分类覆盖
--     （采集顺序按 futures_type 升序，故最大分类赢）。这是用户明确选择，
--     换来的是「不会出现同一合约两行互相打架的报价」，代价是跨分类归属会翻动。
--   · 分类中文名未知（1~8 待补），配置表用 futures_type_name 承载，前端降级显示「分类 N」；
--     回填只需一条 UPDATE，不必发版。
--   · 下架语义：dt_last_seen 记录最近一次出现在列表里的时刻，flag_online='0' 表示
--     「整轮全分类采集成功后仍未再出现」。置 0 只在「全部启用分类都成功」时执行，
--     单分类或部分失败一律不动，避免接口抖动把在售合约误标下架。
--   · 字段口径：接口 fields 是按字段编号的位置化字典，本仓库只收录已验证的
--     1=合约代码 / 2=名称 / 3=价格 / 4=涨跌幅 / 19=市场标签；其余原样进 fields_json，
--     field_list_json 留档（含 fieldName/contentType/precision），
--     日后把成交量/持仓量等提成列时不必重新采集。
--   · usc 可空（用户决策「解析 usc，字段可以先为空」）：先按 stock_id 关联
--     futu.finv_futu_quote_collect.usc，未命中再按 ftmm_symbol(upper(市场标签.合约代码))
--     关联 futu.finv_futu_secu_key.usc；都查不到写 NULL，**不用合约代码冒充 usc**。
--
-- 命名说明：
--   · 结果表名 `finv_quote_realtime_future` 由用户指定，schema 落在 cdeu。表名以
--     `finv_quote_` 开头但不在 quote 域，属 DbSchemaSpec §1 的**刻意例外**，
--     已在文档登记，并由 internal/database/schema_qualify_test.go 的
--     exactTableSchema 固定为 cdeu.；不得按域常规前缀改名。
--
-- 幂等：CREATE TABLE / 约束 / 索引 / 触发器 / 注释均可重复执行；seed 用
--   ON CONFLICT DO NOTHING；BRS 备份登记用 to_regclass 守卫 + ON CONFLICT DO NOTHING。
--   末尾校验列清单、主键、注释数、约束、索引、触发器与配置行数，任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- cdeu schema 由 V100007 建立；保留 IF NOT EXISTS 以便单独重放本脚本。
CREATE SCHEMA IF NOT EXISTS cdeu;

-- ---------------------------------------------------------------------
-- 1. 结果表：cdeu.finv_quote_realtime_future
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_realtime_future (
    stock_id        BIGINT       NOT NULL,
    usc             TEXT,
    futures_type    TEXT         NOT NULL,
    market_type     TEXT         NOT NULL DEFAULT '0',
    market_label    TEXT,
    contract_code   TEXT,
    name            TEXT,
    price_nominal   NUMERIC(20, 6),
    change_ratio    NUMERIC(20, 6),
    price_direct    CHAR(1),
    fields_json     JSONB,
    field_list_json JSONB,
    page            INTEGER,
    dt_update       BIGINT,
    dt_last_seen    BIGINT,
    flag_online     CHAR(1)      NOT NULL DEFAULT '1',
    provider        TEXT         NOT NULL DEFAULT 'FTMM',
    remark          TEXT,
    gmt_create      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    gmt_update      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_realtime_future_pk PRIMARY KEY (stock_id)
);

-- 分类 / 代码 / 更新时间的常用检索索引
CREATE INDEX IF NOT EXISTS idx_finv_quote_realtime_future_futures_type
    ON cdeu.finv_quote_realtime_future USING btree (futures_type);
CREATE INDEX IF NOT EXISTS idx_finv_quote_realtime_future_contract_code
    ON cdeu.finv_quote_realtime_future USING btree (contract_code);
CREATE INDEX IF NOT EXISTS idx_finv_quote_realtime_future_usc
    ON cdeu.finv_quote_realtime_future USING btree (usc);
CREATE INDEX IF NOT EXISTS idx_finv_quote_realtime_future_dt_update
    ON cdeu.finv_quote_realtime_future USING btree (dt_update);
-- 在售合约的部分索引：页面与下架核对只关心 flag_online='1' 的行
CREATE INDEX IF NOT EXISTS idx_finv_quote_realtime_future_online
    ON cdeu.finv_quote_realtime_future USING btree (futures_type)
    WHERE flag_online = '1';

-- ---------------------------------------------------------------------
-- 2. 结果表约束：页码、涨跌方向、在线标志、分类非空
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_realtime_future_page_chk'
           AND conrelid = 'cdeu.finv_quote_realtime_future'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_quote_realtime_future
            ADD CONSTRAINT finv_quote_realtime_future_page_chk
            CHECK (page IS NULL OR page > 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_realtime_future_price_direct_chk'
           AND conrelid = 'cdeu.finv_quote_realtime_future'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_quote_realtime_future
            ADD CONSTRAINT finv_quote_realtime_future_price_direct_chk
            CHECK (price_direct IS NULL OR price_direct IN ('+', '-', '='));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_realtime_future_flag_online_chk'
           AND conrelid = 'cdeu.finv_quote_realtime_future'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_quote_realtime_future
            ADD CONSTRAINT finv_quote_realtime_future_flag_online_chk
            CHECK (flag_online IN ('0', '1'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_realtime_future_futures_type_chk'
           AND conrelid = 'cdeu.finv_quote_realtime_future'::regclass
    ) THEN
        ALTER TABLE cdeu.finv_quote_realtime_future
            ADD CONSTRAINT finv_quote_realtime_future_futures_type_chk
            CHECK (btrim(futures_type) <> '');
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 结果表 gmt_update 自动维护（函数 public.vq_set_gmt_update 由 V200001 建立）
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_realtime_future_gmt_update
    ON cdeu.finv_quote_realtime_future;
CREATE TRIGGER trg_finv_quote_realtime_future_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_realtime_future
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 4. 结果表注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE cdeu.finv_quote_realtime_future
    IS '期货合约实时快照（moomoo get-futures-list，一行一合约，主键 stock_id，按快照覆盖更新，不保留历史；futures_type 由最后采集的分类覆盖）';

COMMENT ON COLUMN cdeu.finv_quote_realtime_future.stock_id
    IS '合约编号（源 stockId，全局唯一；本表主键，一合约一行）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.usc
    IS '统一证券编码（可空：先按 stock_id 关联 futu.finv_futu_quote_collect，再按 upper(市场标签.合约代码) 关联 futu.finv_futu_secu_key；都查不到为 NULL，不用代码冒充）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.futures_type
    IS '期货分类（源接口 futuresType 原值 1~8；同一合约跨分类时由最后采集的分类覆盖）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.market_type
    IS '市场类型（采集时请求的 marketType 原值；默认 0 表示聚合口径）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.market_label
    IS '市场标签（源响应 fields["19"]，如 US / HK）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.contract_code
    IS '合约代码（源响应 fields["1"]，如 GCmain）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.name
    IS '合约名称（源响应 fields["2"]）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.price_nominal
    IS '最新价（源响应 fields["3"] 的展示值，去千分位后入库）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.change_ratio
    IS '涨跌幅（源响应 fields["4"] 的展示值，去百分号后按百分点数值入库）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.price_direct
    IS '价格变动方向符号（+ 涨 / - 跌 / = 平）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.fields_json
    IS '原始展示字段（源响应 fields 全量，键为字段编号字符串，值为展示文本；仅 1/2/3/4/19 已提成列）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.field_list_json
    IS '原始字段定义（源响应 fieldList 全量，含 fieldName/contentType/precision，供日后补提列时追溯）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.page
    IS '来源页码（列表采集时的分页页码，从 1 起；NULL 表示未知）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.dt_update
    IS '本轮采集时刻（Unix 毫秒时间戳）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.dt_last_seen
    IS '最近一次出现在列表中的时刻（Unix 毫秒时间戳；用于判定下架）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.flag_online
    IS '在线标志（1=最近一轮全分类采集中仍在列表里；0=整轮全分类成功后仍未出现，视为下架）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.provider
    IS '数据提供方（快照来源标识，当前固定 FTMM）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.remark
    IS '备注（采集侧补充说明，如 usc 未解析）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.gmt_create
    IS '创建时间（数据库写入时刻）';
COMMENT ON COLUMN cdeu.finv_quote_realtime_future.gmt_update
    IS '更新时间（由触发器 trg_finv_quote_realtime_future_gmt_update 维护）';

-- ---------------------------------------------------------------------
-- 5. 采集配置表：cdeu.finv_quote_cdeu_future_sync_config
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_cdeu_future_sync_config (
    market_type       TEXT        NOT NULL DEFAULT '0',
    futures_type      TEXT        NOT NULL,
    futures_type_name TEXT,
    count_plan        INTEGER,
    count_exists      INTEGER,
    total_available   INTEGER,
    flag_enable       CHAR(1)     NOT NULL DEFAULT '1',
    remark            TEXT,
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_cdeu_future_sync_config_pk PRIMARY KEY (market_type, futures_type),
    CONSTRAINT finv_quote_cdeu_future_sync_config_enable_chk CHECK (flag_enable IN ('0', '1'))
);

DROP TRIGGER IF EXISTS trg_finv_quote_cdeu_future_sync_config_gmt_update
    ON cdeu.finv_quote_cdeu_future_sync_config;
CREATE TRIGGER trg_finv_quote_cdeu_future_sync_config_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_cdeu_future_sync_config
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

COMMENT ON TABLE cdeu.finv_quote_cdeu_future_sync_config
    IS '期货列表采集配置：一行=一个「市场 + 期货分类」，含计划数量与采集后回写的分类合约总数';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.market_type
    IS '市场类型（接口 marketType 原值；默认 0=聚合口径，可按分类覆盖为 1/2 等）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.futures_type
    IS '期货分类（接口 futuresType 原值 1~8）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.futures_type_name
    IS '分类中文名（待确认；NULL 时前端降级显示「分类 N」，回填后立即生效）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.count_plan
    IS '计划采集数量（每个分类采前 N 条）；NULL 表示按 count_exists 全量采集';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.count_exists
    IS '该分类当前存在的合约总数（接口 pagination.total）；采集成功后回写';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.total_available
    IS '本轮实际可采集数量 = min(count_plan, count_exists)；count_plan 为 NULL 时等于 count_exists';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.flag_enable
    IS '是否参与采集：1=参与、0=跳过';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.remark
    IS '备注：该分类采集说明（接口、请求参数、分类名待确认等）';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.gmt_create
    IS '创建时间';
COMMENT ON COLUMN cdeu.finv_quote_cdeu_future_sync_config.gmt_update
    IS '更新时间（由触发器 trg_finv_quote_cdeu_future_sync_config_gmt_update 维护）';

-- 种子：futuresType 1~8，默认聚合市场口径（marketType=0），默认全量采集
INSERT INTO cdeu.finv_quote_cdeu_future_sync_config
    (market_type, futures_type, futures_type_name, count_plan, remark)
VALUES
    ('0', '1', NULL, NULL, '期货分类 1（中文名待确认）：get-futures-list?marketType=0&futuresType=1，全量采集'),
    ('0', '2', NULL, NULL, '期货分类 2（中文名待确认）：get-futures-list?marketType=0&futuresType=2，全量采集'),
    ('0', '3', NULL, NULL, '期货分类 3（中文名待确认）：get-futures-list?marketType=0&futuresType=3，全量采集'),
    ('0', '4', NULL, NULL, '期货分类 4（中文名待确认）：get-futures-list?marketType=0&futuresType=4，全量采集'),
    ('0', '5', NULL, NULL, '期货分类 5（中文名待确认）：get-futures-list?marketType=0&futuresType=5，全量采集'),
    ('0', '6', NULL, NULL, '期货分类 6（中文名待确认）：get-futures-list?marketType=0&futuresType=6，全量采集'),
    ('0', '7', NULL, NULL, '期货分类 7（中文名待确认）：get-futures-list?marketType=0&futuresType=7，全量采集'),
    ('0', '8', NULL, NULL, '期货分类 8（中文名待确认）：get-futures-list?marketType=0&futuresType=8，全量采集')
ON CONFLICT (market_type, futures_type) DO NOTHING;

-- ---------------------------------------------------------------------
-- 6. BRS 全量备份登记（两张新表纳入每日全量备份）
--
-- priority 取 450 / 440：现有登记已用到 460 及以上（V300149 的 510~680、
-- V300150 的 460~490、更早的 690~1000），向下取 10 的整数互不冲突。
-- 用 to_regclass 守卫：某环境缺该表就跳过登记，不制造僵尸登记。
-- ---------------------------------------------------------------------
INSERT INTO brs.brs_bak_rdb_table_full
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
SELECT 'postgresql', 'finv', v.schema_name, v.table_name, '1', v.priority, 'both',
       'pg_dump', 'zstd', '1', 'migration', v.remark
  FROM (VALUES
        ('cdeu', 'finv_quote_realtime_future',            450, '期货合约实时快照（moomoo get-futures-list，需求 11273）'),
        ('cdeu', 'finv_quote_cdeu_future_sync_config',    440, '期货列表采集配置（需求 11273）')
       ) AS v(schema_name, table_name, priority, remark)
 WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 7. 收敛校验：结构、注释、种子行数、备份登记，不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols      text;
    v_want      CONSTANT text := 'stock_id,usc,futures_type,market_type,market_label,'
                                 'contract_code,name,price_nominal,change_ratio,price_direct,'
                                 'fields_json,field_list_json,page,dt_update,dt_last_seen,'
                                 'flag_online,provider,remark,gmt_create,gmt_update';
    v_cfg_cols  text;
    v_cfg_want  CONSTANT text := 'market_type,futures_type,futures_type_name,count_plan,'
                                 'count_exists,total_available,flag_enable,remark,'
                                 'gmt_create,gmt_update';
    v_pk        int;
    v_cmt       int;
    v_cfg_cmt   int;
    v_ck        int;
    v_idx       int;
    v_trg       int;
    v_rows      int;
    v_types     text;
    v_missing   text;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_realtime_future';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300216 结果表列不符：期望 %，实际 %', v_want, v_cols;
    END IF;

    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cfg_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_cdeu_future_sync_config';
    IF v_cfg_cols IS DISTINCT FROM v_cfg_want THEN
        RAISE EXCEPTION 'V300216 配置表列不符：期望 %，实际 %', v_cfg_want, v_cfg_cols;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conname = 'finv_quote_realtime_future_pk'
       AND conrelid = 'cdeu.finv_quote_realtime_future'::regclass
       AND contype = 'p';
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V300216 结果表主键不符：期望 1 个，实际 %', v_pk;
    END IF;

    SELECT count(*) INTO v_ck
      FROM pg_constraint
     WHERE conrelid = 'cdeu.finv_quote_realtime_future'::regclass
       AND contype = 'c';
    IF v_ck <> 4 THEN
        RAISE EXCEPTION 'V300216 结果表 CHECK 约束数不符：期望 4（page/price_direct/flag_online/futures_type），实际 %', v_ck;
    END IF;

    SELECT count(*) INTO v_idx
      FROM pg_indexes
     WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_realtime_future';
    -- 主键索引 1 + 显式索引 5
    IF v_idx <> 6 THEN
        RAISE EXCEPTION 'V300216 结果表索引数不符：期望 6（主键 1 + 显式 5），实际 %', v_idx;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger
     WHERE tgrelid = 'cdeu.finv_quote_realtime_future'::regclass
       AND NOT tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300216 结果表触发器不符：期望 1 个 gmt_update 触发器，实际 %', v_trg;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_realtime_future';
    IF v_cmt <> 21 THEN
        RAISE EXCEPTION 'V300216 结果表注释数不符：期望 21（表 + 20 列），实际 %', v_cmt;
    END IF;

    SELECT count(*) INTO v_cfg_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_cdeu_future_sync_config';
    IF v_cfg_cmt <> 11 THEN
        RAISE EXCEPTION 'V300216 配置表注释数不符：期望 11（表 + 10 列），实际 %', v_cfg_cmt;
    END IF;

    SELECT count(*), string_agg(futures_type, ',' ORDER BY futures_type)
      INTO v_rows, v_types
      FROM cdeu.finv_quote_cdeu_future_sync_config;
    IF v_rows <> 8 OR v_types <> '1,2,3,4,5,6,7,8' THEN
        RAISE EXCEPTION 'V300216 配置表种子不符：期望 8 行且分类为 1..8，实际 % 行 / %', v_rows, v_types;
    END IF;

    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_missing
      FROM (VALUES
              ('cdeu', 'finv_quote_realtime_future'),
              ('cdeu', 'finv_quote_cdeu_future_sync_config')
           ) AS v(schema_name, table_name)
     WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
       AND NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table_full b
              WHERE b.schema_name = v.schema_name AND b.table_name = v.table_name
           );
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300216 备份登记缺失：%', v_missing;
    END IF;

    RAISE NOTICE 'V300216 校验通过：结果表 20 列 / 配置表 10 列 / 种子 8 行 / 已登记 BRS 全量备份';
END $$;

COMMIT;
