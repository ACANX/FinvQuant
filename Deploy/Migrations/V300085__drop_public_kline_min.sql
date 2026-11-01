-- =====================================================================
-- V300085：历史遗留 public.finv_quote_secu_kline_min 的暂缓清理（需求 10899 / 10900）
--
-- ⚠️ 删除动作已**暂缓**（需求 10900）：该表数据需先由 BRS 备份到 R2，
--   备份完成后再执行 Deploy/Upgrade/2610-003-DropPublicKlineMin.sql 手工删除。
--   本迁移只做校验与提示，不删表，保证部署/升级链路先通过。
--
-- 背景：早期版本把分钟 K 线建在 public 下；新版只读写 cdeu.finv_quote_secu_kline_min，
--   数据由一次性脚本 Deploy/Upgrade/MigratePublicKlineToCdeu.sql（需求 10773）搬迁。
--   该脚本刻意「只读源表、不删源」，源码注释写明「验证通过后可作废」。
--   DbSchemaSpec §0.2 的受保护同名表只有 quote. 与 cdeu. 两份，public 这份不在保护名单，
--   代码中也没有任何业务读写它。
--
-- 安全策略（宁可保留，不可误删）：
--   1) 源表不存在 → 跳过；
--   2) 目标表不存在、或源表缺少 usc/secu_code 列 → 无法校验，**保留不删** 并 WARNING；
--   3) 源表仍有行未在 cdeu 中落地（按 (ts, usc) 差集）→ **保留不删** 并 WARNING，
--      提示先执行 Deploy/Upgrade/MigratePublicKlineToCdeu.sql；
--   4) 差集为 0 → 只输出 NOTICE 提示「可以删除」，**不执行 DROP**（暂缓，需求 10900）。
--
-- 后续删除（备份到 R2 之后）：
--   1) 确认 BRS 已把该表备份到对象存储（备份登记见迁移 V300086；R2 路径
--      <prefix>/<env>/<yyyyMMdd>/finv.public.finv_quote_secu_kline_min.dump）；
--   2) 在目标库手工执行 Deploy/Upgrade/2610-003-DropPublicKlineMin.sql
--      （同样先校验差集，为 0 才删除，否则整脚本报错中止）；
--   3) 恢复「迁移内自动删除」时，把本文件的第 4 步还原为删除语句即可。
--
-- 幂等：源表删除后重跑直接跳过。
-- 迁移策略：单事务；只校验与提示，不改变数据。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    c_src CONSTANT text := 'public.finv_quote_secu_kline_min';
    c_dst CONSTANT text := 'cdeu.finv_quote_secu_kline_min';

    v_usc_col  text;
    v_src_rows bigint := 0;
    v_missing  bigint := 0;
BEGIN
    -- 1. 源表不存在 → 跳过
    IF to_regclass(c_src) IS NULL THEN
        RAISE NOTICE 'V300085：% 不存在，跳过', c_src;
        RETURN;
    END IF;

    -- 2. 源表证券代码列（兼容旧结构 secu_code 与新结构 usc）
    SELECT CASE
             WHEN bool_or(column_name = 'usc') THEN 'usc'
             WHEN bool_or(column_name = 'secu_code') THEN 'secu_code'
           END
      INTO v_usc_col
      FROM information_schema.columns
     WHERE table_schema = 'public'
       AND table_name   = 'finv_quote_secu_kline_min';

    EXECUTE format('SELECT count(*) FROM %s', c_src) INTO v_src_rows;

    IF v_usc_col IS NULL THEN
        RAISE WARNING 'V300085：% 缺少 usc/secu_code 列（% 行），无法校验数据是否已搬迁，保留该表不删除',
                      c_src, v_src_rows;
        RETURN;
    END IF;

    IF to_regclass(c_dst) IS NULL THEN
        RAISE WARNING 'V300085：目标表 % 不存在（% 行），保留 % 不删除', c_dst, v_src_rows, c_src;
        RETURN;
    END IF;

    -- 3. 差集校验：源表每条 (ts, usc) 必须已在 cdeu 中落地
    EXECUTE format($sql$
        SELECT count(*)
          FROM %1$s s
         WHERE NOT EXISTS (
               SELECT 1 FROM %2$s d
                WHERE d.ts  = s.ts
                  AND d.usc = s.%3$I
         )
    $sql$, c_src, c_dst, v_usc_col) INTO v_missing;

    IF v_missing > 0 THEN
        RAISE WARNING 'V300085：% 共 % 行，其中 % 行在 % 中无对应记录，保留该表不删除；请先执行 Deploy/Upgrade/MigratePublicKlineToCdeu.sql',
                      c_src, v_src_rows, v_missing, c_dst;
        RETURN;
    END IF;

    -- 4. 数据已全部搬迁 → 本迁移只提示，删除动作暂缓（需求 10900）
    RAISE NOTICE 'V300085：% 共 % 行，与 % 差集 0，数据已搬迁完整，可以删除；删除已暂缓，待 BRS 备份到 R2 后执行 Deploy/Upgrade/2610-003-DropPublicKlineMin.sql',
                 c_src, v_src_rows, c_dst;
END $$;

COMMIT;
