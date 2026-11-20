-- =====================================================================
-- FinvQuant PostgreSQL V300129：修正 3 条美股宽基股的 finv_security.market_code
--
-- 背景：
--   V300068 回填 `finv_security.market_code` 时，BRK.B / EA / MTBLY 属于「无富途采集
--   记录」的历史种子，按人工决策统一归到 1312（注释写的是「美股宽基股」）。但
--   `finv_market` 里 1312 的语义是「美股指数（道琼斯系列）」，对个股是错位编码；
--   且 BRK.B 此后已由富途采集补上 `market_code=10`（NYSE），具备可溯源依据。
--
-- 目标口径（方案 A：按交易所专码，与需求 10945 那批一致）：
--   · 1315 = 美股 NSDAQ交易所（exchange_code 31）
--   · 1316 = 美股 NYSE交易所
--
-- 逐条依据（不编造）：
--   1. BRK.B → 1316（NYSE）
--        `futu.finv_futu_quote_collect` 该 usc 的富途原码 market_code = 10，
--        方案 A 中 10 → 1316；上游快照 `finv_quote_collect_futu_view_20261030`
--        里同一发行人的 BRK.A 记为 region=US / market=NYSE / market_code=10，可交叉印证。
--   2. EA → 1315（NSDAQ）
--        Electronic Arts Inc（艺电）在纳斯达克上市；本库无富途采集记录、
--        映射 CSV 无该证券，故按已知市场归属取值（无富途码可转换这一事实已在
--        V300068 头部记录）。
--   3. MTBLY → 1316（NYSE）⚠ 推断值，发放前请业务确认
--        Moatable Inc（原 Renren Inc，NYSE: RENN）更名并改用现代码，上市地未变，
--        故沿用 NYSE；该推断在库内无富途码可校验，若与实际不符，只改本文件
--        第 3 条目标值后重跑即可（更新为幂等）。
--
-- 不改动：`exchange_code`（三条均已是 30=美国证券市场（全体））、
--   `type_secu_sub` / `provider`（仍为空串，如需补齐另开需求，不在本迁移范围）。
--
-- 幂等：清单入临时表，UPDATE 带 `market_code IS DISTINCT FROM 目标值`，
--   已改过的库重跑影响 0 行；末尾做收敛校验（三条必须等于目标值），不符整事务回滚。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V200019（market_code 列）、V300068（历史回填）。
-- =====================================================================

BEGIN;

CREATE TEMP TABLE _us_broad_stock_mc_fix (
    usc         TEXT    PRIMARY KEY,
    target_mc   INTEGER NOT NULL,
    basis       TEXT    NOT NULL
) ON COMMIT DROP;

INSERT INTO _us_broad_stock_mc_fix (usc, target_mc, basis) VALUES
    ('BRK.B', 1316, 'NYSE：富途原码 market_code=10 → 1316（方案 A）；BRK.A 上游快照 market=NYSE 交叉印证'),
    ('EA',    1315, 'NSDAQ：Electronic Arts 在纳斯达克上市；无富途采集记录，按已知市场归属'),
    ('MTBLY', 1316, '推断：Moatable Inc（原 Renren，NYSE: RENN）更名改码、上市地未变 → NYSE（待业务确认）');

-- ---------------------------------------------------------------------
-- 1. 前置校验：目标 market_code 必须已在 finv_market 登记；三条 usc 必须存在
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad text;
BEGIN
    SELECT string_agg(f.target_mc::text, ',') INTO v_bad
      FROM _us_broad_stock_mc_fix f
     WHERE NOT EXISTS (SELECT 1 FROM public.finv_market m WHERE m.market_code = f.target_mc);
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300129 目标 market_code 未在 finv_market 登记：%', v_bad;
    END IF;

    SELECT string_agg(f.usc, ',') INTO v_bad
      FROM _us_broad_stock_mc_fix f
     WHERE NOT EXISTS (SELECT 1 FROM public.finv_security s WHERE s.usc = f.usc);
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300129 清单内 usc 在 finv_security 不存在：%', v_bad;
    END IF;

    -- 仅允许修「exchange_code = 30 的股票」，防止误伤其它市场的同名 usc
    SELECT string_agg(s.usc, ',') INTO v_bad
      FROM _us_broad_stock_mc_fix f
      JOIN public.finv_security s USING (usc)
     WHERE s.exchange_code <> 30 OR s.security_type <> 'Stock';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300129 清单内存在非「美股股票」行，拒绝执行：%', v_bad;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 修改前快照（写入日志，便于回溯与手工回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT s.usc, s.market_code, f.target_mc
               FROM public.finv_security s JOIN _us_broad_stock_mc_fix f USING (usc)
              ORDER BY s.usc
    LOOP
        RAISE NOTICE 'V300129 修改前：% market_code=%（目标 %）', r.usc, r.market_code, r.target_mc;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 执行修改（幂等）
-- ---------------------------------------------------------------------
UPDATE public.finv_security s
   SET market_code = f.target_mc
  FROM _us_broad_stock_mc_fix f
 WHERE s.usc = f.usc
   AND s.market_code IS DISTINCT FROM f.target_mc;

-- ---------------------------------------------------------------------
-- 4. 收敛校验 + 修改后输出（不符则整事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad text;
    r     record;
BEGIN
    SELECT string_agg(s.usc || '=' || COALESCE(s.market_code::text, 'NULL'), ',') INTO v_bad
      FROM _us_broad_stock_mc_fix f
      JOIN public.finv_security s USING (usc)
     WHERE s.market_code IS DISTINCT FROM f.target_mc;
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300129 收敛校验失败（market_code 未达目标值）：%', v_bad;
    END IF;

    FOR r IN SELECT s.usc, s.market_code, s.security_name_cn
               FROM public.finv_security s JOIN _us_broad_stock_mc_fix f USING (usc)
              ORDER BY s.usc
    LOOP
        RAISE NOTICE 'V300129 修改后：% market_code=%（%）', r.usc, r.market_code, r.security_name_cn;
    END LOOP;
END $$;

COMMIT;
