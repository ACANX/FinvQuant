-- =====================================================================
-- FinvQuant PostgreSQL V76：恢复 quote.finv_quote_secu_kline_min（回测隔离副本）
--
-- 背景与纠偏（ACANX 2026-10-27）：
--   本表属于**双 schema 隔离设计**：
--     quote.finv_quote_secu_kline_min —— 量化回测程序专用（回测只读，写 SQL 必须带 quote. 前缀）
--     dedup.finv_quote_secu_kline_min —— 去重 / 导出 / 上传链路专用（写入与重写）
--   两张表同构但各自独立演进，靠 schema 前缀强制隔离，避免「未写前缀时
--   命中 search_path 里的另一张表」造成误用。
--
--   V38（2026-10-24）曾把 quote 下的这张表 `SET SCHEMA dedup` 迁走，导致
--   quote 侧副本消失、回测链路的隔离手段失效。本迁移按原设计恢复 quote 副本，
--   不改动 dedup 表（导出链路的现有状态不受影响）。
--
-- 做法：
--   1) 在 quote 下按 dedup 的最终结构建表（列、NOT NULL、CHECK、主键、索引名、
--      gmt_update 触发器、注释全部对齐，便于两侧结构与回测 SQL 复用）；
--   2) 仅当 quote 表为空时，从 dedup 回填一次（幂等；后续由导入/同步流程负责，
--      两表独立演进，不再自动覆盖）；
--   3) 收敛校验：行数与 dedup 一致、主键/索引/触发器/约束齐备。
--
-- 注意：
--   - 约束名、索引名在 quote schema 内与 dedup 内的同名不冲突（不同 schema）；
--   - 触发器函数沿用 public.vq_set_gmt_update()；
--   - 本迁移会搬运约 280 万行，单事务耗时较长，迁移超时已放宽到 5 分钟
--     （见 cmd/server/main.go）。
-- 迁移策略：单事务、失败整体回滚；幂等（重复执行仅做校验）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表（结构对齐 dedup）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quote.finv_quote_secu_kline_min (
    usc          text                     NOT NULL,
    ts           bigint                   NOT NULL,
    date         integer,
    time         integer,
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
    gmt_create   timestamptz              NOT NULL DEFAULT now(),
    gmt_update   timestamptz              NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_secu_kline_min_pkey         PRIMARY KEY (ts, usc),
    CONSTRAINT finv_quote_secu_kline_min_ts_check     CHECK (ts >= 0),
    CONSTRAINT finv_quote_secu_kline_min_date_check   CHECK (date IS NULL OR (date >= 19500101 AND date <= 21001231)),
    CONSTRAINT finv_quote_secu_kline_min_time_check   CHECK (time IS NULL OR (time >= 0 AND time <= 235959)),
    CONSTRAINT finv_quote_secu_kline_min_volume_check CHECK (volume IS NULL OR volume >= 0),
    CONSTRAINT finv_quote_secu_kline_min_turnover_check CHECK (turnover IS NULL OR turnover >= 0)
);

-- 索引（与 dedup 同名同定义，便于两侧 SQL 计划一致）
CREATE INDEX IF NOT EXISTS idx_finv_quote_usc_ts
    ON quote.finv_quote_secu_kline_min (usc, ts);
CREATE INDEX IF NOT EXISTS idx_finv_quote_usc_date_ts
    ON quote.finv_quote_secu_kline_min (usc, date, ts);
CREATE INDEX IF NOT EXISTS idx_finv_quote_date_usc_ts
    ON quote.finv_quote_secu_kline_min (date, usc, ts);

-- gmt_update 自动维护（与 dedup 同一触发器函数）
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'quote.finv_quote_secu_kline_min'::regclass
    ) THEN
        CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update
            BEFORE UPDATE ON quote.finv_quote_secu_kline_min
            FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();
    END IF;
END $$;

-- 注释（明确双表分工，避免后来人再"合并"）
COMMENT ON TABLE quote.finv_quote_secu_kline_min IS
    '分钟行情 K 线数据（量化回测专用，与 dedup.finv_quote_secu_kline_min 同构但相互隔离；回测 SQL 必须写 quote. 前缀）';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.usc          IS '证券编码';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.ts           IS '秒级 Unix 时间戳';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.date         IS '交易日（YYYYMMDD）';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.time         IS '交易时刻（HHMMSS）';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.change_price IS '涨跌额';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.change_ratio IS '涨跌幅';
COMMENT ON COLUMN quote.finv_quote_secu_kline_min.provider     IS '数据来源（行情供应商）';

-- ---------------------------------------------------------------------
-- 2. 从 dedup 回填（仅当 quote 表为空；之后两表各自演进）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_quote_rows BIGINT;
    v_dedup_rows BIGINT;
BEGIN
    SELECT count(*) INTO v_quote_rows FROM quote.finv_quote_secu_kline_min;
    IF v_quote_rows = 0 THEN
        INSERT INTO quote.finv_quote_secu_kline_min
            (usc, ts, date, time, open, high, low, close, paocd, volume, turnover,
             change_price, change_ratio, ext_field, last_close, remark, provider,
             gmt_create, gmt_update)
        SELECT usc, ts, date, time, open, high, low, close, paocd, volume, turnover,
               change_price, change_ratio, ext_field, last_close, remark, provider,
               gmt_create, gmt_update
          FROM dedup.finv_quote_secu_kline_min
        ON CONFLICT (ts, usc) DO NOTHING;
        SELECT count(*) INTO v_quote_rows FROM quote.finv_quote_secu_kline_min;
        RAISE NOTICE 'V76: 已从 dedup 回填 % 行到 quote.finv_quote_secu_kline_min', v_quote_rows;
    ELSE
        RAISE NOTICE 'V76: quote.finv_quote_secu_kline_min 已有 % 行，跳过回填', v_quote_rows;
    END IF;

    -- -----------------------------------------------------------------
    -- 3. 收敛校验
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_dedup_rows FROM dedup.finv_quote_secu_kline_min;
    IF v_dedup_rows > 0 AND v_quote_rows = 0 THEN
        RAISE EXCEPTION 'V76: quote 表回填失败（dedup 有 % 行，quote 为 0）', v_dedup_rows;
    END IF;
    IF v_quote_rows <> v_dedup_rows THEN
        RAISE WARNING 'V76: quote 与 dedup 行数不一致（quote=% / dedup=%）——若为后续独立演进属正常，请确认',
            v_quote_rows, v_dedup_rows;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'finv_quote_secu_kline_min_pkey'
           AND conrelid = 'quote.finv_quote_secu_kline_min'::regclass AND contype = 'p'
    ) THEN
        RAISE EXCEPTION 'V76: quote 表缺少主键 finv_quote_secu_kline_min_pkey(ts, usc)';
    END IF;

    IF (SELECT count(*) FROM pg_indexes
         WHERE schemaname = 'quote' AND tablename = 'finv_quote_secu_kline_min') < 4 THEN
        RAISE EXCEPTION 'V76: quote 表索引不足（期望主键 + 3 个业务索引）';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'quote.finv_quote_secu_kline_min'::regclass
    ) THEN
        RAISE EXCEPTION 'V76: quote 表缺少 gmt_update 触发器';
    END IF;

    RAISE NOTICE 'V76: quote.finv_quote_secu_kline_min 恢复完成（quote % 行 / dedup % 行，结构与触发器齐备）',
        v_quote_rows, v_dedup_rows;
END $$;

COMMIT;
