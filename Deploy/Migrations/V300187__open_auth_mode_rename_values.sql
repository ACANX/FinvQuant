-- =====================================================================
-- FinvQuant PostgreSQL V300187：auth_mode 取值统一为 AKSK / APIKEY
--
-- 需求：11189（取值统一）
-- 授权依据：用户在 2026-11-15 会话中明确要求「表中记录的 auth_mode 的值需要统一将
--   AK_SK 改为 AKSK；将 API_KEY 改为 APIKEY」。
--
-- 背景：V300183 首次落表时用 `API_KEY` / `AK_SK`（下划线分隔）。对外展示与人工核对时
--   下划线容易被漏写/误写成 `API-Key`，故统一为**无分隔符大写**：`APIKEY` / `AKSK`。
--   `BEARER` / `MTLS` / `BASIC` 不变（本来就是单词）。
--
-- 改了什么：
--   ① open.open_api_credential.auth_mode 存量值：API_KEY → APIKEY、AK_SK → AKSK；
--   ② open.open_api_invoke_log.auth_mode 存量值同步（历史审计也要能按新口径聚合）；
--   ③ 重建 CHECK 约束（ck_open_api_credential_mode / ck_open_api_credential_secret /
--      ck_open_api_credential_sign_algorithm）与列注释；
--   ④ 凭证部分索引条件不涉及 auth_mode，无需处理。
--
-- 幂等：先用 CASE 只更新旧值（新值行 0 变更），约束先 DROP IF EXISTS 再 ADD，
--   重跑结果一致。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：
--   ① 表中不再存在 'API_KEY' / 'AK_SK'；② CHECK 约束存在且定义里是最新取值；
--   ③ 数据质量计数（两类模式各多少行）只 RAISE NOTICE，不阻断。
-- 前置：V300183（或等价表结构）。
-- =====================================================================

BEGIN;

-- ⓪ 先摘掉引用旧取值的约束：直接 UPDATE 会被旧 CHECK 拦下
--   （ck_open_api_credential_secret 形如「AK_SK 要求密文 / 非 AK_SK 要求哈希」，
--    改值过程中必然有一段不满足任一分支，所以必须「先删约束 → 改数据 → 再建约束」）
ALTER TABLE open.open_api_credential DROP CONSTRAINT IF EXISTS ck_open_api_credential_mode;
ALTER TABLE open.open_api_credential DROP CONSTRAINT IF EXISTS ck_open_api_credential_secret;
ALTER TABLE open.open_api_credential DROP CONSTRAINT IF EXISTS ck_open_api_credential_sign_algorithm;

-- ① 凭证表取值
UPDATE open.open_api_credential
   SET auth_mode = CASE auth_mode
                       WHEN 'API_KEY' THEN 'APIKEY'
                       WHEN 'AK_SK'   THEN 'AKSK'
                       ELSE auth_mode
                   END
 WHERE auth_mode IN ('API_KEY', 'AK_SK');

-- ② 审计表取值
UPDATE open.open_api_invoke_log
   SET auth_mode = CASE auth_mode
                       WHEN 'API_KEY' THEN 'APIKEY'
                       WHEN 'AK_SK'   THEN 'AKSK'
                       ELSE auth_mode
                   END
 WHERE auth_mode IN ('API_KEY', 'AK_SK');

-- ② 重建约束与注释（约束已在 ⓪ 删除，这里按新取值重建）
ALTER TABLE open.open_api_credential ADD CONSTRAINT ck_open_api_credential_mode
    CHECK (auth_mode IN ('APIKEY','AKSK','BEARER','MTLS','BASIC'));

ALTER TABLE open.open_api_credential ADD CONSTRAINT ck_open_api_credential_secret
    CHECK ( (auth_mode =  'AKSK' AND secret_cipher IS NOT NULL AND secret_hash IS NULL)
         OR (auth_mode <> 'AKSK' AND secret_hash   IS NOT NULL) );

ALTER TABLE open.open_api_credential ADD CONSTRAINT ck_open_api_credential_sign_algorithm
    CHECK ( (auth_mode = 'AKSK' AND sign_algorithm IN ('HMAC-SHA256'))
         OR (auth_mode <> 'AKSK' AND sign_algorithm IN ('HMAC-SHA256')) );

COMMENT ON TABLE  open.open_api_credential IS '外部 OpenAPI 凭证：一行 = 一种认证模式的凭证（APIKEY / AKSK / BEARER / MTLS / BASIC）';
COMMENT ON COLUMN open.open_api_credential.auth_mode IS '认证模式：APIKEY 静态密钥 / AKSK 签名 / BEARER JWT / MTLS 双向证书 / BASIC 用户名口令（需求 11189 起无下划线大写）';
COMMENT ON COLUMN open.open_api_credential.access_key_id IS '公开标识（唯一，英文字母一律大写）：APIKEY 模式为 key 本身（SA_LIVE_ + 32 位大写十六进制）；AKSK 模式为 AK（AK_LIVE_ + 32 位大写十六进制）；MTLS 模式为证书指纹';
COMMENT ON COLUMN open.open_api_credential.secret_hash IS '密钥哈希（不可逆）：APIKEY 模式为 sha256、BASIC 模式为 argon2id PHC 串；AKSK 模式必须为 NULL';
COMMENT ON COLUMN open.open_api_credential.secret_cipher IS 'SK 密文（AES-256-GCM，密钥取 sys_config 19005、KID 取 19004）：HMAC 验签需取回明文，故只能加密不能哈希；非 AKSK 模式必须为 NULL';

-- 收敛校验：只断言脚本可保证的结构事实
DO $$
DECLARE
    v_old_cred BIGINT;
    v_old_log  BIGINT;
    v_cnt_key  BIGINT;
    v_cnt_aksk BIGINT;
    v_has_check BIGINT;
BEGIN
    SELECT count(*) INTO v_old_cred FROM open.open_api_credential WHERE auth_mode IN ('API_KEY','AK_SK');
    SELECT count(*) INTO v_old_log  FROM open.open_api_invoke_log  WHERE auth_mode IN ('API_KEY','AK_SK');
    IF v_old_cred > 0 OR v_old_log > 0 THEN
        RAISE EXCEPTION 'V300187 校验失败：仍存在旧取值（credential=% 行，invoke_log=% 行）', v_old_cred, v_old_log;
    END IF;

    SELECT count(*) INTO v_has_check
      FROM pg_constraint c
      JOIN pg_class r ON r.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = r.relnamespace
     WHERE n.nspname = 'open' AND r.relname = 'open_api_credential'
       AND c.conname = 'ck_open_api_credential_mode'
       AND pg_get_constraintdef(c.oid) LIKE '%APIKEY%';
    IF v_has_check = 0 THEN
        RAISE EXCEPTION 'V300187 校验失败：ck_open_api_credential_mode 未包含新取值';
    END IF;

    -- 数据质量计数：只播报，不阻断
    SELECT count(*) INTO v_cnt_key  FROM open.open_api_credential WHERE auth_mode = 'APIKEY';
    SELECT count(*) INTO v_cnt_aksk FROM open.open_api_credential WHERE auth_mode = 'AKSK';
    RAISE NOTICE 'V300187 就绪：auth_mode 取值已统一为 APIKEY/AKSK（凭证 APIKEY=% 条、AKSK=% 条）', v_cnt_key, v_cnt_aksk;
END $$;

COMMIT;
