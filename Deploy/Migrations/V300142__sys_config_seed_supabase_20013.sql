-- =====================================================================
-- FinvQuant PostgreSQL V300142：sys.sys_config 新增 Supabase 数据库连接配置（20013）
--
-- 决策（ACANX 2026-11-09）：
--   - 新增一条系统配置：20013「Supabase 数据库连接配置」，data_type='3'（字符串），
--     取值存 `str_value`，格式固定为：
--         {PROJECT_ID}|{APIKEY}
--     两段之间用**半角竖线** `|` 分隔，均不得为空白：
--         PROJECT_ID   Supabase 项目 ID（即 `https://<PROJECT_ID>.supabase.co` 的子域，如 `abcdefghijklmnop`）
--         APIKEY       Supabase API Key（`sb_publishable_...` 或 `sb_secret_...`/service_role key）
--     示例（脱敏）：`abcdefghijklmnop|sb_publishable_xxxxxxxxxxxxxxxx`
--   - 为什么用字符串：一段 Project ID + 一段 Key，放 `char_value`（char(1)）显然不够，
--     也不适合拆两列（历史配置表只有 int/char/str/json 四类值列），故用 data_type='3'。
--   - 初始值留空（''）：表示**尚未配置**，系统按「不连接 Supabase」处理；
--     由运维在「系统配置」里填写，避免把密钥写进迁移脚本与 Git 历史。
--   - 敏感信息：`remark` 已标注「请勿在日志/前端明文回显」；读取方须自行脱敏。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：只用
--     INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不写 DO UPDATE；本文件不含任何 UPDATE。
--   - create_by='finv'（FinvQuant 服务端/迁移侧写入）。
--   - 幂等：ON CONFLICT DO NOTHING + 迁移前快照对比，重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾校验配置存在、data_type 正确、格式说明齐备、已有值未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若已存在，记录当前值，用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_20013_pre ON COMMIT DROP AS
SELECT config_no, data_type, str_value
  FROM sys.sys_config
 WHERE config_no = 20013;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20013, 'Supabase 数据库连接配置', '3', 0, '', '', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     'Supabase 数据库连接配置，格式 {PROJECT_ID}|{APIKEY}（半角竖线分隔 2 段）：PROJECT_ID=Supabase 项目 ID（https://<PROJECT_ID>.supabase.co 的子域）；APIKEY=Supabase API Key（sb_publishable_... / sb_secret_...）。留空表示未配置（不连接）；格式非法按未配置处理。敏感信息，请勿在日志/前端明文回显')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists  INT;
    v_type    TEXT;
    v_str     TEXT;
    v_remark  TEXT;
    v_pre     TEXT;
    v_now     TEXT;
BEGIN
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 20013;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V300142 配置 20013 未插入成功';
    END IF;

    SELECT data_type, COALESCE(str_value, ''), COALESCE(remark, '')
      INTO v_type, v_str, v_remark
      FROM sys.sys_config WHERE config_no = 20013;

    IF v_type <> '3' THEN
        RAISE EXCEPTION 'V300142 配置 20013 的 data_type 应为 3（字符串），实际 %', v_type;
    END IF;

    IF v_remark NOT LIKE '%PROJECT_ID%' OR v_remark NOT LIKE '%APIKEY%' THEN
        RAISE EXCEPTION 'V300142 配置 20013 的 remark 未写明 {PROJECT_ID}|{APIKEY} 格式（已存在行可能被外部改过，请人工核对）';
    END IF;

    -- 已有值不得被覆盖（迁移前存在且有值 → 迁移后必须仍是同值）
    SELECT str_value INTO v_pre FROM tmp_sys_config_20013_pre;
    IF v_pre IS NOT NULL AND btrim(v_pre) <> '' THEN
        v_now := btrim(COALESCE(v_str, ''));
        IF v_now <> btrim(v_pre) THEN
            RAISE EXCEPTION 'V300142 配置 20013 已有值被改动：迁移前=[%] 迁移后=[%]', v_pre, v_now;
        END IF;
    END IF;

    RAISE NOTICE 'V300142 配置 20013 就绪：data_type=3，当前值=%（留空=未配置）',
        CASE WHEN btrim(v_str) = '' THEN '<空>' ELSE '已配置（不回显）' END;
END $$;

COMMIT;
