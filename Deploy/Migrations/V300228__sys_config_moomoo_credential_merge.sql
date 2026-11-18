-- =====================================================================
-- V300228：moomoo OpenAPI 凭据合并为单配置项（需求 11319，修订 11318）
--
-- 背景：V300227 把凭据拆成 20016（AppKey）与 20017（私钥）两项，用户认为多余——
--       两项必须成对维护，拆开反而增加配置步骤。本迁移合并为一项：
--
--   20016  moomoo OpenAPI 凭据（AppKey|私钥 PEM）
--          值格式：{AppKey}|{Ed25519 PKCS#8 私钥 PEM}（半角竖线分隔；PEM 内不含竖线）
--          只配 AppKey 时写成 {AppKey}|；空串表示未配置
--
-- 20017 删除（值先合并进 20016，不丢已配置内容）。
-- 幂等：合并逻辑带「20017 存在且非空」判断；重跑影响 0 行。
-- 回滚：把 20016 按第一个 | 拆回两项（人工重建 20017）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_app   text;
    v_key   text;
    v_exist int;
BEGIN
    SELECT COALESCE(str_value, '') INTO v_app
      FROM sys.sys_config WHERE config_no = 20016;
    SELECT COALESCE(str_value, '') INTO v_key
      FROM sys.sys_config WHERE config_no = 20017;

    -- 把 20017 的私钥合并进 20016（AppKey 为空时也保留私钥，格式为 `|PEM`）
    IF COALESCE(btrim(v_key), '') <> '' THEN
        UPDATE sys.sys_config
           SET str_value = btrim(COALESCE(v_app, '')) || '|' || btrim(v_key)
         WHERE config_no = 20016;
    END IF;

    -- 若 20016 仍不存在（老库未执行 V300227），补一行空占位
    SELECT count(*) INTO v_exist FROM sys.sys_config WHERE config_no = 20016;
    IF v_exist = 0 THEN
        INSERT INTO sys.sys_config
            (config_no, config_name, data_type, char_value, str_value, access_level, remark)
        VALUES (20016,
                'moomoo OpenAPI 凭据（AppKey|私钥 PEM）',
                '3', '', '', 3,
                'moomoo OpenAPI 凭据，格式 {AppKey}|{Ed25519 PKCS#8 私钥 PEM}（半角竖线分隔）；只配 AppKey 时写成 {AppKey}|；留空表示未配置（回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）。敏感信息，请勿在日志/前端明文回显');
    ELSE
        UPDATE sys.sys_config
           SET config_name  = 'moomoo OpenAPI 凭据（AppKey|私钥 PEM）',
               access_level = 3,
               remark       = 'moomoo OpenAPI 凭据，格式 {AppKey}|{Ed25519 PKCS#8 私钥 PEM}（半角竖线分隔）；只配 AppKey 时写成 {AppKey}|；留空表示未配置（回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）。敏感信息，请勿在日志/前端明文回显'
         WHERE config_no = 20016;
    END IF;

    -- 删除已合并的 20017
    DELETE FROM sys.sys_config WHERE config_no = 20017;

    -- 校验：20017 不存在、20016 存在
    IF EXISTS (SELECT 1 FROM sys.sys_config WHERE config_no = 20017) THEN
        RAISE EXCEPTION 'V300228 校验失败：20017 未删除';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM sys.sys_config WHERE config_no = 20016) THEN
        RAISE EXCEPTION 'V300228 校验失败：20016 不存在';
    END IF;
    RAISE NOTICE 'V300228 完成：moomoo 凭据合并为单配置项 20016（AppKey|私钥 PEM）';
END $$;

COMMIT;
