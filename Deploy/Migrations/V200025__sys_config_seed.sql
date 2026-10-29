-- =====================================================================
-- FinvQuant PostgreSQL V100025：sys.sys_config 初始配置数据（5 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），在所有结构迁移之后执行；依赖
--     V71（sys schema）与 V72（sys.sys_config 表结构）。
--   - 数据来源：ACANX 提供的原系统配置导出，config_no / 各字段值与时间戳
--     原样保留（gmt_create / gmt_modify 为原库时间，插入时显式赋值，
--     不受 BEFORE UPDATE 触发器影响）。
--   - data_type 按原系统约定：'1'=整型（取 int_value）、'2'=字符（取
--     char_value）。本批数据：1010/1031/5001/5002 为 '2'（char_value='1'，
--     开关型配置），1020 为 '1'（int_value=2000，分页上限）。
--   - 幂等策略：INSERT ... ON CONFLICT (config_no) DO NOTHING ——
--     重复执行不报错，也**不覆盖**生产环境已调整过的配置值（种子只负责首次铺设，
--     后续改动由运维/管理界面负责，需回滚到出厂值时另行显式 UPDATE）。
-- 迁移策略：单事务、失败整体回滚；末尾校验 5 个 config_no 均已存在。
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (1010, '系统前后端分离标志', '2', 0, '1', '', NULL, NULL, NULL,
     '2026-04-10 00:09:02.517058'::timestamp(6),
     '2026-04-10 00:18:56.716279'::timestamp(6),
     NULL, '1-启用；0-停用'),

    (1020, '默认最大单页展示记录数', '1', 2000, '', '', NULL, NULL, NULL,
     '2026-04-10 00:09:02.517058'::timestamp(6),
     '2026-04-10 00:22:26.461136'::timestamp(6),
     NULL, '默认最大单页展示记录数'),

    (1031, '1001配置', '2', 0, '1', '', NULL, NULL, NULL,
     '2026-04-10 00:09:02.517058'::timestamp(6),
     '2026-04-10 00:24:51.127744'::timestamp(6),
     NULL, '1001字符配置值'),

    (5001, '后端自定义分页插件启用标志', '2', 0, '1', '', NULL, NULL, NULL,
     '2026-04-10 00:09:02.517058'::timestamp(6),
     '2026-04-10 00:18:32.926497'::timestamp(6),
     NULL, '1-启用；0-停用'),

    (5002, '后端MyBatis分页缓存启用标志', '2', 0, '1', '', NULL, NULL, NULL,
     '2026-04-10 00:09:02.517058'::timestamp(6),
     '2026-04-10 00:18:35.260786'::timestamp(6),
     NULL, '1-启用；0-停用')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：5 个配置项齐备，且类型与关键值符合定义
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n        INT;
    bad      INT;
BEGIN
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no IN (1010, 1020, 1031, 5001, 5002);
    IF n <> 5 THEN
        RAISE EXCEPTION 'V100025: sys_config 种子数据不完整（实际 %，预期 5）', n;
    END IF;

    -- data_type '1' 应走 int_value，'2' 应走 char_value
    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE (config_no = 1020 AND NOT (data_type = '1' AND int_value = 2000))
        OR (config_no IN (1010, 1031, 5001, 5002) AND NOT (data_type = '2' AND char_value = '1'));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100025: sys_config 种子数据与预期类型/取值不符（% 行）', bad;
    END IF;

    RAISE NOTICE 'V100025: sys_config 种子数据就绪（5 条；已存在的 config_no 保持原值未覆盖）';
END $$;

COMMIT;
