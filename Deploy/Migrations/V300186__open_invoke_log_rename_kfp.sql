-- =====================================================================
-- FinvQuant PostgreSQL V300186：open.open_api_invoke_log 的密钥指纹列改名为 kfp
--
-- 需求：11187
-- 授权依据：用户在 2026-11-15 会话中明确要求「key_fingerprint 字段改为 kfp」。
--
-- 背景：V300185（需求 11186）首次落列时用的名字是 `key_fingerprint`，用户定稿为 `kfp`（短名，
--   审计表列多，短名更省版面）。V300185 尚未推送到任何环境、且与本迁移同属一个需求批次，
--   故 V300185 **同批回改**为 `kfp`（文件名同步为 `V300185__open_invoke_log_add_kfp.sql`）；
--   本迁移负责把「已经应用过旧版 V300185」的库（本机、测试库等）就地改名，不是重建列。
--
-- 改了什么：列名 + 列注释 + 索引名（数据与类型不动）
--   open.open_api_invoke_log.key_fingerprint / key_fp  → kfp
--   idx_open_api_invoke_log_key_fingerprint / _key_fp  → idx_open_api_invoke_log_kfp
--
-- 幂等：只在「旧列存在且新列不存在」时改名；重跑进入 ELSE 分支，0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：
--   ① 新列存在且类型为 varchar；② 表上不再残留 key_fingerprint / key_fp 列。
-- 前置：V300185（或它的旧版）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_type TEXT;
BEGIN
    -- 旧名 key_fingerprint → kfp
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='open' AND table_name='open_api_invoke_log'
                  AND column_name='key_fingerprint')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                        WHERE table_schema='open' AND table_name='open_api_invoke_log'
                          AND column_name='kfp') THEN
        ALTER TABLE open.open_api_invoke_log RENAME COLUMN key_fingerprint TO kfp;
        IF to_regclass('open.idx_open_api_invoke_log_key_fingerprint') IS NOT NULL
           AND to_regclass('open.idx_open_api_invoke_log_kfp') IS NULL THEN
            ALTER INDEX open.idx_open_api_invoke_log_key_fingerprint RENAME TO idx_open_api_invoke_log_kfp;
        END IF;
        RAISE NOTICE 'V300186：key_fingerprint 已改名为 kfp';
    END IF;

    -- 中间名 key_fp → kfp（同一批次内先后两次改名，兼容已应用中间态的库）
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='open' AND table_name='open_api_invoke_log'
                  AND column_name='key_fp')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                        WHERE table_schema='open' AND table_name='open_api_invoke_log'
                          AND column_name='kfp') THEN
        ALTER TABLE open.open_api_invoke_log RENAME COLUMN key_fp TO kfp;
        IF to_regclass('open.idx_open_api_invoke_log_key_fp') IS NOT NULL
           AND to_regclass('open.idx_open_api_invoke_log_kfp') IS NULL THEN
            ALTER INDEX open.idx_open_api_invoke_log_key_fp RENAME TO idx_open_api_invoke_log_kfp;
        END IF;
        RAISE NOTICE 'V300186：key_fp 已改名为 kfp';
    END IF;

    -- 收敛校验：只断言脚本可保证的结构事实
    SELECT data_type INTO v_type
      FROM information_schema.columns
     WHERE table_schema='open' AND table_name='open_api_invoke_log' AND column_name='kfp';
    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300186 校验失败：open.open_api_invoke_log.kfp 不存在';
    END IF;
    IF v_type <> 'character varying' THEN
        RAISE EXCEPTION 'V300186 校验失败：kfp 类型应为 character varying，实际 %', v_type;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='open' AND table_name='open_api_invoke_log'
                  AND column_name IN ('key_fingerprint','key_fp')) THEN
        RAISE EXCEPTION 'V300186 校验失败：仍残留 key_fingerprint / key_fp 列';
    END IF;

    RAISE NOTICE 'V300186 就绪：审计表密钥指纹列名为 kfp';
END $$;

COMMIT;
