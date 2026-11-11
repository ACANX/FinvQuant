-- =====================================================================
-- FinvQuant PostgreSQL V300204：open.open_api_endpoint 增加 catalog / func 两列（生成列）
--
-- 需求：11238。
-- 授权依据：用户在 2026-11-11 会话中要求「open_api_endpoint 表增加 catalog / func 两列」，
--   并定稿语义：catalog = api_code 中**最后一个点之前**的部分，func = **最后一个点之后**的部分。
--   列名按规范拼写用 `catalog`（用户原话 catelog 为笔误）。
--
-- 为什么用**生成列（GENERATED ALWAYS AS … STORED）**而不是普通列：
--   这两列是 api_code 的纯派生值（`catalog || '.' || func = api_code` 恒成立）。
--   若做成普通列，就得靠每条 INSERT/UPDATE 手工同步——接口清单本来就要靠人维护，
--   再叠一层"忘了同步"的漂移风险没有必要。生成列让"不一致"在结构上不可能出现。
--   【红线】生成列不可显式写入：INSERT/UPDATE 语句不得列出 catalog / func
--   （与 `usc_lower`、`ftmm_symbol` 同规则，PostgreSQL 会直接报错）。
--
-- 派生表达式说明：
--   reverse + position 取最后一个点的位置，left/right 切两段；
--   api_code 的 CHECK 保证「至少含一个点」，所以 position 必然 > 0，不会出现负数长度。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；列已存在时不动（含用户后来手工改过的情况）。
-- 收敛校验：只断言脚本自己能保证的结构事实——两列存在且为 STORED 生成列、
--   且当前数据的 `catalog || '.' || func` 与 api_code 完全一致。
-- 前置：V300193（接口清单）、V300199~V300203（编码前缀与改名）。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_endpoint
    ADD COLUMN IF NOT EXISTS catalog VARCHAR(48)
        GENERATED ALWAYS AS (left(api_code, length(api_code) - position('.' in reverse(api_code)))) STORED,
    ADD COLUMN IF NOT EXISTS func VARCHAR(48)
        GENERATED ALWAYS AS (right(api_code, position('.' in reverse(api_code)) - 1)) STORED;

COMMENT ON COLUMN open.open_api_endpoint.catalog IS
    '功能目录 = api_code 最后一个点之前的部分（如 finv.quote.cdeu.min、open.auth.credential.sts）；生成列，由 api_code 派生，不可显式写入（需求 11238）';
COMMENT ON COLUMN open.open_api_endpoint.func IS
    '功能点 = api_code 最后一个点之后的部分（如 pageuni、ftstockupsert、issue）；生成列，由 api_code 派生，不可显式写入（需求 11238）';

DO $$
DECLARE
    v_cols INT;
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

    -- 2. 派生结果必须能拼回 api_code（生成列的唯一正确性判据）
    SELECT count(*) INTO v_bad
      FROM open.open_api_endpoint
     WHERE catalog || '.' || func <> api_code;
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300204 校验失败：% 条记录的 catalog.func 无法拼回 api_code', v_bad;
    END IF;

    RAISE NOTICE 'V300204 就绪：catalog / func 已按「最后一个点」切分（当前 % 条）',
        (SELECT count(*) FROM open.open_api_endpoint);
    RAISE NOTICE 'V300204 提示：目录分布 %',
        (SELECT string_agg(DISTINCT catalog, ', ' ORDER BY catalog) FROM open.open_api_endpoint);
END $$;

COMMIT;
