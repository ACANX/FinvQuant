-- =====================================================================
-- FinvQuant PostgreSQL V300077：finv_region 补 FX / CRYPTO 并统一 US（需求 10877）
--
-- 背景（用户 2026-11-01 决策）：
--   1. `finv_exchange` 中 2 个交易所（100 FX、101 FX-CFD）的 `region='FX'`，
--      但 `finv_region` 里没有 `FX` 行 —— `cdeu/store.go` 的元数据回填
--      `LEFT JOIN public.finv_region r ON r.region = e.region` 永远 join 不上，
--      只能退化取 `finv_exchange.region` 原始文本。用户在 2026-11-01 明确要求
--      「finv_region 表中需要加 FX 这个」，并指定 `idx=2`、`name='外汇'`。
--   2. 用户同期要求「顺便把加密货币的 3 也加上」，即 `idx=3`、`region='CRYPTO'`、
--      `name='加密货币'`（`idx` 编码依据 `V300004`：0=UNK、1=GLOBAL、2~100 预留扩展段）。
--   3. 全新库 `finv_region` 103 仍是 `V300004` 种子值 `USA`，而生产库与
--      `finv_exchange.region`（9 个美股交易所）都是 `US` —— 全新库自身不一致，
--      会导致同一只美股在全新库导出为 `USA_...`、生产库导出为 `US_...`。
--      用户确认「全新库 region 103 = USA 需对齐成 US」。
--
-- 说明（本期不改）：
--   - `finv_exchange` 中 8 个加密货币交易所（10000~10007）的 `region` 目前是
--     `GLOBAL`，本次**不改**；因此新增的 `CRYPTO` 行暂未被任何交易所引用，
--     属「字典先行、交易所归属后续单独决策」；
--   - `finv_exchange.region='N/A'` 的 4 行（120/121/122/9003）本次**不处理**，
--     `N/A` 仍未入字典，留待后续需求；
--   - 不改 `futu.finv_futu_mapping_region`（富途侧无独立加密货币 region，1=GLOBAL
--     覆盖「全球/外汇/加密货币」）。
--
-- 幂等：INSERT 用「idx 与 region 双不存在」条件，UPDATE 用「值不同」条件，
--       重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 补 `FX`（idx=2）
-- ---------------------------------------------------------------------
INSERT INTO public.finv_region (idx, region, name)
SELECT 2, 'FX', '外汇'
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_region WHERE region = 'FX')
   AND NOT EXISTS (SELECT 1 FROM public.finv_region WHERE idx = 2);

-- ---------------------------------------------------------------------
-- 2. 补 `CRYPTO`（idx=3）
-- ---------------------------------------------------------------------
INSERT INTO public.finv_region (idx, region, name)
SELECT 3, 'CRYPTO', '加密货币'
 WHERE NOT EXISTS (SELECT 1 FROM public.finv_region WHERE region = 'CRYPTO')
   AND NOT EXISTS (SELECT 1 FROM public.finv_region WHERE idx = 3);

-- ---------------------------------------------------------------------
-- 3. 103 的 region：`USA` → `US`（与生产库、`finv_exchange.region` 一致）
--    生产库已是 `US`，此处影响 0 行；全新库由 `USA` 改 `US`。
-- ---------------------------------------------------------------------
UPDATE public.finv_region
   SET region     = 'US',
       gmt_update = now()
 WHERE idx = 103
   AND region = 'USA';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_fx      bigint;
    v_crypto  bigint;
    v_us      bigint;
    v_missing bigint;
    v_total   bigint;
BEGIN
    SELECT count(*) INTO v_fx FROM public.finv_region WHERE idx = 2 AND region = 'FX' AND name = '外汇';
    IF v_fx <> 1 THEN
        RAISE EXCEPTION 'V300077 校验失败：finv_region 缺少 (2, FX, 外汇)';
    END IF;

    SELECT count(*) INTO v_crypto FROM public.finv_region WHERE idx = 3 AND region = 'CRYPTO' AND name = '加密货币';
    IF v_crypto <> 1 THEN
        RAISE EXCEPTION 'V300077 校验失败：finv_region 缺少 (3, CRYPTO, 加密货币)';
    END IF;

    SELECT count(*) INTO v_us FROM public.finv_region WHERE idx = 103 AND region = 'US';
    IF v_us <> 1 THEN
        RAISE EXCEPTION 'V300077 校验失败：finv_region 的 103 不是 US';
    END IF;

    -- `finv_exchange` 中 FX / US 的交易所必须全部能在字典里找到（本期已闭环的两类）
    SELECT count(*) INTO v_missing
      FROM public.finv_exchange e
      LEFT JOIN public.finv_region r ON r.region = e.region
     WHERE e.region IN ('FX', 'US')
       AND r.region IS NULL;
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V300077 校验失败：% 个 FX/US 交易所仍未匹配到 finv_region', v_missing;
    END IF;

    SELECT count(*) INTO v_total FROM public.finv_region;
    RAISE NOTICE 'V300077 完成：finv_region 共 % 行（新增 FX=2、CRYPTO=3，103 统一为 US）', v_total;
END $$;

COMMIT;
