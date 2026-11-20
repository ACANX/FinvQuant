-- =====================================================================
-- V300221：public.finv_security 增加 usn（统一证券编号）与 type_secu（证券类别）
--
-- 决策（用户 2026-11-18，需求 11306）：
--   · usn：统一证券编号，按内部编制规则生成；**当前可空**
--     （存量 51,398 行暂不回填，后续逐步严格约束、逐步写入数据）
--     → 本期 INTEGER NULL，不建唯一索引，不设默认值（NULL = 尚未分配）。
--   · type_secu：type_secu_sub 的**父类**（如 Stock / ETF / StockIndex）；
--     存量直接取 security_type 字段的值。
--
-- 位置：PostgreSQL ADD COLUMN 只能追加到表尾（物理位置）。
--       语义上 usn 属身份信息、type_secu 紧跟 security_type / type_secu_sub。
-- 幂等：ADD COLUMN IF NOT EXISTS；回填只更新空串，重复执行影响 0 行。
-- 回滚：ALTER TABLE public.finv_security DROP COLUMN usn, DROP COLUMN type_secu
--       （本迁移不建索引/约束/触发器，无其他对象依赖）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 加列（usn 可空；type_secu 沿用本表 TEXT NOT NULL DEFAULT '' 口径）
-- ---------------------------------------------------------------------
ALTER TABLE public.finv_security
    ADD COLUMN IF NOT EXISTS usn       INTEGER,
    ADD COLUMN IF NOT EXISTS type_secu TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------
-- 2. 列注释（口径写进库，避免后续再猜）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN public.finv_security.usn IS
    '统一证券编号（按内部编制规则生成；当前可空 = 尚未分配，后续逐步严格约束）';
COMMENT ON COLUMN public.finv_security.type_secu IS
    '证券类别（type_secu_sub 的父类，如 Stock / ETF / StockIndex；存量取自 security_type）';

-- ---------------------------------------------------------------------
-- 3. 存量回填 type_secu = security_type（只填空串，幂等）
--    usn 按决策暂不回填（全部保持 NULL）
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET type_secu = security_type
 WHERE NULLIF(btrim(type_secu), '') IS NULL;

-- ---------------------------------------------------------------------
-- 4. 校验：列存在 / 类型 / 可空性 / 回填一致
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type    text;
    v_notnull boolean;
    v_bad     bigint;
BEGIN
    SELECT format_type(atttypid, atttypmod), attnotnull
      INTO v_type, v_notnull
      FROM pg_attribute
     WHERE attrelid = 'public.finv_security'::regclass
       AND attname = 'usn'
       AND NOT attisdropped;
    IF v_type IS NULL OR v_type <> 'integer' OR v_notnull THEN
        RAISE EXCEPTION 'V300221 校验失败：usn 期望 integer 且可空，实际 % / notnull=%', v_type, v_notnull;
    END IF;

    SELECT format_type(atttypid, atttypmod), attnotnull
      INTO v_type, v_notnull
      FROM pg_attribute
     WHERE attrelid = 'public.finv_security'::regclass
       AND attname = 'type_secu'
       AND NOT attisdropped;
    IF v_type IS NULL OR v_type <> 'text' OR NOT v_notnull THEN
        RAISE EXCEPTION 'V300221 校验失败：type_secu 期望 text NOT NULL，实际 % / notnull=%', v_type, v_notnull;
    END IF;

    SELECT count(*) INTO v_bad
      FROM public.finv_security
     WHERE type_secu IS DISTINCT FROM security_type;
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V300221 校验失败：% 行 type_secu 与 security_type 不一致', v_bad;
    END IF;
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 5. 执行结果概览（便于迁移日志核对）
-- ---------------------------------------------------------------------
SELECT count(*)                                          AS rows_total,
       count(*) FILTER (WHERE usn IS NOT NULL)            AS usn_filled,
       count(*) FILTER (WHERE type_secu = security_type)  AS type_secu_ok
  FROM public.finv_security;
