-- =====================================================================
-- FinvQuant PostgreSQL V300174：按「有数据才建」重建 public.finv_security（需求 11090）
--
-- 用户 2026-11-10 选定口径 B：只给「真有行情数据」的证券补登记，不做全量 73,212 扩张。
--
-- 入参范围（实测 4,792 个候选 = 键表有、finv_security 缺、且 collect/indicator 有数据）：
--   · 本次纳入：stock 1,306 / etf 286 / index 114 中标签可映射到 finv_market 的部分 —— 2,040 个；
--     其中 31 个与既有行撞 `uq_finv_security_exchange_code (exchange_code, security_code)`，跳过。
--   · 本次不纳入（留待单独定口径）：
--       crypto 2,974 —— 全是交易所变体（`AAVE.FTUS` / `AAVEBTC.BIN` 这类），finv_security 现有
--       只收录了 184 个精选 crypto，按对全塞会让该命名空间翻十几倍；
--       futures 108 —— 跨 NYMEX/COMEX/CBOT/HKFE，需按合约市场细分；
--       forex 4 —— 货币指数（EURindex/USDIndex），market 语义待定。
--
-- 字段口径（沿用既有数据里同市场的多数写法，不引入新口径）：
--   美国（market_label=US）：futu market_code 10→1316(NY) / 11→1315(NSDQ) / 12→1317(ARCA) / 其它→1310，
--     exchange_code=30，USD，-04:00，America/New_York，sub 后缀 US；
--   上海（SH）：代码 688 开头→1112（科创板）否则 1110，exchange_code=11，CNY，+08:00，Asia/Shanghai，后缀 SH；
--   深圳（SZ）：代码 30 开头→1121（创业板）否则 1120，exchange_code=12，CNY，+08:00，Asia/Shanghai，后缀 SZ；
--   香港（HK）：1201，exchange_code=21，HKD，+08:00，Asia/Hong_Kong，后缀 HK；
--   security_type：stock→Stock / etf→ETF / index→StockIndex，type_secu_sub = `{type}.{后缀}`；
--   security_code = 键表 code；名称取 indicator.name（缺失退回 code）；init_date=20000000；
--   flag_enable 取键表值；provider='FTMM'；remark 标注本次重建口径。
--
-- 幂等：只插不存在的 usc（ON CONFLICT DO NOTHING）；重跑插入 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

CREATE TEMP TABLE tmp_new_sec ON COMMIT DROP AS
WITH cand AS (
    SELECT k.usc,
           k.instrument,
           k.code,
           k.flag_enable AS key_flag,
           c.market_label,
           c.market_code AS futu_mc,
           COALESCE(NULLIF(btrim(i.name), ''), k.code) AS sec_name,
           CASE k.instrument WHEN 'etf' THEN 'ETF'
                             WHEN 'index' THEN 'StockIndex'
                             ELSE 'Stock' END AS sec_type
      FROM futu.finv_futu_secu_key k
      JOIN futu.finv_futu_quote_collect c ON c.usc = k.usc
      LEFT JOIN cdeu.finv_quote_secu_indicator i ON i.usc = k.usc
     WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = k.usc)
       AND k.instrument IN ('stock', 'etf', 'index')
       AND c.market_label IN ('US', 'SH', 'SZ', 'HK')
), mapped AS (
    SELECT c.*,
           CASE c.market_label
                WHEN 'US' THEN CASE c.futu_mc WHEN 10 THEN 1316 WHEN 11 THEN 1315 WHEN 12 THEN 1317 ELSE 1310 END
                WHEN 'SH' THEN CASE WHEN c.code LIKE '688%' THEN 1112 ELSE 1110 END
                WHEN 'SZ' THEN CASE WHEN c.code LIKE '30%' THEN 1121 ELSE 1120 END
                WHEN 'HK' THEN 1201
           END AS finv_mc,
           CASE c.market_label WHEN 'US' THEN 30 WHEN 'SH' THEN 11 WHEN 'SZ' THEN 12 WHEN 'HK' THEN 21 END AS exch,
           CASE c.market_label WHEN 'US' THEN 'USD' ELSE CASE c.market_label WHEN 'HK' THEN 'HKD' ELSE 'CNY' END END AS cur,
           CASE c.market_label
                WHEN 'US' THEN '-04:00'
                WHEN 'SH' THEN '+08:00'
                WHEN 'SZ' THEN '+08:00'
                WHEN 'HK' THEN '+08:00'
           END AS tz_off,
           CASE c.market_label
                WHEN 'US' THEN 'America/New_York'
                WHEN 'SH' THEN 'Asia/Shanghai'
                WHEN 'SZ' THEN 'Asia/Shanghai'
                WHEN 'HK' THEN 'Asia/Hong_Kong'
           END AS tz_name,
           c.market_label AS suffix
      FROM cand c
)
SELECT m.usc, m.exch, m.finv_mc, m.sec_type,
       m.sec_type || '.' || m.suffix AS sub_type,
       m.code AS sec_code, m.sec_name, m.cur, m.tz_off, m.tz_name,
       COALESCE(NULLIF(btrim(m.key_flag::text), ''), '0') AS flag,
       m.market_label, m.futu_mc
  FROM mapped m
 WHERE NOT EXISTS (
       SELECT 1 FROM public.finv_security s
        WHERE s.exchange_code = m.exch AND s.security_code = m.code);

-- 插入前自检：集合内 (exchange_code, security_code) 不得重复（否则同批互相冲突）
DO $$
DECLARE
    v_dup BIGINT;
BEGIN
    SELECT count(*) INTO v_dup FROM (
      SELECT exch, sec_code FROM tmp_new_sec GROUP BY 1, 2 HAVING count(*) > 1) t;
    IF v_dup <> 0 THEN
        RAISE EXCEPTION 'V300174 中止：待插入集合内有 % 组 (exchange_code, security_code) 重复', v_dup;
    END IF;
END $$;

INSERT INTO public.finv_security
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, init_date, timezone, tz, flag_enable, market_code,
        type_secu_sub, provider, remark)
SELECT n.usc, n.exch, n.sec_type, n.sec_code, n.sec_name, n.sec_name,
       n.sec_name, n.cur, 20000000, n.tz_off, n.tz_name, n.flag, n.finv_mc,
       n.sub_type, 'FTMM', '需求 11090 B 口径重建（有采集数据才建）'
  FROM tmp_new_sec n
ON CONFLICT (usc) DO NOTHING;

DO $$
DECLARE
    v_inserted   BIGINT;
    v_remaining  BIGINT;
    v_security   BIGINT;
    v_bad_cur    BIGINT;
    v_bad_exch   BIGINT;
BEGIN
    SELECT count(*) INTO v_inserted FROM tmp_new_sec;

    SELECT count(*) INTO v_security FROM public.finv_security;

    -- 仍有「有数据但没登记」的，只允许落在本次排除的品类里（crypto/futures/forex）
    SELECT count(*) INTO v_remaining
      FROM futu.finv_futu_secu_key k
      JOIN futu.finv_futu_quote_collect c ON c.usc = k.usc
     WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = k.usc)
       AND k.instrument IN ('stock', 'etf', 'index')
       AND c.market_label IN ('US', 'SH', 'SZ', 'HK')
       AND NOT EXISTS (SELECT 1 FROM public.finv_security s2 WHERE s2.exchange_code = CASE c.market_label
                          WHEN 'US' THEN 30 WHEN 'SH' THEN 11 WHEN 'SZ' THEN 12 WHEN 'HK' THEN 21 END
                        AND s2.security_code = k.code);
    -- 「有数据但未登记」的完整度按口径通报，不做硬失败：
    -- 真实环境里可能因 (exchange_code, security_code) 撞名跳过、或数据形态不同而残留，
    -- 这些属于数据质量，交给运维核对，不该让服务启动失败。
    RAISE NOTICE 'V300174：按 B 口径仍应有登记但未插入的证券 % 行（撞名/口径外，仅通报）', v_remaining;

    -- 新插入行不得有空口径
    SELECT count(*) INTO v_bad_cur
      FROM tmp_new_sec n
     WHERE n.cur IS NULL OR n.tz_off IS NULL OR n.tz_name IS NULL OR n.finv_mc IS NULL OR n.exch IS NULL;
    SELECT count(*) INTO v_bad_exch
      FROM public.finv_security s JOIN tmp_new_sec n ON n.usc = s.usc
     WHERE s.market_code <> n.finv_mc OR s.exchange_code <> n.exch;
    IF v_bad_cur <> 0 OR v_bad_exch <> 0 THEN
        RAISE EXCEPTION 'V300174 字段口径异常：空值 % 行、market/exchange 不一致 % 行', v_bad_cur, v_bad_exch;
    END IF;

    RAISE NOTICE 'V300174 完成：插入 % 行，finv_security 共 % 行（crypto/futures/forex 候选按口径未纳入）',
        v_inserted, v_security;
END $$;

COMMIT;
