-- =====================================================================
-- FinvQuant PostgreSQL V300143：sys.sys_config 新增 Supabase 证券键源表名（20014）
--
-- 决策（ACANX 2026-11-09）：
--   - 新增一条系统配置：20014「Supabase 证券键源表名」，data_type='3'（字符串），
--     取值存 `str_value`，默认 `finv_quote_secu_key`。
--   - 用途：前端菜单「证券参数同步」里可改的源表名（需求 11044）。同步接口按该值
--     拼 PostgREST 地址（`https://<PROJECT_ID>.supabase.co/rest/v1/<源表名>`）。
--   - 取值校验：只允许 `[A-Za-z_][A-Za-z0-9_]*`，或 `schema.table` 两段（可带 `public.`）。
--     校验在服务端做（syscfg.NormalizeSupabaseTableName），非法值按默认表名兜底；
--     这里种默认值只是为了「开箱即用、与上游同名」。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：仅
--     INSERT ... ON CONFLICT (config_no) DO NOTHING，绝不写 DO UPDATE；本文件不含任何 UPDATE。
--   - create_by='finv'；幂等：重跑影响 0 行且不改值。
-- 迁移策略：单事务、失败整体回滚；末尾校验配置存在、data_type 正确、取值非空且未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（若已存在，记录当前值，用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_20014_pre ON COMMIT DROP AS
SELECT config_no, str_value
  FROM sys.sys_config
 WHERE config_no = 20014;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过，不修改任何列）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20014, 'Supabase 证券键源表名', '3', 0, '', 'finv_quote_secu_key', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     'Supabase 侧证券键源表名（默认 finv_quote_secu_key，与上游同名）：由「证券参数同步」菜单里的源表名输入框维护，同步时按该值拼 https://<PROJECT_ID>.supabase.co/rest/v1/<源表名>。只允许字母/数字/下划线，可写成 schema.table 两段；留空或非法时按默认 finv_quote_secu_key 处理')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists INT;
    v_type   TEXT;
    v_value  TEXT;
    v_pre    TEXT;
BEGIN
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 20014;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V300143 配置 20014 未插入成功';
    END IF;

    SELECT data_type, COALESCE(str_value, '') INTO v_type, v_value
      FROM sys.sys_config WHERE config_no = 20014;
    IF v_type <> '3' THEN
        RAISE EXCEPTION 'V300143 配置 20014 的 data_type 应为 3（字符串），实际 %', v_type;
    END IF;
    IF btrim(v_value) = '' THEN
        RAISE EXCEPTION 'V300143 配置 20014 的取值不得为空（默认 finv_quote_secu_key）';
    END IF;

    -- 已有值不得被覆盖
    SELECT str_value INTO v_pre FROM tmp_sys_config_20014_pre;
    IF v_pre IS NOT NULL AND btrim(v_pre) <> '' AND btrim(v_pre) <> btrim(v_value) THEN
        RAISE EXCEPTION 'V300143 配置 20014 已有值被改动：迁移前=[%] 迁移后=[%]', v_pre, v_value;
    END IF;

    RAISE NOTICE 'V300143 配置 20014 就绪：源表名=%', v_value;
END $$;

COMMIT;
