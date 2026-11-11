-- =====================================================================
-- FinvQuant PostgreSQL V300205：open_api_endpoint.func 改为从 path 末段派生（大驼峰）
--
-- 需求：11239。
-- 授权依据：用户在 2026-11-11 会话中要求「func 字段改成 path 中最后一个 `/` 后面的部分，
--   采用大驼峰风格的字符串」。
--
-- 变更点（相对 V300204）：
--   旧：func = api_code 最后一个点之后的部分（如 pageuni / ftstockupsert，小写）
--   新：func = path 最后一个 `/` 之后的部分（如 PageUni / FTStockUpsert，大驼峰）
--   `catalog` 保持原样（仍取 api_code 最后一个点之前），用户只要求改 func。
--
-- 为什么用「取 path 末段 + CHECK 约束」而不是在 SQL 里做大小写转换：
--   项目 API 路径规范本身要求末段是 PascalCase（`.../Min/PageUni`、`.../STS/Issue`），
--   直接取末段就是大驼峰；而 SQL 无法"智能"地把 `ftstockupsert` 还原成 `FTStockUpsert`
--   （会丢内部大写）。所以这里取末段，并用 CHECK 强制风格（首字母大写、只含字母数字），
--   末段不合规时插入/更新直接失败——让风格问题在写入时暴露，而不是在派生列里猜。
--
-- 结构变更说明：生成列的表达式不能 ALTER，只能 DROP COLUMN + ADD COLUMN 重建。
--   func 是派生值（无独立数据），重建后由 path 重算，不丢信息，因此不需要备份行级数据。
--   单事务内完成「删列 → 建列 → 加 CHECK → 校验」，失败整体回滚。
--
-- 幂等：DROP COLUMN IF EXISTS + ADD COLUMN；重跑结果相同（重新派生一次，值不变）。
-- 收敛校验：断言 func 为 STORED 生成列、值等于 path 末段、全部满足大驼峰、CHECK 存在。
-- 前置：V300193（接口清单）、V300204（catalog / func 首次建立）。
-- =====================================================================

BEGIN;

-- 1. 删旧列（表达式基于 api_code），再按 path 末段重建
ALTER TABLE open.open_api_endpoint DROP COLUMN IF EXISTS func;

ALTER TABLE open.open_api_endpoint
    ADD COLUMN func VARCHAR(48)
        GENERATED ALWAYS AS (right(path, position('/' in reverse(path)) - 1)) STORED;

-- 2. 大驼峰约束：首字母大写 + 只含字母数字（`PageUni` / `FTStockUpsert` / `Issue`）
ALTER TABLE open.open_api_endpoint DROP CONSTRAINT IF EXISTS ck_open_api_endpoint_func_pascal;
ALTER TABLE open.open_api_endpoint ADD CONSTRAINT ck_open_api_endpoint_func_pascal
    CHECK (func ~ '^[A-Z][A-Za-z0-9]*$');

COMMENT ON COLUMN open.open_api_endpoint.func IS
    '功能点 = path 最后一个斜杠之后的部分（大驼峰，如 PageUni / FTStockUpsert / Issue）；生成列，由 path 派生，不可显式写入（需求 11239）';
COMMENT ON COLUMN open.open_api_endpoint.catalog IS
    '功能目录 = api_code 最后一个点之前的部分（如 finv.quote.cdeu.min、open.auth.credential.sts）；生成列，由 api_code 派生，不可显式写入（需求 11238）';

DO $$
DECLARE
    v_gen   TEXT;
    v_bad   BIGINT;
    v_chk   BIGINT;
BEGIN
    -- 1) func 必须是 STORED 生成列
    SELECT is_generated INTO v_gen FROM information_schema.columns
     WHERE table_schema = 'open' AND table_name = 'open_api_endpoint' AND column_name = 'func';
    IF v_gen IS DISTINCT FROM 'ALWAYS' THEN
        RAISE EXCEPTION 'V300205 校验失败：func 应为生成列，实际 is_generated=%', COALESCE(v_gen, 'NULL');
    END IF;

    -- 2) 值必须等于 path 末段
    SELECT count(*) INTO v_bad
      FROM open.open_api_endpoint
     WHERE func IS DISTINCT FROM right(path, position('/' in reverse(path)) - 1);
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300205 校验失败：% 条记录的 func 与 path 末段不一致', v_bad;
    END IF;

    -- 3) 大驼峰约束已建立
    SELECT count(*) INTO v_chk FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE n.nspname = 'open' AND r.relname = 'open_api_endpoint'
       AND c.conname = 'ck_open_api_endpoint_func_pascal';
    IF v_chk <> 1 THEN
        RAISE EXCEPTION 'V300205 校验失败：ck_open_api_endpoint_func_pascal 约束缺失';
    END IF;

    RAISE NOTICE 'V300205 就绪：func 改为 path 末段（大驼峰）';
    RAISE NOTICE 'V300205 提示：%',
        (SELECT string_agg(api_code || ' → ' || func, ', ' ORDER BY idn) FROM open.open_api_endpoint);
END $$;

COMMIT;
