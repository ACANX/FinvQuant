-- =====================================================================
-- FinvQuant 一次性维护脚本：删除历史遗留 public.finv_quote_secu_kline_min
-- （需求 10899 / 10900）
--
-- 前置（必须全部满足）：
--   1) 该表的数据已由 Deploy/Upgrade/MigratePublicKlineToCdeu.sql 搬入
--      cdeu.finv_quote_secu_kline_min；
--   2) 该表已由 BRS 备份到对象存储（迁移 V300086 已把它加入备份登记），
--      确认 R2 上存在 <prefix>/<env>/<yyyyMMdd>/finv.public.finv_quote_secu_kline_min.dump；
--   3) 本脚本会**再次校验**差集，不为 0 时整体报错中止（不删除）。
--
-- 用法（在目标库上执行）：
--   psql "postgres://finv@127.0.0.1:5432/finv" -f Deploy/Upgrade/2610-003-DropPublicKlineMin.sql
--
-- 行为：
--   · 源表不存在 → 提示并结束（幂等）；
--   · 目标表缺失 / 源表缺 usc·secu_code 列 / (ts, usc) 差集不为 0 → RAISE EXCEPTION 中止；
--   · 差集为 0 → DROP TABLE，同步删除 brs.brs_bak_rdb_table 中的该表登记，并做收敛校验（to_regclass 为 NULL）。
--
-- 与迁移 V300085 的关系：V300085 只做同样的校验与提示、**不删表**（暂缓删除）；
--   备份完成后执行本脚本即可，不需要再改迁移。
-- =====================================================================

\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
    c_src CONSTANT text := 'public.finv_quote_secu_kline_min';
    c_dst CONSTANT text := 'cdeu.finv_quote_secu_kline_min';

    v_usc_col  text;
    v_src_rows bigint := 0;
    v_missing  bigint := 0;
BEGIN
    IF to_regclass(c_src) IS NULL THEN
        RAISE NOTICE '源表 % 不存在，无需删除', c_src;
        RETURN;
    END IF;

    IF to_regclass(c_dst) IS NULL THEN
        RAISE EXCEPTION '目标表 % 不存在，无法校验数据是否已搬迁，终止', c_dst;
    END IF;

    SELECT CASE
             WHEN bool_or(column_name = 'usc') THEN 'usc'
             WHEN bool_or(column_name = 'secu_code') THEN 'secu_code'
           END
      INTO v_usc_col
      FROM information_schema.columns
     WHERE table_schema = 'public'
       AND table_name   = 'finv_quote_secu_kline_min';

    IF v_usc_col IS NULL THEN
        RAISE EXCEPTION '源表缺少 usc / secu_code 列，无法校验数据是否已搬迁，终止';
    END IF;

    EXECUTE format('SELECT count(*) FROM %s', c_src) INTO v_src_rows;

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
        RAISE EXCEPTION '校验失败：% 共 % 行，其中 % 行在 % 中无对应记录；请先执行 Deploy/Upgrade/MigratePublicKlineToCdeu.sql，本次未删除',
                        c_src, v_src_rows, v_missing, c_dst;
    END IF;

    RAISE NOTICE '校验通过：% 共 % 行，与 % 差集 0，开始删除', c_src, v_src_rows, c_dst;

    DROP TABLE public.finv_quote_secu_kline_min;

    -- 同步删除备份登记（表已不存在，保留登记会让每轮备份报错，同 V300084 的处理）
    DELETE FROM brs.brs_bak_rdb_table
     WHERE schema_name = 'public'
       AND table_name = 'finv_quote_secu_kline_min';

    IF to_regclass(c_src) IS NOT NULL THEN
        RAISE EXCEPTION '删除后校验失败：% 仍存在', c_src;
    END IF;

    RAISE NOTICE '已删除 %（原 % 行），并移除其备份登记', c_src, v_src_rows;
END $$;

COMMIT;
