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
-- 匹配口径：按 `upper(futu_symbol)` + `upper(market_label)` 定位，兼容任意大小写变体，
--   因此同一脚本在「本地库（只有 FX.USDINDEX 且已正确）」与「有 GDRnext / FX.EURindex 的库」都能跑。
--
-- 【本迁移只改数据，绝不阻断启动】(需求 11094 的教训)
--   V300182 首版在「目标 usc 已被另一行占用」时 RAISE EXCEPTION，远端 instance-arm-a 因此进入
--   systemd 重启循环（V300182 中止：usc=GDRNext 已被另一行占用）。数据修复类迁移的异常分支
--   一律降级为 `RAISE NOTICE` + 跳过：能改的改掉，改不了的把现场信息打进日志交人工裁决，
--   服务必须照常起来。
--
-- 判定优先级（逐条目标键）：
--   ① 匹配到 0 行         → NOTICE「本库无该行」，跳过；
--   ② 匹配到 >1 行        → 大小写两种形态并存，NOTICE 打印各行明细（stock_id / usc / 符号），跳过；
--   ③ 目标 usc 被别的行占用 → NOTICE 打印占用行明细，跳过（不强改主键）；
--   ④ 其余               → 改基列 + usc（`IS DISTINCT FROM` 保证幂等，已正确的行不更新）。
--
-- 幂等：复跑影响 0 行；单事务（BEGIN/COMMIT）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    r           RECORD;
    v_hits      BIGINT;
    v_occupied  BIGINT;
    v_details   TEXT;
    v_updated   BIGINT := 0;
    v_skipped   BIGINT := 0;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('HK', 'GDRNEXT', 'GDRNext'),
            ('FX', 'EURINDEX', 'EURIndex'),
            ('FX', 'USDINDEX', 'USDIndex')
        ) AS t(market_label, futu_symbol, target_usc)
    LOOP
        -- ① 大小写兼容匹配到的行数
        SELECT count(*) INTO v_hits
          FROM futu.finv_futu_quote_collect
         WHERE upper(futu_symbol) = r.futu_symbol
           AND upper(market_label) = r.market_label;

        IF v_hits = 0 THEN
            RAISE NOTICE 'V300182 跳过：本库没有 % . % 这一行', r.market_label, r.futu_symbol;
            v_skipped := v_skipped + 1;
            CONTINUE;
        END IF;

        -- ② 同一市场内大小写两种形态并存：不猜，打印明细交人工裁决
        IF v_hits > 1 THEN
            SELECT string_agg(format('stock_id=%s usc=%s symbol=%s', stock_id, usc, futu_symbol), ' | ' ORDER BY stock_id)
              INTO v_details
              FROM futu.finv_futu_quote_collect
             WHERE upper(futu_symbol) = r.futu_symbol
               AND upper(market_label) = r.market_label;
            RAISE NOTICE 'V300182 跳过：% . % 匹配到 % 行（大小写形态并存），明细：%',
                r.market_label, r.futu_symbol, v_hits, v_details;
            v_skipped := v_skipped + 1;
            CONTINUE;
        END IF;

        -- ③ 目标 usc（主键）是否已被**另一行**占用：不强改主键，打印占用行明细
        SELECT count(*) INTO v_occupied
          FROM futu.finv_futu_quote_collect
         WHERE usc = r.target_usc
           AND NOT (upper(futu_symbol) = r.futu_symbol AND upper(market_label) = r.market_label);
        IF v_occupied > 0 THEN
            SELECT string_agg(format('stock_id=%s usc=%s symbol=%s.%s', stock_id, usc, market_label, futu_symbol), ' | ' ORDER BY stock_id)
              INTO v_details
              FROM futu.finv_futu_quote_collect
             WHERE usc = r.target_usc
               AND NOT (upper(futu_symbol) = r.futu_symbol AND upper(market_label) = r.market_label);
            RAISE NOTICE 'V300182 跳过：目标 usc=% 已被 % 行占用，明细：%',
                r.target_usc, v_occupied, v_details;
            v_skipped := v_skipped + 1;
            CONTINUE;
        END IF;

        -- ④ 改基列（market_label / futu_symbol）+ usc；生成列 ftmm_symbol 自动跟随
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
            v_updated := v_updated + 1;
        ELSE
            RAISE NOTICE 'V300182 无需修改：% . % 已是目标状态', r.market_label, r.futu_symbol;
        END IF;
    END LOOP;

    RAISE NOTICE 'V300182 完成：本次修正 % 条，跳过 % 条（跳过项见上方明细）', v_updated, v_skipped;
END $$;

COMMIT;
