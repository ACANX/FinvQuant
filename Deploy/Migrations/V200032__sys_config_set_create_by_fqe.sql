-- =====================================================================
-- FinvQuant PostgreSQL V100032：sys.sys_config 20000/20001 的 create_by 指定为 fqe
--
-- 决策（ACANX 2026-10-25）：
--   - V100031 新增的两条配置（20000 FinvQuote服务运行模式、20001 历史行情
--     收集模式）首次落库时 create_by 为 NULL；本迁移把创建人补为 'fqe'
--     （FinvQuote 引擎侧写入标识）。
--   - 为什么不改 V100031：该迁移已在生产库应用（schema_version=100031），
--     按 DbSchemaSpec §4「历史迁移文件不可回改」，改文件对已应用库无效且会
--     造成新库/存量库不一致；故以独立迁移做存量修正。
--   - 触发器行为：sys_config 的 BEFORE UPDATE 触发器只刷新 gmt_modify，
--     gmt_create 保持不变（创建时间仅在插入时生成）。
--   - 幂等：只更新 create_by 尚非 'fqe' 的行，重复执行影响 0 行、无副作用。
-- 迁移策略：单事务、失败整体回滚；末尾校验两条配置的 create_by。
-- =====================================================================

BEGIN;

UPDATE sys.sys_config
   SET create_by = 'fqe'
 WHERE config_no IN (20000, 20001)
   AND (create_by IS NULL OR create_by <> 'fqe');

-- ---------------------------------------------------------------------
-- 收敛校验：两条配置的 create_by 均为 fqe，且 gmt_create 未被改写
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n    INT;
    bad  INT;
BEGIN
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no IN (20000, 20001)
       AND create_by = 'fqe';
    IF n <> 2 THEN
        RAISE EXCEPTION 'V100032: create_by 未全部更新为 fqe（实际 % 条，预期 2）', n;
    END IF;

    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE config_no IN (20000, 20001)
       AND (gmt_create IS NULL OR gmt_modify IS NULL);
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100032: 时间戳缺失（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100032: 20000/20001 的 create_by 已设为 fqe（gmt_create 未变，gmt_modify 已刷新）';
END $$;

COMMIT;
