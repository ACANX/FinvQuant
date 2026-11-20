-- =====================================================================
-- FinvQuant PostgreSQL V300076：USDIndex 归属市场固定为 1253（需求 10876）
--
-- 背景与决策（用户 2026-11-01 明确）：
--   「1253 是我手工加上的，需要按照规范发放数据，按 1253 来；后续 USDIndex
--    就是按照 1253 来定义。」
--   即 `usc='USDIndex'`（美元指数）固定归属 `market_code=1253`
--   （`market_abbr='FX-IDX'`、`market_symbol='CIX'`、`en_security_type='1100'`）。
--
-- 为什么需要这条迁移：
--   `V300062` 从 Supabase 批量导入种子时，`USDIndex` 写的是 `market_code=1251`
--   （`FX`/`CER`）。生产库的 `1253` 来自早前手工回填（未落成迁移），因此
--   「全新库」无法复现生产口径 —— 全新库会出现「1251 市场 + type_secu_sub=CIX」
--   的自相矛盾行（`V300075` 已把该行 `type_secu_sub` 改为 `CIX`）。
--   本迁移按「以生产库为准」的既定口径（同需求 10871）收口，两库一致。
--
-- 范围（严格只改这一列、这一行）：
--   - `usc='USDIndex'`：`market_code` → `1253`；
--   - 不动 `type_secu_sub`（`V300075` 已是 `CIX`）、`currency_type`、`flag_enable`
--     等其它字段；
--   - 不动小写 `USDindex`（`V300003` 老种子，`V300068` 已置 1253，与本次同口径）。
--
-- 幂等：`IS DISTINCT FROM` 条件，重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. `USDIndex` 归属市场对齐 1253
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET market_code = 1253,
       gmt_update  = now()
 WHERE usc = 'USDIndex'
   AND market_code IS DISTINCT FROM 1253;

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_row      bigint;
    v_bad      bigint;
    v_1251_bad bigint;
BEGIN
    -- 1. USDIndex 必须是 1253 + CIX
    SELECT count(*) INTO v_row
      FROM public.finv_security
     WHERE usc = 'USDIndex' AND market_code = 1253 AND type_secu_sub = 'CIX';
    IF v_row <> 1 THEN
        RAISE EXCEPTION 'V300076 校验失败：USDIndex 不是唯一一行 1253/CIX（实际 % 行）', v_row;
    END IF;

    -- 2. 1253（FX-IDX）下不应存在非 CIX 的非空 type_secu_sub
    SELECT count(*) INTO v_bad
      FROM public.finv_security
     WHERE market_code = 1253
       AND type_secu_sub <> ''
       AND type_secu_sub <> 'CIX';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300076 校验失败：1253 市场下仍有 % 行 type_secu_sub 不是 CIX', v_bad;
    END IF;

    -- 3. 1251（FX）下不应出现 CIX（防止本迁移误伤 FX 主市场）
    SELECT count(*) INTO v_1251_bad
      FROM public.finv_security
     WHERE market_code = 1251 AND type_secu_sub = 'CIX';
    IF v_1251_bad <> 0 THEN
        RAISE EXCEPTION 'V300076 校验失败：1251 市场下出现 % 行 type_secu_sub=CIX', v_1251_bad;
    END IF;

    RAISE NOTICE 'V300076 完成：USDIndex 固定归属 1253（FX-IDX/CIX），后续按 1253 定义';
END $$;

COMMIT;
