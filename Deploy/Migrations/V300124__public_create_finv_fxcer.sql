-- =====================================================================
-- V300124：新建 public.finv_fxcer（缺省汇率表，现行唯一写入目标）
--
-- 列口径（需求 11002）：
--   · usc    —— 上游标识：get-fx-rate-list 的 stockCode / get-fx-overview 的 map key；
--   · name   —— 展示名称：列表接口返回「美元/日元」这类中文名，概览接口按币种中文名合成；
--   · rate / ts_quote / source 为缺省汇率、报价时间（Unix 毫秒）与来源标识。
--
-- 说明：早期版本（V300123）建过一张 public.finv_cer 并已停写；该脚本与对应文档已在
--   需求 11021 从版本库移除，本文件是该表唯一来源。
--
-- 数据口径：
--   · 一行 = 一个货币对的最新缺省汇率，主键 usc；
--   · rate = 1 单位 currency_src 折合的 currency_dest 数量；
--   · 两个来源（overview / rateList，需求 11001）写同一张表、互不删除；
--     同一来源内按 ts_quote 只前进，跨来源以最后一次采集为准（两个接口的时钟不可比）；
--   · CNY 由 CNH 同价派生（flag_derive='1'），派生行不得覆盖直采行。
--
-- 幂等：CREATE TABLE IF NOT EXISTS、触发器先 DROP 再建、COMMENT 可重复执行；
--   末尾校验列清单/主键/CHECK 数/触发器/列注释数，任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 缺省汇率表 v2
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.finv_fxcer (
    usc           TEXT            NOT NULL,
    name          TEXT            NOT NULL DEFAULT '',
    currency_src  TEXT            NOT NULL,
    currency_dest TEXT            NOT NULL,
    rate          NUMERIC(24,10)  NOT NULL,
    ts_quote      BIGINT          NOT NULL,
    source        TEXT            NOT NULL,
    flag_derive   CHAR(1)         NOT NULL DEFAULT '0',
    remark        TEXT,
    gmt_create    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    gmt_update    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT finv_fxcer_pk         PRIMARY KEY (usc),
    CONSTRAINT finv_fxcer_usc_chk    CHECK (usc ~ '^[A-Z]{6}$'),
    CONSTRAINT finv_fxcer_src_chk    CHECK (currency_src  ~ '^[A-Z]{3}$'),
    CONSTRAINT finv_fxcer_dest_chk   CHECK (currency_dest ~ '^[A-Z]{3}$'),
    CONSTRAINT finv_fxcer_diff_chk   CHECK (currency_src <> currency_dest),
    CONSTRAINT finv_fxcer_rate_chk   CHECK (rate > 0),
    CONSTRAINT finv_fxcer_time_chk   CHECK (ts_quote > 0),
    CONSTRAINT finv_fxcer_derive_chk CHECK (flag_derive IN ('0', '1'))
);

-- gmt_update 自动维护（复用 public.vq_set_gmt_update，避免重建函数）
DROP TRIGGER IF EXISTS trg_finv_fxcer_gmt_update ON public.finv_fxcer;
CREATE TRIGGER trg_finv_fxcer_gmt_update
    BEFORE UPDATE ON public.finv_fxcer
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 2. 表与列注释
-- ---------------------------------------------------------------------
COMMENT ON TABLE public.finv_fxcer IS
    '缺省汇率表 v2（V300124）：一行=一个货币对的最新汇率，主键 usc（上游标识），供多货币金额统一换算；只保最新';

COMMENT ON COLUMN public.finv_fxcer.usc IS
    '上游标识：get-fx-rate-list 的 stockCode、get-fx-overview 的 map key；派生行为「源币种+目标币种」（如 CNYUSD）';
COMMENT ON COLUMN public.finv_fxcer.name IS
    '展示名称（简体中文）：列表接口原样使用其 name（如「美元/日元」），概览与派生行按币种中文名合成（如「美元/离岸人民币」）';
COMMENT ON COLUMN public.finv_fxcer.currency_src IS
    '源币种（ISO 4217 alpha-3 大写；离岸人民币 CNH、在岸人民币 CNY）';
COMMENT ON COLUMN public.finv_fxcer.currency_dest IS
    '目标币种（ISO 4217 alpha-3 大写）';
COMMENT ON COLUMN public.finv_fxcer.rate IS
    '缺省汇率：1 单位 currency_src 折合的 currency_dest 数量（USDCNH=6.67333 即 1 USD = 6.67333 CNH）';
COMMENT ON COLUMN public.finv_fxcer.ts_quote IS
    '报价时间（Unix 毫秒）：overview 取接口逐对 time；rateList 接口不返回时间，取本次采集时刻';
COMMENT ON COLUMN public.finv_fxcer.source IS
    '数据来源：moomoo.get-fx-overview / moomoo.get-fx-rate-list / derive:CNH（CNY 派生）';
COMMENT ON COLUMN public.finv_fxcer.flag_derive IS
    '是否派生行：0=上游直采、1=派生（CNY 代 CNH）；派生行不得覆盖直采行';
COMMENT ON COLUMN public.finv_fxcer.remark IS
    '备注：派生依据与口径说明';
COMMENT ON COLUMN public.finv_fxcer.gmt_create IS
    '创建时间';
COMMENT ON COLUMN public.finv_fxcer.gmt_update IS
    '更新时间（触发器维护；同一轮采集各行基本一致，可作批次时间）';

-- ---------------------------------------------------------------------
-- 3. 校验：结构不符则整事务回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols text;
    v_want CONSTANT text := 'usc,name,currency_src,currency_dest,rate,ts_quote,source,'
                            'flag_derive,remark,gmt_create,gmt_update';
    v_pk   text;
    v_chk  int;
    v_trg  int;
    v_cmt  int;
BEGIN
    SELECT string_agg(column_name, ',' ORDER BY ordinal_position)
      INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'public' AND table_name = 'finv_fxcer';
    IF v_cols IS DISTINCT FROM v_want THEN
        RAISE EXCEPTION 'V300124 列结构不符：期望 %，实际 %', v_want, COALESCE(v_cols, '<表不存在>');
    END IF;

    SELECT string_agg(a.attname, ',' ORDER BY k.ord)
      INTO v_pk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
      CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
      JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer'
       AND c.contype = 'p';
    IF v_pk IS DISTINCT FROM 'usc' THEN
        RAISE EXCEPTION 'V300124 主键不符：期望 usc，实际 %', COALESCE(v_pk, '<无主键>');
    END IF;

    SELECT count(*) INTO v_chk
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer' AND c.contype = 'c';
    IF v_chk <> 7 THEN
        RAISE EXCEPTION 'V300124 CHECK 约束数不符：期望 7，实际 %', v_chk;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer'
       AND tg.tgname = 'trg_finv_fxcer_gmt_update'
       AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300124 缺少 gmt_update 触发器';
    END IF;

    SELECT count(*) INTO v_cmt
      FROM pg_description d
      JOIN pg_class t ON t.oid = d.objoid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'public' AND t.relname = 'finv_fxcer'
       AND d.objsubid > 0;
    IF v_cmt <> 11 THEN
        RAISE EXCEPTION 'V300124 列注释数量不符：期望 11，实际 %', v_cmt;
    END IF;

    RAISE NOTICE 'V300124 校验通过：11 列 / 主键(usc) / 7 CHECK / 触发器 / 列注释齐全';
END $$;

COMMIT;
