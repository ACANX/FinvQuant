-- =====================================================================
-- FinvQuant PostgreSQL V39：分钟行情主表列顺序调整（last_close 移到 ext_field 之后）
--
-- 背景（ACANX 2026-10-24）：
--   - PostgreSQL 不支持调整列顺序，只能重建表；本迁移在保持全部约束、
--     索引、触发器、注释、数据与 gmt_* 原值的前提下重建主表；
--   - 目标顺序：
--       usc, ts, date, time, open, high, low, close, paocd, volume, turnover,
--       change_price, change_ratio, ext_field, last_close, remark, provider,
--       gmt_create, gmt_update
--   - 幂等：先比对当前列顺序，已符合则跳过重建；索引/触发器/注释/export 副本
--     另行幂等补齐。
--   - 单事务；重建失败整体回滚（旧表保持原样）。
-- 前置：V38（表已迁入 dedup，包含 change_price/change_ratio/provider）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    desired CONSTANT text := 'usc,ts,date,time,open,high,low,close,paocd,volume,turnover,change_price,change_ratio,ext_field,last_close,remark,provider,gmt_create,gmt_update';
    current_order text;
    old_rows bigint;
    new_rows bigint;
    rec record;
    new_name text;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO current_order
      FROM information_schema.columns
     WHERE table_schema = 'dedup'
       AND table_name = 'finv_quote_secu_kline_min';

    IF current_order = desired THEN
        RAISE NOTICE 'V39: 列顺序已符合要求，跳过重建';
        RETURN;
    END IF;

    RAISE NOTICE 'V39: 当前列顺序与目标不一致，开始重建（原顺序：%）', current_order;

    EXECUTE 'LOCK TABLE dedup.finv_quote_secu_kline_min IN ACCESS EXCLUSIVE MODE';
    EXECUTE 'DROP TABLE IF EXISTS dedup.finv_quote_secu_kline_min_new';

    EXECUTE $ddl$
        CREATE TABLE dedup.finv_quote_secu_kline_min_new (
            usc          text          NOT NULL,
            ts           bigint        NOT NULL,
            date         integer,
            "time"       integer,
            open         numeric(20,6),
            high         numeric(20,6),
            low          numeric(20,6),
            close        numeric(20,6),
            paocd        numeric(20,6),
            volume       bigint,
            turnover     numeric(30,8),
            change_price numeric(20,6),
            change_ratio numeric(20,6),
            ext_field    text,
            last_close   numeric(20,6),
            remark       text,
            provider     varchar(255),
            gmt_create   timestamptz   NOT NULL DEFAULT now(),
            gmt_update   timestamptz   NOT NULL DEFAULT now(),
            CONSTRAINT finv_quote_secu_kline_min_new_pkey PRIMARY KEY (ts, usc),
            CONSTRAINT finv_quote_secu_kline_min_date_check
                CHECK (date IS NULL OR (date >= 19500101 AND date <= 21001231)),
            CONSTRAINT finv_quote_secu_kline_min_time_check
                CHECK ("time" IS NULL OR ("time" >= 0 AND "time" <= 235959)),
            CONSTRAINT finv_quote_secu_kline_min_ts_check CHECK (ts >= 0),
            CONSTRAINT finv_quote_secu_kline_min_turnover_check
                CHECK (turnover IS NULL OR turnover >= 0),
            CONSTRAINT finv_quote_secu_kline_min_volume_check
                CHECK (volume IS NULL OR volume >= 0)
        )
    $ddl$;

    EXECUTE $ins$
        INSERT INTO dedup.finv_quote_secu_kline_min_new
            (usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
             change_price, change_ratio, ext_field, last_close, remark, provider,
             gmt_create, gmt_update)
        SELECT usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
               change_price, change_ratio, ext_field, last_close, remark, provider,
               gmt_create, gmt_update
          FROM dedup.finv_quote_secu_kline_min
    $ins$;

    SELECT count(*) INTO old_rows FROM dedup.finv_quote_secu_kline_min;
    SELECT count(*) INTO new_rows FROM dedup.finv_quote_secu_kline_min_new;
    IF old_rows <> new_rows THEN
        RAISE EXCEPTION 'V39: 复制行数不一致（原=%，新=%），回滚', old_rows, new_rows;
    END IF;
    RAISE NOTICE 'V39: 已复制 % 行', new_rows;

    EXECUTE 'CREATE INDEX idx_finv_quote_usc_ts_new ON dedup.finv_quote_secu_kline_min_new (usc, ts)';
    EXECUTE 'DROP TABLE dedup.finv_quote_secu_kline_min';
    EXECUTE 'ALTER TABLE dedup.finv_quote_secu_kline_min_new RENAME TO finv_quote_secu_kline_min';
    EXECUTE 'ALTER TABLE dedup.finv_quote_secu_kline_min RENAME CONSTRAINT finv_quote_secu_kline_min_new_pkey TO finv_quote_secu_kline_min_pkey';
    EXECUTE 'ALTER INDEX dedup.idx_finv_quote_usc_ts_new RENAME TO idx_finv_quote_usc_ts';

    -- NOT NULL 约束恢复为历史名称（usc 历史名为 secu_code_not_null）。
    FOR rec IN
        SELECT conname FROM pg_constraint
         WHERE conrelid = 'dedup.finv_quote_secu_kline_min'::regclass
           AND contype = 'n'
    LOOP
        IF rec.conname LIKE '%_new_%' THEN
            new_name := replace(rec.conname, '_new_', '_');
            IF new_name = 'finv_quote_secu_kline_min_usc_not_null' THEN
                new_name := 'finv_quote_secu_kline_min_secu_code_not_null';
            END IF;
            EXECUTE format('ALTER TABLE dedup.finv_quote_secu_kline_min RENAME CONSTRAINT %I TO %I',
                           rec.conname, new_name);
        END IF;
    END LOOP;

    RAISE NOTICE 'V39: 主表重建完成';
END $$;

-- ---------------------------------------------------------------------
-- 索引 / 触发器 / 注释（幂等，覆盖「已符合顺序而跳过重建」的分支）
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_finv_quote_usc_ts
    ON dedup.finv_quote_secu_kline_min (usc, ts);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'dedup.finv_quote_secu_kline_min'::regclass
    ) THEN
        CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update
            BEFORE UPDATE ON dedup.finv_quote_secu_kline_min
            FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();
        RAISE NOTICE 'V39: 已重建 gmt_update 触发器';
    END IF;
END $$;

COMMENT ON TABLE dedup.finv_quote_secu_kline_min IS
    '分钟行情 K 线数据（融合 FinvQuoteDataDeDup；ts 为秒级 Unix 时间戳；last_close 位于 ext_field 之后）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.usc IS '证券唯一编码';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.ts IS '时间戳（秒级 Unix 时间戳）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.date IS '交易日期（yyyyMMdd）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min."time" IS '交易时间（HHmmss）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.open IS '开盘价';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.high IS '最高价';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.low IS '最低价';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.close IS '收盘价（当前分钟收盘价）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.paocd IS '当日实时累计均价';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.volume IS '成交量';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.turnover IS '成交额';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.change_price IS '涨跌额';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.change_ratio IS '涨跌幅';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.ext_field IS '扩展字段（预留，存附加信息）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.last_close IS
    '前收盘价（数据源 MVSV 的 PrevClose / p 列）；与 open 独立，不可作为开盘价或开盘价的替代';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.remark IS '备注';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.provider IS '数据来源（行情供应商）';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.gmt_update IS '更新时间';

-- ---------------------------------------------------------------------
-- export 转存表按新结构重建（暂存表，允许清空）
-- ---------------------------------------------------------------------
DROP TABLE IF EXISTS export.finv_quote_secu_kline_min;
CREATE TABLE export.finv_quote_secu_kline_min (
    LIKE dedup.finv_quote_secu_kline_min
    INCLUDING DEFAULTS
    INCLUDING CONSTRAINTS
    INCLUDING INDEXES
    INCLUDING STORAGE
    INCLUDING COMMENTS
);
CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update
    BEFORE UPDATE ON export.finv_quote_secu_kline_min
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();
COMMENT ON TABLE export.finv_quote_secu_kline_min IS
    '行情导出转存表：结构与 dedup.finv_quote_secu_kline_min 一致（列顺序同源），用于导出前暂存；导出完成后可 TRUNCATE 复用。';

-- ---------------------------------------------------------------------
-- 收敛校验：列顺序、export 一致性、索引与触发器
-- ---------------------------------------------------------------------
DO $$
DECLARE
    desired CONSTANT text := 'usc,ts,date,time,open,high,low,close,paocd,volume,turnover,change_price,change_ratio,ext_field,last_close,remark,provider,gmt_create,gmt_update';
    current_order text;
    src_cols int;
    dst_cols int;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO current_order
      FROM information_schema.columns
     WHERE table_schema = 'dedup' AND table_name = 'finv_quote_secu_kline_min';
    IF current_order <> desired THEN
        RAISE EXCEPTION 'V39: 列顺序校验失败（当前：%）', current_order;
    END IF;

    SELECT count(*) INTO src_cols FROM information_schema.columns
     WHERE table_schema = 'dedup' AND table_name = 'finv_quote_secu_kline_min';
    SELECT count(*) INTO dst_cols FROM information_schema.columns
     WHERE table_schema = 'export' AND table_name = 'finv_quote_secu_kline_min';
    IF src_cols <> dst_cols THEN
        RAISE EXCEPTION 'V39: export 副本列数不一致（dedup=%，export=%）', src_cols, dst_cols;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes
         WHERE schemaname = 'dedup' AND tablename = 'finv_quote_secu_kline_min'
           AND indexname = 'idx_finv_quote_usc_ts'
    ) THEN
        RAISE EXCEPTION 'V39: 缺少索引 idx_finv_quote_usc_ts';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'dedup.finv_quote_secu_kline_min'::regclass
    ) THEN
        RAISE EXCEPTION 'V39: 缺少 gmt_update 触发器';
    END IF;

    RAISE NOTICE 'V39: 校验通过（列=%，export 列=%）', src_cols, dst_cols;
END $$;

ANALYZE dedup.finv_quote_secu_kline_min;

COMMIT;
