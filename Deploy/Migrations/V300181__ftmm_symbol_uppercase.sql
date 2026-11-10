-- =====================================================================
-- FinvQuant PostgreSQL V300181：ftmm_symbol 英文字母统一大写（需求 11143）
--
-- 背景（用户 2026-11-12 要求）：
--   入库存到 `ftmm_symbol` 的英文字母统一大写。写入侧已在代码里固定：
--     · futu.finv_futu_secu_key        —— 批量 upsert 的 src CTE 用 `upper(btrim(...))`
--     · futu.finv_futu_usc_map         —— 应用时 `SELECT upper(d.ftmm_symbol) ...`
--     · cdeu.finv_quote_secu_indicator —— 指标批量 upsert 的 src CTE 用 `upper(btrim(...))`
--   本迁移处理**存量**：只改写侧会让大写新键与小写旧键在同一张表里并存（主键不同 → 同一只证券两行）。
--
-- 实测存量（2026-11-12）：键表与结果表各 383 行含小写（`GL.quote-...` / `AU.heatmap-...` 这类
--   富途合成的板块与地图键），指标表 0 行。
--
-- 影响面（须知）：
--   1. 上游 `public.finv_quote_secu_key` 里这些键仍是小写，Tab 3「本地 usc 回推上游」会把它们判成
--      「上游无对应」而跳过（PostgREST 的 `in.(...)` 大小写敏感），这是刻意保留的行为：不动上游主键。
--   2. Tab 1「证券键表同步」拉回小写键时会经写入侧 `upper()` 归一，仍落在大写行上，不会新增重复行。
--   3. 归档表（`archive.*`）是历史快照，**不改**。
--
-- 安全：
--   - 大写后若与既有键撞车（同一键的大小写两种形态同时存在），直接 RAISE EXCEPTION 中止，交人工裁决；
--   - 单事务（BEGIN/COMMIT）；
--   - 幂等：只处理 `ftmm_symbol <> upper(ftmm_symbol)` 的行，复跑影响 0 行。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_dup BIGINT;
BEGIN
    SELECT count(*) INTO v_dup FROM (
        SELECT upper(ftmm_symbol) AS k
          FROM futu.finv_futu_secu_key
         GROUP BY 1 HAVING count(*) > 1) d;
    IF v_dup > 0 THEN
        RAISE EXCEPTION 'V300181 中止：键表有 % 组「大写后重复」的 ftmm_symbol，需先人工裁决', v_dup;
    END IF;

    SELECT count(*) INTO v_dup FROM (
        SELECT upper(ftmm_symbol) AS k
          FROM futu.finv_futu_usc_map
         GROUP BY 1 HAVING count(*) > 1) d;
    IF v_dup > 0 THEN
        RAISE EXCEPTION 'V300181 中止：结果表有 % 组「大写后重复」的 ftmm_symbol，需先人工裁决', v_dup;
    END IF;

    SELECT count(*) INTO v_dup FROM (
        SELECT upper(ftmm_symbol) AS k
          FROM cdeu.finv_quote_secu_indicator
         GROUP BY 1 HAVING count(*) > 1) d;
    IF v_dup > 0 THEN
        RAISE EXCEPTION 'V300181 中止：指标表有 % 组「大写后重复」的 ftmm_symbol，需先人工裁决', v_dup;
    END IF;
END $$;

UPDATE futu.finv_futu_secu_key
   SET ftmm_symbol = upper(ftmm_symbol)
 WHERE ftmm_symbol <> upper(ftmm_symbol);

UPDATE futu.finv_futu_usc_map
   SET ftmm_symbol = upper(ftmm_symbol)
 WHERE ftmm_symbol <> upper(ftmm_symbol);

UPDATE cdeu.finv_quote_secu_indicator
   SET ftmm_symbol = upper(ftmm_symbol)
 WHERE ftmm_symbol <> upper(ftmm_symbol);

DO $$
DECLARE
    v_key BIGINT;
    v_map BIGINT;
    v_ind BIGINT;
BEGIN
    SELECT count(*) INTO v_key FROM futu.finv_futu_secu_key
     WHERE ftmm_symbol <> upper(ftmm_symbol);
    SELECT count(*) INTO v_map FROM futu.finv_futu_usc_map
     WHERE ftmm_symbol <> upper(ftmm_symbol);
    SELECT count(*) INTO v_ind FROM cdeu.finv_quote_secu_indicator
     WHERE ftmm_symbol <> upper(ftmm_symbol);

    IF v_key <> 0 OR v_map <> 0 OR v_ind <> 0 THEN
        RAISE EXCEPTION 'V300181 未收敛：仍含小写 ftmm_symbol（键表 % / 结果表 % / 指标表 %）',
            v_key, v_map, v_ind;
    END IF;

    RAISE NOTICE 'V300181 完成：ftmm_symbol 已全部大写（键表/结果表/指标表各 0 行残留）';
END $$;

COMMIT;
