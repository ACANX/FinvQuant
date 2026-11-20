-- =====================================================================
-- FinvQuant PostgreSQL V300075：修正 USDIndex 的 type_secu_sub 为 CIX（需求 10875）
--
-- 背景：
--   `finv_security.type_secu_sub` 存的是上游细分品种符号（与 `finv_market.market_symbol`
--   同源），正常应与所属市场的 market_symbol 一致：
--     1251 FX      → CER（569 行一致）
--     1252 FX-CFD  → CFD
--     1253 FX-IDX  → CIX
--   但 `USDIndex`（美元指数）由 `V300062` 从 Supabase 批量导入时，上游
--   `sub_instrument_type=10001` 被统一映射成 `CER`，导致它虽挂 1253（FX-IDX/CIX）
--   却带着 `type_secu_sub='CER'`，与市场符号不一致。
--
-- 范围（严格只改这一行）：
--   - `usc='USDIndex'`（大写 I，provider='FTMM'，来自 V300062）：`CER` → `CIX`；
--   - **不动** `usc='USDindex'`（小写 d，来自 V300003 老种子、provider=''、
--     type_secu_sub 本就为空串），两者大小写不同属存量重复，另行处理；
--   - 不动 market_code / currency_type / flag_enable 等其它字段。
--
-- 决策：用户 2026-11-01 明确「USDIndex 的 type_secu_sub 改为 CIX」。
-- 幂等：`IS DISTINCT FROM` 条件，重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

UPDATE public.finv_security
   SET type_secu_sub = 'CIX',
       gmt_update    = now()
 WHERE usc = 'USDIndex'
   AND type_secu_sub IS DISTINCT FROM 'CIX';

-- ---------------------------------------------------------------------
-- 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad     bigint;
    v_target  bigint;
    v_lower   bigint;
BEGIN
    -- 1. 目标行必须是 CIX
    SELECT count(*) INTO v_target
      FROM public.finv_security
     WHERE usc = 'USDIndex' AND type_secu_sub = 'CIX';
    IF v_target <> 1 THEN
        RAISE EXCEPTION 'V300075 校验失败：usc=USDIndex 且 type_secu_sub=CIX 的行数为 %（预期 1）', v_target;
    END IF;

    -- 2. 不得误改小写 USDindex（老种子行，type_secu_sub 应为空串）
    SELECT count(*) INTO v_lower
      FROM public.finv_security
     WHERE usc = 'USDindex' AND type_secu_sub = '';
    IF v_lower <> 1 THEN
        RAISE EXCEPTION 'V300075 校验失败：usc=USDindex 的 type_secu_sub 不是空串（预期 1 行）';
    END IF;

    -- 3. 1253（FX-IDX）下不应再有不等于 CIX 的 type_secu_sub 非空值
    SELECT count(*) INTO v_bad
      FROM public.finv_security
     WHERE market_code = 1253
       AND type_secu_sub <> ''
       AND type_secu_sub <> 'CIX';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300075 校验失败：1253 市场下仍有 % 行 type_secu_sub 不是 CIX', v_bad;
    END IF;

    -- 4. 其它市场未受影响（1251 下 CER 行数应保持不变，仅提示）
    RAISE NOTICE 'V300075 完成：USDIndex.type_secu_sub CER → CIX；1253 市场符号口径已统一';
END $$;

COMMIT;
