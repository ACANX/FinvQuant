-- =====================================================================
-- FinvQuant PostgreSQL V100038：finv_dedup_usc.currency 按主数据刷新
--
-- 背景（ACANX 2026-10-27）：
--   finv_security 里部分证券的 currency_type 已修正（如 81810 小米集团-WR
--   人民币柜台：HKD → CNY），但 dedup.finv_dedup_usc.currency 仍停在旧值。
--   原因：RefreshSecurityMetadata 原先对 currency 采用「只补 NULL」策略，
--   主数据修正后不会覆盖已有值。
--
-- 本次调整：
--   1) 代码层：RefreshSecurityMetadata 的 currency 改为「以主数据为准覆盖」
--      （新值为 NULL 时保留旧值），与 region / market 同策略；name / timezone
--      仍保持只补缺，避免覆盖人工维护的名称与时区别名。
--   2) 数据层（本迁移）：按 finv_security.currency_type 重刷 usc.currency，
--      只更新实际有差异的行。
--   currency 会写入 MVsv 文件头 "# Currency" 行，务必在重新导出前对齐。
--
-- 幂等：UPDATE 带差异判断，重跑影响 0 行；末尾校验全库无 currency 偏差。
-- 迁移策略：单事务、失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 按主数据覆盖 currency
-- ---------------------------------------------------------------------
WITH computed AS (
    SELECT s.usc,
           NULLIF(s.currency_type, '') AS currency
      FROM public.finv_security s
     WHERE COALESCE(s.flag_enable, '1') = '1'
)
UPDATE dedup.finv_dedup_usc u
   SET currency = c.currency,
       gmt_update = now()
  FROM computed c
 WHERE c.usc = u.usc
   AND c.currency IS NOT NULL
   AND u.currency IS DISTINCT FROM c.currency;

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_bad INT;
    v_row RECORD;
BEGIN
    -- 2.1 全库 currency 已与主数据一致（主数据为空的行不参与）
    SELECT count(*) INTO v_bad
      FROM dedup.finv_dedup_usc u
      JOIN public.finv_security s ON s.usc = u.usc
     WHERE NULLIF(s.currency_type, '') IS NOT NULL
       AND COALESCE(u.currency, '') <> s.currency_type;
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100038: % 行的 currency 仍未与 finv_security.currency_type 对齐', v_bad;
    END IF;

    -- 2.2 currency 不应为空
    SELECT count(*) INTO v_bad
      FROM dedup.finv_dedup_usc
     WHERE NULLIF(currency, '') IS NULL;
    IF v_bad > 0 THEN
        RAISE EXCEPTION 'V100038: % 行的 currency 为空', v_bad;
    END IF;

    -- 2.3 人民币柜台样例：81810 应为 CNY
    SELECT usc, currency INTO v_row
      FROM dedup.finv_dedup_usc WHERE usc = '81810';
    IF FOUND AND v_row.currency <> 'CNY' THEN
        RAISE EXCEPTION 'V100038: 81810 的 currency 应为 CNY，实际 %', v_row.currency;
    END IF;

    RAISE NOTICE 'V100038: usc.currency 已按主数据对齐（81810 = %）',
        COALESCE((SELECT currency FROM dedup.finv_dedup_usc WHERE usc = '81810'), '(无该证券)');
END $$;

COMMIT;
