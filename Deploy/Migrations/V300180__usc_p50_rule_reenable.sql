-- =====================================================================
-- FinvQuant PostgreSQL V300180：恢复启用 P50 批量例外层（需求 11140）
--
-- 背景（用户 2026-11-12 要求）：
--   V300163 曾把 P50 整层停用（`flag_enable='0'`，需求 11079）；需求 11134 之后 P50 只保留
--   4 条单点例外（`CC.ANT` / `CC.ATOM` / `US.SPTSX` / `THA.SET`），其余 637 条已不随脚本发放。
--   用户现要求把该层改回启用：这 4 条按富途全码区分同名代码。
--
-- 影响：启用后这 4 条参与计算 —— `CC.ANT → CC.ANT`、`CC.ATOM → CC.ATOM`、
--   `US.SPTSX → US.SPTSX`、`THA.SET → THA.SET`（此前落 P20 市场默认取裸代码）。
--   本迁移**只改规则层开关**；结果表落库仍需走页面「预演 → 立即应用」（差异 4 行）。
--
-- 幂等：UPDATE 带目标值判断，重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

UPDATE futu.finv_futu_usc_rule
   SET flag_enable = '1',
       remark      = trim(both ' ' FROM COALESCE(remark, '') ||
                     ' | 2026-11-12 需求 11140 恢复启用（此前按需求 11079 整层停用）')
 WHERE rule_no = 'P50'
   AND flag_enable <> '1';

DO $$
DECLARE
    v_flag   CHAR(1);
    v_active BIGINT;
    v_total  BIGINT;
BEGIN
    SELECT flag_enable INTO v_flag FROM futu.finv_futu_usc_rule WHERE rule_no = 'P50';
    IF v_flag IS NULL THEN
        RAISE EXCEPTION 'V300180 校验失败：规则 P50 不存在';
    END IF;
    IF v_flag <> '1' THEN
        RAISE EXCEPTION 'V300180 校验失败：P50 的 flag_enable 应为 1，实际 %', v_flag;
    END IF;

    SELECT count(*) FILTER (WHERE status = 'ACTIVE'), count(*)
      INTO v_active, v_total
      FROM futu.finv_futu_usc_rule_item WHERE rule_no = 'P50';

    RAISE NOTICE 'V300180 完成：P50 已启用，生效条目 % 条 / 全部 % 条（结果表落库走页面预演与应用）',
        v_active, v_total;
END $$;

COMMIT;
