-- =====================================================================
-- FinvQuant PostgreSQL V300145：证券指标表改名 cdeu.finv_quote_secu_meta → finv_quote_secu_indicator（需求 11049）
--
-- 授权：表改名属 DbSchemaSpec §0 红线操作（RENAME），本次由用户**明确点名**：
--   「表名 finv_quote_secu_indicator」。
--
-- 背景：`V300144`（需求 11048）按用户当时提供的上游脚本建的是 `cdeu.finv_quote_secu_meta`，
--   列结构与上游 `public.finv_quote_secu_meta` 同构；用户随后确认**本地表名用
--   `finv_quote_secu_indicator`**（列、类型、可空性、主键、索引、触发器语义全部不变）。
--
-- 改动（全部幂等，先判存在再改）：
--   1. 表名：cdeu.finv_quote_secu_meta → cdeu.finv_quote_secu_indicator
--   2. 主键：finv_quote_secu_meta_pkey → finv_quote_secu_indicator_pkey
--   3. 索引：idx_finv_quote_secu_meta_ts → idx_finv_quote_secu_indicator_ts
--            idx_finv_quote_secu_meta_dt_update → idx_finv_quote_secu_indicator_dt_update
--   4. 触发器：finv_quote_secu_meta_set_dt_update → finv_quote_secu_indicator_set_dt_update
--   5. 触发器函数：cdeu.finv_quote_secu_meta_set_dt_update() → cdeu.finv_quote_secu_indicator_set_dt_update()
--   6. 表/列注释按新表名重写（`dt_update` 触发器说明同步更新）
--
-- 不回改：`V300144` 仍按旧名建表（历史迁移不改写，与本仓库 V300137→V300141 同一处理口径）；
--   空库按 144 → 145 顺序执行后，最终表名同样是 `cdeu.finv_quote_secu_indicator`。
--
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾收敛校验
--   （新表 131 列、旧表/旧约束/旧索引/旧触发器/旧函数全部消失、触发器行为自检）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 表改名（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('cdeu.finv_quote_secu_meta') IS NOT NULL
       AND to_regclass('cdeu.finv_quote_secu_indicator') IS NULL THEN
        ALTER TABLE cdeu.finv_quote_secu_meta RENAME TO finv_quote_secu_indicator;
        RAISE NOTICE 'V300145 表改名完成：cdeu.finv_quote_secu_meta → cdeu.finv_quote_secu_indicator';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 约束 / 索引 / 触发器 / 触发器函数改名（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    -- 主键约束
    IF EXISTS (SELECT 1 FROM pg_constraint
                WHERE conrelid = 'cdeu.finv_quote_secu_indicator'::regclass
                  AND conname = 'finv_quote_secu_meta_pkey')
       AND NOT EXISTS (SELECT 1 FROM pg_constraint
                        WHERE conrelid = 'cdeu.finv_quote_secu_indicator'::regclass
                          AND conname = 'finv_quote_secu_indicator_pkey') THEN
        ALTER TABLE cdeu.finv_quote_secu_indicator
            RENAME CONSTRAINT finv_quote_secu_meta_pkey TO finv_quote_secu_indicator_pkey;
    END IF;

    -- 索引：ts
    IF EXISTS (SELECT 1 FROM pg_indexes
                WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
                  AND indexname = 'idx_finv_quote_secu_meta_ts')
       AND NOT EXISTS (SELECT 1 FROM pg_indexes
                        WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
                          AND indexname = 'idx_finv_quote_secu_indicator_ts') THEN
        ALTER INDEX cdeu.idx_finv_quote_secu_meta_ts RENAME TO idx_finv_quote_secu_indicator_ts;
    END IF;

    -- 索引：dt_update
    IF EXISTS (SELECT 1 FROM pg_indexes
                WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
                  AND indexname = 'idx_finv_quote_secu_meta_dt_update')
       AND NOT EXISTS (SELECT 1 FROM pg_indexes
                        WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
                          AND indexname = 'idx_finv_quote_secu_indicator_dt_update') THEN
        ALTER INDEX cdeu.idx_finv_quote_secu_meta_dt_update RENAME TO idx_finv_quote_secu_indicator_dt_update;
    END IF;

    -- 触发器
    IF EXISTS (SELECT 1 FROM pg_trigger tg
                 JOIN pg_class t ON t.oid = tg.tgrelid
                 JOIN pg_namespace n ON n.oid = t.relnamespace
                WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_indicator'
                  AND tg.tgname = 'finv_quote_secu_meta_set_dt_update' AND NOT tg.tgisinternal)
       AND NOT EXISTS (SELECT 1 FROM pg_trigger tg
                        JOIN pg_class t ON t.oid = tg.tgrelid
                        JOIN pg_namespace n ON n.oid = t.relnamespace
                       WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_indicator'
                         AND tg.tgname = 'finv_quote_secu_indicator_set_dt_update' AND NOT tg.tgisinternal) THEN
        ALTER TRIGGER finv_quote_secu_meta_set_dt_update ON cdeu.finv_quote_secu_indicator
            RENAME TO finv_quote_secu_indicator_set_dt_update;
    END IF;

    -- 触发器函数（触发器按 OID 引用函数，改名不影响已建触发器）
    IF to_regprocedure('cdeu.finv_quote_secu_meta_set_dt_update()') IS NOT NULL
       AND to_regprocedure('cdeu.finv_quote_secu_indicator_set_dt_update()') IS NULL THEN
        ALTER FUNCTION cdeu.finv_quote_secu_meta_set_dt_update()
            RENAME TO finv_quote_secu_indicator_set_dt_update;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2.1 NOT NULL 约束改名（PostgreSQL 18 会把 NOT NULL 登记进 pg_constraint，
--     名字由建表时的表名派生，如 finv_quote_secu_meta_usc_not_null；一并改成新前缀）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT conname AS old_name,
               'finv_quote_secu_indicator' || substr(conname, length('finv_quote_secu_meta') + 1) AS new_name
          FROM pg_constraint
         WHERE conrelid = 'cdeu.finv_quote_secu_indicator'::regclass
           AND contype = 'n'
           AND conname LIKE 'finv_quote_secu_meta%'
    LOOP
        BEGIN
            EXECUTE format('ALTER TABLE cdeu.finv_quote_secu_indicator RENAME CONSTRAINT %I TO %I',
                           r.old_name, r.new_name);
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'V300145 NOT NULL 约束 % 改名跳过（%），不影响功能', r.old_name, SQLERRM;
        END;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 注释按新表名重写
-- ---------------------------------------------------------------------
COMMENT ON FUNCTION cdeu.finv_quote_secu_indicator_set_dt_update() IS
    'cdeu.finv_quote_secu_indicator 的 BEFORE UPDATE 触发器函数：把 dt_update 刷新为 now()（列结构与上游 public.finv_quote_secu_meta 同构）';

COMMENT ON TABLE cdeu.finv_quote_secu_indicator IS
    '证券指标表（列结构与上游 Supabase public.finv_quote_secu_meta 同构，131 列）：一行=一个证券的最新指标快照，主键 usc；'
    '只保最新、不建历史表；由「证券参数同步 → 证券指标数据同步」分批增量写入（需求 11047/11048/11049）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.usc IS '证券统一编码（主键，同步对齐键）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ftmm_symbol IS '富途 ftmm_symbol（如 US.AAPL；冗余，便于与 futu.finv_futu_secu_key 关联）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.ts IS '行情时间戳（epoch，上游原值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_collect IS '上游采集时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_create IS '上游创建时间（可空，同步写入上游值）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.dt_update IS '最后更新时间（可空；由触发器 finv_quote_secu_indicator_set_dt_update 在 UPDATE 时刷新）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.count_update IS '上游累计更新次数（原样同步）';
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.json_unmapped IS '上游未映射字段的原始 JSON（排查字段口径变化用）';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols    int;
    v_pk      int;
    v_pk_cols text;
    v_idx     int;
    v_trg     int;
    v_bad     text;
    v_before  timestamptz;
    v_after   timestamptz;
BEGIN
    IF to_regclass('cdeu.finv_quote_secu_meta') IS NOT NULL THEN
        RAISE EXCEPTION 'V300145 旧表名 cdeu.finv_quote_secu_meta 仍存在';
    END IF;
    IF to_regclass('cdeu.finv_quote_secu_indicator') IS NULL THEN
        RAISE EXCEPTION 'V300145 新表 cdeu.finv_quote_secu_indicator 不存在';
    END IF;

    SELECT count(*) INTO v_cols FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_indicator';
    IF v_cols <> 131 THEN
        RAISE EXCEPTION 'V300145 列数不符：期望 131，实际 %', v_cols;
    END IF;

    SELECT count(*), string_agg(a.attname, ',')
      INTO v_pk, v_pk_cols
      FROM pg_constraint c
      JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
     WHERE c.conrelid = 'cdeu.finv_quote_secu_indicator'::regclass AND c.contype = 'p';
    IF v_pk <> 1 OR v_pk_cols <> 'usc' THEN
        RAISE EXCEPTION 'V300145 主键不符：约束数=% 列=%', v_pk, v_pk_cols;
    END IF;

    SELECT count(*) INTO v_idx FROM pg_indexes
     WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
       AND indexname IN ('idx_finv_quote_secu_indicator_ts', 'idx_finv_quote_secu_indicator_dt_update');
    IF v_idx <> 2 THEN
        RAISE EXCEPTION 'V300145 新索引缺失：期望 2，实际 %', v_idx;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger tg
      JOIN pg_class t ON t.oid = tg.tgrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'cdeu' AND t.relname = 'finv_quote_secu_indicator'
       AND tg.tgname = 'finv_quote_secu_indicator_set_dt_update' AND NOT tg.tgisinternal;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V300145 新触发器缺失（实际 %）', v_trg;
    END IF;

    -- 旧名前缀的主键/CHECK 约束必须清零；NOT NULL 约束名仅作提示（改名失败不影响功能）
    SELECT string_agg(conname, ',') INTO v_bad FROM pg_constraint
     WHERE conrelid = 'cdeu.finv_quote_secu_indicator'::regclass
       AND contype IN ('p', 'c') AND conname LIKE 'finv_quote_secu_meta%';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300145 残留旧名主键/CHECK 约束：%', v_bad;
    END IF;
    SELECT string_agg(conname, ',') INTO v_bad FROM pg_constraint
     WHERE conrelid = 'cdeu.finv_quote_secu_indicator'::regclass
       AND contype = 'n' AND conname LIKE 'finv_quote_secu_meta%';
    IF v_bad IS NOT NULL THEN
        RAISE NOTICE 'V300145 仍有旧名前缀的 NOT NULL 约束名（不影响功能）：%', v_bad;
    END IF;
    SELECT string_agg(indexname, ',') INTO v_bad FROM pg_indexes
     WHERE schemaname = 'cdeu' AND tablename = 'finv_quote_secu_indicator'
       AND indexname LIKE 'idx_finv_quote_secu_meta%';
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300145 残留旧名索引：%', v_bad;
    END IF;
    IF to_regprocedure('cdeu.finv_quote_secu_meta_set_dt_update()') IS NOT NULL THEN
        RAISE EXCEPTION 'V300145 旧触发器函数仍存在：cdeu.finv_quote_secu_meta_set_dt_update()';
    END IF;

    -- 触发器行为自检（与 V300144 同口径）
    INSERT INTO cdeu.finv_quote_secu_indicator (usc, ftmm_symbol, name, dt_create, dt_update)
    VALUES ('__V300145_SELFCHECK__', 'US.SELFCHECK', '自检临时行', now() - interval '1 day', now() - interval '1 day');
    SELECT dt_create INTO v_before FROM cdeu.finv_quote_secu_indicator WHERE usc = '__V300145_SELFCHECK__';
    UPDATE cdeu.finv_quote_secu_indicator SET name = '自检临时行2' WHERE usc = '__V300145_SELFCHECK__';
    SELECT dt_update INTO v_after FROM cdeu.finv_quote_secu_indicator WHERE usc = '__V300145_SELFCHECK__';
    DELETE FROM cdeu.finv_quote_secu_indicator WHERE usc = '__V300145_SELFCHECK__';
    IF v_after IS NULL OR v_after <= now() - interval '1 hour' THEN
        RAISE EXCEPTION 'V300145 触发器未刷新 dt_update（值=%）', v_after;
    END IF;

    RAISE NOTICE 'V300145 改名完成：cdeu.finv_quote_secu_indicator 131 列 / 主键(usc) / 2 索引 / 触发器与函数均新名；自检通过（dt_create 保持 %）', v_before;
END $$;

COMMIT;
