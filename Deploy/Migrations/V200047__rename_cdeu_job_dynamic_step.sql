-- =====================================================================
-- FinvQuant PostgreSQL V100047：表名对齐 cdeu_job_dynamic_step → cdeu_job_step_dynamic
--
-- 决策依据：用户明确要求（2026-10-27）「cdeu_job_dynamic_step 表需要重命名为
-- cdeu_job_step_dynamic」。
--
-- 命名理由：
--   CDEU 的 Step 相关表有两张——定义/运行成对的 cdeu_job_step（Step 定义）与
--   cdeu_job_step_run（Step 运行记录）。动态 JOB 的 Step 表原先叫
--   cdeu_job_dynamic_step（"dynamic" 在 "step" 之前），与两者都不对齐；
--   改名为 cdeu_job_step_dynamic 后与 cdeu_job_step / cdeu_job_step_run 同族，
--   前缀统一为 cdeu_job_step_*。
--
-- 改名范围（旧名统一由 cdeu_dynamic_job_step_* / cdeu_job_dynamic_step 迁移到
-- cdeu_job_step_dynamic_*）：
--   表      cdeu_job_dynamic_step                  → cdeu_job_step_dynamic
--   序列    cdeu_dynamic_job_step_dynamic_step_id_seq
--                                                  → cdeu_job_step_dynamic_dynamic_step_id_seq
--   主键    cdeu_dynamic_job_step_pkey             → cdeu_job_step_dynamic_pkey
--   唯一    cdeu_dynamic_job_step_dynamic_job_id_seq_no_key
--                                                  → cdeu_job_step_dynamic_dynamic_job_id_seq_no_key
--   唯一    cdeu_dynamic_job_step_dynamic_job_id_step_code_key
--                                                  → cdeu_job_step_dynamic_dynamic_job_id_step_code_key
--   检查    cdeu_dynamic_job_step_status_check     → cdeu_job_step_dynamic_status_check
--   非空    cdeu_dynamic_job_step_<col>_not_null   → cdeu_job_step_dynamic_<col>_not_null
--   索引    idx_cdeu_job_dynamic_step_job          → idx_cdeu_job_step_dynamic_job
--
-- 安全性：
--   1. 幂等——若表已是新名，整段跳过；若旧表缺失且新表缺失才报错。
--   2. 约束/序列改名用 DO 块逐个探测存在性，缺名不报错。
--   3. 全程只改名字，不动数据、不动列定义、不重建表。
--
-- 注意：V63 / V100045 为历史已应用迁移，按规范不得修改，其内部的旧表名保持原样。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置状态检查
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('dedup.cdeu_job_dynamic_step') IS NULL
       AND to_regclass('dedup.cdeu_job_step_dynamic') IS NULL THEN
        RAISE EXCEPTION 'V100047 前置校验失败：dedup.cdeu_job_dynamic_step 与 dedup.cdeu_job_step_dynamic 同时不存在';
    END IF;

    IF to_regclass('dedup.cdeu_job_dynamic_step') IS NOT NULL
       AND to_regclass('dedup.cdeu_job_step_dynamic') IS NOT NULL THEN
        RAISE EXCEPTION 'V100047 前置校验失败：dedup.cdeu_job_dynamic_step 与 dedup.cdeu_job_step_dynamic 同时存在，需人工确认';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 表改名（已改名则跳过）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('dedup.cdeu_job_dynamic_step') IS NOT NULL THEN
        ALTER TABLE dedup.cdeu_job_dynamic_step RENAME TO cdeu_job_step_dynamic;
        RAISE NOTICE 'V100047：表已改名 dedup.cdeu_job_dynamic_step → dedup.cdeu_job_step_dynamic';
    ELSE
        RAISE NOTICE 'V100047：表已是 dedup.cdeu_job_step_dynamic，跳过表改名';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 序列改名
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('dedup.cdeu_dynamic_job_step_dynamic_step_id_seq') IS NOT NULL THEN
        ALTER SEQUENCE dedup.cdeu_dynamic_job_step_dynamic_step_id_seq
            RENAME TO cdeu_job_step_dynamic_dynamic_step_id_seq;
        RAISE NOTICE 'V100047：序列已改名 → cdeu_job_step_dynamic_dynamic_step_id_seq';
    ELSE
        RAISE NOTICE 'V100047：旧序列不存在，跳过';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 4. 约束改名（主键 / 唯一 / 检查）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    pair text[];
    pairs text[][] := ARRAY[
        ['cdeu_dynamic_job_step_pkey',                        'cdeu_job_step_dynamic_pkey'],
        ['cdeu_dynamic_job_step_dynamic_job_id_seq_no_key',   'cdeu_job_step_dynamic_dynamic_job_id_seq_no_key'],
        ['cdeu_dynamic_job_step_dynamic_job_id_step_code_key','cdeu_job_step_dynamic_dynamic_job_id_step_code_key'],
        ['cdeu_dynamic_job_step_status_check',                'cdeu_job_step_dynamic_status_check']
    ];
BEGIN
    FOREACH pair SLICE 1 IN ARRAY pairs LOOP
        IF EXISTS (
            SELECT 1
              FROM pg_constraint con
              JOIN pg_class c ON c.oid = con.conrelid
              JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'dedup'
               AND c.relname = 'cdeu_job_step_dynamic'
               AND con.conname = pair[1]
        ) THEN
            EXECUTE format('ALTER TABLE dedup.cdeu_job_step_dynamic RENAME CONSTRAINT %I TO %I',
                           pair[1], pair[2]);
            RAISE NOTICE 'V100047：约束已改名 % → %', pair[1], pair[2];
        ELSE
            RAISE NOTICE 'V100047：约束 % 不存在，跳过', pair[1];
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 5. 非空约束改名（PG 15+ 起 NOT NULL 在 pg_constraint 中独立存在）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r      record;
    v_old  text;
    v_new  text;
BEGIN
    FOR r IN
        SELECT con.conname
          FROM pg_constraint con
          JOIN pg_class c ON c.oid = con.conrelid
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'dedup'
           AND c.relname = 'cdeu_job_step_dynamic'
           AND con.contype = 'n'
           AND con.conname LIKE 'cdeu\_dynamic\_job\_step\_%\_not\_null'
         ORDER BY con.conname
    LOOP
        v_old := r.conname;
        v_new := 'cdeu_job_step_dynamic_' || substr(v_old, length('cdeu_dynamic_job_step_') + 1);
        EXECUTE format('ALTER TABLE dedup.cdeu_job_step_dynamic RENAME CONSTRAINT %I TO %I', v_old, v_new);
        RAISE NOTICE 'V100047：非空约束已改名 % → %', v_old, v_new;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 6. 索引改名
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('dedup.idx_cdeu_job_dynamic_step_job') IS NOT NULL THEN
        ALTER INDEX dedup.idx_cdeu_job_dynamic_step_job
            RENAME TO idx_cdeu_job_step_dynamic_job;
        RAISE NOTICE 'V100047：索引已改名 idx_cdeu_job_dynamic_step_job → idx_cdeu_job_step_dynamic_job';
    ELSE
        RAISE NOTICE 'V100047：旧索引不存在，跳过';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 7. 表注释同步
-- ---------------------------------------------------------------------
COMMENT ON TABLE dedup.cdeu_job_step_dynamic IS
    '动态 JOB（cdeu_job_dynamic）的串行 Step 运行记录；V100047 由 cdeu_job_dynamic_step 改名，命名与 cdeu_job_step / cdeu_job_step_run 同族。';

-- ---------------------------------------------------------------------
-- 8. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_old_table   text;
    v_new_table   text;
    v_seq         int;
    v_old_named   int;
    v_idx         text;
    v_pk          text;
    v_uk          int;
    v_ck          int;
BEGIN
    -- 8.1 表：旧名必须消失、新名必须存在
    v_old_table := to_regclass('dedup.cdeu_job_dynamic_step')::text;
    v_new_table := to_regclass('dedup.cdeu_job_step_dynamic')::text;
    IF v_old_table IS NOT NULL THEN
        RAISE EXCEPTION 'V100047 收敛失败：旧表 dedup.cdeu_job_dynamic_step 仍然存在';
    END IF;
    IF v_new_table IS NULL THEN
        RAISE EXCEPTION 'V100047 收敛失败：新表 dedup.cdeu_job_step_dynamic 不存在';
    END IF;

    -- 8.2 序列：新名必须存在
    IF to_regclass('dedup.cdeu_job_step_dynamic_dynamic_step_id_seq') IS NULL THEN
        RAISE EXCEPTION 'V100047 收敛失败：序列 cdeu_job_step_dynamic_dynamic_step_id_seq 不存在';
    END IF;

    -- 8.3 不得残留任何 cdeu_dynamic_job_step 开头的约束名
    SELECT count(*) INTO v_old_named
      FROM pg_constraint con
      JOIN pg_class c ON c.oid = con.conrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'dedup'
       AND c.relname = 'cdeu_job_step_dynamic'
       AND con.conname LIKE 'cdeu\_dynamic\_job\_step\_%';
    IF v_old_named <> 0 THEN
        RAISE EXCEPTION 'V100047 收敛失败：仍有 % 个约束沿用旧前缀 cdeu_dynamic_job_step_', v_old_named;
    END IF;

    -- 8.4 索引名必须为新名
    SELECT indexname INTO v_idx FROM pg_indexes
     WHERE schemaname = 'dedup' AND tablename = 'cdeu_job_step_dynamic'
       AND indexname = 'idx_cdeu_job_step_dynamic_job';
    IF v_idx IS NULL THEN
        RAISE EXCEPTION 'V100047 收敛失败：索引 idx_cdeu_job_step_dynamic_job 不存在';
    END IF;

    -- 8.5 主键名必须为新名
    SELECT con.conname INTO v_pk
      FROM pg_constraint con
      JOIN pg_class c ON c.oid = con.conrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'dedup' AND c.relname = 'cdeu_job_step_dynamic'
       AND con.contype = 'p';
    IF v_pk IS DISTINCT FROM 'cdeu_job_step_dynamic_pkey' THEN
        RAISE EXCEPTION 'V100047 收敛失败：主键名为 %，期望 cdeu_job_step_dynamic_pkey', v_pk;
    END IF;

    -- 8.6 唯一约束必须仍是 2 个（seq_no / step_code 双唯一，ON CONFLICT DO NOTHING 依赖）
    SELECT count(*) INTO v_uk
      FROM pg_constraint con
      JOIN pg_class c ON c.oid = con.conrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'dedup' AND c.relname = 'cdeu_job_step_dynamic'
       AND con.contype = 'u';
    IF v_uk <> 2 THEN
        RAISE EXCEPTION 'V100047 收敛失败：唯一约束数量为 %，期望 2', v_uk;
    END IF;

    -- 8.7 状态检查约束必须存在
    SELECT count(*) INTO v_ck
      FROM pg_constraint con
      JOIN pg_class c ON c.oid = con.conrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'dedup' AND c.relname = 'cdeu_job_step_dynamic'
       AND con.contype = 'c';
    IF v_ck <> 1 THEN
        RAISE EXCEPTION 'V100047 收敛失败：检查约束数量为 %，期望 1', v_ck;
    END IF;

    -- 8.8 序列归属列仍指向 dynamic_step_id
    SELECT count(*) INTO v_seq
      FROM pg_class seq
      JOIN pg_depend d ON d.objid = seq.oid AND d.deptype IN ('a','i')
      JOIN pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
      JOIN pg_class tbl ON tbl.oid = d.refobjid
      JOIN pg_namespace n ON n.oid = tbl.relnamespace
     WHERE seq.relname = 'cdeu_job_step_dynamic_dynamic_step_id_seq'
       AND n.nspname = 'dedup'
       AND tbl.relname = 'cdeu_job_step_dynamic'
       AND a.attname = 'dynamic_step_id';
    IF v_seq <> 1 THEN
        RAISE EXCEPTION 'V100047 收敛失败：序列与 dynamic_step_id 列的归属关系不成立';
    END IF;

    RAISE NOTICE 'V100047 收敛校验通过：表/序列/主键/唯一约束(2)/检查约束/索引 均已对齐 cdeu_job_step_dynamic';
END $$;

COMMIT;
