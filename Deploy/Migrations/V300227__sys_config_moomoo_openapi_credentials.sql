-- =====================================================================
-- V300227：sys.sys_config 新增 moomoo OpenAPI 凭据配置项（需求 11318）
--
-- 用途：在「行情快照同步 → 加密货币」Tab 的配置入口中维护 moomoo OpenAPI 的
--       AppKey 与 Ed25519 私钥；保存到 sys.sys_config 后，收藏加密货币快照采集
--       优先使用这里的配置（其次才回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）。
--
-- 配置项（接 20015 号段，data_type='3' 即 str_value）：
--   20016  moomoo OpenAPI AppKey            （普通）
--   20017  moomoo OpenAPI 私钥（Ed25519 PEM） （敏感，access_level=3，接口不回显明文）
--
-- 初始数据：两行均以空串占位（= 未配置），由前端配置入口写入。
-- 幂等：ON CONFLICT (config_no) DO NOTHING；不改动已有值。
-- 回滚：DELETE FROM sys.sys_config WHERE config_no IN (20016, 20017);
-- =====================================================================

BEGIN;

INSERT INTO sys.sys_config
    (config_no, config_name, data_type, char_value, str_value, access_level, remark)
VALUES
    (20016,
     'moomoo OpenAPI AppKey',
     '3',
     '',
     '',
     2,
     'moomoo OpenAPI 的 AppKey ID（控制台 AppKey 管理创建）。用于「行情快照同步 → 加密货币」Tab 的收藏加密货币快照采集；留空表示未配置（回退环境变量 FINV_MOOMOO_APP_KEY）'),
    (20017,
     'moomoo OpenAPI 私钥（Ed25519 PEM）',
     '3',
     '',
     '',
     3,
     'moomoo OpenAPI 的 Ed25519 PKCS#8 私钥 PEM（含 BEGIN/END 行，可含字面 \\n）。与 20016 配对用于请求签名；留空表示未配置（回退环境变量 FINV_MOOMOO_PRIVATE_KEY）。敏感信息：接口只回显是否已配置，请勿在日志/前端明文回显')
ON CONFLICT (config_no) DO NOTHING;

-- 校验：两行必须存在（值为空也算已发放占位）
DO $$
DECLARE
    v_missing text;
BEGIN
    SELECT string_agg(no::text, ', ') INTO v_missing
      FROM unnest(ARRAY[20016, 20017]) AS no
     WHERE NOT EXISTS (SELECT 1 FROM sys.sys_config c WHERE c.config_no = no);
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300227 校验失败：缺少配置项 %', v_missing;
    END IF;
    RAISE NOTICE 'V300227 完成：sys_config 20016/20017 已登记（初始为空 = 未配置）';
END $$;

COMMIT;
