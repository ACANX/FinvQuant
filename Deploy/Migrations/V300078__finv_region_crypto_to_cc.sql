-- =====================================================================
-- FinvQuant PostgreSQL V300078：加密货币 region 取值由 CRYPTO 修正为 CC（需求 10878）
--
-- 背景（用户 2026-11-01 明确）：
--   「加密货币的 REGION 为 CC。」
--   `V300077` 新增 `(idx=3, region='CRYPTO', name='加密货币')` 时取值系按
--   `exchange_abbr`/`market_flag` 的习惯推测（`CRYPTO`），用户确认正确取值为
--   两字母短码 `CC`（与 `finv_exchange` 的 `exchange_abbr='CC'`、
--   `finv_region` 的 `CN`/`HK`/`US` 等短码风格一致）。
--
-- 范围（严格只改这一行、这一列）：
--   - `idx=3` 的 `region`：`CRYPTO` → `CC`；`name` 保持 `加密货币`；
--   - 不改 `finv_exchange.region`（8 个加密货币交易所 10000~10007 仍为 `GLOBAL`，
--     是否切换到 `CC` 属另一件事，需单独决策并评估导出路径 `region` 段变化）；
--   - 不改 `futu.finv_futu_mapping_region`。
--
-- 幂等：`region = 'CRYPTO'` 条件；已纠正的库影响 0 行；idx=3 若因故缺失则补插。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. idx=3 的 region 由 CRYPTO 改为 CC
-- ---------------------------------------------------------------------
UPDATE public.finv_region
   SET region     = 'CC',
       gmt_update = now()
 WHERE idx = 3
   AND region = 'CRYPTO';

-- ---------------------------------------------------------------------
-- 2. 补救：idx=3 缺失时按正确取值补插（幂等，双不存在条件）
-- ---------------------------------------------------------------------
INSERT INTO public.finv_region (idx, region, name)
SELECT 3, 'CC', '加密货币'
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_region WHERE idx = 3)
   AND NOT EXISTS (SELECT 1 FROM public.finv_region WHERE region = 'CC');

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cc       bigint;
    v_crypto   bigint;
    v_idx3     bigint;
BEGIN
    SELECT count(*) INTO v_cc FROM public.finv_region WHERE idx = 3 AND region = 'CC' AND name = '加密货币';
    IF v_cc <> 1 THEN
        RAISE EXCEPTION 'V300078 校验失败：finv_region 的 idx=3 不是 (CC, 加密货币)';
    END IF;

    SELECT count(*) INTO v_crypto FROM public.finv_region WHERE region = 'CRYPTO';
    IF v_crypto <> 0 THEN
        RAISE EXCEPTION 'V300078 校验失败：finv_region 仍存在 % 行 region=CRYPTO', v_crypto;
    END IF;

    SELECT count(*) INTO v_idx3 FROM public.finv_region WHERE idx = 3;
    IF v_idx3 <> 1 THEN
        RAISE EXCEPTION 'V300078 校验失败：idx=3 行数 %（预期 1）', v_idx3;
    END IF;

    RAISE NOTICE 'V300078 完成：加密货币 region 取值统一为 CC（idx=3）';
END $$;

COMMIT;
