-- =====================================================================
-- FinvQuant PostgreSQL V100031：sys.sys_config 新增 FinvQuote 运行/收集模式配置（2 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与
--     V72（sys.sys_config 表结构）。
--   - 新增两条**开关型字符配置**（data_type='2'，取值落 char_value），
--     默认值均为 '0'：
--       20000 FinvQuote服务运行模式    remark: 0-云端模式（默认） 1-本地模式
--       20001 历史行情收集模式         remark: 0 被动收集（默认，外部程序通过暴露的
--                                              API接口添加），1-主动采集模式，服务主动
--                                              从外部源采集入库保存
--   - 业务含义：20000 决定 FinvQuote 以云端/本地模式运行；20001 决定历史行情是
--     被动接收（外部程序调用 API 写入）还是主动采集（服务自行从外部源拉取入库）。
--     两项均只存标志，具体行为由后续服务端代码读取后实现。
--   - 时间戳取迁移执行时刻（CURRENT_TIMESTAMP(6)），非原库数据；其余可空列为 NULL。
--   - 幂等策略：INSERT ... ON CONFLICT (config_no) DO NOTHING ——
--     重复执行不报错，也不覆盖生产环境已调整过的配置值。
-- 迁移策略：单事务、失败整体回滚；末尾校验两条配置的值与备注。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20000, 'FinvQuote服务运行模式', '2', 0, '0', '', NULL, NULL, NULL,
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '0-云端模式（默认） 1-本地模式'),

    (20001, '历史行情收集模式', '2', 0, '0', '', NULL, NULL, NULL,
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '0 被动收集（默认，外部程序通过暴露的API接口添加），1-主动采集模式，服务主动从外部源采集入库保存')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：两条配置齐备，且类型/默认值/备注与约定一致
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n   INT;
    bad INT;
BEGIN
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no IN (20000, 20001);
    IF n <> 2 THEN
        RAISE EXCEPTION 'V100031: 配置项不完整（实际 %，预期 2）', n;
    END IF;

    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE (config_no = 20000
            AND NOT (config_name = 'FinvQuote服务运行模式'
                     AND data_type = '2'
                     AND char_value = '0'
                     AND remark = '0-云端模式（默认） 1-本地模式'))
        OR (config_no = 20001
            AND NOT (config_name = '历史行情收集模式'
                     AND data_type = '2'
                     AND char_value = '0'
                     AND remark = '0 被动收集（默认，外部程序通过暴露的API接口添加），1-主动采集模式，服务主动从外部源采集入库保存'));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100031: 配置项内容与约定不符（% 行；已有配置保持原值未覆盖）', bad;
    END IF;

    RAISE NOTICE 'V100031: FinvQuote 运行/收集模式配置就绪（20000=运行模式、20001=历史行情收集模式，默认均为 0）';
END $$;

COMMIT;
