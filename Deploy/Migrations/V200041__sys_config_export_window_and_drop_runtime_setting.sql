-- =====================================================================
-- FinvQuant PostgreSQL V100041：导出低峰窗口改为 sys_config 20006，并删除
--                           dedup.finv_dedup_runtime_setting
--
-- 决策（ACANX 2026-10-27）：
--   - 为「两个时间」单独建一张 3 列的键值表（V43 建的
--     dedup.finv_dedup_runtime_setting）属于过度设计：平台已有
--     sys.sys_config 这套统一的键值配置体系（20000–20005 均在使用），
--     低峰窗口没有理由再维护第二套存储。
--   - 新增 20006「导出低峰窗口」：
--       data_type='3'（字符串），str_value 格式 **HH:MM:SS|HH:MM:SS**
--       （精确到秒，两个时间用半角竖线分隔），初始值 '00:30:00|06:30:00'。
--       支持跨零点（如 '23:00:00|06:00:00'）；start == end 视为全天放行。
--   - 环境变量 FINV_QUOTE_DEDUB_WINDOW_START / WINDOW_END **废弃**：
--     窗口一律取 20006；配置缺失或取值非法时按代码内置降级策略兜底
--     （'00:30:00|06:30:00'），不再回读环境变量。
--   - 【删除表】按 DbSchemaSpec §0 红线流程执行：用户在本次会话中明确
--     授权删除 dedup.finv_dedup_runtime_setting（"表删除，留着过年？"），
--     并已在 Docs/RequirementList.md 登记（10653 条目）。该表仅有
--     EXPORT_WINDOW_START / EXPORT_WINDOW_END 两行，均为窗口键，无其他
--     数据；删除前做「仅含窗口键」硬校验，避免误删未知运行时设置。
--   - 【初始发放值】20006 一律以**规范初始值 '00:30:00|06:30:00'** 发放，
--     不从旧表搬运现值：初始发放值与部署后实际运行值不一致是正常现象，
--     实际窗口由运维通过 PUT /Settings/Window 自行配置（用户明确要求）。
--     写入用 ON CONFLICT DO NOTHING，绝不覆盖用户已配置的值。
--   - 幂等：20006 用 ON CONFLICT DO NOTHING；DROP TABLE 用 IF EXISTS；
--     重跑影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾校验 20006 存在且格式合法、旧表已删除。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（用于验证 20006 已有值未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_20006_pre ON COMMIT DROP AS
SELECT config_no, char_value, str_value
  FROM sys.sys_config
 WHERE config_no = 20006;

-- ---------------------------------------------------------------------
-- 2. 插入 20006（固定规范初始值；已存在则完全跳过，不覆盖用户配置）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20006, '导出低峰窗口', '3', 0, '', '00:30:00|06:30:00', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '证券行情月归档导出的低峰窗口，格式 HH:MM:SS|HH:MM:SS（精确到秒，半角竖线分隔起止，支持跨零点）如 00:30:00|06:30:00；start=end 视为全天放行；缺失或非法时按内置默认 00:30:00|06:30:00 兜底')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 3. 删除旧运行时设置表（用户明确授权；先硬校验只含窗口键）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_has_table BOOLEAN;
    v_total     INT;
    v_window    INT;
    v_other     TEXT;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_tables
         WHERE schemaname = 'dedup' AND tablename = 'finv_dedup_runtime_setting'
    ) INTO v_has_table;
    IF NOT v_has_table THEN
        RAISE NOTICE 'V100041: dedup.finv_dedup_runtime_setting 不存在，跳过删除';
        RETURN;
    END IF;

    SELECT count(*) INTO v_total FROM dedup.finv_dedup_runtime_setting;
    SELECT count(*) INTO v_window FROM dedup.finv_dedup_runtime_setting
     WHERE setting_key IN ('EXPORT_WINDOW_START', 'EXPORT_WINDOW_END');

    IF v_total <> v_window THEN
        SELECT string_agg(setting_key, ', ' ORDER BY setting_key) INTO v_other
          FROM dedup.finv_dedup_runtime_setting
         WHERE setting_key NOT IN ('EXPORT_WINDOW_START', 'EXPORT_WINDOW_END');
        RAISE EXCEPTION 'V100041: 旧表含非窗口键（%），可能承载其它运行时设置，需人工确认后再删', v_other;
    END IF;

    DROP TABLE dedup.finv_dedup_runtime_setting;
    RAISE NOTICE 'V100041: 已删除 dedup.finv_dedup_runtime_setting（原有 % 行，均为窗口键）', v_total;
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_value    TEXT;
    v_pre      TEXT;
    v_old_left BOOLEAN;
BEGIN
    SELECT COALESCE(str_value, '') INTO v_value
      FROM sys.sys_config WHERE config_no = 20006;
    IF btrim(v_value) = '' THEN
        RAISE EXCEPTION 'V100041: 20006 缺失或为空';
    END IF;
    IF v_value !~ '^([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]\|([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]$' THEN
        RAISE EXCEPTION 'V100041: 20006 取值 % 不符合 HH:MM:SS|HH:MM:SS 格式', v_value;
    END IF;

    SELECT str_value INTO v_pre FROM tmp_sys_config_20006_pre;
    IF v_pre IS NOT NULL AND v_pre IS DISTINCT FROM v_value THEN
        RAISE EXCEPTION 'V100041: 20006 既有值被改写（原 %, 现 %）', v_pre, v_value;
    END IF;

    SELECT EXISTS (
        SELECT 1 FROM pg_tables
         WHERE schemaname = 'dedup' AND tablename = 'finv_dedup_runtime_setting'
    ) INTO v_old_left;
    IF v_old_left THEN
        RAISE EXCEPTION 'V100041: 旧表 dedup.finv_dedup_runtime_setting 仍存在';
    END IF;

    RAISE NOTICE 'V100041: 完成 — 20006 = %，旧表已删除', v_value;
END $$;

COMMIT;
