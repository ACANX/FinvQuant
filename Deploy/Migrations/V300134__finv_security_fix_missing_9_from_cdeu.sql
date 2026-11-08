-- =====================================================================
-- FinvQuant PostgreSQL V300134：按 stock_id 从 cdeu 快照就地补出两表记录（需求 11033）
--
-- 背景：
--   线上扫描给出 9 个 stock_id，两表（public.finv_security / futu.finv_futu_quote_collect）
--   都没有对应记录。逐条核对本地与上游后的结论：
--     · 87660282761853（Bitwise XRP ETF）：上游 `finv_quote_secu_meta` 有记录，但其 `usc` 为
--       `XRP`，与既有「XRP 加密币」（finv_security.usc='XRP', market_code=10001, Crypto）
--       同名不同标的 → **不可写入**（本迁移用 usc 存在性守卫自动跳过，不会污染主键）；
--     · 其余 8 个（88776974262392 / 89124866614294 / 89077621973905 / 88356067465977 /
--       88716844720087 / 88441966812179 / 88983132693127 等）：Supabase 视图
--       `finv_quote_secu_meta`、`finv_quote_collect_futu_view`、本地 20261030 快照、
--       本地 futu 映射表全部查不到 → 唯一可用来源是**线上自己的 cdeu 快照**
--       （它们正是从 `cdeu.finv_cdeu_secu_realtime_stock` 扫描出来的）。
--
--   因此本迁移不用外部数据，直接从 `cdeu.finv_cdeu_secu_realtime_stock` 按 stock_id 派生：
--   线上 cdeu 有这几行就补出两表记录；没有则本迁移影响 0 行（安全、可重复执行）。
--
-- 口径（cdeu 缺交易所码/英文名，故按最保守方式落库）：
--   · `finv_security.market_code = 0`（未维护）、`futu.market_code = 0`（未维护）；
--   · `exchange_code` / `market_type` / `currency_type` 按 `market_label` 换算（US/HK/SH/SZ/BJ），
--     未登记市场分别落 100 / 0 / ''（与 V300068 的兜底桶口径一致）；
--   · `security_name = security_name_cn = cdeu.name`（cdeu 只有中文名）；
--   · `flag_enable='0'`（待人工启用，与其它批次一致）、`provider='FTMM'`；
--   · `type_secu_sub` 留空串（cdeu 无该字段）；`type_symbol` 按 `instrument_type` 换算。
--
-- 守卫（关键）：
--   两条 INSERT 都带「usc 在 finv_security 与目标表里都不存在」的守卫，
--   因此 **usc 已被其它证券占用的行会自动跳过**（如 XRP），不会覆盖也不会报错。
--
-- 幂等：`WHERE NOT EXISTS` + `ON CONFLICT DO NOTHING`；重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾打印本批次落库行数。
-- 前置：V200082（finv_security 扩列）、V200083（futu_futu_quote_collect 建表）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 目标 stock_id 清单（线上扫描给出的 9 个）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE _todo_stock_id (
    stock_id BIGINT PRIMARY KEY
) ON COMMIT DROP;

INSERT INTO _todo_stock_id (stock_id) VALUES
    (88776974262392),
    (89124866614294),
    (89077621973905),
    (88356067465977),
    (88716844720087),
    (88441966812179),
    (87660282761853),
    (88983132693137),
    (88321707727523);

-- ---------------------------------------------------------------------
-- 1. 前置校验：结构齐全（缺列直接回滚，避免写半截数据）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('cdeu.finv_cdeu_secu_realtime_stock') IS NULL THEN
        RAISE EXCEPTION 'V300134 缺少 cdeu.finv_cdeu_secu_realtime_stock';
    END IF;
    IF to_regclass('futu.finv_futu_quote_collect') IS NULL THEN
        RAISE EXCEPTION 'V300134 缺少 futu.finv_futu_quote_collect（需 V200083）';
    END IF;
    IF (SELECT count(*) FROM information_schema.columns
         WHERE table_schema = 'public' AND table_name = 'finv_security'
           AND column_name IN ('usc','security_name_full','provider','remark')) < 4 THEN
        RAISE EXCEPTION 'V300134 finv_security 缺列（需 V200005 + V200082）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 公共来源视图：cdeu 快照中这批 stock_id 的行 + 派生字段
-- ---------------------------------------------------------------------
CREATE TEMP TABLE _src ON COMMIT DROP AS
SELECT c.stock_id::bigint                                  AS stock_id,
       btrim(c.usc)                                        AS usc,
       btrim(c.stock_code)                                 AS stock_code,
       btrim(c.name)                                       AS name,
       upper(btrim(c.market_label))                        AS market_label,
       btrim(c.instrument_type::text)::integer             AS instrument_type,
       CASE upper(btrim(c.market_label))
            WHEN 'US' THEN 30
            WHEN 'HK' THEN 21
            WHEN 'SH' THEN 11
            WHEN 'SZ' THEN 12
            WHEN 'BJ' THEN 13
            ELSE 100
       END                                                 AS exchange_code,
       CASE upper(btrim(c.market_label))
            WHEN 'US' THEN 2
            WHEN 'HK' THEN 1
            WHEN 'SH' THEN 4
            WHEN 'SZ' THEN 4
            WHEN 'BJ' THEN 4
            ELSE 0
       END                                                 AS market_type,
       CASE upper(btrim(c.market_label))
            WHEN 'US' THEN 'USD'
            WHEN 'HK' THEN 'HKD'
            WHEN 'SH' THEN 'CNY'
            WHEN 'SZ' THEN 'CNY'
            WHEN 'BJ' THEN 'CNY'
            ELSE ''
       END                                                 AS currency_type,
       CASE btrim(c.instrument_type::text)
            WHEN '3'  THEN 'Stock'
            WHEN '4'  THEN 'ETF'
            WHEN '12' THEN 'Stock'
            ELSE 'Stock'
       END                                                 AS security_type,
       CASE btrim(c.instrument_type::text)
            WHEN '3'  THEN '3002'
            WHEN '4'  THEN '4002'
            WHEN '12' THEN '3002'
            ELSE '3002'
       END                                                 AS sub_instrument_type,
       CASE btrim(c.instrument_type::text)
            WHEN '3'  THEN 'stock'
            WHEN '4'  THEN 'etfs'
            WHEN '12' THEN 'stock'
            ELSE 'stock'
       END                                                 AS type_symbol
  FROM cdeu.finv_cdeu_secu_realtime_stock c
  JOIN _todo_stock_id t ON t.stock_id = c.stock_id::bigint
 WHERE c.usc IS NOT NULL AND btrim(c.usc) <> '';

DO $$
DECLARE
    v_src    int;
    v_exists int;
BEGIN
    SELECT count(*) INTO v_src FROM _src;
    SELECT count(*) INTO v_exists
      FROM _src s
     WHERE EXISTS (SELECT 1 FROM public.finv_security x WHERE x.usc = s.usc)
        OR EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect y WHERE y.usc = s.usc);
    RAISE NOTICE 'V300134 待处理 % 个 stock_id：cdeu 命中 % 行，其中 % 行的 usc 已被既有证券占用（将跳过）', 9, v_src, v_exists;
END $$;

-- ---------------------------------------------------------------------
-- 3. 补 futu.finv_futu_quote_collect
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_quote_collect
    (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
     sub_instrument_type, req_section, quote_market, type_symbol, flag_enable, remark)
SELECT s.usc, s.stock_id, s.stock_code, s.market_type, 0, s.instrument_type,
       s.sub_instrument_type::integer, NULL, s.market_label, s.type_symbol, '1',
       '20261109-线上缺失证券补齐-第二批'
  FROM _src s
 WHERE NOT EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect f WHERE f.usc = s.usc)
   -- usc 是全局主键：若已被既有证券（如 XRP 加密币）占用，两表都不得写入，避免同名不同标的
   AND NOT EXISTS (SELECT 1 FROM public.finv_security x WHERE x.usc = s.usc)
 ORDER BY s.usc
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 4. 补 public.finv_security
-- ---------------------------------------------------------------------
INSERT INTO public.finv_security
    (usc, exchange_code, market_code, security_type, type_secu_sub, security_code,
     security_name, security_name_cn, security_name_full, currency_type, init_date,
     timezone, tz, flag_enable, provider, remark)
SELECT s.usc, s.exchange_code, 0, s.security_type, '', s.stock_code,
       s.name, s.name, s.name, s.currency_type, 20000000,
       CASE s.market_label WHEN 'US' THEN '-04:00' WHEN 'HK' THEN '+08:00'
                           WHEN 'SH' THEN '+08:00' WHEN 'SZ' THEN '+08:00'
                           WHEN 'BJ' THEN '+08:00' ELSE NULL END,
       CASE s.market_label WHEN 'US' THEN 'America/New_York' WHEN 'HK' THEN 'Asia/Hong_Kong'
                           WHEN 'SH' THEN 'Asia/Shanghai' WHEN 'SZ' THEN 'Asia/Shanghai'
                           WHEN 'BJ' THEN 'Asia/Shanghai' ELSE NULL END,
       '0', 'FTMM', '20261109-线上缺失证券补齐-第二批'
  FROM _src s
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_security x WHERE x.usc = s.usc)
 ORDER BY s.usc
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 5. 收敛说明 + 未补出的清单（供线上核对）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
    v_secu int;
    v_futu int;
BEGIN
    SELECT count(*) INTO v_secu FROM public.finv_security
     WHERE remark = '20261109-线上缺失证券补齐-第二批';
    SELECT count(*) INTO v_futu FROM futu.finv_futu_quote_collect
     WHERE remark = '20261109-线上缺失证券补齐-第二批';
    RAISE NOTICE 'V300134 本批次落库：finv_security=% 行，futu_futu_quote_collect=% 行', v_secu, v_futu;

    FOR r IN SELECT t.stock_id,
                    (SELECT s.usc FROM _src s WHERE s.stock_id = t.stock_id)   AS usc,
                    (SELECT count(*) FROM _src s WHERE s.stock_id = t.stock_id) AS in_cdeu
               FROM _todo_stock_id t
              ORDER BY t.stock_id
    LOOP
        IF r.in_cdeu = 0 THEN
            RAISE NOTICE 'V300134 未处理 stock_id=%：cdeu 快照中无此行（需另行采集）', r.stock_id;
        ELSIF EXISTS (SELECT 1 FROM public.finv_security x WHERE x.usc = r.usc AND x.remark <> '20261109-线上缺失证券补齐-第二批') THEN
            RAISE NOTICE 'V300134 未处理 stock_id=%：usc=% 已被其它证券占用（同名不同标的，需 usc 归一化）', r.stock_id, r.usc;
        END IF;
    END LOOP;
END $$;

COMMIT;
