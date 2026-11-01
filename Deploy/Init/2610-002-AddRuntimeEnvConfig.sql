-- =====================================================================
-- FinvQuant 一次性初始化脚本：发放运行环境标识配置（需求 10896）
--
-- 目标：在 sys.sys_config 中写入 config_no = 1002，用 str_value 标识
--   当前实例的运行环境（如 dev / test / uat / prod）。
--
-- 为什么是一次性脚本而不是迁移：
--   运行环境是**部署时**决定的事实，不同环境取值不同；放进迁移链会被
--   所有环境套用同一个值，且升级时无法区分。故放在 Deploy/Init/，
--   由部署/运维在目标库上手工执行一次。
--
-- 用法（连目标库执行）：
--   psql "postgres://finv@127.0.0.1:5432/finv" \
--        -v env=prod \
--        -f Deploy/Init/2610-002-AddRuntimeEnvConfig.sql
--
-- 参数（psql 变量）：
--   env        必填语义，缺省 prod；取值 2~32 位字母/数字/下划线/短横线
--   overwrite  缺省 0：已存在 1002 时**不覆盖**（避免误改生产标识）；
--              传 1 则用新值覆盖并刷新 gmt_modify / modify_by
--
-- 幂等：
--   - 1002 不存在 → 插入；
--   - 1002 已存在且 overwrite=0 → 保持原值（输出提示）；
--   - 1002 已存在且 overwrite=1 → 覆盖 str_value。
--
-- 前置：sys.sys_config 已存在（迁移 V200072 已应用，即服务至少启动过一次）。
-- =====================================================================

\set ON_ERROR_STOP on

-- ---------------------------------------------------------------------
-- 0. 参数缺省值
-- ---------------------------------------------------------------------
\if :{?env}
\else
  \set env prod
\endif
\if :{?overwrite}
\else
  \set overwrite 0
\endif

\echo '>>> 目标：sys.sys_config(1002) 运行环境 =' :env '，overwrite =' :overwrite

-- ---------------------------------------------------------------------
-- 1. 前置校验：取值格式
-- ---------------------------------------------------------------------
SELECT (:'env' ~ '^[A-Za-z0-9_-]{2,32}$') AS fq_env_ok \gset
\if :fq_env_ok
\else
  \echo '!!! env 取值非法：只允许 2~32 位字母、数字、下划线、短横线（如 dev / test / uat / prod）'
  -- 用 RAISE EXCEPTION 结束：ON_ERROR_STOP 下 psql 返回非 0 退出码（\quit 不支持自定义码）
  DO $$ BEGIN RAISE EXCEPTION 'env 参数非法，已中止'; END $$;
\endif

-- ---------------------------------------------------------------------
-- 2. 前置校验：sys_config 已存在（迁移已应用）
-- ---------------------------------------------------------------------
SELECT (to_regclass('sys.sys_config') IS NOT NULL) AS fq_table_ok \gset
\if :fq_table_ok
\else
  \echo '!!! sys.sys_config 不存在：请先启动服务执行迁移（或手工应用 Deploy/Migrations）后重跑本脚本'
  DO $$ BEGIN RAISE EXCEPTION 'sys.sys_config 不存在，已中止'; END $$;
\endif

-- ---------------------------------------------------------------------
-- 3. 写入配置（已存在且 overwrite=0 时不动原值）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, json_value, remark, create_by)
VALUES
    (1002, '运行环境标识', '3', 0, '', :'env', NULL,
     '当前实例的运行环境标识（如 dev / test / uat / prod）；由部署侧一次性写入，服务只读展示', 'init')
ON CONFLICT (config_no) DO UPDATE
   SET str_value  = EXCLUDED.str_value,
       remark     = EXCLUDED.remark,
       gmt_modify = CURRENT_TIMESTAMP(6),
       modify_by  = 'init'
 WHERE :overwrite = 1;

-- ---------------------------------------------------------------------
-- 4. 结果展示与收敛校验
-- ---------------------------------------------------------------------
SELECT config_no, config_name, data_type, str_value, modify_by, gmt_modify
  FROM sys.sys_config
 WHERE config_no = 1002 \gset fq_

SELECT (:'fq_str_value' = :'env') AS fq_saved \gset
\if :fq_saved
  \echo '<<< 完成：1002 运行环境 =' :fq_str_value
\else
  \echo '<<< 已存在且未覆盖：1002 当前运行环境 = ' :fq_str_value '（如需改为 ' :env '，加 -v overwrite=1 重跑）'
\endif
