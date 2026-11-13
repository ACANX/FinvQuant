-- =====================================================================
-- V300104：补丁规则的目标 usc 补齐登记（需求 10935）
--
-- 背景：
--   启用补丁规则后，主表的行情会从旧 usc 迁到新 usc（如 SH000888 → SH0888）。
--   若新 usc 没有登记在 cdeu.finv_quote_cdeu_usc 里，就会出现「主表有行情、
--   登记表没有」的空档：页面看不到、导出缺元数据，只能等下一轮 DiscoverUsc 补。
--   本迁移把规则表里**全部规则**（启用与停用都算）的 usc_new 从 usc_old 复制登记补齐，
--   作为一次性存量修正：规则停用不代表以后不启用，提前登记好，启用时不会出现空档。
--   运行期由 PatchOutdatedUsc 与 SetUscPatchRuleEnable 自动做同样的事（自愈）。
--
-- 安全：
--   · ON CONFLICT (usc) DO NOTHING，已登记的 usc 一律不覆盖；
--   · 只复制元数据列，不涉及行情；priority_rank / sla_days / max_wait_days 保持 NULL
--     （满足三列同 NULL 或同非 NULL 的 CHECK 约束）。
-- =====================================================================

BEGIN;

WITH src AS (
    SELECT DISTINCT ON (p.usc_new)
           p.usc_new AS usc,
           u.category, u.day_expect_rows, u.date_list, u.date_delist,
           u.region, u.market, u.provider, u.name, u.timezone, u.currency
      FROM cdeu.finv_quote_cdeu_usc_patch_rule p
      JOIN cdeu.finv_quote_cdeu_usc u ON u.usc = p.usc_old
     ORDER BY p.usc_new, p.priority
)
INSERT INTO cdeu.finv_quote_cdeu_usc
    (usc, category, day_expect_rows, date_list, date_delist,
     flag_auto_discover, region, market, provider, name, timezone, currency)
SELECT usc, category, day_expect_rows, date_list, date_delist,
       false, region, market, provider, name, timezone, currency
  FROM src
ON CONFLICT (usc) DO NOTHING;

DO $$
DECLARE
    v_missing bigint;
BEGIN
    SELECT count(*) INTO v_missing
      FROM cdeu.finv_quote_cdeu_usc_patch_rule p
     WHERE EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc o WHERE o.usc = p.usc_old)
       AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc u WHERE u.usc = p.usc_new);
    RAISE NOTICE '完成：旧 usc 已登记但目标 usc 未登记的数量 = %（旧 usc 本身未登记的规则跳过）', v_missing;
    IF v_missing <> 0 THEN
        RAISE EXCEPTION '仍有 % 个目标 usc 未登记，回滚', v_missing;
    END IF;
END $$;

COMMIT;

-- 执行后核对
-- SELECT p.usc_old, p.usc_new, (u.usc IS NOT NULL) AS registered
--   FROM cdeu.finv_quote_cdeu_usc_patch_rule p
--   LEFT JOIN cdeu.finv_quote_cdeu_usc u ON u.usc = p.usc_new
--  ORDER BY p.usc_old;
