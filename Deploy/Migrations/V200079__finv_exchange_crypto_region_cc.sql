-- =====================================================================
-- FinvQuant PostgreSQL V200079：8 个加密货币交易所的 region 由 GLOBAL 切到 CC
--                               （需求 10879）
--
-- 背景（用户 2026-11-01 明确）：
--   `finv_region` 已新增 `(idx=3, region='CC', name='加密货币')`（`V200077` 新增
--   `CRYPTO`、`V200078` 修正为 `CC`），但 `finv_exchange` 中 8 个加密货币交易所
--   （10000 `CC` ~ 10007 `CCPT`）的 `region` 仍是 `GLOBAL`，导致 `CC` 这一行
--   没有任何交易所引用。用户要求「把这 8 行改成 CC」。
--
-- 影响面（评估结论）：
--   1. `cdeu/store.go` 的元数据回填 `region = COALESCE(NULLIF(r.region,''), …)`
--      对加密证券将由 `GLOBAL` 变为 `CC`；`cdeu.finv_quote_cdeu_usc` 当前**无**
--      加密证券行（region 分布为 CN/HK/US/FX/空），无存量需要回刷；
--   2. 加密证券的导出文件名 `region` 段将由 `GLOBAL_*` 变为 `CC_*`；
--      `cdeu.finv_quote_cdeu_export_file` 中此类证券的历史导出记录为 **0** 条，
--      远端 GitHub 仓库也无对应归档文件，改名无历史包袱；
--   3. `finv_security` 中另有 20 只挂 `exchange_code=100`（FX 外汇市场）却
--      `market_code` 为 10001/10011（加密货币市场）的证券，其 `region` 仍随
--      exchange=100 取 `FX` —— 属既有数据归属问题，**本次不动**，另行评估。
--
-- 范围：仅 `finv_exchange` 的 8 行 `region` 列；不动 `finv_security` /
--       `cdeu.*` / `futu.*`，不动 `base_currency`、`flag_enable`、`exchange_abbr`。
-- 幂等：`IS DISTINCT FROM 'CC'` 条件，重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 8 个加密货币交易所 region → CC
-- ---------------------------------------------------------------------
UPDATE public.finv_exchange
   SET region     = 'CC',
       gmt_update = now()
 WHERE exchange_code BETWEEN 10000 AND 10007
   AND region IS DISTINCT FROM 'CC';

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cc       bigint;
    v_global   bigint;
    v_unmatch  bigint;
BEGIN
    -- 8 个加密货币交易所必须全部是 CC
    SELECT count(*) INTO v_cc
      FROM public.finv_exchange
     WHERE exchange_code BETWEEN 10000 AND 10007
       AND region = 'CC';
    IF v_cc <> 8 THEN
        RAISE EXCEPTION 'V200079 校验失败：10000~10007 中 region=CC 的交易所为 % 行（预期 8）', v_cc;
    END IF;

    -- 不应再有交易所挂在 GLOBAL（切完后该 region 只保留给 finv_region 的兜底语义）
    SELECT count(*) INTO v_global
      FROM public.finv_exchange
     WHERE region = 'GLOBAL';
    IF v_global <> 0 THEN
        RAISE EXCEPTION 'V200079 校验失败：仍有 % 个交易所 region=GLOBAL', v_global;
    END IF;

    -- 8 行必须能匹配到 finv_region(3, CC)
    SELECT count(*) INTO v_unmatch
      FROM public.finv_exchange e
      LEFT JOIN public.finv_region r ON r.region = e.region
     WHERE e.exchange_code BETWEEN 10000 AND 10007
       AND (r.region IS NULL OR r.idx <> 3);
    IF v_unmatch <> 0 THEN
        RAISE EXCEPTION 'V200079 校验失败：% 个加密货币交易所未能匹配 finv_region(3, CC)', v_unmatch;
    END IF;

    RAISE NOTICE 'V200079 完成：8 个加密货币交易所 region GLOBAL → CC，已与 finv_region(3) 对齐';
END $$;

COMMIT;
