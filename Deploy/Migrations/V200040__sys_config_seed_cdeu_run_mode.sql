-- =====================================================================
-- FinvQuant PostgreSQL V100040：sys.sys_config 新增 CDEU 工作流运行模式（20005）
--
-- 决策（ACANX 2026-10-27）：
--   - 本文件属于**种子段**（V100000+），依赖 V71（sys schema）与 V72
--     （sys.sys_config 表结构）。
--   - 新增一条配置：
--       20005 CDEU工作流运行模式
--             data_type='3'（字符串），str_value 默认 'Manual'
--               Manual —— 手动模式（默认）：只有手动触发（Job/Run）才执行流程，
--                         进程内调度器不自动触发任何 CDEU Job；
--               Auto   —— 自动模式：每天按定时任务设定的时间（cdeu_job.schedule_value）
--                         自动跑流程。
--   - 为什么需要它：现有实现中「调度触发」与「手动触发」共用同一条 Job 领取路径
--     （dedup.Store.ClaimJob 只按 status + 到期时间领取，CDEUTrigger* 仅写入运行日志），
--     因此只要处于低峰窗口且 Job 到期，调度器就一定会自动执行，运维无法只跑手动流程。
--     20005 把「是否允许自动调度」从窗口配置中独立出来：
--       窗口（EXPORT_WINDOW_*） —— 控制什么时段可以跑；
--       20005                  —— 控制是否允许自动跑。
--   - 存储位置：char_value 为 char(1) 放不下 'Manual'/'Auto'，故用 data_type='3' / str_value。
--   - 取值大小写不敏感（代码侧按等值忽略大小写比较并归一化为 Manual/Auto）；
--     空值或未知取值一律按 Manual 处理（安全侧：不自动跑，避免误触发归档）。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：仅使用
--     INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不写成 DO UPDATE；
--     本文件不含任何针对 20005 的 UPDATE 语句，并用迁移前快照硬校验。
--   - create_by='finv'（FinvQuant 服务端/迁移侧写入）。
--   - 幂等：ON CONFLICT DO NOTHING + 临时快照对比，重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾校验配置存在、取值非空且已有值未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若已存在，记录当前值，用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_20005_pre ON COMMIT DROP AS
SELECT config_no, char_value, str_value
  FROM sys.sys_config
 WHERE config_no = 20005;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20005, 'CDEU工作流运行模式', '3', 0, '', 'Manual', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     'CDEU 工作流运行模式：Manual-手动模式（默认），只有手动触发（Job/Run）才执行流程，进程内调度器不自动触发；Auto-自动模式，每天按定时任务设定的时间（cdeu_job.schedule_value）自动跑流程。取值大小写不敏感，空值或未知取值按 Manual 处理')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists   INT;
    v_value    TEXT;
    v_pre      TEXT;
    v_overwrit INT;
BEGIN
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 20005;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V100040: 20005 配置记录不存在（期望 1 条，实际 %）', v_exists;
    END IF;

    SELECT COALESCE(str_value, '') INTO v_value FROM sys.sys_config WHERE config_no = 20005;
    IF btrim(v_value) = '' THEN
        RAISE EXCEPTION 'V100040: 20005 的 str_value 为空，运行模式必须有值（Manual/Auto）';
    END IF;

    -- 取值合法性：大小写不敏感；非法值只告警不阻断（避免用户写错导致升级失败）
    IF lower(btrim(v_value)) NOT IN ('manual', 'auto') THEN
        RAISE WARNING 'V100040: 20005 当前取值 % 不是 Manual/Auto，服务端将按 Manual 处理；请尽快修正',
            v_value;
    END IF;

    -- 关键校验：迁移前已存在的值不得被改写
    SELECT str_value INTO v_pre FROM tmp_sys_config_20005_pre;
    IF v_pre IS NOT NULL AND v_pre IS DISTINCT FROM v_value THEN
        RAISE EXCEPTION 'V100040: 20005 既有值被改写（原 %, 现 %），违反"不覆盖用户配置"红线',
            v_pre, v_value;
    END IF;

    SELECT count(*) INTO v_overwrit
      FROM tmp_sys_config_20005_pre p
      JOIN sys.sys_config c ON c.config_no = p.config_no
     WHERE p.str_value IS DISTINCT FROM c.str_value;
    IF v_overwrit > 0 THEN
        RAISE EXCEPTION 'V100040: 检测到 % 行既有配置被改写', v_overwrit;
    END IF;

    RAISE NOTICE 'V100040: 20005 CDEU工作流运行模式就绪，当前取值 = %', v_value;
END $$;

COMMIT;
