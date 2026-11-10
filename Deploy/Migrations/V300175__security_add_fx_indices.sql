-- =====================================================================
-- FinvQuant PostgreSQL V300175：纳入外汇/货币指数证券（需求 11091）
--
-- 用户 2026-11-10：USDIndex / EURIndex 需要纳入；并追问「另外两个」——
--   上一轮排除的 4 个 forex 候选其实是 `MYRKRW`（马来西亚令吉/韩元）、`NZDTHB`（新西兰元/泰铢）、
--   `THBNZD`（泰铢/新西兰元）、`XNGUSD`（天然气/美元），并不是 USDIndex / EURIndex。
--
-- 现状核对：
--   · `USDIndex`（美元指数）已在 public.finv_security：Forex / type_secu_sub=CIX / market 1253 /
--     exchange 100 / USD / name 美元指数 —— 无需重建；
--   · `EURIndex`（欧元指数）是 V300173 删掉的那批里的 `EURindex`（小写 i），与键表 `FX.EURINDEX → EURIndex`
--     大小写不一致，当时被判为无依据；本次按键表 usc 补回，模板沿用 USDIndex；
--   · 另有 `USDindex`（小写 i、name='USD'、无 provider/remark）是历史垃圾行，**本迁移不动**（另行确认）。
--
-- 本次插入（模板沿用表内同类行的多数写法）：
--   · `EURIndex`：Forex / CIX / market 1253 / exchange 100 / USD（与 USDIndex 对齐）；
--   · 4 个外汇/商品对：Forex / CER / market 1251 / exchange 100（与 USDJPY 等既有 $C{4},000+ 行对齐），
--     币种取交易对左侧货币（MYRKRW→MYR、NZDTHB→NZD、THBNZD→THB、XNGUSD→USD）。
--
-- 幂等：只插不存在的 usc；重跑插入 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

CREATE TEMP TABLE tmp_fx_new ON COMMIT DROP AS
SELECT v.usc,
       v.code,
       v.name,
       v.sec_type,
       v.sub_type,
       v.finv_mc,
       v.exch,
       v.cur,
       v.flag
  FROM (VALUES
        -- usc, code, name, security_type, type_secu_sub, market_code, exchange_code, currency, flag
        ('EURIndex', 'EURINDEX', '欧元指数', 'Forex', 'CIX', 1253, 100, 'USD', '1'),
        ('MYRKRW',   'MYRKRW',   '马来西亚令吉/韩元', 'Forex', 'CER', 1251, 100, 'MYR', '1'),
        ('NZDTHB',   'NZDTHB',   '新西兰元/泰铢',     'Forex', 'CER', 1251, 100, 'NZD', '1'),
        ('THBNZD',   'THBNZD',   '泰铢/新西兰元',     'Forex', 'CER', 1251, 100, 'THB', '1'),
        ('XNGUSD',   'XNGUSD',   '天然气/美元',       'Forex', 'CER', 1251, 100, 'USD', '1')
       ) AS v(usc, code, name, sec_type, sub_type, finv_mc, exch, cur, flag)
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = v.usc);

DO $$
DECLARE
    v_collision BIGINT;
    v_bad       BIGINT;
BEGIN
    SELECT count(*) INTO v_collision
      FROM tmp_fx_new n
     WHERE EXISTS (SELECT 1 FROM public.finv_security s
                    WHERE s.exchange_code = n.exch AND s.security_code = n.code);
    IF v_collision <> 0 THEN
        RAISE EXCEPTION 'V300175 中止：% 行与既有 (exchange_code, security_code) 冲突', v_collision;
    END IF;

    SELECT count(*) INTO v_bad
      FROM tmp_fx_new n
     WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_code = n.finv_mc);
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300175 中止：% 行 market_code 不在 public.finv_market 内', v_bad;
    END IF;
END $$;

INSERT INTO public.finv_security
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, init_date, timezone, tz, flag_enable, market_code,
        type_secu_sub, provider, remark)
SELECT n.usc, n.exch, n.sec_type, n.code, n.name, n.name,
       n.name, n.cur, 20000000, '', '', n.flag, n.finv_mc,
       n.sub_type, 'FTMM', '需求 11091 纳入外汇/货币指数（有采集数据）'
  FROM tmp_fx_new n
ON CONFLICT (usc) DO NOTHING;

DO $$
DECLARE
    v_inserted BIGINT;
    v_rows     BIGINT;
    v_missing  BIGINT;
BEGIN
    SELECT count(*) INTO v_inserted FROM tmp_fx_new;
    SELECT count(*) INTO v_rows FROM public.finv_security;

    -- 有采集数据的 forex 证券必须全部登记
    SELECT count(*) INTO v_missing
      FROM futu.finv_futu_secu_key k
      JOIN futu.finv_futu_quote_collect c ON c.usc = k.usc
     WHERE k.instrument = 'forex'
       AND NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = k.usc);
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V300175 未收敛：仍有 % 个有数据的 forex 证券未登记', v_missing;
    END IF;

    RAISE NOTICE 'V300175 完成：插入 % 行，finv_security 共 % 行（USDIndex 原本已在册，未重复插入）',
        v_inserted, v_rows;
END $$;

COMMIT;
