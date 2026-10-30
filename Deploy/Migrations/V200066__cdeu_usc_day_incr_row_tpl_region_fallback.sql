-- =====================================================================
-- FinvQuant PostgreSQL V200066：日增量行数模板补充「区域级兜底」行（需求 10823）
--
-- 目标表：cdeu.finv_quote_cdeu_usc_day_incr_row_tpl（V100084 创建，V200065 首批 10 行）
--
-- 背景（ACANX 2026-10-30）：
--   服务器实际数据里大量自动发现的 usc 只有 usc + provider，region / market 都为空
--   （finv_market.market_symbol 未维护），按 V200065 的「精确 region + market」模板
--   一个都匹配不上，day_expect_rows 永远补不出来 → 成熟度恒为 PARTIAL、永不导出。
--   因此补一组**区域级兜底模板**（market / category 用空串通配）：
--   只要 region 已知（或仅 category 已知）就能拿到可判定的期望条数。
--
-- 取值依据：
--   · CN / '' / ''        → 241   A 股各市场（SH / SZ / BJ）口径一致，见 V200065
--   · HK / '' / ''        → 331   港股口径（330 分钟 + 1），见 V200065
--   · US / '' / ''        → 390   美股主板连续交易，见 V200065
--   · US / '' / Futures   → 1380  美股期货（COMEX / NYMEX / CME / CBOT）23 小时连续交易；
--                                比 US / '' / '' 多命一个 category 维度，优先命中
--   · FX / '' / ''        → 1380  外汇 24 小时（与 V200065 的 FX/CFD 一致）
--   · GLOBAL / '' / ''    → 1440  GLOBAL 区域下为加密货币/全球品种（7×24 分钟线）
--   · GLOBAL / '' / Forex → 1380  GLOBAL 区域下的外汇对（比 1440 兜底更特异）
--
-- 说明：
--   · 精确行（V200065 的 CN/SH、HK/HKR、US/NSDQ、US/COMEX、FX/CFD 等）仍然优先：
--     匹配排序为「实际命中维度数 → 模板特异性」降序，精确行必然排在这批兜底行之前。
--   · 不加全通配行（region / market / category 全空）：region 与 category 全未知时
--     给任何默认值都是猜测，会把残缺月的成熟度判成 COMPLETE，风险高于收益。
--
-- 幂等：ON CONFLICT DO NOTHING（主键 (region, market, category)），重复执行无副作用，
--       且不覆盖运维已调整的值。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验兜底行齐全。
-- =====================================================================

BEGIN;

INSERT INTO cdeu.finv_quote_cdeu_usc_day_incr_row_tpl
    (region, market, category, day_expect_rows, flag_enable, remark)
VALUES
    ('CN',     '', '',        241, '1', '区域级兜底：A 股（market 未维护时），口径同 CN/SH'),
    ('HK',     '', '',        331, '1', '区域级兜底：港股（market 未维护时），330 分钟 + 1'),
    ('US',     '', '',        390, '1', '区域级兜底：美股（market 未维护时）'),
    ('US',     '', 'Futures', 1380, '1', '美股期货（COMEX/NYMEX/CME/CBOT）23 小时连续交易'),
    ('FX',     '', '',        1380, '1', '区域级兜底：外汇 24 小时'),
    ('GLOBAL', '', '',        1440, '1', '区域级兜底：GLOBAL（加密货币 7×24 分钟线）'),
    ('GLOBAL', '', 'Forex',   1380, '1', 'GLOBAL 区域下的外汇对（比 1440 兜底更特异）')
ON CONFLICT (region, market, category) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：兜底行齐全且取值为预期
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total BIGINT;
    v_cnt   BIGINT;
BEGIN
    SELECT count(*) INTO v_total FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl;
    IF v_total < 17 THEN
        RAISE EXCEPTION 'V200066 收敛失败：模板行数为 %，预期至少 17（V200065 的 10 行 + 本迁移 7 行）', v_total;
    END IF;

    SELECT count(*) INTO v_cnt
      FROM cdeu.finv_quote_cdeu_usc_day_incr_row_tpl t
      JOIN (VALUES
            ('CN',     '', '',        241),
            ('HK',     '', '',        331),
            ('US',     '', '',        390),
            ('US',     '', 'Futures', 1380),
            ('FX',     '', '',        1380),
            ('GLOBAL', '', '',        1440),
            ('GLOBAL', '', 'Forex',   1380)
           ) AS e(region, market, category, day_expect_rows)
        ON t.region = e.region
       AND t.market = e.market
       AND t.category = e.category
       AND t.day_expect_rows = e.day_expect_rows;
    IF v_cnt <> 7 THEN
        RAISE EXCEPTION 'V200066 收敛失败：兜底模板命中 % 条，预期 7', v_cnt;
    END IF;

    RAISE NOTICE 'V200066 完成：模板共 % 行（新增区域级兜底 7 行）', v_total;
END $$;

COMMIT;
