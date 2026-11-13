-- =====================================================================
-- FinvQuant PostgreSQL V300139：futu.finv_quote_secu_key 增加 stock_id / usc（需求 11038）
--
-- 背景（用户 2026-11-09）：键表需要能挂上富途证券内部 ID 与我方统一证券代码，
--   便于同步任务从「instrument + market + code」直接定位到 stock_id / usc，
--   而不必每次都回查 finv_futu_quote_collect。
--
-- 口径：
--   · `stock_id BIGINT` 可空、无默认（即 NULL）——对应 `futu.finv_futu_quote_collect.stock_id`；
--   · `usc TEXT` 可空、无默认（即 NULL）——对应 `public.finv_security.usc`；
--   · 两列都**允许为空**：键表先登记待采集的代码（status='PENDING'），
--     采集成功后再回填 stock_id / usc；NULL 表示「尚未解析到」。
--   · 物理位置追加在表尾（PostgreSQL ADD COLUMN 语义），逻辑上紧随 ftmm_symbol 之后使用。
--
-- 幂等：`ADD COLUMN IF NOT EXISTS` + `COMMENT` 可重复执行；
--   末尾收敛校验（列存在 / 类型 / 可空 / 无默认值 / 列数 12）。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：V300137（建表）、V300138（status 取值约束）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 加列（幂等）
-- ---------------------------------------------------------------------
ALTER TABLE futu.finv_quote_secu_key
    ADD COLUMN IF NOT EXISTS stock_id BIGINT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS usc      TEXT   DEFAULT NULL;

-- ---------------------------------------------------------------------
-- 2. 列注释
-- ---------------------------------------------------------------------
COMMENT ON COLUMN futu.finv_quote_secu_key.stock_id IS
    '富途证券内部 ID（对应 futu.finv_futu_quote_collect.stock_id）；可空，NULL=尚未解析到';
COMMENT ON COLUMN futu.finv_quote_secu_key.usc IS
    '统一证券代码（对应 public.finv_security.usc）；可空，NULL=尚未解析到';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt  int;
    v_bad  text;
BEGIN
    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_quote_secu_key'
       AND column_name IN ('stock_id', 'usc');
    IF v_cnt <> 2 THEN
        RAISE EXCEPTION 'V300139 stock_id / usc 列缺失（命中 % 列，期望 2）', v_cnt;
    END IF;

    SELECT string_agg(column_name || ':' || data_type || ':' || is_nullable || ':' || COALESCE(column_default, '<none>'), ', ')
      INTO v_bad
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_quote_secu_key'
       AND column_name IN ('stock_id', 'usc')
       AND NOT (data_type IN ('bigint', 'text') AND is_nullable = 'YES' AND column_default IS NULL);
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION 'V300139 列定义不符（应为可空且无默认值）：%', v_bad;
    END IF;

    SELECT count(*) INTO v_cnt
      FROM information_schema.columns
     WHERE table_schema = 'futu' AND table_name = 'finv_quote_secu_key';
    IF v_cnt <> 12 THEN
        RAISE EXCEPTION 'V300139 列数不符：期望 12，实际 %', v_cnt;
    END IF;

    RAISE NOTICE 'V300139 stock_id / usc 已追加（均可空、默认 NULL），当前共 12 列';
END $$;

COMMIT;
