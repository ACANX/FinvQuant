-- =====================================================================
-- FinvQuant PostgreSQL V100034：sys.sys_config 归档自动上传 Git 开关 + PAT（2 条）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与 V72
--     （sys.sys_config 表结构）。
--   - 新增两条配置：
--       20002 启用月归档数据自动上传Git仓库标志
--             data_type='2'（字符），char_value='0'（关闭），
--             remark: 0-关闭(默认) 1-启用
--       20003 归档数据上传GitHub仓库的PAT值
--             data_type='3'（字符串），**str_value 无默认值（NULL）**，
--             由用户自行配置；留空即视为未配置、不做自动上传。
--   - 为什么 20003 用 data_type='3' / str_value：
--     sys_config.char_value 定义为 char(1)，只能容纳 1 个字符，装不下 GitHub
--     PAT（classic 约 40 字符，fine-grained 可达 90+）；故按 V72 头注释中
--     '3'→str_value 的映射，把令牌放进 varchar(6000) 的 str_value。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：
--     仅使用 INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不能写成
--     DO UPDATE；本文件也**不含**任何针对 20002/20003 的 UPDATE 语句。
--     末尾还会用迁移前的快照（临时表）硬校验两行的值未被改写。
--   - 敏感信息：PAT 以明文存于 str_value，应用层读取后不得写入日志、不得在
--     接口响应中明文回显（建议掩码为前 4 位 + `****`）。
--   - create_by='finv'（FinvQuant 服务端/迁移侧写入；区别于 fqe 行情引擎）。
--   - 幂等：ON CONFLICT DO NOTHING + 临时快照对比，重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾校验两条配置齐备且已有值未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若两行已存在，记录当前值，用于后面验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_pre ON COMMIT DROP AS
SELECT config_no, char_value, str_value
  FROM sys.sys_config
 WHERE config_no IN (20002, 20003);

-- ---------------------------------------------------------------------
-- 2. 插入两条配置（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20002, '启用月归档数据自动上传Git仓库标志', '2', 0, '0', '', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '0-关闭(默认) 1-启用'),

    (20003, '归档数据上传GitHub仓库的PAT值', '3', 0, '', NULL, NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     'GitHub PAT（Personal Access Token），留空表示未配置（不做自动上传）；敏感信息，请勿在日志/前端明文回显')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n        INT;
    bad      INT;
    covered  INT;
BEGIN
    -- 3.1 两条配置齐备
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no IN (20002, 20003);
    IF n <> 2 THEN
        RAISE EXCEPTION 'V100034: 配置项不完整（实际 %，预期 2）', n;
    END IF;

    -- 3.2 字段口径（char_value 允许已被用户改为 0/1；str_value 不校验内容）
    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE (config_no = 20002
            AND NOT (config_name = '启用月归档数据自动上传Git仓库标志'
                     AND data_type = '2'
                     AND char_value IN ('0', '1')
                     AND remark = '0-关闭(默认) 1-启用'))
        OR (config_no = 20003
            AND NOT (config_name = '归档数据上传GitHub仓库的PAT值'
                     AND data_type = '3'
                     AND char_value = ''
                     -- 无默认值：NULL 或用户配置后的非空值都合法
                     AND (str_value IS NULL OR length(str_value) > 0)));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100034: 配置项内容与约定不符（% 行）', bad;
    END IF;

    -- 3.3 红线：迁移前的已有值不得被覆盖
    SELECT count(*) INTO covered
      FROM tmp_sys_config_pre p
      JOIN sys.sys_config c USING (config_no)
     WHERE p.char_value IS DISTINCT FROM c.char_value
        OR p.str_value  IS DISTINCT FROM c.str_value;
    IF covered > 0 THEN
        RAISE EXCEPTION 'V100034: 覆盖了已存在的配置值（% 行），必须保持 DO NOTHING', covered;
    END IF;

    RAISE NOTICE 'V100034: 20002（开关，默认 0）与 20003（PAT，无默认值待用户配置）就绪；已有值未被覆盖';
END $$;

COMMIT;
