-- =====================================================================
-- FinvQuant PostgreSQL V38：分钟行情 K 线表迁入 dedup schema + API 列补齐
--
-- 决策（ACANX 2026-10-24，DC-03/DC-04）：
--   - 融合 FinvQuoteDataDeDup 后，分钟行情业务表统一归属 dedup schema：
--     quote.finv_quote_secu_kline_min -> dedup.finv_quote_secu_kline_min；
--   - SET SCHEMA 为 catalog 级操作，不重写数据，索引/约束/触发器自动跟随；
--   - 补齐原 Python 表已有、Go 表缺失的三列：change_price / change_ratio / provider；
--   - ts 保持秒级（Python 存量数据本身即秒级），本迁移不做任何数据换算；
--   - export 转存表为 LIKE 快照，需按新结构重建（它只是导出暂存，允许清空）；
--   - 幂等：quote 下无表则跳过迁移；列用 IF NOT EXISTS；export 表先 DROP 再建；
--   - 单事务，失败回滚；末尾做收敛校验。
-- 前置：V36（dedup schema）、V37（archive schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 业务表迁入 dedup（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_tables
         WHERE schemaname = 'quote' AND tablename = 'finv_quote_secu_kline_min'
    ) THEN
        IF EXISTS (
            SELECT 1 FROM pg_tables
             WHERE schemaname = 'dedup' AND tablename = 'finv_quote_secu_kline_min'
        ) THEN
            RAISE EXCEPTION 'V38: quote 与 dedup 同时存在 finv_quote_secu_kline_min，需人工确认后重跑';
        END IF;
        ALTER TABLE quote.finv_quote_secu_kline_min SET SCHEMA dedup;
        RAISE NOTICE 'V38: quote.finv_quote_secu_kline_min -> dedup.finv_quote_secu_kline_min';
    ELSE
        RAISE NOTICE 'V38: quote 下无该表，跳过迁移（已迁入 dedup 或全新库）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 补齐 API 所需三列（幂等）
-- ---------------------------------------------------------------------
ALTER TABLE dedup.finv_quote_secu_kline_min
    ADD COLUMN IF NOT EXISTS change_price numeric(20,6),
    ADD COLUMN IF NOT EXISTS change_ratio numeric(20,6),
    ADD COLUMN IF NOT EXISTS provider     varchar(255);

COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.change_price IS '涨跌额';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.change_ratio IS '涨跌幅';
COMMENT ON COLUMN dedup.finv_quote_secu_kline_min.provider     IS '数据来源（行情供应商）';
COMMENT ON TABLE  dedup.finv_quote_secu_kline_min IS
    '分钟行情 K 线数据（融合 FinvQuoteDataDeDup；ts 为秒级 Unix 时间戳；读写必须写 dedup. 前缀）';

-- ---------------------------------------------------------------------
-- 3. export 转存表按新结构重建（暂存表允许清空）
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
    '行情导出转存表：结构与 dedup.finv_quote_secu_kline_min 一致，用于某证券月度/日行情导出前暂存；导出完成后可 TRUNCATE 复用。读写必须写 export. 前缀（与 quote/dedup 同名）。';

-- ---------------------------------------------------------------------
-- 4. 收敛校验：表归属、列集合、export 一致性、旧表已迁出
-- ---------------------------------------------------------------------
DO $$
DECLARE
    src_cols INT;
    dst_cols INT;
    missing  TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'dedup' AND tablename = 'finv_quote_secu_kline_min') THEN
        RAISE EXCEPTION 'V38: dedup.finv_quote_secu_kline_min 不存在';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'quote' AND tablename = 'finv_quote_secu_kline_min') THEN
        RAISE EXCEPTION 'V38: quote 下仍存在 finv_quote_secu_kline_min';
    END IF;

    SELECT string_agg(column_name, ',') INTO missing
      FROM (VALUES ('change_price'), ('change_ratio'), ('provider')) AS required(column_name)
     WHERE NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = 'dedup'
           AND table_name = 'finv_quote_secu_kline_min'
           AND column_name = required.column_name
     );
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'V38: dedup 表缺少列 %', missing;
    END IF;

    SELECT count(*) INTO src_cols FROM information_schema.columns
     WHERE table_schema = 'dedup' AND table_name = 'finv_quote_secu_kline_min';
    SELECT count(*) INTO dst_cols FROM information_schema.columns
     WHERE table_schema = 'export' AND table_name = 'finv_quote_secu_kline_min';
    IF src_cols <> dst_cols THEN
        RAISE EXCEPTION 'V38: export 副本列数与 dedup 不一致（dedup=% export=%）', src_cols, dst_cols;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'export.finv_quote_secu_kline_min'::regclass
    ) THEN
        RAISE EXCEPTION 'V38: export 副本缺少 gmt_update 触发器';
    END IF;

    RAISE NOTICE 'V38: 校验通过（dedup 列=%，export 列=%，quote 已无同名表）', src_cols, dst_cols;
END $$;

COMMIT;
