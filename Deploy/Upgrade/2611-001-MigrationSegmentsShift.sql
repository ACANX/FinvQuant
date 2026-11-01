-- =====================================================================
-- FinvQuant 一次性维护脚本：迁移号段平移（需求 10881）
--
-- 背景：
--   需求 10881 把迁移号段重排为三段：
--     V100000～V199999  基础准备（角色 / 数据库校验 / schema）
--     V200000～V299999  表结构 DDL      ← 原 V1xxxxx 整体 +100000
--     V300000～V399999  数据            ← 原 V2xxxxx 整体 +100000
--   迁移文件已全部重命名，**存量库的 `public.schema_version` 仍是旧号**
--   （100001～100089 / 200000～200080）。若不处理，服务重启后迁移器会认为
--   新号（200001～300080）全部未执行，从而对已有库重跑 170 个迁移。
--
-- 本脚本做什么：
--   把 `public.schema_version.version` 全部 `+100000`，使旧记录与新文件名对齐。
--
-- 什么时候执行：
--   **在部署/启动改造后的版本之前**，由具备 DDL 权限的账号手工执行一次：
--     psql "postgres://finv:<pwd>@<host>:5432/finv" -f 2611-001-MigrationSegmentsShift.sql
--   全新库（`schema_version` 为空）**无需执行**，直接跑全部迁移即可。
--   ⚠ 执行前库中不应已有基础准备段记录（`100001`～`100012`）：本脚本会把
--   `< 200000` 的行全部 `+100000`；已在改造后版本启动过的库无需再跑本脚本。
--
-- 幂等机制：
--   用 `public.schema_version_plan_shift` 作为 marker 表——
--   同一 `plan` 只允许插入一次，只有「本次插入成功」才执行 UPDATE；
--   重复执行输出「此前已执行，跳过」，不会二次 +100000。
--
-- 安全性：
--   - 只改纯数字版本号（`version ~ '^[0-9]+$'`）；
--   - 前后各打印一次版本区间与行数，便于人工核对；
--   - 单事务，失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. marker 表
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.schema_version_plan_shift (
    plan       TEXT        PRIMARY KEY,
    applied_on TIMESTAMPTZ NOT NULL DEFAULT now(),
    note       TEXT        NOT NULL DEFAULT ''
);

COMMENT ON TABLE public.schema_version_plan_shift IS
    '迁移号段方案标记表：记录已执行过的一次性号段平移方案，防止重复 +100000（需求 10881）';

-- ---------------------------------------------------------------------
-- 2. 执行平移（仅首次）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_inserted  integer;
    v_before_min text;
    v_before_max text;
    v_rows      integer;
    v_rows2     integer;
    v_after_min text;
    v_after_max text;
    v_nonnum    integer;
BEGIN
    SELECT count(*) INTO v_nonnum FROM public.schema_version WHERE version !~ '^[0-9]+$';
    IF v_nonnum <> 0 THEN
        RAISE EXCEPTION '号段平移中止：schema_version 中有 % 行版本号不是纯数字，需人工核对', v_nonnum;
    END IF;

    SELECT min(version::bigint)::text, max(version::bigint)::text
      INTO v_before_min, v_before_max
      FROM public.schema_version;

    INSERT INTO public.schema_version_plan_shift (plan, note)
    VALUES ('segments-v2-plus-100000',
            '旧 V1xxxxx→V2xxxxx、旧 V2xxxxx→V3xxxxx（需求 10881）')
    ON CONFLICT (plan) DO NOTHING;
    GET DIAGNOSTICS v_inserted = ROW_COUNT;

    IF v_inserted = 1 THEN
        -- 分批执行：单条 `+100000` 会自撞主键（旧 100077 → 200077，而旧 200077 尚未处理）。
        --   ① 先把旧数据段（>= 200000）→ 3xxxxx（此时 3xxxxx 区间为空）
        --   ② 再把旧 DDL 段（< 200000）→ 2xxxxx（此时 2xxxxx 区间已空）
        UPDATE public.schema_version
           SET version = (version::bigint + 100000)::text
         WHERE version ~ '^[0-9]+$'
           AND version::bigint >= 200000;
        GET DIAGNOSTICS v_rows = ROW_COUNT;

        UPDATE public.schema_version
           SET version = (version::bigint + 100000)::text
         WHERE version ~ '^[0-9]+$'
           AND version::bigint < 200000;
        GET DIAGNOSTICS v_rows2 = ROW_COUNT;
        v_rows := v_rows + v_rows2;

        SELECT min(version::bigint)::text, max(version::bigint)::text
          INTO v_after_min, v_after_max
          FROM public.schema_version;

        RAISE NOTICE '号段平移完成：% 行 +100000；平移前 % ~ %，平移后 % ~ %',
                     v_rows, v_before_min, v_before_max, v_after_min, v_after_max;
    ELSE
        RAISE NOTICE '号段平移此前已执行（marker=segments-v2-plus-100000），本次跳过；当前区间 % ~ %',
                     v_before_min, v_before_max;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2.1 同步 description（历史上写作 `V<n> migration`，版本号平移后需同改）
--     本步骤天然幂等：按当前 version 重算，任何状态执行都收敛到正确值。
-- ---------------------------------------------------------------------
UPDATE public.schema_version
   SET description = 'V' || version || ' migration'
 WHERE description ~ '^V[0-9]+ migration$'
   AND description <> 'V' || version || ' migration';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_old   integer;
    v_new   integer;
BEGIN
    -- 平移完成后不应再有旧号段（< 200000 且不是基础准备段 100001~100010）
    SELECT count(*) INTO v_old
      FROM public.schema_version
     WHERE version ~ '^[0-9]+$'
       AND version::bigint BETWEEN 100013 AND 199999;
    IF v_old <> 0 THEN
        RAISE EXCEPTION '号段平移校验失败：仍有 % 行落在旧 V1xxxxx 段（100013~199999）', v_old;
    END IF;

    -- 新 DDL 段与数据段应各有记录
    SELECT count(*) INTO v_new
      FROM public.schema_version
     WHERE version ~ '^[0-9]+$'
       AND version::bigint >= 200001;
    IF v_new = 0 THEN
        RAISE EXCEPTION '号段平移校验失败：schema_version 中没有任何新号段（>= 200001）记录';
    END IF;

    RAISE NOTICE '号段平移校验通过：新号段记录 % 行', v_new;
END $$;

COMMIT;
