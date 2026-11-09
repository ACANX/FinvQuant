-- =====================================================================
-- FinvQuant PostgreSQL V300164：P50 条目逐条停用（status=RETIRED，需求 11079）
--
-- 背景（用户 2026-11-10 反馈）：
--   V300163 只把规则层 `P50.flag_enable` 置 0，引擎确实不再命中（实测 `US.AESG → AESG`、
--   `CC.AMPL → CC.AMPL(ESCALATE)`），但**条目行的 status 仍是 ACTIVE**——
--   页面「规则与条目」里 P50 的 640 条看着还是绿色 ACTIVE，等于「看不出来停用了」。
--   因此本迁移把 P50 的条目也逐条置为 RETIRED，做到页面可见、口径一致。
--
-- 处理范围：`rule_no='P50'` 且 `status='ACTIVE'` 的 640 条。
--   已被用户手工停用的那 1 条（界面操作）保持原样，不覆盖其 remark。
--   `effective_to` 不写日期：这是「临时停用」，不是证券退市。
--
-- 恢复（两处都要还原）：
--   UPDATE futu.finv_futu_usc_rule_item
--      SET status = 'ACTIVE'
--    WHERE rule_no = 'P50' AND remark LIKE '%需求 11079 停用%';
--   UPDATE futu.finv_futu_usc_rule SET flag_enable = '1' WHERE rule_no = 'P50';
--
-- 幂等：只处理 status='ACTIVE' 的行，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

UPDATE futu.finv_futu_usc_rule_item
   SET status = 'RETIRED',
       remark = trim(both ' ' FROM COALESCE(remark, '') ||
                ' | 2026-11-10 需求 11079 停用（按用户要求整层停用，恢复时置回 ACTIVE）')
 WHERE rule_no = 'P50'
   AND status = 'ACTIVE';

DO $$
DECLARE
    v_active   BIGINT;
    v_retired  BIGINT;
    v_total    BIGINT;
BEGIN
    SELECT count(*) FILTER (WHERE status = 'ACTIVE'),
           count(*) FILTER (WHERE status = 'RETIRED'),
           count(*)
      INTO v_active, v_retired, v_total
      FROM futu.finv_futu_usc_rule_item WHERE rule_no = 'P50';

    IF v_active <> 0 THEN
        RAISE EXCEPTION 'V300164 校验失败：P50 仍有 % 条 ACTIVE 条目', v_active;
    END IF;
    IF v_total < 640 THEN
        RAISE EXCEPTION 'V300164 校验失败：P50 条目总数异常（%）', v_total;
    END IF;

    RAISE NOTICE 'V300164 完成：P50 条目 % 条全部 RETIRED（其中 % 条本次停用）', v_total, v_retired;
END $$;

COMMIT;
