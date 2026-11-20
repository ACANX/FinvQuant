-- =====================================================================
-- FinvQuant PostgreSQL V300189：open.open_api_invoke_log.token_fp 改名为 tfp
--
-- 需求：11196
-- 授权依据：用户在 2026-11-15 会话中明确要求「token_fp 改为 tfp」。
--
-- 背景：V300188（需求 11191）首次落列时用的名字是 `token_fp`，用户定稿为 `tfp`
--   （与 `kfp` 同一缩写风格，审计表列多，短名省版面）。
--   V300188 尚未推送到任何环境、且与本迁移同属一个需求批次，故 V300188 **同批回改**为 `tfp`
--   （文件名同步为 `V300188__open_invoke_log_add_tfp.sql`）；
--   本迁移负责把「已经应用过旧版 V300188」的库（本机、测试库等）就地改名，不是重建列。
--
-- 改了什么：列名 + 列注释 + 索引名（数据与类型不动）
--   open.open_api_invoke_log.token_fp                    → tfp
--   idx_open_api_invoke_log_token_fp                     → idx_open_api_invoke_log_tfp
--
-- 幂等：只在「旧列存在且新列不存在」时改名；重跑进入 ELSE 分支，0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：新列存在、类型 varchar、无残留旧列。
-- 前置：V300188（或它的旧版）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_type TEXT;
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='open' AND table_name='open_api_invoke_log'
                  AND column_name='token_fp')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                        WHERE table_schema='open' AND table_name='open_api_invoke_log'
                          AND column_name='tfp') THEN
        ALTER TABLE open.open_api_invoke_log RENAME COLUMN token_fp TO tfp;
        IF to_regclass('open.idx_open_api_invoke_log_token_fp') IS NOT NULL
           AND to_regclass('open.idx_open_api_invoke_log_tfp') IS NULL THEN
            ALTER INDEX open.idx_open_api_invoke_log_token_fp RENAME TO idx_open_api_invoke_log_tfp;
        END IF;
        RAISE NOTICE 'V300189：token_fp 已改名为 tfp';
    ELSE
        RAISE NOTICE 'V300189：无需改名（已是 tfp 或列不存在）';
    END IF;

    SELECT data_type INTO v_type
      FROM information_schema.columns
     WHERE table_schema='open' AND table_name='open_api_invoke_log' AND column_name='tfp';
    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V300189 校验失败：open.open_api_invoke_log.tfp 不存在';
    END IF;
    IF v_type <> 'character varying' THEN
        RAISE EXCEPTION 'V300189 校验失败：tfp 类型应为 character varying，实际 %', v_type;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema='open' AND table_name='open_api_invoke_log'
                  AND column_name='token_fp') THEN
        RAISE EXCEPTION 'V300189 校验失败：仍残留 token_fp 列';
    END IF;

    RAISE NOTICE 'V300189 就绪：审计表列名为 tfp';
END $$;

COMMIT;
