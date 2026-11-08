-- =====================================================================
-- FinvQuant PostgreSQL V300137：新建证券键表 futu.finv_quote_secu_key
--
-- 来源：用户在 Supabase 侧已建的 `finv_quote_secu_key`（ACANX 2026-11-09 提供 DDL），
--   本次按其结构在本地/正式库落地，用于「instrument + market + code」到
--   「富途 ftmm_symbol」的键映射与采集状态跟踪（status / priority / url）。
--
-- 与用户原始 DDL 的差异（均为适配本仓库既有约定，语义不变）：
--   1. 索引改为建在 `futu.finv_quote_secu_key` 上（原始 DDL 里写的是 `public.`，
--      属 Supabase 侧残留；本表在 `futu` schema）；
--   2. gmt_update 触发器复用既有 `public.vq_set_gmt_update()`（与
--      `futu.finv_futu_quote_collect`、`public.finv_security` 同一套机制），
--      不新建专用函数 `finv_quote_secu_key_set_gmt_update()`；
--   3. 触发器命名沿用仓库前缀 `trg_<表名>_gmt_update`；
--   4. 补齐表/列注释（DbSchemaSpec 的建表要求）。
--   其余结构（列、类型、默认值、主键、priority 取值范围）与用户 DDL 完全一致。
--
-- 幂等：`CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS` +
--   触发器先 DROP IF EXISTS 再建 + 注释可重复执行；
--   末尾收敛校验（列数 / 主键 / 索引 / 触发器 / 注释），任一不符整事务回滚。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：`public.vq_set_gmt_update()` 已存在（V200018 起）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表（幂等）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_quote_secu_key (
    instrument   TEXT        NOT NULL,                          -- 品种（如 ETF / STOCK）
    market       TEXT        NOT NULL,                          -- 市场（如 US / HK / SH / SZ）
    code         TEXT        NOT NULL,                          -- 交易所代码（如 ANGU）
    ftmm_symbol  TEXT        NOT NULL,                          -- 富途代码标识（如 US.ANGU），主键
    flag_enable  SMALLINT    NOT NULL DEFAULT 1,                -- 启用标志（1=启用）
    priority     INTEGER     NOT NULL DEFAULT 10,               -- 采集优先级（1~1000000，数值越小越优先）
    status       TEXT        NOT NULL DEFAULT 'PENDING',        -- 处理状态（缺省 PENDING）
    url          TEXT        NULL,                              -- 关联链接（可空）
    gmt_create   TIMESTAMPTZ NOT NULL DEFAULT now(),            -- 首次插入时间
    gmt_update   TIMESTAMPTZ NOT NULL DEFAULT now(),            -- 最后更新时间（触发器维护）
    CONSTRAINT finv_quote_secu_key_pkey PRIMARY KEY (ftmm_symbol),
    CONSTRAINT finv_quote_secu_key_priority_check CHECK (priority >= 1 AND priority <= 1000000)
);

-- ---------------------------------------------------------------------
-- 2. 索引
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_finvquotesecukey_market
    ON futu.finv_quote_secu_key USING btree (instrument, market);

-- ---------------------------------------------------------------------
-- 3. gmt_update 自动维护（复用既有函数）
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_secu_key_gmt_update ON futu.finv_quote_secu_key;
CREATE TRIGGER trg_finv_quote_secu_key_gmt_update
    BEFORE UPDATE ON futu.finv_quote_secu_key
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 4. 表与列注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE futu.finv_quote_secu_key IS
    '证券键表：instrument + market + code → 富途 ftmm_symbol 的键映射与采集状态跟踪（status / priority / url）';

COMMENT ON COLUMN futu.finv_quote_secu_key.instrument IS '品种（如 ETF / STOCK）';
COMMENT ON COLUMN futu.finv_quote_secu_key.market IS '市场（如 US / HK / SH / SZ）';
COMMENT ON COLUMN futu.finv_quote_secu_key.code IS '交易所代码（如 ANGU）';
COMMENT ON COLUMN futu.finv_quote_secu_key.ftmm_symbol IS '富途代码标识（如 US.ANGU），主键';
COMMENT ON COLUMN futu.finv_quote_secu_key.flag_enable IS '启用标志（1=启用，其它=禁用）';
COMMENT ON COLUMN futu.finv_quote_secu_key.priority IS '采集优先级（1~1000000，数值越小越优先；缺省 10）';
COMMENT ON COLUMN futu.finv_quote_secu_key.status IS '处理状态（缺省 PENDING）';
COMMENT ON COLUMN futu.finv_quote_secu_key.url IS '关联链接（可空）';
COMMENT ON COLUMN futu.finv_quote_secu_key.gmt_create IS '首次插入时间';
COMMENT ON COLUMN futu.finv_quote_secu_key.gmt_update IS '最后更新时间（由 public.vq_set_gmt_update() 触发器维护）';

-- ---------------------------------------------------------------------
-- 5. 收敛校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt int;
    v_pk  int;
    v_idx int;
    v_trg int;
    v_cmt int;
BEGIN
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_quote_secu_key';
    IF v_cnt <> 10 THEN
        RAISE EXCEPTION 'V300137 列数不符：期望 10，实际 %', v_cnt;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_quote_secu_key' AND c.contype = 'p';
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V300137 主键缺失或重复';
    END IF;

    SELECT count(*) INTO v_idx
      FROM pg_indexes
     WHERE schemaname = 'futu' AND tablename = 'finv_quote_secu_key'
       AND indexname = 'idx_finvquotesecukey_market';
    IF v_idx <> 1 THEN
        RAISE EXCEPTION 'V300137 缺少索引 idx_finvquotesecukey_market';
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_quote_secu_key'
       AND tg.tgname = 'trg_finv_quote_secu_key_gmt_update' AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300137 缺少 gmt_update 触发器';
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.objsubid
     WHERE n.nspname = 'futu' AND t.relname = 'finv_quote_secu_key';
    IF v_cmt < 10 THEN
        RAISE EXCEPTION 'V300137 注释不全：期望 ≥10 条（表+9 列），实际 %', v_cmt;
    END IF;
END $$;

COMMIT;
