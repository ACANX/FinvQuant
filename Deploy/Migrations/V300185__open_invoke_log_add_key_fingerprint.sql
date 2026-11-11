-- =====================================================================
-- FinvQuant PostgreSQL V300185：open.open_api_invoke_log 增加 key_fingerprint（密钥指纹）
--
-- 需求：11186
--
-- 背景：静态 API Key 是密钥本身，审计里只存脱敏形态（`<前缀><前 8><等量星号><后 8>`）。
--   脱敏形态只保留部分位，理论上无法**严格**区分「到底是哪一把 Key」
--   （同一凭证轮换前后、或前缀/尾段巧合的两把 Key 会落到同一掩码），也无法按
--   「调用方报上来的完整 Key」反查历史记录。
--
-- 做法：新增 `key_fingerprint`，存 sha256(完整 Key) 的前 16 位大写十六进制。
--   - 不可逆：不能从指纹还原密钥，导出日志/备份不带明文；
--   - 确定性：同一 Key 恒为同一指纹，可 GROUP BY / 建索引 / 精确 WHERE；
--   - 长度固定 16 字符，碰撞概率可忽略（16^16 空间 + 哈希均匀性）。
--   只对 `API_KEY`（静态 Key）模式写入；`AK_SK` 模式的 AK 本来就是公开标识、已存明文。
--
-- 幂等：ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS，重跑 0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：列存在、类型为 varchar、索引存在。
-- 存量行不刷历史指纹（无法从脱敏形态反推完整 Key），保持 NULL。
-- 前置：V300183/V300184（open.open_api_invoke_log）。
-- =====================================================================

BEGIN;

ALTER TABLE open.open_api_invoke_log
    ADD COLUMN IF NOT EXISTS key_fingerprint VARCHAR(16) NULL;

COMMENT ON COLUMN open.open_api_invoke_log.key_fingerprint IS
    '密钥指纹：sha256(完整 Key) 前 16 位大写十六进制（仅 API_KEY 模式写入）；不可逆，用于严格区分密钥与按 Key 反查（需求 11186）';

CREATE INDEX IF NOT EXISTS idx_open_api_invoke_log_key_fingerprint
    ON open.open_api_invoke_log (key_fingerprint, dt_request DESC);

DO $$
DECLARE
    v_type TEXT;
    v_has_index BOOLEAN;
BEGIN
    SELECT data_type INTO v_type
      FROM information_schema.columns
     WHERE table_schema = 'open'
       AND table_name = 'open_api_invoke_log'
       AND column_name = 'key_fingerprint';

    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300185 校验失败：open.open_api_invoke_log.key_fingerprint 不存在';
    END IF;
    IF v_type <> 'character varying' THEN
        RAISE EXCEPTION 'V300185 校验失败：key_fingerprint 类型应为 character varying，实际 %', v_type;
    END IF;

    SELECT to_regclass('open.idx_open_api_invoke_log_key_fingerprint') IS NOT NULL INTO v_has_index;
    IF NOT v_has_index THEN
        RAISE EXCEPTION 'V300185 校验失败：索引 idx_open_api_invoke_log_key_fingerprint 不存在';
    END IF;

    RAISE NOTICE 'V300185 就绪：open.open_api_invoke_log.key_fingerprint 与索引已建立';
END $$;

COMMIT;
