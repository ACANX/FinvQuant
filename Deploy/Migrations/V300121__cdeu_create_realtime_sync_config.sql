-- =====================================================================
-- V300121：创建 cdeu.finv_cdeu_realtime_sync_config（实时行情列表采集配置）
--
-- 背景：把原先散落在 Python 采集脚本常量与 Stat.json 里的配置收进一张表：
--   每个「市场 + 证券品种」一行，网页菜单点击采集时读配置 → 分页拉
--   moomoo get-stock-list → 结果写入 cdeu.finv_cdeu_secu_realtime_stock。
--
-- 动态回写：采集成功后把接口 pagination.total 写进 count_exists，
--   并把本轮实际可采集数量写进 total_available（= min(count_plan, count_exists)）。
--
-- 幂等：CREATE TABLE IF NOT EXISTS + seed 用 ON CONFLICT DO NOTHING；
--   末尾校验列数/注释数/初始行数/主键，不符回滚。
-- =====================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS cdeu.finv_cdeu_realtime_sync_config (
    instrument_symbol TEXT        NOT NULL DEFAULT 'stock',
    market_type       TEXT        NOT NULL,
    count_plan        INTEGER,
    count_exists      INTEGER,
    total_available   INTEGER,
    flag_enable       CHAR(1)     NOT NULL DEFAULT '1',
    remark            TEXT,
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT finv_cdeu_realtime_sync_config_pk PRIMARY KEY (market_type, instrument_symbol),
    CONSTRAINT finv_cdeu_realtime_sync_config_enable_chk CHECK (flag_enable IN ('0', '1'))
);

COMMENT ON TABLE cdeu.finv_cdeu_realtime_sync_config
    IS '实时行情列表采集配置：一行=一个「市场 + 证券品种」，含采集计划数量与采集后回写的市场证券总数';

COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.instrument_symbol
    IS '证券品种符号：stock（股票）/ etf / future，供后续扩展采集其它品种';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.market_type
    IS '市场类型编码（接口 marketType 原值）：1=HK 港股、2=US 美股、4=CN A股';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.count_plan
    IS '采集计划数量（计划采集排名前 N 只）；NULL 表示按 count_exists 全量采集';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.count_exists
    IS '该市场当前存在的证券总数（接口 pagination.total，随行情动态变化）；采集成功后回写';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.total_available
    IS '本轮实际可采集数量 = min(count_plan, count_exists)；count_plan 为 NULL 时等于 count_exists';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.flag_enable
    IS '是否参与采集：1=参与、0=跳过';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.remark
    IS '备注：该市场采集说明（接口、板块类型、排行口径、计划范围与回写说明）';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.gmt_create
    IS '创建时间';
COMMENT ON COLUMN cdeu.finv_cdeu_realtime_sync_config.gmt_update
    IS '更新时间';

INSERT INTO cdeu.finv_cdeu_realtime_sync_config
    (instrument_symbol, market_type, count_plan, remark)
VALUES
    ('stock', '1', 2500,
     'HK 港股（marketType=1）：调用 moomoo get-stock-list 分页采集港股股票列表，板块类型 plateType=1、按市值排行 rankType=3，'
     '计划采集前 2500 只；采集完成后回写 count_exists（接口 pagination.total）与 total_available。'),
    ('stock', '2', 6000,
     'US 美股（marketType=2）：调用 moomoo get-stock-list 分页采集美股股票列表，plateType=1、按市值排行 rankType=3，'
     '计划采集前 6000 只；采集完成后回写 count_exists 与 total_available。'),
    ('stock', '4', 4500,
     'CN A股（marketType=4）：调用 moomoo get-stock-list 分页采集 A 股股票列表（含沪市、深市、北交所），plateType=1、'
     '按市值排行 rankType=3，计划采集前 4500 只；采集完成后回写 count_exists 与 total_available。')
ON CONFLICT (market_type, instrument_symbol) DO NOTHING;

DO $$
DECLARE
    v_cols  int;
    v_cmt   int;
    v_rows  int;
    v_pk    int;
BEGIN
    SELECT count(*) INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_cdeu_realtime_sync_config';
    IF v_cols <> 9 THEN
        RAISE EXCEPTION 'V300121 列数不符：期望 9，实际 %', v_cols;
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_cdeu_realtime_sync_config';
    IF v_cmt <> 10 THEN
        RAISE EXCEPTION 'V300121 注释数不符：期望 10（表 + 9 列），实际 %', v_cmt;
    END IF;

    SELECT count(*) INTO v_rows FROM cdeu.finv_cdeu_realtime_sync_config;
    IF v_rows < 3 THEN
        RAISE EXCEPTION 'V300121 初始配置行数不足：期望 ≥ 3，实际 %', v_rows;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conname = 'finv_cdeu_realtime_sync_config_pk'
       AND conrelid = 'cdeu.finv_cdeu_realtime_sync_config'::regclass;
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V300121 主键不符：期望 1 个，实际 %', v_pk;
    END IF;

    RAISE NOTICE 'V300121 校验通过：9 列 / 10 条注释 / 配置行 % / 主键 (market_type, instrument_symbol)', v_rows;
END $$;

COMMIT;
