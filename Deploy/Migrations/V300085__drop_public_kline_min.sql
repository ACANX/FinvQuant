-- =====================================================================
-- V300085：清理历史遗留的 public.finv_quote_secu_kline_min（需求 10899）
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
--   4) 差集为 0 → DROP TABLE；
--   5) DROP 若因依赖（视图/外键）失败 → 捕获异常、**保留表** 并 WARNING，迁移本身仍成功，
--      避免把整个服务和迁移链一起拖垮（V300083 曾因校验过严导致反复重启）。
--
-- **执行前请先备份该表**（迁移内无法调用 pg_dump，需由运维在部署前完成）：
--   pg_dump -h <host> -p <port> -U <user> -d finv -Fc \
--           -t public.finv_quote_secu_kline_min \
--           -f Deploy/Backup/finv_public_kline_min_$(date +%Y%m%d_%H%M%S).dump
--
-- 幂等：源表删除后重跑直接跳过。
-- 迁移策略：单事务；校验失败只告警不中断，保证服务可启动。
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

    -- 4. 数据已全部搬迁 → 删除；任何依赖导致的失败都只告警
    BEGIN
        DROP TABLE public.finv_quote_secu_kline_min;
        RAISE NOTICE 'V300085：已删除 %（源表 % 行，与 % 差集 0）', c_src, v_src_rows, c_dst;
    EXCEPTION WHEN OTHERS THEN
        RAISE WARNING 'V300085：删除 % 失败（%），保留该表；请人工处理依赖后重试', c_src, SQLERRM;
    END;
END $$;

COMMIT;
