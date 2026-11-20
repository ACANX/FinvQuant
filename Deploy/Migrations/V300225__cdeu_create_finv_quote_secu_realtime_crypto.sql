-- =====================================================================
-- V300225：新建 cdeu.finv_quote_secu_realtime_crypto（需求 11314）
--
-- 用途：把「证券收藏表 quote.finv_quote_secu_fav」里收藏的**加密货币** usc，
--       通过 moomoo OpenAPI（POST https://webapi.moomoo.com/api/v1.0/quote/snapshot）
--       拉取实时快照，落到本表（一行一交易对，按 usc 覆盖）。
--
-- 字段口径（对齐 cdeu.finv_quote_secu_realtime 的命名习惯）：
--   code            moomoo 代码（CC.BTCUSD）
--   name            接口 name（BTC/USD）
--   quote_currency  计价货币（从 usc 后缀推断：*USD → USD，*HKD → HKD，*EUR → EUR …）
--   price_nominal   last_price
--   price_open      open_price
--   price_highest   high_price
--   price_lowest    low_price
--   price_last_close prev_close_price
--   price_avg       avg_price
--   change_price    last_price - prev_close_price（接口未直接给，落库时计算）
--   change_ratio    涨跌幅（接口 amplitude 是振幅，涨跌幅用 change_price / prev_close * 100）
--   price_amplitude amplitude（%）
--   price_direct    + / - / =
--   trade_volumn    volume
--   trade_trunover  turnover
--   volumn_ratio    volume_ratio
--   bid_price / ask_price / bid_vol / ask_vol / buysell_ratio（bid_ask_ratio，%）
--   highest52weeks / lowest52weeks / highest_history / lowest_history
--   sec_status / suspension / data_date
--   dt_update       接口 update_time（Unix 毫秒）
--   provider        固定 'MMAPI'（moomoo OpenAPI）
--
-- 幂等：CREATE TABLE / 索引 / 触发器 / 注释 / 备份登记均可重复执行。
-- 回滚：DROP TABLE cdeu.finv_quote_secu_realtime_crypto;
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cdeu.finv_quote_secu_realtime_crypto (
    usc               TEXT          NOT NULL,
    code              TEXT          NOT NULL DEFAULT '',
    name              TEXT          NOT NULL DEFAULT '',
    quote_currency    TEXT          NOT NULL DEFAULT '',
    price_nominal     NUMERIC(24,6),
    price_open        NUMERIC(24,6),
    price_highest     NUMERIC(24,6),
    price_lowest      NUMERIC(24,6),
    price_last_close  NUMERIC(24,6),
    price_avg         NUMERIC(24,6),
    change_price      NUMERIC(24,6),
    change_ratio      NUMERIC(24,6),
    price_amplitude   NUMERIC(24,6),
    price_direct      TEXT,
    trade_volumn      NUMERIC(24,6),
    trade_trunover    NUMERIC(24,6),
    volumn_ratio      NUMERIC(24,6),
    bid_price         NUMERIC(24,6),
    ask_price         NUMERIC(24,6),
    bid_vol           NUMERIC(24,6),
    ask_vol           NUMERIC(24,6),
    buysell_ratio     NUMERIC(24,6),
    highest52weeks    NUMERIC(24,6),
    lowest52weeks     NUMERIC(24,6),
    highest_history   NUMERIC(24,6),
    lowest_history    NUMERIC(24,6),
    sec_status        TEXT          NOT NULL DEFAULT '',
    suspension        BOOLEAN       NOT NULL DEFAULT false,
    data_date         TEXT          NOT NULL DEFAULT '',
    dt_update         BIGINT,
    provider          TEXT          NOT NULL DEFAULT 'MMAPI',
    gmt_create        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    gmt_update        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT finv_quote_secu_realtime_crypto_pkey PRIMARY KEY (usc)
);

COMMENT ON TABLE cdeu.finv_quote_secu_realtime_crypto IS
    '加密货币实时快照（moomoo OpenAPI market-snapshot；数据来源为 quote.finv_quote_secu_fav 收藏的加密货币，需求 11314）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.usc IS
    '证券统一编码（对齐 public.finv_security.usc，如 BTCUSD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.code IS
    'moomoo OpenAPI 代码（CC.BTCUSD）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.quote_currency IS
    '计价货币（从 usc 后缀推断，如 USD / HKD / EUR）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.change_ratio IS
    '涨跌幅（百分点数值：1.23 表示 1.23%）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.price_amplitude IS
    '振幅（百分点数值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.buysell_ratio IS
    '买卖比（接口 bid_ask_ratio，百分点数值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.dt_update IS
    '接口 update_time（Unix 毫秒）';
COMMENT ON COLUMN cdeu.finv_quote_secu_realtime_crypto.provider IS
    '数据提供方（固定 MMAPI = moomoo OpenAPI）';

-- ---------------------------------------------------------------------
-- 2. gmt_update 自动维护
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_quote_secu_realtime_crypto_gmt_update
    ON cdeu.finv_quote_secu_realtime_crypto;
CREATE TRIGGER trg_finv_quote_secu_realtime_crypto_gmt_update
    BEFORE UPDATE ON cdeu.finv_quote_secu_realtime_crypto
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 3. BRS 全量备份登记（动态取最小空闲 priority）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_priority int;
    v_inserted int;
BEGIN
    SELECT gs.slot INTO v_priority
      FROM generate_series(10, 1000, 10) AS gs(slot)
     WHERE NOT EXISTS (SELECT 1 FROM brs.brs_bak_rdb_table_full b WHERE b.priority = gs.slot)
     ORDER BY gs.slot ASC
     LIMIT 1;

    IF v_priority IS NULL THEN
        RAISE EXCEPTION 'V300225 备份登记失败：1..1000 内没有空闲 priority';
    END IF;

    INSERT INTO brs.brs_bak_rdb_table_full
        (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
         bak_method, compression, flag_upload, creator, remark)
    VALUES ('postgresql', 'finv', 'cdeu', 'finv_quote_secu_realtime_crypto', '1', v_priority, 'both',
            'pg_dump', 'zstd', '1', 'migration', '加密货币实时快照（moomoo OpenAPI，需求 11314）')
    ON CONFLICT DO NOTHING;

    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RAISE NOTICE 'V300225 备份登记完成：priority=%（本次写入 % 行）', v_priority, v_inserted;
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols int;
    v_pk   int;
    v_trg  int;
    v_bak  int;
BEGIN
    SELECT count(*) INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_realtime_crypto';
    IF v_cols <> 33 THEN
        RAISE EXCEPTION 'V300225 校验失败：列数 = %（期望 33）', v_cols;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conrelid = 'cdeu.finv_quote_secu_realtime_crypto'::regclass AND contype = 'p';
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V300225 校验失败：主键数量 = %（期望 1）', v_pk;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger
     WHERE tgrelid = 'cdeu.finv_quote_secu_realtime_crypto'::regclass
       AND tgname = 'trg_finv_quote_secu_realtime_crypto_gmt_update'
       AND NOT tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300225 校验失败：gmt_update 触发器缺失';
    END IF;

    SELECT count(*) INTO v_bak
      FROM brs.brs_bak_rdb_table_full
     WHERE schema_name = 'cdeu' AND table_name = 'finv_quote_secu_realtime_crypto';
    IF v_bak <> 1 THEN
        RAISE EXCEPTION 'V300225 校验失败：备份登记行数 = %（期望 1）', v_bak;
    END IF;

    RAISE NOTICE 'V300225 完成：cdeu.finv_quote_secu_realtime_crypto（33 列 / 主键 usc / 触发器 / 备份登记）';
END $$;

COMMIT;
