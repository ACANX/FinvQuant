-- =====================================================================
-- FinvQuant PostgreSQL V300148：cdeu.finv_quote_secu_indicator 补充 stock_id 字段（需求 11061）
--
-- 背景：上游 Supabase `public.finv_quote_secu_meta` 已有 `stock_id`（bigint，NOT NULL，10,910 行全非空），
--   本地上游镜像表 `V300144` 建表时把 14 个身份/标记列排除在外，`stock_id` 也在其中。
--   现按用户要求补列，并让「FT证券指标数据同步」（Tab 2）把它一起同步下来（131 → 132 列）。
--
-- 形态：只加列（DbSchemaSpec §0 允许），`stock_id bigint`，**可空**——
--   NULL 表示「尚未同步到」（与 `usc` 为空用 `ftmm_symbol` 占位是两回事，`stock_id` 无占位值，留 NULL 更诚实）；
--   同步时该列随上游值写入，存量行会在下一次 Tab 2 同步中自动收敛。
--
-- 顺带回填：用本地已有的映射把能定出的 `stock_id` 先填上，避免必须等一次全量同步：
--   ① `futu.finv_futu_secu_key`（按 `ftmm_symbol`，已同步到 19,811 行 stock_id）
--   ② `futu.finv_futu_quote_collect`（按生成列 `ftmm_symbol`，`V300147`）
--   只填 NULL 或与映射不同的行（`IS DISTINCT FROM` 守卫），重复执行为 no-op。
--
-- 幂等：ADD COLUMN / CREATE INDEX 均 IF NOT EXISTS；回填带 IS DISTINCT FROM 守卫；注释可重复覆盖。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚；末尾收敛校验（列存在 / 类型 / 注释 / 索引）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 加列
-- ---------------------------------------------------------------------
ALTER TABLE cdeu.finv_quote_secu_indicator
    ADD COLUMN IF NOT EXISTS stock_id bigint;

-- ---------------------------------------------------------------------
-- 2. 注释
-- ---------------------------------------------------------------------
COMMENT ON COLUMN cdeu.finv_quote_secu_indicator.stock_id IS
    '富途证券内部 ID（上游 public.finv_quote_secu_meta.stock_id，bigint；与 futu.finv_futu_quote_collect.stock_id 同口径）。'
    '可空：NULL = 尚未同步到该值（无占位口径）。由「FT证券指标数据同步」随其余列一起写入。';

-- ---------------------------------------------------------------------
-- 3. 索引：按 stock_id 反查指标行 / 与 futu 表关联用
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_finv_quote_secu_indicator_stock_id
    ON cdeu.finv_quote_secu_indicator (stock_id);

-- ---------------------------------------------------------------------
-- 4. 存量回填（能从本地映射定出的先填上；定不出的保持 NULL，等同步）
-- ---------------------------------------------------------------------
-- ① 本地证券键表：ftmm_symbol → stock_id
UPDATE cdeu.finv_quote_secu_indicator AS i
   SET stock_id = k.stock_id
  FROM futu.finv_futu_secu_key AS k
 WHERE k.stock_id IS NOT NULL
   AND i.ftmm_symbol = k.ftmm_symbol
   AND i.stock_id IS DISTINCT FROM k.stock_id;

-- ② 本地采集属性表（生成列 ftmm_symbol）：同名多行时取最小 stock_id（实测 75 组撞名，均为同一标的不同市场编码）
UPDATE cdeu.finv_quote_secu_indicator AS i
   SET stock_id = q.stock_id
  FROM (
        SELECT ftmm_symbol, min(stock_id) AS stock_id
          FROM futu.finv_futu_quote_collect
         GROUP BY ftmm_symbol
       ) AS q
 WHERE i.ftmm_symbol = q.ftmm_symbol
   AND i.stock_id IS DISTINCT FROM q.stock_id;

-- ---------------------------------------------------------------------
-- 5. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type     text;
    v_nullable text;
    v_comment  text;
    v_rows     bigint;
    v_filled   bigint;
    v_has_idx  boolean;
BEGIN
    SELECT data_type, is_nullable INTO v_type, v_nullable
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_indicator'
       AND column_name = 'stock_id';

    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300148 收敛失败：cdeu.finv_quote_secu_indicator.stock_id 不存在';
    END IF;
    IF v_type <> 'bigint' THEN
        RAISE EXCEPTION 'V300148 收敛失败：stock_id 类型为 %，期望 bigint', v_type;
    END IF;
    IF v_nullable <> 'YES' THEN
        RAISE EXCEPTION 'V300148 收敛失败：stock_id 应为可空（当前 is_nullable=%）', v_nullable;
    END IF;

    v_comment := col_description('cdeu.finv_quote_secu_indicator'::regclass,
        (SELECT ordinal_position FROM information_schema.columns
          WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_indicator'
            AND column_name = 'stock_id'));
    IF COALESCE(btrim(v_comment), '') = '' THEN
        RAISE EXCEPTION 'V300148 收敛失败：stock_id 缺少中文注释';
    END IF;

    SELECT to_regclass('cdeu.idx_finv_quote_secu_indicator_stock_id') IS NOT NULL INTO v_has_idx;
    IF NOT v_has_idx THEN
        RAISE EXCEPTION 'V300148 收敛失败：索引 idx_finv_quote_secu_indicator_stock_id 缺失';
    END IF;

    SELECT count(*), count(stock_id) INTO v_rows, v_filled FROM cdeu.finv_quote_secu_indicator;
    RAISE NOTICE 'V300148 完成：cdeu.finv_quote_secu_indicator 增加 stock_id（总 % 行，已回填 % 行，其余等同步；索引就绪）',
        v_rows, v_filled;
END $$;

COMMIT;
