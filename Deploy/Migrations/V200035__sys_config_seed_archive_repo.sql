-- =====================================================================
-- FinvQuant PostgreSQL V100035：sys.sys_config 文件上传仓库配置（20004）
--
-- 决策（ACANX 2026-10-25）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与 V72
--     （sys.sys_config 表结构）。
--   - 新增一条配置：
--       20004 文件上传仓库配置
--             data_type='3'（字符串），**str_value 无默认值（NULL）**，
--             由用户自行配置；留空即视为未配置、不做上传。
--             取值格式：{owner}:{repo}:{branch}:{pre_path_key}
--               owner        —— 仓库所有者
--               repo         —— 仓库名
--               branch       —— 分支名
--               pre_path_key —— 文件保存的目录前缀
--             示例：ACANX:FinvQuant-Archive:main:Archive/FinvQuant/Monthly
--   - 与 20002/20003 配套：20002 控制是否自动上传、20003 提供 PAT 凭据、
--     20004 定位目标仓库与目录前缀。三者同时就绪才会真正上传。
--   - 存储位置同 20003：char_value 为 char(1) 放不下 `owner:repo:branch:path`
--     这种复合串，故用 data_type='3' / str_value（varchar(6000)）。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：仅使用
--     INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不写成 DO UPDATE；
--     本文件也不含任何针对 20004 的 UPDATE 语句，并用迁移前快照硬校验。
--   - 格式校验策略：str_value 为 NULL（未配置）时跳过；一旦用户已配置，
--     则要求按半角冒号切分恰好 4 段且各段非空——对已配置值只读校验、不改写。
--   - create_by='finv'（FinvQuant 服务端/迁移侧写入）。
--   - 幂等：ON CONFLICT DO NOTHING + 临时快照对比，重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾校验配置齐备、格式合法且已有值未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若已存在，记录当前值，用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_pre ON COMMIT DROP AS
SELECT config_no, char_value, str_value
  FROM sys.sys_config
 WHERE config_no = 20004;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20004, '文件上传仓库配置', '3', 0, '', NULL, NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '文件上传仓库配置，格式 {owner}:{repo}:{branch}:{pre_path_key}；owner=仓库所有者，repo=仓库名，branch=分支名，pre_path_key=文件保存的目录前缀（半角冒号分隔共 4 段）；留空表示未配置（不做上传）')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    n        INT;
    bad      INT;
    covered  INT;
    v        TEXT;
    segs     INT;
    sw_on    TEXT;
    pat_set  BOOLEAN;
BEGIN
    -- 3.1 配置存在
    SELECT count(*) INTO n
      FROM sys.sys_config
     WHERE config_no = 20004;
    IF n <> 1 THEN
        RAISE EXCEPTION 'V100035: 配置项 20004 缺失（实际 % 行，预期 1）', n;
    END IF;

    -- 3.2 字段口径（str_value 不校验内容：NULL 或用户配置后的非空值都合法）
    SELECT count(*) INTO bad
      FROM sys.sys_config
     WHERE config_no = 20004
       AND NOT (config_name = '文件上传仓库配置'
                AND data_type = '3'
                AND char_value = ''
                AND (str_value IS NULL OR length(btrim(str_value)) > 0));
    IF bad > 0 THEN
        RAISE EXCEPTION 'V100035: 20004 的字段口径与约定不符';
    END IF;

    -- 3.3 已配置时校验格式：恰好 4 段且各段非空（只读校验，不改写）
    --     注：此处只告警、不阻断——用户配置内容不合约定不应导致部署/升级失败，
    --     运行时由应用层读取时校验并向用户报错。
    SELECT str_value INTO v FROM sys.sys_config WHERE config_no = 20004;
    IF v IS NOT NULL THEN
        segs := array_length(string_to_array(v, ':'), 1);
        IF segs <> 4
           OR EXISTS (
                SELECT 1 FROM unnest(string_to_array(v, ':')) AS t(s)
                 WHERE btrim(t.s) = ''
              ) THEN
            RAISE WARNING
              'V100035: 20004 格式不合约定（当前 % 段，预期 {owner}:{repo}:{branch}:{pre_path_key} 共 4 段且非空）：%',
              segs, v;
        END IF;
    END IF;

    -- 3.4 红线：迁移前的已有值不得被覆盖
    SELECT count(*) INTO covered
      FROM tmp_sys_config_pre p
      JOIN sys.sys_config c USING (config_no)
     WHERE p.char_value IS DISTINCT FROM c.char_value
        OR p.str_value  IS DISTINCT FROM c.str_value;
    IF covered > 0 THEN
        RAISE EXCEPTION 'V100035: 覆盖了已存在的配置值（% 行），必须保持 DO NOTHING', covered;
    END IF;

    -- 3.5 配置完整性提示（不阻断迁移）
    SELECT char_value INTO sw_on FROM sys.sys_config WHERE config_no = 20002;
    SELECT count(*) > 0 INTO pat_set
      FROM sys.sys_config WHERE config_no = 20003 AND str_value IS NOT NULL;
    IF sw_on = '1' AND (NOT pat_set OR v IS NULL) THEN
        RAISE NOTICE 'V100035: 20002 已启用自动上传，但 20003(PAT)/20004(仓库配置) 尚未配置完整，上传不会生效';
    END IF;

    RAISE NOTICE 'V100035: 20004 文件上传仓库配置就绪（无默认值，待用户按 {owner}:{repo}:{branch}:{pre_path_key} 配置）';
END $$;

COMMIT;
