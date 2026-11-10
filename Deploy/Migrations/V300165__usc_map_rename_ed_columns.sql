-- =====================================================================
-- FinvQuant PostgreSQL V300165：USC 结果表列名去 `-ed` 形式（需求 11080）
--
-- 背景（用户 2026-11-10 要求）：
--   字段名一律不得使用 `-ed` 形式（过去分词）。V300157 建表时留下了 3 个这样的列：
--     `futu.finv_futu_usc_map.retired_at`        → `retire_at`
--     `futu.finv_futu_usc_map.computed_at`       → `compute_at`
--     `futu.finv_futu_usc_map.first_assigned_at` → `first_assign_at`
--   同时把代码侧的 JSON 字段一并改名（`added/changed/removed/reused/escalated/archived/
--   key_updated/key_matched/unchanged` → `insert/update/retire/reuse/escalate/archive/
--   key_update/key_match/keep`），前后端与文档统一。
--
-- 说明：结果表在改名时仍为空（尚未应用过），因此本迁移只改结构、不搬数据；
--   若将来在已有数据的库上重跑，`RENAME COLUMN` 会保留列内数据，同样安全。
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
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='retired_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='retire_at') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN retired_at TO retire_at;
        v_renamed := v_renamed + 1;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='computed_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='compute_at') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN computed_at TO compute_at;
        v_renamed := v_renamed + 1;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='first_assigned_at')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='futu' AND table_name='finv_futu_usc_map' AND column_name='first_assign_at') THEN
        ALTER TABLE futu.finv_futu_usc_map RENAME COLUMN first_assigned_at TO first_assign_at;
        v_renamed := v_renamed + 1;
    END IF;

    RAISE NOTICE 'V300165：本次改名 % 列', v_renamed;
END $$;

-- 列注释跟随新列名重写（旧注释已随列改名保留，这里统一口径）
COMMENT ON COLUMN futu.finv_futu_usc_map.retire_at       IS '退市时间（status=RETIRED 时必填；字段名不用 -ed 形式）';
COMMENT ON COLUMN futu.finv_futu_usc_map.compute_at      IS '最近一次计算时间';
COMMENT ON COLUMN futu.finv_futu_usc_map.first_assign_at IS '该 usc 首次分配给本证券的时间（跨期统计的时间下界）';

DO $$
DECLARE
    v_old INT;
    v_new INT;
BEGIN
    SELECT count(*) INTO v_old FROM information_schema.columns
     WHERE table_schema='futu' AND table_name='finv_futu_usc_map'
       AND column_name IN ('retired_at', 'computed_at', 'first_assigned_at');
    SELECT count(*) INTO v_new FROM information_schema.columns
     WHERE table_schema='futu' AND table_name='finv_futu_usc_map'
       AND column_name IN ('retire_at', 'compute_at', 'first_assign_at');

    IF v_old <> 0 THEN
        RAISE EXCEPTION 'V300165 校验失败：仍有 % 个 -ed 列名残留', v_old;
    END IF;
    IF v_new <> 3 THEN
        RAISE EXCEPTION 'V300165 校验失败：新列名应有 3 个，实际 %', v_new;
    END IF;

    RAISE NOTICE 'V300165 完成：finv_futu_usc_map 列名已去 -ed（retire_at / compute_at / first_assign_at）';
END $$;

COMMIT;
