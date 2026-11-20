-- =====================================================================
-- FinvQuant PostgreSQL V300163：临时禁用 P50 批量例外（需求 11077 补充 / 用户 2026-11-10 要求）
--
-- 要求：「P50 中的规则先全部禁用掉。」
--
-- 做法：把规则层 `P50` 的 `flag_enable` 置 '0'——引擎按 `ru.flag_enable = '1'` 过滤条目，
--   一层开关即可让 P50 的 637 条例外全部不参与计算，无需逐条改 status（可随时一键恢复）。
--   条目本身保持 `ACTIVE`、不删除，便于后续挑着恢复。
--
-- 影响（恢复 P50 前需注意）：P50 关掉后，原先靠它加市场前缀的 637 行会落回市场默认
--   （P20 裸代码 / P21 全码），其中与其它市场同名的会再次撞车，由冲突升级兜到全码；
--   若升级后仍撞车，整次预演/应用会被拒绝（宁可报错也不让两家共用一个 usc）。
--
-- 幂等：UPDATE 带目标值判断，重跑影响 0 行。
-- 恢复：`UPDATE futu.finv_futu_usc_rule SET flag_enable='1' WHERE rule_no='P50';`
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

UPDATE futu.finv_futu_usc_rule
   SET flag_enable = '0',
       remark      = trim(both ' ' FROM COALESCE(remark, '') ||
                     ' | 2026-11-10 按用户要求临时禁用整层（条目保留 ACTIVE，恢复只需把 flag_enable 置 1）')
 WHERE rule_no = 'P50'
   AND flag_enable <> '0';

DO $$
DECLARE
    v_flag  CHAR(1);
    v_items BIGINT;
BEGIN
    SELECT flag_enable INTO v_flag FROM futu.finv_futu_usc_rule WHERE rule_no = 'P50';
    IF v_flag IS NULL THEN
        RAISE EXCEPTION 'V300163 校验失败：规则 P50 不存在';
    END IF;
    IF v_flag <> '0' THEN
        RAISE EXCEPTION 'V300163 校验失败：P50 的 flag_enable 应为 0，实际 %', v_flag;
    END IF;

    -- 条目保持 ACTIVE（只停层、不停条目），以便随时恢复
    SELECT count(*) INTO v_items FROM futu.finv_futu_usc_rule_item WHERE rule_no = 'P50' AND status = 'ACTIVE';
    IF v_items = 0 THEN
        RAISE EXCEPTION 'V300163 校验失败：P50 条目不应为空（本迁移只停层，不动条目）';
    END IF;

    RAISE NOTICE 'V300163 完成：P50 已停用（flag_enable=0），条目 % 条仍为 ACTIVE', v_items;
END $$;

COMMIT;
