-- =====================================================================
-- FinvQuant PostgreSQL V300166：finv_futu_usc_map 时间列改为 dt_ 前缀（需求 11082）
--
-- 要求（用户 2026-11-10）：
--   `retire_at` / `compute_at` / `first_assign_at`
--   → `dt_retire` / `dt_compute` / `dt_first_assign`
--
-- 说明：只看前缀容易和「时刻（at）」混淆，统一成仓库既有的 `dt_` 前缀口径（与 dt_collect /
--   dt_create / dt_update 一致）。仅改列名，列类型、默认值、注释、数据全部保留；
--   引用这些列的 SQL 已同步改（uscrule/store.go），文档见 Docs/API/UscMap.md。
--
-- 幂等：仅当旧列存在、新列不存在时改名，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_renamed INT := 0;
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='retire_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='dt_retire') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN retire_at TO dt_retire;
        v_renamed := v_renamed + 1;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='compute_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='dt_compute') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN compute_at TO dt_compute;
        v_renamed := v_renamed + 1;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='first_assign_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='dt_first_assign') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN first_assign_at TO dt_first_assign;
        v_renamed := v_renamed + 1;
    END IF;

    RAISE NOTICE 'V300166：本次改名 % 列', v_renamed;
END $$;

COMMENT ON COLUMN futu.finv_futu_usc_map.dt_retire       IS '退市时间（status=RETIRED 时必填）';
COMMENT ON COLUMN futu.finv_futu_usc_map.dt_compute      IS '最近一次计算时间';
COMMENT ON COLUMN futu.finv_futu_usc_map.dt_first_assign IS '该 usc 首次分配给本证券的时间（跨期统计的时间下界）';

DO $$
DECLARE
    v_old INT;
    v_new INT;
    v_rows BIGINT;
BEGIN
    SELECT count(*) INTO v_old FROM information_schema.columns
     WHERE table_schema='futu' AND table_name='finv_futu_usc_map'
       AND column_name IN ('retire_at', 'compute_at', 'first_assign_at');
    SELECT count(*) INTO v_new FROM information_schema.columns
     WHERE table_schema='futu' AND table_name='finv_futu_usc_map'
       AND column_name IN ('dt_retire', 'dt_compute', 'dt_first_assign');
    SELECT count(*) INTO v_rows FROM futu.finv_futu_usc_map;

    IF v_old <> 0 THEN
        RAISE EXCEPTION 'V300166 校验失败：仍有 % 个 *_at 列名残留', v_old;
    END IF;
    IF v_new <> 3 THEN
        RAISE EXCEPTION 'V300166 校验失败：dt_ 列应有 3 个，实际 %', v_new;
    END IF;

    RAISE NOTICE 'V300166 完成：列名改为 dt_retire / dt_compute / dt_first_assign（现有 % 行数据保留）', v_rows;
END $$;

COMMIT;
