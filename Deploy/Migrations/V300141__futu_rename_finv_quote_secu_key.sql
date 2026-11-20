-- =====================================================================
-- FinvQuant PostgreSQL V300141：表改名 futu.finv_quote_secu_key → finv_futu_secu_key（需求 11040）
--
-- 授权：表改名属 DbSchemaSpec §0 红线操作（RENAME），本次由用户**明确点名**：
--   「finv_quote_secu_key 表改名为 finv_futu_secu_key」。
--
-- 改动（全部幂等，先判存在再改）：
--   1. 表名：`futu.finv_quote_secu_key` → `futu.finv_futu_secu_key`
--      （命名前缀与同域 `finv_futu_mapping_*` / `finv_futu_quote_collect` 对齐）；
--   2. 约束名：`finv_quote_secu_key_pkey` → `finv_futu_secu_key_pkey`、
--      `finv_quote_secu_key_priority_check` → `finv_futu_secu_key_priority_check`、
--      `finv_quote_secu_key_status_check` → `finv_futu_secu_key_status_check`；
--   3. 索引名：`idx_finvquotesecukey_market` → `idx_finvfutusecukey_market`；
--   4. 触发器名：`trg_finv_quote_secu_key_gmt_update` → `trg_finv_futu_secu_key_gmt_update`
--      （触发器函数仍是共用的 `public.vq_set_gmt_update()`，不动）。
--   表结构（12 列）、数据（当前为空表，后续走同步）、注释一律不变。
--
-- 不回改的历史迁移：V300137（建表）/ V300138（status 约束）/ V300139（补 stock_id、usc）
--   均在本迁移之前执行，仍用旧表名，顺序上不冲突（与 V300140 的列改名同一处理口径）。
--
-- 幂等：`DO` 块按「旧对象存在且新对象不存在」判断后改名，重跑为 no-op；
--   末尾收敛校验（旧表名/旧约束/旧索引/旧触发器必须全部不存在，新名必须齐全，列数 12）。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V300137 / V300138 / V300139（旧表名时期的建表与加列）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 表改名（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('futu.finv_quote_secu_key') IS NOT NULL
       AND to_regclass('futu.finv_futu_secu_key') IS NULL THEN
        ALTER TABLE futu.finv_quote_secu_key RENAME TO finv_futu_secu_key;
        RAISE NOTICE 'V300141 表改名完成：futu.finv_quote_secu_key → futu.finv_futu_secu_key';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 约束 / 索引 / 触发器改名（幂等；表不存在于旧名时按新名处理）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    -- 主键约束
    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                  AND conname = 'finv_quote_secu_key_pkey')
       AND NOT EXISTS (SELECT 1 FROM pg_constraint
                        WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                          AND conname = 'finv_futu_secu_key_pkey') THEN
        ALTER TABLE futu.finv_futu_secu_key RENAME CONSTRAINT finv_quote_secu_key_pkey TO finv_futu_secu_key_pkey;
    END IF;

    -- priority 取值范围约束
    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                  AND conname = 'finv_quote_secu_key_priority_check')
       AND NOT EXISTS (SELECT 1 FROM pg_constraint
                        WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                          AND conname = 'finv_futu_secu_key_priority_check') THEN
        ALTER TABLE futu.finv_futu_secu_key RENAME CONSTRAINT finv_quote_secu_key_priority_check TO finv_futu_secu_key_priority_check;
    END IF;

    -- status 取值约束
    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                  AND conname = 'finv_quote_secu_key_status_check')
       AND NOT EXISTS (SELECT 1 FROM pg_constraint
                        WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
                          AND conname = 'finv_futu_secu_key_status_check') THEN
        ALTER TABLE futu.finv_futu_secu_key RENAME CONSTRAINT finv_quote_secu_key_status_check TO finv_futu_secu_key_status_check;
    END IF;

    -- 业务索引
    IF EXISTS (SELECT 1 FROM pg_indexes
                WHERE schemaname = 'futu' AND tablename = 'finv_futu_secu_key'
                  AND indexname = 'idx_finvquotesecukey_market')
       AND NOT EXISTS (SELECT 1 FROM pg_indexes
                        WHERE schemaname = 'futu' AND tablename = 'finv_futu_secu_key'
                          AND indexname = 'idx_finvfutusecukey_market') THEN
        ALTER INDEX futu.idx_finvquotesecukey_market RENAME TO idx_finvfutusecukey_market;
    END IF;

    -- gmt_update 触发器
    IF EXISTS (SELECT 1 FROM pg_trigger tg
                 JOIN pg_class t ON t.oid = tg.tgrelid
                 JOIN pg_namespace n ON n.oid = t.relnamespace
                WHERE n.nspname = 'futu' AND t.relname = 'finv_futu_secu_key'
                  AND tg.tgname = 'trg_finv_quote_secu_key_gmt_update' AND NOT tg.tgisinternal)
       AND NOT EXISTS (SELECT 1 FROM pg_trigger tg
                        JOIN pg_class t ON t.oid = tg.tgrelid
                        JOIN pg_namespace n ON n.oid = t.relnamespace
                       WHERE n.nspname = 'futu' AND t.relname = 'finv_futu_secu_key'
                         AND tg.tgname = 'trg_finv_futu_secu_key_gmt_update' AND NOT tg.tgisinternal) THEN
        ALTER TRIGGER trg_finv_quote_secu_key_gmt_update ON futu.finv_futu_secu_key
            RENAME TO trg_finv_futu_secu_key_gmt_update;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2.1 NOT NULL 约束改名（PostgreSQL 18 会把 NOT NULL 登记进 pg_constraint，
--     名字由建表时的表名派生，如 finv_quote_secu_key_code_not_null；一并改成新前缀）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT conname, conname AS old_name,
               'finv_futu_secu_key' || substr(conname, length('finv_quote_secu_key') + 1) AS new_name
          FROM pg_constraint
         WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
           AND contype = 'n'
           AND conname LIKE 'finv_quote_secu_key%'
    LOOP
        BEGIN
            EXECUTE format('ALTER TABLE futu.finv_futu_secu_key RENAME CONSTRAINT %I TO %I',
                           r.old_name, r.new_name);
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'V300141 NOT NULL 约束 % 改名跳过（%），不影响功能', r.old_name, SQLERRM;
        END;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 表注释重写（指向新表名）
-- ---------------------------------------------------------------------
COMMENT ON TABLE futu.finv_futu_secu_key IS
    '证券键表（原 finv_quote_secu_key）：instrument + market + code → 富途 ftmm_symbol 的键映射与采集状态跟踪（status / priority / url）';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_old      int;
    v_cols     int;
    v_pk       int;
    v_checks   int;
    v_idx      int;
    v_trg      int;
    v_bad      text;
BEGIN
    IF to_regclass('futu.finv_quote_secu_key') IS NOT NULL THEN
        RAISE EXCEPTION 'V300141 旧表名 futu.finv_quote_secu_key 仍存在';
    END IF;
    IF to_regclass('futu.finv_futu_secu_key') IS NULL THEN
        RAISE EXCEPTION 'V300141 新表 futu.finv_futu_secu_key 不存在';
    END IF;

    SELECT count(*) INTO v_cols FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_futu_secu_key';
    IF v_cols <> 12 THEN
        RAISE EXCEPTION 'V300141 列数不符：期望 12，实际 %', v_cols;
    END IF;

    SELECT count(*) INTO v_pk FROM pg_constraint
     WHERE conrelid = 'futu.finv_futu_secu_key'::regclass AND conname = 'finv_futu_secu_key_pkey';
    SELECT count(*) INTO v_checks FROM pg_constraint
     WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
       AND conname IN ('finv_futu_secu_key_priority_check', 'finv_futu_secu_key_status_check');
    SELECT count(*) INTO v_idx FROM pg_indexes
     WHERE schemaname = 'futu' AND tablename = 'finv_futu_secu_key' AND indexname = 'idx_finvfutusecukey_market';
    SELECT count(*) INTO v_trg FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'futu' AND t.relname = 'finv_futu_secu_key'
       AND tg.tgname = 'trg_finv_futu_secu_key_gmt_update' AND NOT tg.tgisinternal;

    IF v_pk <> 1 OR v_checks <> 2 OR v_idx <> 1 OR v_trg <> 1 THEN
        RAISE EXCEPTION 'V300141 改名后对象不全：pkey=% checks=% idx=% trg=%（均应为 1/2/1/1）',
            v_pk, v_checks, v_idx, v_trg;
    END IF;

    -- 旧名前缀的残留（主键/CHECK 必须为 0；NOT NULL 约束名仅作提示，不影响功能）
    SELECT string_agg(conname, ',') INTO v_bad FROM pg_constraint
     WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
       AND contype IN ('p', 'c') AND conname LIKE 'finv_quote_secu_key%';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300141 残留旧名主键/CHECK 约束：%', v_bad;
    END IF;

    SELECT string_agg(conname, ',') INTO v_bad FROM pg_constraint
     WHERE conrelid = 'futu.finv_futu_secu_key'::regclass
       AND contype = 'n' AND conname LIKE 'finv_quote_secu_key%';
    IF v_bad IS NOT NULL THEN
        RAISE NOTICE 'V300141 仍有旧名前缀的 NOT NULL 约束名（不影响功能）：%', v_bad;
    END IF;

    RAISE NOTICE 'V300141 改名完成：表/主键/2 个 CHECK/索引/触发器均为新名，列数 12';
END $$;

COMMIT;
