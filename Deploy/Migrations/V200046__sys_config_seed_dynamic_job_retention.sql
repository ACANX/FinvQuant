-- =====================================================================
-- FinvQuant PostgreSQL V100046：系统配置 20008（历史运行动态 JOB 保留天数）
--
-- 决策依据：用户明确要求（2026-10-27）把 V100045 引入的动态实例保留策略
-- 从代码常量改为可在线配置。
--
-- 背景：
--   自 V100045 起 cdeu_job_dynamic 每次流程运行各自持有一条实例，
--   实例数量随运行次数线性增长；CLEAN_EXPIRED_EXPORT_ARTIFACTS 需要按
--   保留期清理历史实例，保留期原先固化为代码常量 30 天。
--
-- 配置口径：
--   config_no   = 20008
--   data_type   = '1'（整数），取值存放于 int_value
--   默认值      = 30（天）
--   合法区间    = 1 ~ 3650（约 10 年）；越界或非法时运行期回退默认 30
--
-- 注意：本迁移只做幂等 INSERT，绝不覆盖用户已配置的值。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 种子（ON CONFLICT DO NOTHING：不覆盖用户已改过的值）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value, remark, create_by)
VALUES
    (20008, '历史运行动态JOB保留天数', '1', 30, '', '',
     'CDEU 动态 JOB 每次流程运行各持有一条实例（V100045），超过该天数的历史运行实例由 CLEAN_EXPIRED_EXPORT_ARTIFACTS 清理；合法区间 1~3650，越界或非法时回退默认 30 天。',
     'finv')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 2. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type   text;
    v_int    int;
    v_count  int;
BEGIN
    SELECT count(*) INTO v_count FROM sys.sys_config WHERE config_no = 20008;
    IF v_count <> 1 THEN
        RAISE EXCEPTION 'V100046: 20008 记录数异常 %', v_count;
    END IF;

    SELECT data_type, COALESCE(int_value, 0) INTO v_type, v_int
      FROM sys.sys_config WHERE config_no = 20008;

    IF btrim(v_type) <> '1' THEN
        RAISE EXCEPTION 'V100046: 20008 data_type 应为 1，实际 %', v_type;
    END IF;

    -- 只校验取值范围，不阻断（用户可能已配置其它合法值）
    IF v_int < 1 OR v_int > 3650 THEN
        RAISE WARNING 'V100046: 20008 当前值 % 超出建议区间 1~3650，运行期将回退默认 30', v_int;
    END IF;

    RAISE NOTICE 'V100046 完成：20008 保留天数 = % 天', v_int;
END $$;

COMMIT;
