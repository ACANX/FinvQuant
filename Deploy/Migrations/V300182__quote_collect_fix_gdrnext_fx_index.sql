-- =====================================================================
-- FinvQuant PostgreSQL V300182：修正采集属性表三条 GDR / 外汇指数的键与 usc（需求 11144）
--
-- 背景（用户 2026-11-12 要求）：
--   以下三条语句执行会报错 `column "ftmm_symbol" can only be updated to DEFAULT`——
--   `futu.finv_futu_quote_collect.ftmm_symbol` 是**生成列**：
--       ftmm_symbol = market_label || '.' || futu_symbol
--   生成列不可写，要改它必须改基列 `market_label` / `futu_symbol`，生成值自然跟着变。
--
-- 目标状态（三条）：
--
--   | ftmm_symbol    | usc      | 原 usc（待纠正的大小写变体） |
--   |----------------|----------|------------------------------|
--   | HK.GDRNEXT     | GDRNext  | GDRnext                      |
--   | FX.EURINDEX    | EURIndex | FX.EURindex                  |
--   | FX.USDINDEX    | USDIndex | FX.USDindex                  |
--
-- 匹配口径：按 `upper(futu_symbol)` + `upper(market_label)` 定位，compatible 任意大小写变体，
--   因此同一脚本在「本地库（只有 FX.USDINDEX 且已正确）」与「有 GDRnext / FX.EURindex 的库」都能跑。
--
-- 安全设计（逐条都先查后改）：
--   1. 目标 `usc` 是主键：若已被**另一行**（`futu_symbol` 不同）占用 → 直接 RAISE EXCEPTION，交人工裁决；
--   2. 唯一键 `(market_code, futu_symbol)`：若同一市场已有大写的同符号行，会命中多行 → 同样中止；
--   3. 幂等：`WHERE ... IS DISTINCT FROM (...) `，已正确的行不更新；复跑影响 0 行；
--   4. 单事务；末尾只对「匹配到的行是否已全部到位」做结构性断言，行数类信息一律 `RAISE NOTICE`。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    r          RECORD;
    v_hits     BIGINT;
    v_conflict TEXT;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('HK', 'GDRNEXT', 'GDRNext'),
            ('FX', 'EURINDEX', 'EURIndex'),
            ('FX', 'USDINDEX', 'USDIndex')
        ) AS t(market_label, futu_symbol, target_usc)
    LOOP
        -- ① 同一市场内按符号大小写兼容匹配到的行数（>1 说明大小写两种形态并存，先人工裁决）
        SELECT count(*) INTO v_hits
          FROM futu.finv_futu_quote_collect
         WHERE upper(futu_symbol) = r.futu_symbol
           AND upper(market_label) = r.market_label;
        IF v_hits > 1 THEN
            RAISE EXCEPTION 'V300182 中止：% . % 匹配到 % 行（大小写两种形态并存），需人工裁决',
                r.market_label, r.futu_symbol, v_hits;
        END IF;

        -- ② 目标 usc 是否被另一行占用（usc 是主键）
        SELECT usc INTO v_conflict
          FROM futu.finv_futu_quote_collect
         WHERE usc = r.target_usc
           AND NOT (upper(futu_symbol) = r.futu_symbol AND upper(market_label) = r.market_label)
         LIMIT 1;
        IF FOUND THEN
            RAISE EXCEPTION 'V300182 中止：usc=% 已被另一行占用（futu_symbol=%），需人工裁决',
                r.target_usc, v_conflict;
        END IF;

        -- ③ 改基列（market_label / futu_symbol）+ usc；生成列 ftmm_symbol 自动跟随
        UPDATE futu.finv_futu_quote_collect q
           SET market_label = r.market_label,
               futu_symbol  = r.futu_symbol,
               usc          = r.target_usc
         WHERE upper(q.futu_symbol) = r.futu_symbol
           AND upper(q.market_label) = r.market_label
           AND (q.market_label, q.futu_symbol, q.usc)
               IS DISTINCT FROM (r.market_label, r.futu_symbol, r.target_usc);
        IF FOUND THEN
            RAISE NOTICE 'V300182 已修正：% . % → usc=%', r.market_label, r.futu_symbol, r.target_usc;
        ELSE
            RAISE NOTICE 'V300182 无需修改或本库无 % . % 这一行（跳过）',
                r.market_label, r.futu_symbol;
        END IF;
    END LOOP;
END $$;

-- 收敛校验：匹配到的行必须已处于目标状态（结构性判据，不含行数阈值）
DO $$
DECLARE
    v_bad BIGINT;
    v_hit BIGINT;
BEGIN
    SELECT count(*) INTO v_bad
      FROM futu.finv_futu_quote_collect
     WHERE (upper(futu_symbol), upper(market_label)) IN
           (('GDRNEXT', 'HK'), ('EURINDEX', 'FX'), ('USDINDEX', 'FX'))
       AND (market_label, futu_symbol, usc)
           IS DISTINCT FROM (upper(market_label), upper(futu_symbol),
                             CASE upper(futu_symbol)
                                 WHEN 'GDRNEXT' THEN 'GDRNext'
                                 WHEN 'EURINDEX' THEN 'EURIndex'
                                 WHEN 'USDINDEX' THEN 'USDIndex'
                             END);
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300182 未收敛：仍有 % 行未落到目标键/usc', v_bad;
    END IF;

    SELECT count(*) INTO v_hit
      FROM futu.finv_futu_quote_collect
     WHERE (upper(futu_symbol), upper(market_label)) IN
           (('GDRNEXT', 'HK'), ('EURINDEX', 'FX'), ('USDINDEX', 'FX'));
    RAISE NOTICE 'V300182 完成：三条目标键中本库命中 % 行，均已是「大写 ftmm_symbol + 驼峰 usc」', v_hit;
END $$;

COMMIT;
