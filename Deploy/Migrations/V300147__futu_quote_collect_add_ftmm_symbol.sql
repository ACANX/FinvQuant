-- =====================================================================
-- FinvQuant PostgreSQL V300147：futu.finv_futu_quote_collect 增加 ftmm_symbol 字段（需求 11060）
--
-- 授权依据：用户在会话中明确要求「finv_futu_quote_collect 表需要增加 ftmm_symbol 字段，
--   请帮我完成表结构的修改及脚本的发放」（DbSchemaSpec §0 允许的加列形态：只加列，不改既有列、不删不改表）。
--
-- 语义：ftmm_symbol 是上游 Supabase `public.finv_quote_secu_key` / `public.finv_quote_secu_meta`
--   的主键口径 `{market}.{code}`（如 `US.AAPL`、`CC.DASHTRY.FTUS`、`HK.00700`）。
--   本表此前把它拆成 `market_label` + `futu_symbol` 两列，跨表关联时必须现拼
--   `market_label || '.' || futu_symbol`（如 11057 的上游键表回填），既易漏也走不上索引；
--   加列后可直接 `= ` 关联。
--
-- 实现方式：**生成列（GENERATED ALWAYS AS ... STORED）**，不是普通列 + 触发器。
--   理由：值是纯派生（market_label、futu_symbol 都是 NOT NULL），生成列由 PG 自身维护，
--   天然不会与两列漂移，也无需触发器、无需回填脚本；ALTER 时自动为存量行计算。
--   代价：INSERT/UPDATE **不得显式写入** ftmm_symbol（会报
--   `cannot insert a non-DEFAULT value into column "ftmm_symbol"`）；本仓库内所有写入
--   （`V300063`/`V300131` 种子、`internal/meta/finv/quote/indicator/sql.go` 的 upsert、
--   两个集成测试）都是显式列清单且不含该列，兼容。
--
-- 唯一性：**不唯一**。market_label 与 market_code 非 1:1（CC 有 26 个、US 有 14 个 market_code），
--   实测 24,418 行里 75 组（150 行）重建后会撞名，例如 `CC.AAVEUSD` 同时来自
--   market_code=360（usc=CC.AAVEUSD）与 413（usc=AAVEUSD）。因此只建普通索引，不建唯一约束。
--
-- 幂等：ADD COLUMN / CREATE INDEX 均 IF NOT EXISTS；列已存在时校验其「生成列表达式」是否与预期
--   一致（attgenerated='s' 且表达式文本匹配），不一致直接报错回滚，避免静默留下错误定义。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 加列（生成列 + NOT NULL：两个来源列都 NOT NULL，派生值不可能为空）
-- ---------------------------------------------------------------------
ALTER TABLE futu.finv_futu_quote_collect
    ADD COLUMN IF NOT EXISTS ftmm_symbol text
        GENERATED ALWAYS AS (market_label || '.'::text || futu_symbol) STORED;

-- ---------------------------------------------------------------------
-- 2. 定义校验：必须是 STORED 生成列，且表达式与预期一致
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_generated "char";
    v_expr      text;
BEGIN
    SELECT a.attgenerated, pg_get_expr(d.adbin, d.adrelid)
      INTO v_generated, v_expr
      FROM pg_attribute a
      JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
     WHERE a.attrelid = 'futu.finv_futu_quote_collect'::regclass
       AND a.attname = 'ftmm_symbol';

    IF v_generated IS NULL THEN
        RAISE EXCEPTION 'V300147 收敛失败：ftmm_symbol 不是生成列（attgenerated 为空）';
    END IF;
    IF v_generated <> 's' THEN
        RAISE EXCEPTION 'V300147 收敛失败：ftmm_symbol 生成方式为 %，期望 STORED（s）', v_generated;
    END IF;
    -- 规范化后比对：PG 会把表达式重写成 `((market_label || '.'::text) || futu_symbol)`
    -- （自动补括号、给字面量加 `::text`），因此去掉空白与括号后只比对语义主体。
    v_expr := regexp_replace(regexp_replace(v_expr, '\s+', '', 'g'), '[()]', '', 'g');
    IF v_expr NOT IN ('market_label||''.''::text||futu_symbol',
                      'market_label||''.''||futu_symbol') THEN
        RAISE EXCEPTION 'V300147 收敛失败：ftmm_symbol 生成表达式为 %，期望 market_label || ''.'' || futu_symbol', v_expr;
    END IF;
END $$;

-- 生成列无法通过 ADD COLUMN 直接带 NOT NULL 时，这里补上（重复执行安全）
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
         WHERE attrelid = 'futu.finv_futu_quote_collect'::regclass
           AND attname = 'ftmm_symbol' AND attnotnull
    ) THEN
        ALTER TABLE futu.finv_futu_quote_collect ALTER COLUMN ftmm_symbol SET NOT NULL;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 注释（列中文说明，与指标表注释风格一致）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN futu.finv_futu_quote_collect.ftmm_symbol IS
    '富途统一符号 {市场}.{代码}（生成列 = market_label || ''.'' || futu_symbol，如 US.AAPL／CC.DASHTRY.FTUS／HK.00700；'
    '对应上游 public.finv_quote_secu_key.ftmm_symbol 口径，非唯一：同一 market_label 下存在多个 market_code，'
    '实测 75 组重建后撞名）。写入时不得显式赋值（GENERATED ALWAYS ... STORED）。';

-- ---------------------------------------------------------------------
-- 4. 索引：跨表按符号关联用（非唯一，见上文唯一性说明）
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_finv_futu_quote_collect_ftmm_symbol
    ON futu.finv_futu_quote_collect (ftmm_symbol);

-- ---------------------------------------------------------------------
-- 5. 收敛校验：存量行已全部按规则生成，且与表达式一致
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rows      bigint;
    v_not_null  bigint;
    v_mismatch  bigint;
    v_dup_groups int;
    v_has_index boolean;
BEGIN
    SELECT count(*), count(ftmm_symbol),
           count(*) FILTER (WHERE ftmm_symbol <> market_label || '.' || futu_symbol)
      INTO v_rows, v_not_null, v_mismatch
      FROM futu.finv_futu_quote_collect;

    IF v_rows <> v_not_null THEN
        RAISE EXCEPTION 'V300147 收敛失败：共 % 行，其中 ftmm_symbol 非空 % 行', v_rows, v_not_null;
    END IF;
    IF v_mismatch <> 0 THEN
        RAISE EXCEPTION 'V300147 收敛失败：% 行的 ftmm_symbol 与 market_label.futu_symbol 不一致', v_mismatch;
    END IF;

    SELECT count(*) INTO v_dup_groups FROM (
        SELECT ftmm_symbol FROM futu.finv_futu_quote_collect
         GROUP BY ftmm_symbol HAVING count(*) > 1) t;

    SELECT to_regclass('futu.idx_finv_futu_quote_collect_ftmm_symbol') IS NOT NULL INTO v_has_index;
    IF NOT v_has_index THEN
        RAISE EXCEPTION 'V300147 收敛失败：索引 idx_finv_futu_quote_collect_ftmm_symbol 缺失';
    END IF;

    RAISE NOTICE 'V300147 完成：futu.finv_futu_quote_collect 增加生成列 ftmm_symbol（% 行全部生成，非唯一，% 组撞名；索引就绪）',
        v_rows, v_dup_groups;
END $$;

COMMIT;
