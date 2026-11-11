-- =====================================================================
-- FinvQuant PostgreSQL V300204：open.open_api_endpoint 增加 catalog / func 两列（生成列）
--
-- 需求：11238（catalog / func 两列）、11239（func 改取 path 末段）、11241（与本文件合并）。
-- 授权依据：用户在 2026-11-11 会话中要求「open_api_endpoint 表增加 catalog / func 两列」，
--   并定稿语义：catalog = api_code 中**最后一个点之前**的部分，func = **最后一个点之后**的部分。
--   列名按规范拼写用 `catalog`（用户原话 catelog 为笔误）；
--   随后用户又定稿「func 取 path 最后一个 `/` 之后的部分，大驼峰」——
--   原先是另一条迁移（V300205）先建 func 再 DROP/重建，同一批次内未推送，此处直接合并为最终形态。
--
-- 为什么用**生成列（GENERATED ALWAYS AS … STORED）**而不是普通列：
--   这两列是 api_code 的纯派生值（`catalog || '.' || func = api_code` 恒成立）。
--   若做成普通列，就得靠每条 INSERT/UPDATE 手工同步——接口清单本来就要靠人维护，
--   再叠一层"忘了同步"的漂移风险没有必要。生成列让"不一致"在结构上不可能出现。
--   【红线】生成列不可显式写入：INSERT/UPDATE 语句不得列出 catalog / func
--   （与 `usc_lower`、`ftmm_symbol` 同规则，PostgreSQL 会直接报错）。
--
-- 派生表达式说明：
--   catalog：reverse + position 取最后一个点，left 切出前缀；
--   func   ：reverse + position 取最后一个斜杠，right 切出末段（大驼峰，如 PageUni / FTStockUpsert），
--            并用 CHECK `^[A-Z][A-Za-z0-9]*$` 收敛风格——路径末段不是大驼峰会直接写入失败。
--            为什么不在 SQL 里做大小写转换：无法把 `ftstockupsert` 智能还原成 `FTStockUpsert`
--            （会压平内部大写），项目路径规范本身要求末段 PascalCase，取末段 + 约束更可靠。
--   api_code 的 CHECK 保证「至少含一个点」、path 的 CHECK 保证「以 /OpenAPI/V1/ 开头」，
--   所以 position 必然 > 0，不会出现负数长度。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；列已存在时不动（含用户后来手工改过的情况）。
-- 收敛校验：只断言脚本自己能保证的结构事实——两列存在且为 STORED 生成列、
--   且当前数据的 `catalog || '.' || func` 与 api_code 完全一致。
-- 前置：V300193（接口清单，种子已收敛为最终编码）。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_endpoint
    ADD COLUMN IF NOT EXISTS catalog VARCHAR(48)
        GENERATED ALWAYS AS (left(api_code, length(api_code) - position('.' in reverse(api_code)))) STORED,
    ADD COLUMN IF NOT EXISTS func VARCHAR(48)
        GENERATED ALWAYS AS (right(path, position('/' in reverse(path)) - 1)) STORED;

-- 大驼峰约束：首字母大写 + 只含字母数字（`PageUni` / `FTStockUpsert` / `Issue`）
ALTER TABLE open.open_api_endpoint DROP CONSTRAINT IF EXISTS ck_open_api_endpoint_func_pascal;
ALTER TABLE open.open_api_endpoint ADD CONSTRAINT ck_open_api_endpoint_func_pascal
    CHECK (func ~ '^[A-Z][A-Za-z0-9]*$');

COMMENT ON COLUMN open.open_api_endpoint.catalog IS
    '功能目录 = api_code 最后一个点之前的部分（如 finv.quote.cdeu.min、open.auth.credential.sts）；生成列，由 api_code 派生，不可显式写入（需求 11238）';
COMMENT ON COLUMN open.open_api_endpoint.func IS
    '功能点 = api_code 最后一个点之后的部分（如 pageuni、ftstockupsert、issue）；生成列，由 api_code 派生，不可显式写入（需求 11238）';

DO $$
DECLARE
    v_cols INT;
    v_chk  BIGINT;
    v_bad  BIGINT;
BEGIN
    -- 1. 两列存在且为 STORED 生成列
    SELECT count(*) INTO v_cols
      FROM information_schema.columns
     WHERE table_schema = 'open' AND table_name = 'open_api_endpoint'
       AND column_name IN ('catalog', 'func')
       AND is_generated = 'ALWAYS';
    IF v_cols <> 2 THEN
        RAISE EXCEPTION 'V300204 校验失败：catalog / func 应为 2 个 STORED 生成列，实际 % 个', v_cols;
    END IF;

    -- 1b. func 的大驼峰 CHECK 已建立
    SELECT count(*) INTO v_chk FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE n.nspname = 'open' AND r.relname = 'open_api_endpoint'
       AND c.conname = 'ck_open_api_endpoint_func_pascal';
    IF v_chk <> 1 THEN
        RAISE EXCEPTION 'V300204 校验失败：ck_open_api_endpoint_func_pascal 约束缺失';
    END IF;

    -- 2. 派生结果正确性：catalog 拼回编码末段、func 等于 path 末段
    SELECT count(*) INTO v_bad
      FROM open.open_api_endpoint
     WHERE catalog || '.' || right(api_code, position('.' in reverse(api_code)) - 1) <> api_code
        OR func <> right(path, position('/' in reverse(path)) - 1);
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300204 校验失败：% 条记录的 catalog / func 派生结果不符合预期', v_bad;
    END IF;

    RAISE NOTICE 'V300204 就绪：catalog / func 已按「最后一个点」切分（当前 % 条）',
        (SELECT count(*) FROM open.open_api_endpoint);
    RAISE NOTICE 'V300204 提示：目录分布 %',
        (SELECT string_agg(DISTINCT catalog, ', ' ORDER BY catalog) FROM open.open_api_endpoint);
END $$;

COMMIT;
