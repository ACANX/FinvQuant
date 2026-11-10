-- =====================================================================
-- FinvQuant PostgreSQL V300184：open.open_api_call_log 改名为 open.open_api_invoke_log
--
-- 需求：11170
-- 授权依据：用户在 2026-11-15 会话中明确点名「open.open_api_call_log 需要改名为 open.open_api_invoke_log」
--   → 属 DbSchemaSpec §0.3「用户明确点名」的白名单情形，可执行 RENAME。
--
-- 背景：V300183（需求 11167）首次落表时用的名字是 `open_api_call_log`；定稿后改为 `open_api_invoke_log`
--   （与「一次 OpenAPI 调用（invoke）」的业务语义对齐，也避免与内部业务「调用日志」概念混淆）。
--   V300183 尚未推送到任何环境、且与本迁移同属一个需求批次，故**同批回改**为最终表名；
--   本迁移负责把「已经应用过旧版 V300183」的库（本机、测试库、以及任何在回改前就升级过的实例）
--   就地改名，不是重复建表。
--
-- 改了什么（表 + 3 个索引 + 5 个约束的**名字**，列/数据/注释不动）：
--   open.open_api_call_log        → open.open_api_invoke_log
--   pk_open_api_call_log          → pk_open_api_invoke_log
--   ck_open_api_call_log_flag_pass / _status_code / _count_body / _latency_ms
--                                 → ck_open_api_invoke_log_*（同名对应）
--   idx_open_api_call_log_dt_request / _credential / _deny
--                                 → idx_open_api_invoke_log_*（同名对应）
--
-- 幂等：只用「旧表存在且新表不存在」时才改名；重跑进入 ELSE 分支，0 变更。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 收敛校验（只断言本脚本自己能保证的结构事实）：
--   ① 新表存在；② 表上不再残留 `*_call_log` 命名的索引/约束；③ 行数在改名前后一致（RAISE NOTICE，不阻断）。
-- 前置：open schema（V100013）、V300183（或等价的旧表）。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_has_old BOOLEAN := to_regclass('open.open_api_call_log') IS NOT NULL;
    v_has_new BOOLEAN := to_regclass('open.open_api_invoke_log') IS NOT NULL;
    v_cnt_old BIGINT;
    v_cnt_new BIGINT;
    v_item    RECORD;
    v_left    TEXT;
BEGIN
    IF v_has_old AND NOT v_has_new THEN
        EXECUTE 'SELECT count(*) FROM open.open_api_call_log' INTO v_cnt_old;

        -- 1) 表改名（列、数据、注释、索引/约束的**名字**都保持不变，改名后再统一纠正名字）
        ALTER TABLE open.open_api_call_log RENAME TO open_api_invoke_log;

        -- 2) 约束改名（存在才改，兼容旧库索引/约束名被人工改过的情形）
        FOR v_item IN
            SELECT * FROM (VALUES
                ('pk_open_api_call_log',               'pk_open_api_invoke_log'),
                ('ck_open_api_call_log_flag_pass',     'ck_open_api_invoke_log_flag_pass'),
                ('ck_open_api_call_log_status_code',   'ck_open_api_invoke_log_status_code'),
                ('ck_open_api_call_log_count_body',    'ck_open_api_invoke_log_count_body'),
                ('ck_open_api_call_log_latency_ms',    'ck_open_api_invoke_log_latency_ms')
            ) AS t(old_name, new_name)
        LOOP
            IF EXISTS (SELECT 1 FROM pg_constraint c
                         JOIN pg_class r ON r.oid = c.conrelid
                         JOIN pg_namespace n ON n.oid = r.relnamespace
                        WHERE n.nspname = 'open' AND r.relname = 'open_api_invoke_log'
                          AND c.conname = v_item.old_name)
            THEN
                EXECUTE format('ALTER TABLE open.open_api_invoke_log RENAME CONSTRAINT %I TO %I',
                               v_item.old_name, v_item.new_name);
            END IF;
        END LOOP;

        -- 3) 索引改名（约束自带的索引随约束改名一起变，这里只处理普通索引）
        FOR v_item IN
            SELECT * FROM (VALUES
                ('idx_open_api_call_log_dt_request', 'idx_open_api_invoke_log_dt_request'),
                ('idx_open_api_call_log_credential', 'idx_open_api_invoke_log_credential'),
                ('idx_open_api_call_log_deny',       'idx_open_api_invoke_log_deny')
            ) AS t(old_name, new_name)
        LOOP
            IF EXISTS (SELECT 1 FROM pg_class i
                         JOIN pg_namespace n ON n.oid = i.relnamespace
                        WHERE n.nspname = 'open' AND i.relname = v_item.old_name AND i.relkind = 'i')
            THEN
                EXECUTE format('ALTER INDEX open.%I RENAME TO %I', v_item.old_name, v_item.new_name);
            END IF;
        END LOOP;

        EXECUTE 'SELECT count(*) FROM open.open_api_invoke_log' INTO v_cnt_new;
        RAISE NOTICE 'V300184 完成：open_api_call_log → open_api_invoke_log，行数 % → %', v_cnt_old, v_cnt_new;
    ELSIF v_has_old AND v_has_new THEN
        EXECUTE 'SELECT count(*) FROM open.open_api_call_log'  INTO v_cnt_old;
        EXECUTE 'SELECT count(*) FROM open.open_api_invoke_log' INTO v_cnt_new;
        RAISE NOTICE 'V300184 提示：call_log（% 行）与 invoke_log（% 行）同时存在，未自动处理，请人工核对后保留其一',
            v_cnt_old, v_cnt_new;
    ELSE
        RAISE NOTICE 'V300184 跳过：旧表不存在（新表存在=%），无需改名', v_has_new;
    END IF;

    -- 收敛校验 ①：新表必须存在
    IF to_regclass('open.open_api_invoke_log') IS NULL THEN
        RAISE EXCEPTION 'V300184 校验失败：open.open_api_invoke_log 不存在';
    END IF;

    -- 收敛校验 ②：新表上不得残留 `*_call_log` 命名的索引/约束（改名漏项会在此暴露）
    SELECT string_agg(x.relname, ', ') INTO v_left
      FROM pg_class x
      JOIN pg_namespace n ON n.oid = x.relnamespace
     WHERE n.nspname = 'open'
       AND x.relname LIKE '%\_call\_log%'
       AND x.relkind IN ('i','r');

    SELECT v_left || COALESCE((
        SELECT ', ' || string_agg(c.conname, ', ')
          FROM pg_constraint c
          JOIN pg_class r ON r.oid = c.conrelid
          JOIN pg_namespace n ON n.oid = r.relnamespace
         WHERE n.nspname = 'open' AND c.conname LIKE '%\_call\_log%'
    ), '') INTO v_left;

    IF v_left IS NOT NULL AND v_left <> '' THEN
        RAISE EXCEPTION 'V300184 校验失败：仍残留 *_call_log 命名的对象（%）', v_left;
    END IF;

    RAISE NOTICE 'V300184 就绪：审计表名为 open.open_api_invoke_log，无残留 call_log 命名对象';
END $$;

COMMIT;
