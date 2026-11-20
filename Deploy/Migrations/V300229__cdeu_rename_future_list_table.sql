-- =====================================================================
-- V300229：cdeu.finv_quote_realtime_future → cdeu.finv_quote_secu_realtime_future
--
-- 需求 11325（用户明确点名改名）：期货合约实时快照表与证券/加密货币快照表
-- （cdeu.finv_quote_secu_realtime、cdeu.finv_quote_secu_realtime_crypto）命名对齐，
-- 统一为 `finv_quote_secu_realtime_*`。
--
-- 变更内容：
--   1. 表名 finv_quote_realtime_future → finv_quote_secu_realtime_future（数据/注释随表保留）；
--   2. 主键 / 检查约束 / NOT NULL 约束 / 索引 / 触发器名同步换前缀，避免名字与表名不一致；
--   3. brs.brs_bak_rdb_table_full 备份登记行改为新表名（不改 priority，保留原备份优先级）；
--   4. 末尾校验：旧名不存在、新名存在、备份登记已更新。
--
-- 幂等：每一步都带「旧名存在且新名不存在」判断，重跑影响 0 行。
-- 回滚：反向执行 RENAME（含约束/索引/触发器名）并改回备份登记。
-- =====================================================================

BEGIN;

-- 1. 表改名（幂等）
DO $$
BEGIN
    IF to_regclass('cdeu.finv_quote_realtime_future') IS NOT NULL
       AND to_regclass('cdeu.finv_quote_secu_realtime_future') IS NULL THEN
        ALTER TABLE cdeu.finv_quote_realtime_future RENAME TO finv_quote_secu_realtime_future;
        RAISE NOTICE 'V300229：表已改名为 cdeu.finv_quote_secu_realtime_future';
    END IF;
END $$;

-- 2. 索引改名（主键索引 + idx_* 普通/部分索引）
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT c.relname AS old_name
          FROM pg_class c
          JOIN pg_index i ON i.indexrelid = c.oid
          JOIN pg_class t ON t.oid = i.indrelid
          JOIN pg_namespace n ON n.oid = t.relnamespace
         WHERE n.nspname = 'cdeu'
           AND t.relname = 'finv_quote_secu_realtime_future'
           AND c.relname LIKE '%finv_quote_realtime_future%'
    LOOP
        EXECUTE format('ALTER INDEX cdeu.%I RENAME TO %I',
                       r.old_name,
                       replace(r.old_name, 'finv_quote_realtime_future', 'finv_quote_secu_realtime_future'));
    END LOOP;
END $$;

-- 3. 约束改名（主键 / CHECK / NOT NULL）
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT conname AS old_name
          FROM pg_constraint
         WHERE conrelid = 'cdeu.finv_quote_secu_realtime_future'::regclass
           AND conname LIKE '%finv_quote_realtime_future%'
    LOOP
        EXECUTE format('ALTER TABLE cdeu.finv_quote_secu_realtime_future RENAME CONSTRAINT %I TO %I',
                       r.old_name,
                       replace(r.old_name, 'finv_quote_realtime_future', 'finv_quote_secu_realtime_future'));
    END LOOP;
END $$;

-- 4. 触发器改名（gmt_update 维护）
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_trigger
                WHERE tgrelid = 'cdeu.finv_quote_secu_realtime_future'::regclass
                  AND tgname = 'trg_finv_quote_realtime_future_gmt_update') THEN
        ALTER TRIGGER trg_finv_quote_realtime_future_gmt_update
            ON cdeu.finv_quote_secu_realtime_future
            RENAME TO trg_finv_quote_secu_realtime_future_gmt_update;
    END IF;
END $$;

-- 5. 备份登记（BRS 全量备份清单）改成新表名，priority 保持不变
UPDATE brs.brs_bak_rdb_table_full
   SET table_name = 'finv_quote_secu_realtime_future',
       remark = '期货合约实时快照（moomoo get-futures-list，需求 11273；V300229 改名，需求 11325）'
 WHERE rdb_type = 'postgresql'
   AND db = 'finv'
   AND schema_name = 'cdeu'
   AND table_name = 'finv_quote_realtime_future';

-- 6. 校验
DO $$
BEGIN
    IF to_regclass('cdeu.finv_quote_secu_realtime_future') IS NULL THEN
        RAISE EXCEPTION 'V300229 校验失败：新表 cdeu.finv_quote_secu_realtime_future 不存在';
    END IF;
    IF to_regclass('cdeu.finv_quote_realtime_future') IS NOT NULL THEN
        RAISE EXCEPTION 'V300229 校验失败：旧表 cdeu.finv_quote_realtime_future 仍存在';
    END IF;
    IF EXISTS (SELECT 1 FROM brs.brs_bak_rdb_table_full
                WHERE schema_name = 'cdeu' AND table_name = 'finv_quote_realtime_future') THEN
        RAISE EXCEPTION 'V300229 校验失败：备份登记仍有旧表名';
    END IF;
    RAISE NOTICE 'V300229 完成：期货快照表改名为 cdeu.finv_quote_secu_realtime_future';
END $$;

COMMIT;
