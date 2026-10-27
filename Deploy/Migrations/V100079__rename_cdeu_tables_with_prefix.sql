-- =====================================================================
-- FinvQuant PostgreSQL V100079：修正 cdeu schema 的命名规则（需求 10755）
--
-- 背景：V100078 把命名规则理解成「只有 finv_dedup_ 前缀需要替换」，
-- 于是 12 张 cdeu_* 表沿用了原名。用户明确要求：
--   **cdeu_ 开头的表同样要把 cdeu_ 替换为 finv_quote_cdeu_。**
--
-- 正确规则（最终版）：
--   1. `cdeu_` 开头      → `finv_quote_cdeu_`（12 张）
--   2. `finv_dedup_` 开头 → `finv_quote_cdeu_`（13 张，V100078 已正确）
--   3. 其他表名保持不变（`finv_quote_secu_kline_min`）
--   4. `prep_bak_` 开头不在 cdeu 中创建
--
-- 本迁移只对 V100078 建错的 12 张表及其派生标识符（序列 / 约束 / 索引）做 RENAME，
-- **不 DROP、不重建**，表结构与数据完全不动（当前均为空表）。
--
-- 标识符映射（统一 +11 字符「finv_quote_」）：
--   cdeu_x          → finv_quote_cdeu_x
--   idx_cdeu_x      → idx_finv_quote_cdeu_x
--   uq_cdeu_x       → uq_finv_quote_cdeu_x
-- 改名后最长标识符 61 字符（cdeu_job_step_dynamic_dynamic_job_id_step_code_key），
-- 未超 PostgreSQL 的 63 字符上限。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_old bigint;
BEGIN
    SELECT count(*) INTO v_old FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'r' AND c.relname LIKE 'cdeu\_%';
    IF v_old = 0 THEN
        RAISE NOTICE 'V100079 提示：cdeu 下已无 cdeu_ 前缀的表，视为已修正，跳过';
    ELSE
        RAISE NOTICE 'V100079 前置校验：待重命名 % 张 cdeu_* 表', v_old;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 重命名表：cdeu_x → finv_quote_cdeu_x
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
    n bigint := 0;
BEGIN
    FOR r IN
        SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'cdeu' AND c.relkind = 'r' AND c.relname LIKE 'cdeu\_%'
         ORDER BY c.relname
    LOOP
        EXECUTE format('ALTER TABLE cdeu.%I RENAME TO %I', r.relname, 'finv_quote_' || r.relname);
        n := n + 1;
    END LOOP;
    RAISE NOTICE 'V100079 表重命名：% 张', n;
END $$;

-- ---------------------------------------------------------------------
-- 3. 重命名序列：cdeu_x → finv_quote_cdeu_x
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
    n bigint := 0;
BEGIN
    FOR r IN
        SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'cdeu' AND c.relkind = 'S' AND c.relname LIKE 'cdeu\_%'
         ORDER BY c.relname
    LOOP
        EXECUTE format('ALTER SEQUENCE cdeu.%I RENAME TO %I', r.relname, 'finv_quote_' || r.relname);
        n := n + 1;
    END LOOP;
    RAISE NOTICE 'V100079 序列重命名：% 个', n;
END $$;

-- ---------------------------------------------------------------------
-- 4. 重命名约束（PK / UNIQUE / CHECK / NOT NULL）
--    RENAME CONSTRAINT 会连带重命名其底层索引
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
    n bigint := 0;
BEGIN
    FOR r IN
        SELECT co.conname, c.relname AS table_name
          FROM pg_constraint co
          JOIN pg_namespace ns ON ns.oid = co.connamespace
          JOIN pg_class c ON c.oid = co.conrelid
         WHERE ns.nspname = 'cdeu'
           AND co.conname LIKE 'cdeu\_%'
         ORDER BY co.conname
    LOOP
        EXECUTE format('ALTER TABLE cdeu.%I RENAME CONSTRAINT %I TO %I',
                       r.table_name, r.conname, 'finv_quote_' || r.conname);
        n := n + 1;
    END LOOP;
    RAISE NOTICE 'V100079 约束重命名：% 个', n;
END $$;

-- ---------------------------------------------------------------------
-- 5. 重命名独立索引：idx_cdeu_x → idx_finv_quote_cdeu_x
--    （已被约束覆盖的 pkey / unique 索引不在此列，它们随约束一起改过）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r record;
    n bigint := 0;
BEGIN
    FOR r IN
        SELECT c.relname FROM pg_class c JOIN pg_namespace ns ON ns.oid = c.relnamespace
         WHERE ns.nspname = 'cdeu' AND c.relkind = 'i' AND c.relname LIKE 'idx\_cdeu\_%'
         ORDER BY c.relname
    LOOP
        EXECUTE format('ALTER INDEX cdeu.%I RENAME TO %I', r.relname, 'idx_finv_quote_' || substr(r.relname, 5));
        n := n + 1;
    END LOOP;
    RAISE NOTICE 'V100079 索引重命名：% 个', n;
END $$;

-- ---------------------------------------------------------------------
-- 6. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_tables   bigint;
    v_leftover bigint;
    v_seq      bigint;
    v_idx      bigint;
    v_con      bigint;
    v_missing  text;
BEGIN
    -- 6.1 表数量不变
    SELECT count(*) INTO v_tables FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'r';
    IF v_tables <> 26 THEN
        RAISE EXCEPTION 'V100079 收敛失败：cdeu 下应有 26 张表，实际 %', v_tables;
    END IF;

    -- 6.2 不得残留任何 cdeu_ 前缀的表 / 序列 / 索引 / 约束
    SELECT count(*) INTO v_leftover
      FROM (
        SELECT c.relname AS nm FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'cdeu' AND c.relkind IN ('r','S','i') AND c.relname LIKE 'cdeu\_%'
        UNION ALL
        SELECT co.conname FROM pg_constraint co JOIN pg_namespace n ON n.oid = co.connamespace
         WHERE n.nspname = 'cdeu' AND co.conname LIKE 'cdeu\_%'
        UNION ALL
        SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'cdeu' AND c.relkind = 'i' AND c.relname LIKE 'idx\_cdeu\_%'
      ) x;
    IF v_leftover > 0 THEN
        RAISE EXCEPTION 'V100079 收敛失败：仍残留 % 个 cdeu_ 前缀对象', v_leftover;
    END IF;

    -- 6.3 12 张改名后的表必须齐备
    SELECT string_agg(t, ',') INTO v_missing
      FROM unnest(ARRAY[
        'finv_quote_cdeu_workflow','finv_quote_cdeu_phase','finv_quote_cdeu_process',
        'finv_quote_cdeu_job','finv_quote_cdeu_job_step','finv_quote_cdeu_job_dynamic',
        'finv_quote_cdeu_job_step_dynamic','finv_quote_cdeu_workflow_run',
        'finv_quote_cdeu_phase_run','finv_quote_cdeu_process_run','finv_quote_cdeu_job_run',
        'finv_quote_cdeu_job_step_run'
      ]) t
     WHERE NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                        WHERE n.nspname = 'cdeu' AND c.relname = t AND c.relkind = 'r');
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V100079 收敛失败：缺少表 %', v_missing;
    END IF;

    SELECT count(*) INTO v_seq FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'S';
    SELECT count(*) INTO v_idx FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'i';
    SELECT count(*) INTO v_con FROM pg_constraint co JOIN pg_namespace n ON n.oid = co.connamespace
     WHERE n.nspname = 'cdeu';

    RAISE NOTICE 'V100079 完成：26 张表 / % 序列 / % 索引 / % 约束，无 cdeu_ 前缀残留', v_seq, v_idx, v_con;
END $$;

COMMIT;
