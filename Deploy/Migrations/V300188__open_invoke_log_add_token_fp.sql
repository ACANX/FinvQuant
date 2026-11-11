-- =====================================================================
-- FinvQuant PostgreSQL V300188：open.open_api_invoke_log 增加 token_fp（STS 令牌指纹）
--
-- 需求：11191（STS 临时凭据：2 小时有效的 AK/SK + SecurityToken）
--
-- 背景：STS 的三要素里 SecurityToken 是敏感串，不能明文落审计；
--   但按会话排查「谁用哪一次临时凭据调的接口」需要可聚合的稳定标识，
--   因此存 token 的 sha256 前 16 位大写十六进制（不可逆、确定性、可 GROUP BY）。
--   长期 AK/SK 与静态 Key 不写该列（保持 NULL）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS。
-- 迁移策略：单事务（BEGIN/COMMIT）。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：列存在、类型为 varchar。
-- 说明：临时凭据本身**不落表**（存 Redis，TTL 2 小时），因此 open_api_credential 的
--   auth_mode CHECK 无需增加 'STS'（该取值只出现在审计表的 auth_mode 列）。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_invoke_log
    ADD COLUMN IF NOT EXISTS token_fp VARCHAR(16) NULL;

COMMENT ON COLUMN open.open_api_invoke_log.token_fp IS
    'STS SecurityToken 指纹：sha256(完整 token) 前 16 位大写十六进制（仅 auth_mode=STS 写入）；不可逆，用于按临时凭据聚合排查（需求 11191）';

CREATE INDEX IF NOT EXISTS idx_open_api_invoke_log_token_fp
    ON open.open_api_invoke_log (token_fp, dt_request DESC);

DO $$
DECLARE
    v_type TEXT;
BEGIN
    SELECT data_type INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'open' AND table_name = 'open_api_invoke_log'
       AND column_name = 'token_fp';
    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300188 校验失败：open.open_api_invoke_log.token_fp 不存在';
    END IF;
    IF v_type <> 'character varying' THEN
        RAISE EXCEPTION 'V300188 校验失败：token_fp 类型应为 character varying，实际 %', v_type;
    END IF;
    RAISE NOTICE 'V300188 就绪：审计表已增加 token_fp 与索引';
END $$;

COMMIT;
