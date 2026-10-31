-- =====================================================================
-- FinvQuant PostgreSQL V200080：20 只加密货币证券的 exchange_code 由 100 修正为 10001
--                               （需求 10880）
--
-- 背景：
--   这 20 只（`BCHUSDC`/`BTC`/`BTCUSD`/`ETH`/`USDT`…）是 `V200009` 的增量种子
--   （2026-08-05）插入的，当时把 `exchange_code` 直接写成了 `100`（`FX` 外汇市场），
--   而 `V200062`（Supabase 权威导入）给它们的是 `exchange_code=10001`
--   （`CRYPTO` Cryptocurrency Exchange）；因 `V200062` 用
--   `INSERT ... ON CONFLICT (usc) DO NOTHING`，`usc` 已存在故正确值从未覆盖进来。
--   `V200068` 只修了它们的 `market_code`（10001/10011），`exchange_code` 一直留着 100。
--
-- 依据（用户 2026-11-01 同意修改）：
--   · `futu.finv_futu_quote_collect` 中这批 `quote_market='CC'`、`market_type=17`、
--     富途市场码 360/400 —— 与 exchange=100（`quote_market='FX'`）明显不同品种；
--   · `V200062` 中 19 只明确写 `exchange_code=10001`；`PAXG` 不在 `V200062`
--     （上游视图未含），但富途镜像同属 `CC`/17/360，一并归 10001；
--   · `V200079` 已把 8 个加密货币交易所（10000~10007）的 `region` 切到 `CC`，
--     修正后这 20 只的回填 `region` 将由 `FX` 变为 `CC`，与品种口径一致。
--
-- 影响面（评估结论）：
--   1. `cdeu/store.go` 元数据回填：这 20 只的 `region` 由 `FX` 变 `CC`
--      （`cdeu.finv_quote_cdeu_usc` 当前无这批证券行，无存量回刷）；
--   2. 导出文件名 `region` 段由 `FX_*` 变 `CC_*`；`cdeu.finv_quote_cdeu_export_file`
--      中这批证券历史导出记录为 **0** 条，远端无归档文件，无回溯包袱；
--   3. `market_code` **不变**（19 只 10001、1 只 `BTCUSD` 保持 10011）。
--
-- 范围：仅 `finv_security` 这 20 行的 `exchange_code` 列，用明确 usc 清单限定，
--       不做「条件批量匹配」，避免误伤后续新增行。
-- 幂等：`exchange_code = 100` 条件，重复执行影响 0 行。
-- 迁移策略：单事务、失败整体回滚；写入前做唯一约束预检，末尾做收敛校验。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 唯一约束预检：改成 10001 后不得与既有 (10001, security_code) 撞车
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_conflict bigint;
BEGIN
    SELECT count(*) INTO v_conflict
      FROM public.finv_security s
     WHERE s.exchange_code = 100
       AND s.market_code IN (10001, 10011)
       AND s.usc IN ('BCHUSDC','BTC','BTCHKD','BTCUSDC','BTCUSDT','DOGE','ETH','ETHBTC',
                     'ETHUSDC','PAXG','PAXGUSD','SOL','SOLBTC','UNI','USDC','USDCUSD',
                     'USDCUSDT','USDT','USDTUSD','BTCUSD')
       AND EXISTS (SELECT 1 FROM public.finv_security x
                    WHERE x.exchange_code = 10001
                      AND x.security_code = s.security_code
                      AND x.usc <> s.usc);
    IF v_conflict <> 0 THEN
        RAISE EXCEPTION 'V200080 预检失败：% 行改成 exchange_code=10001 后会与 uq_finv_security_exchange_code 冲突', v_conflict;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 1. 20 只加密货币证券 exchange_code：100 → 10001
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET exchange_code = 10001,
       gmt_update    = now()
 WHERE exchange_code = 100
   AND market_code IN (10001, 10011)
   AND usc IN ('BCHUSDC','BTC','BTCHKD','BTCUSDC','BTCUSDT','DOGE','ETH','ETHBTC',
               'ETHUSDC','PAXG','PAXGUSD','SOL','SOLBTC','UNI','USDC','USDCUSD',
               'USDCUSDT','USDT','USDTUSD','BTCUSD');

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_fixed     bigint;   -- 20 只已指向 10001
    v_leftover  bigint;   -- 仍挂 100 的加密证券
    v_offmarket bigint;   -- 加密市场但 exchange 不在 10000~10007
    v_region    bigint;   -- 20 只回填 region 应为 CC
BEGIN
    -- 2.1 20 只全部是 10001
    SELECT count(*) INTO v_fixed
      FROM public.finv_security
     WHERE usc IN ('BCHUSDC','BTC','BTCHKD','BTCUSDC','BTCUSDT','DOGE','ETH','ETHBTC',
                   'ETHUSDC','PAXG','PAXGUSD','SOL','SOLBTC','UNI','USDC','USDCUSD',
                   'USDCUSDT','USDT','USDTUSD','BTCUSD')
       AND exchange_code = 10001;
    IF v_fixed <> 20 THEN
        RAISE EXCEPTION 'V200080 校验失败：只有 % 行指向 10001（预期 20）', v_fixed;
    END IF;

    -- 2.2 不应再有加密市场证券挂在 exchange=100
    SELECT count(*) INTO v_leftover
      FROM public.finv_security
     WHERE exchange_code = 100 AND market_code IN (10001, 10011);
    IF v_leftover <> 0 THEN
        RAISE EXCEPTION 'V200080 校验失败：仍有 % 行加密市场证券挂在 exchange_code=100', v_leftover;
    END IF;

    -- 2.3 所有加密市场证券的 exchange 必须落在 10000~10007
    SELECT count(*) INTO v_offmarket
      FROM public.finv_security
     WHERE market_code IN (10001, 10011, 10013, 10014)
       AND exchange_code NOT BETWEEN 10000 AND 10007;
    IF v_offmarket <> 0 THEN
        RAISE EXCEPTION 'V200080 校验失败：% 行加密市场证券的 exchange_code 不在 10000~10007', v_offmarket;
    END IF;

    -- 2.4 元数据回填口径：这 20 只的 region 应为 CC
    SELECT count(*) INTO v_region
      FROM public.finv_security s
      JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      JOIN public.finv_region   r ON r.region = e.region
     WHERE s.usc IN ('BCHUSDC','BTC','BTCHKD','BTCUSDC','BTCUSDT','DOGE','ETH','ETHBTC',
                     'ETHUSDC','PAXG','PAXGUSD','SOL','SOLBTC','UNI','USDC','USDCUSD',
                     'USDCUSDT','USDT','USDTUSD','BTCUSD')
       AND r.region = 'CC';
    IF v_region <> 20 THEN
        RAISE EXCEPTION 'V200080 校验失败：只有 % 行的 region 回填为 CC（预期 20）', v_region;
    END IF;

    RAISE NOTICE 'V200080 完成：20 只加密货币证券 exchange_code 100 → 10001，region 回填口径为 CC';
END $$;

COMMIT;
