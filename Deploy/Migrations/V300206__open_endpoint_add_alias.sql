-- =====================================================================
-- FinvQuant PostgreSQL V300206：open.open_api_endpoint 增加 alias 列
--
-- 需求：11240。
-- 授权依据：用户在 2026-11-11 会话中要求「open_api_endpoint 表再加一个 alia 字段，
--   填入的内容是 {catalog}.{func}」。
--   列名按规范拼写用 `alias`（用户原话 alia / 前一轮 catelog 均为笔误，catelog 已按 catalog 落地）。
--
-- 值定义：`alias = catalog || '.' || func`
--   例：finv.quote.cdeu.min + PageUni → finv.quote.cdeu.min.PageUni
--       open.auth.credential.sts + Issue → open.auth.credential.sts.Issue
--
-- 实现注意（踩过的坑）：PostgreSQL **不允许生成列引用其它生成列**
--   （`ERROR: cannot use generated column "b" in column generation expression`），
--   所以这里不能写 `catalog || '.' || func`，必须把两个派生表达式**内联展开**。
--   结果等价于 catalog || '.' || func，收敛校验里再断言这条恒等式（SELECT 里可以引用生成列）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；重跑影响 0 行。
-- 收敛校验：断言列存在且为 STORED 生成列、且每条记录 alias = catalog || '.' || func。
-- 前置：V300193（接口清单）、V300204（catalog / func，func 已直接取 path 末段）。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_endpoint
    ADD COLUMN IF NOT EXISTS alias VARCHAR(96)
        GENERATED ALWAYS AS (
            left(api_code, length(api_code) - position('.' in reverse(api_code)))
            || '.' ||
            right(path, position('/' in reverse(path)) - 1)
        ) STORED;

COMMENT ON COLUMN open.open_api_endpoint.alias IS
    '别名 = {catalog}.{func}（如 finv.quote.cdeu.min.PageUni）；生成列，不可显式写入（需求 11240）';

DO $$
DECLARE
    v_gen TEXT;
    v_bad BIGINT;
BEGIN
    SELECT is_generated INTO v_gen FROM information_schema.columns
     WHERE table_schema = 'open' AND table_name = 'open_api_endpoint' AND column_name = 'alias';
    IF v_gen IS DISTINCT FROM 'ALWAYS' THEN
        RAISE EXCEPTION 'V300206 校验失败：alias 应为生成列，实际 is_generated=%', COALESCE(v_gen, 'NULL');
    END IF;

    -- 生成表达式内联展开，这里断言它确实等于 {catalog}.{func}
    SELECT count(*) INTO v_bad
      FROM open.open_api_endpoint
     WHERE alias IS DISTINCT FROM catalog || '.' || func;
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300206 校验失败：% 条记录的 alias 与 catalog.func 不一致', v_bad;
    END IF;

    RAISE NOTICE 'V300206 就绪：alias = {catalog}.{func}（当前 % 条）',
        (SELECT count(*) FROM open.open_api_endpoint);
    RAISE NOTICE 'V300206 提示：%',
        (SELECT string_agg(alias, ', ' ORDER BY idn) FROM open.open_api_endpoint);
END $$;

COMMIT;
