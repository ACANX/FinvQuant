-- =====================================================================
-- FinvQuant PostgreSQL V200065：日增量行数模板初始数据（需求 10820）
--
-- 目标表：cdeu.finv_quote_cdeu_usc_day_incr_row_tpl（V100084 创建）
--
-- 取值依据（逐行给出，避免凭空编造）：
--   · CN / SH、CN / SZ、CN / BJ → 241
--     —— A 股每日 240 分钟 + 1 根收盘 bar，口径见 V100078 对
--        cdeu.finv_quote_cdeu_usc.day_expect_rows 的列注释「A 股 241」。
--   · HK / HK、HK / HKR → 331
--     —— 港交所交易时段 09:30–12:00（150 分钟）+ 13:00–16:00（180 分钟）= 330 分钟，
--        按与 A 股相同的「首尾各含一根」口径 +1 = 331。HKR 为人民币柜台，交易时段一致。
--        说明：若上游分钟源在 12:00 / 13:00 各出一根导致实际为 332，
--        在成熟度阈值 0.95 下两种取值都能判定 COMPLETE（331×0.95≈314.5），
--        运维可直接改本表对应行的值，无需改代码。
--   · US / NSDQ → 390
--     —— 生产库既有数据（usc=NVDA，day_expect_rows=390）。
--   · US / PINK → 390
--     —— ADR 与美股主板同为 09:30–16:00 连续交易，与 NSDQ 同口径。
--   · US / COMEX → 1380
--     —— 生产库既有数据（usc=GCMain，day_expect_rows=1380；23 小时连续交易）。
--   · FX / CFD → 1380
--     —— 生产库既有数据（usc=XAUUSD，day_expect_rows=1380）。
--   · GLOBAL / * / Crypto → 1440
--     —— 加密货币 7×24 分钟线，口径见 V100078 列注释「加密货币 1440」；
--        上游市场符号未统一，故 market 用空串通配、按 category 匹配。
--
-- 未预置（待业务确认后追加或由运维在本表维护）：
--   · CN 以外的其它区域市场、美股其它交易所专属口径、港股期权/窝轮等特殊品种。
--
-- 幂等：ON CONFLICT (region, market, category) DO NOTHING ——
-- 重复执行不影响，且不覆盖运维已在本表调整过的值。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验模板行数。
-- =====================================================================

BEGIN;

INSERT INTO cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
    (region, market, category, day_expect_rows, flag_enable, remark)
VALUES
    ('CN',     'SH',    '',       241, '1', 'A 股 240 分钟 + 1（V100078 列注释口径）'),
    ('CN',     'SZ',    '',       241, '1', 'A 股 240 分钟 + 1（V100078 列注释口径）'),
    ('CN',     'BJ',    '',       241, '1', 'A 股 240 分钟 + 1（V100078 列注释口径）'),
    ('HK',     'HK',    '',       331, '1', '港股 330 分钟 + 1（09:30-12:00 + 13:00-16:00）'),
    ('HK',     'HKR',   '',       331, '1', '港股人民币柜台，交易时段与港股主板一致'),
    ('US',     'NSDQ',  '',       390, '1', '生产库既有数据（NVDA=390）'),
    ('US',     'PINK',  '',       390, '1', '美股 ADR 与主板同为连续交易 390'),
    ('US',     'COMEX', '',      1380, '1', '生产库既有数据（GCMain=1380，23 小时连续交易）'),
    ('FX',     'CFD',   '',      1380, '1', '生产库既有数据（XAUUSD=1380）'),
    ('GLOBAL', '',      'Crypto', 1440, '1', '加密货币 7×24 分钟线（V100078 列注释口径）；market 通配')
ON CONFLICT (region, market, category) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：模板行数与关键行取值
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total BIGINT;
    v_cnt   BIGINT;
BEGIN
    SELECT count(*) INTO v_total
      FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl;
    IF v_total < 10 THEN
        RAISE EXCEPTION 'V200065 收敛失败：模板行数为 %，预期至少 10', v_total;
    END IF;

    SELECT count(*) INTO v_cnt
      FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl t
      JOIN (VALUES
            ('CN',     'SH',    '',       241),
            ('CN',     'SZ',    '',       241),
            ('CN',     'BJ',    '',       241),
            ('HK',     'HK',    '',       331),
            ('HK',     'HKR',   '',       331),
            ('US',     'NSDQ',  '',       390),
            ('US',     'PINK',  '',       390),
            ('US',     'COMEX', '',      1380),
            ('FX',     'CFD',   '',      1380),
            ('GLOBAL', '',      'Crypto', 1440)
           ) AS e(region, market, category, day_expect_rows)
        ON t.region = e.region
       AND t.market = e.market
       AND t.category = e.category
       AND t.day_expect_rows = e.day_expect_rows;
    IF v_cnt <> 10 THEN
        RAISE EXCEPTION 'V200065 收敛失败：关键模板行命中 % 条，预期 10', v_cnt;
    END IF;

    RAISE NOTICE 'V200065 完成：日增量行数模板共 % 行（关键行 10 条就绪）', v_total;
END $$;

COMMIT;
