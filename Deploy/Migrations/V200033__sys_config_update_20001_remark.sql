-- =====================================================================
-- FinvQuant PostgreSQL V100033：sys.sys_config 20001 remark 文案修订
--
-- 决策（ACANX 2026-10-25）：
--   - 修订 20001「历史行情收集模式」的备注文案，明确两档语义与分隔符：
--       旧：0 被动收集（默认，外部程序通过暴露的API接口添加），1-主动采集模式，服务主动从外部源采集入库保存
--       新：0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存
--     变化点：`0 ` → `0-`；括号内不再包含「外部程序通过暴露的API接口添加」，
--     该说明移至括号后用「；」与第 2 档分隔。
--   - 只改 remark，不动 config_name / data_type / char_value / create_by。
--   - 触发器行为：gmt_modify 由 BEFORE UPDATE 触发器刷新；gmt_create 不变。
--   - 幂等：借助 IS DISTINCT FROM 只更新文案不同的行，重跑影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾校验文案与其它字段未被改动。
-- =====================================================================

BEGIN;

UPDATE sys.sys_config
   SET remark = '0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存'
 WHERE config_no = 20001
   AND remark IS DISTINCT FROM '0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存';

-- ---------------------------------------------------------------------
-- 收敛校验：文案已更新，其余字段保持不变
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n   INT;
    bad INT;
BEGIN
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no = 20001
       AND remark = '0-被动收集（默认），外部程序通过暴露的API接口添加；1-主动采集模式，服务主动从外部源采集入库保存';
    IF n <> 1 THEN
        RAISE EXCEPTION 'V100033: 20001 的 remark 未更新到位（匹配 % 行，预期 1）', n;
    END IF;

    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE config_no = 20001
       AND NOT (config_name = '历史行情收集模式'
                AND data_type = '2'
                AND char_value = '0'
                AND create_by = 'fqe'
                AND gmt_create IS NOT NULL
                AND gmt_modify IS NOT NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100033: 20001 的其它字段被意外改动（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100033: 20001 备注文案已更新（gmt_modify 已刷新，gmt_create 未变）';
END $$;

COMMIT;
