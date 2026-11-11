-- =====================================================================
-- FinvQuant PostgreSQL V300190：STS 临时凭据改为**独立凭证行**（不再寄生在 AKSK 凭证下）
--
-- 需求：11202
-- 授权依据：用户在 2026-11-15 会话中明确要求「STS 凭证需要单独的凭证，不能寄生在 AKSK 认证模式的凭据下」。
--
-- 背景：V300191 之前的实现把 STS 临时凭据只放 Redis，审计里 credential_idn 直接记**父凭证** idn，
--   页面上看不到独立的凭证记录，也无法回答「这套 STS 是谁的、怎么来的」。
--
-- 改动（open.open_api_credential）：
--   ① auth_mode 取值集合加 'STS'；
--   ② 新增 parent_idn BIGINT NOT NULL DEFAULT 0 —— 派生来源凭证 idn（0 = 管理员页面直发，无父凭证）；
--   ③ 新增 token_fp VARCHAR(16) —— SecurityToken 的 sha256 前 16 位大写十六进制（STS 专用，其它模式 NULL）；
--   ④ 重建 ck_open_api_credential_secret：STS 与 AKSK 一样「有密文、无哈希」；
--   ⑤ 注释同步。
--
-- 说明：STS 行的 dt_expire 由签发方写为 now() + 2 小时，过期后由程序按需清理
--   （DELETE ... WHERE auth_mode='STS' AND dt_expire < now() - 保留期），不做软删。
--
-- 幂等：ADD COLUMN IF NOT EXISTS / 约束先 DROP IF EXISTS 再按新取值 ADD；重跑 0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：两个新列存在且类型正确；
--   auth_mode 约束含 'STS'；secret 约束接受 STS；数据计数只 RAISE NOTICE。
-- 前置：V300183 / V300187 / V300189。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_credential
    ADD COLUMN IF NOT EXISTS parent_idn BIGINT NOT NULL DEFAULT 0;

ALTER TABLE open.open_api_credential
    ADD COLUMN IF NOT EXISTS token_fp VARCHAR(16) NULL;

-- 约束：先删后建（改 auth_mode 取值集合必须重建）
ALTER TABLE open.open_api_credential DROP CONSTRAINT IF EXISTS ck_open_api_credential_mode;
ALTER TABLE open.open_api_credential ADD CONSTRAINT ck_open_api_credential_mode
    CHECK (auth_mode IN ('APIKEY','AKSK','STS','BEARER','MTLS','BASIC'));

ALTER TABLE open.open_api_credential DROP CONSTRAINT IF EXISTS ck_open_api_credential_secret;
ALTER TABLE open.open_api_credential ADD CONSTRAINT ck_open_api_credential_secret
    CHECK ( (auth_mode IN ('AKSK','STS') AND secret_cipher IS NOT NULL AND secret_hash IS NULL)
         OR (auth_mode NOT IN ('AKSK','STS') AND secret_hash   IS NOT NULL) );

COMMENT ON COLUMN open.open_api_credential.parent_idn IS
    '派生来源凭证 idn：STS 临时凭据记录它是从哪个长期凭证换来的（0 = 管理员页面直发，无父凭证）；其它模式恒为 0（需求 11202）';
COMMENT ON COLUMN open.open_api_credential.token_fp IS
    'STS SecurityToken 指纹：sha256(完整 token) 前 16 位大写十六进制；仅 auth_mode=STS 写入，其它模式 NULL（需求 11202）';
COMMENT ON COLUMN open.open_api_credential.auth_mode IS
    '认证模式：APIKEY 静态密钥 / AKSK 签名 / STS 临时凭据（2 小时，独立凭证行）/ BEARER JWT / MTLS 双向证书 / BASIC 用户名口令';
COMMENT ON COLUMN open.open_api_credential.secret_cipher IS
    'SK 密文（AES-256-GCM，密钥取 sys_config 19005、KID 取 19004）：HMAC 验签需取回明文，故只能加密不能哈希；AKSK 与 STS 必填，其它模式必须为 NULL';

CREATE INDEX IF NOT EXISTS idx_open_api_credential_parent
    ON open.open_api_credential (parent_idn)
    WHERE parent_idn > 0;

CREATE INDEX IF NOT EXISTS idx_open_api_credential_sts_expire
    ON open.open_api_credential (dt_expire)
    WHERE auth_mode = 'STS';

DO $$
DECLARE
    v_type TEXT;
    v_has_parent BOOLEAN;
    v_has_token BOOLEAN;
    v_chk BIGINT;
    v_sts BIGINT;
BEGIN
    SELECT data_type INTO v_type FROM information_schema.columns
     WHERE table_schema='open' AND table_name='open_api_credential' AND column_name='parent_idn';
    IF v_type IS NULL OR v_type NOT IN ('bigint') THEN
        RAISE EXCEPTION 'V300190 校验失败：parent_idn 缺失或类型不对（%）', v_type;
    END IF;

    SELECT data_type INTO v_type FROM information_schema.columns
     WHERE table_schema='open' AND table_name='open_api_credential' AND column_name='token_fp';
    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300190 校验失败：token_fp 缺失';
    END IF;

    SELECT count(*) INTO v_chk FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE n.nspname='open' AND r.relname='open_api_credential'
       AND c.conname='ck_open_api_credential_mode'
       AND pg_get_constraintdef(c.oid) LIKE '%STS%';
    IF v_chk = 0 THEN
        RAISE EXCEPTION 'V300190 校验失败：ck_open_api_credential_mode 未包含 STS';
    END IF;

    SELECT count(*) INTO v_chk FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE n.nspname='open' AND r.relname='open_api_credential'
       AND c.conname='ck_open_api_credential_secret'
       AND pg_get_constraintdef(c.oid) LIKE '%STS%';
    IF v_chk = 0 THEN
        RAISE EXCEPTION 'V300190 校验失败：ck_open_api_credential_secret 未接受 STS';
    END IF;

    SELECT count(*) INTO v_sts FROM open.open_api_credential WHERE auth_mode = 'STS';
    RAISE NOTICE 'V300190 就绪：STS 凭证行独立落库（当前 % 行）', v_sts;
END $$;

COMMIT;
