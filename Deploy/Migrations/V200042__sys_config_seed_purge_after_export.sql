-- =====================================================================
-- FinvQuant PostgreSQL V100042：sys.sys_config 新增「导出成功后立即删除主表数据」开关（20007）
--
-- 决策（ACANX 2026-10-27）：
--   - 新增 20007「导出成功后立即删除主表行情数据标志」：
--       data_type='2'（字符开关，与 20002 一致）→ char_value
--       '1' —— 立即删除（默认）：**远端上传成功后**删除
--              dedup.finv_quote_secu_kline_min 中该 (usc, biz_month) 的行情数据；
--              此后同一证券月表内再次插入历史数据时，按增量方式导出 INCR 文件，
--              文件名中的序号 N 每次 +1（_Incr_1、_Incr_2 …）。
--       '0' —— 保留数据：不删主表；每次导出直接重写同名 BASE 文件覆盖已有
--              .mvsv，**不生成 INCR 文件**。
--   - 语义要点（已与需求方确认）：
--       ① 删除时机 = 远端上传成功之后（不是本地写盘成功），避免「远端上传失败 +
--          主表已删」造成数据只剩本地一份；
--       ② BASE 文件本身没有任何版本号，不存在「版本号递增」；递增的只有 INCR
--          文件名里的 N（沿用现有 patch_count 机制，天然递增）；
--       ③ 采用本开关后**取代**环境变量 FINV_QUOTE_DEDUB_PURGE_MAIN_ENABLED /
--          FINV_QUOTE_DEDUB_PURGE_MAIN_MONTHS（原「延迟按月清理」逻辑下线）；
--       ④ 首次 BASE 导出上传成功后同样删除主表数据；
--       ⑤ 删除不做行级备份，仅记录删除行数与文件/批次信息（需求方明确要求）；
--       ⑥ 只影响 dedup schema 的主表；quote schema 是回测/查询专用的独立副本，
--          不参与本开关的删除动作。
--   - 【红线】发放的初始数据**不得覆盖用户后来配置的值**：仅使用
--     INSERT ... ON CONFLICT DO NOTHING；本文件不含任何针对 20007 的 UPDATE，
--     并用迁移前快照硬校验。
--   - create_by='finv'。幂等：ON CONFLICT DO NOTHING，重跑影响 0 行。
-- 迁移策略：单事务、失败整体回滚；末尾校验配置存在、取值合法且已有值未被覆盖。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（用于验证未被覆盖）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_sys_config_20007_pre ON COMMIT DROP AS
SELECT config_no, char_value, str_value
  FROM sys.sys_config
 WHERE config_no = 20007;

-- ---------------------------------------------------------------------
-- 2. 插入配置记录（已存在则完全跳过）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20007, '导出成功后立即删除主表行情数据标志', '2', 0, '1', '', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     '导出成功后是否从 dedup.finv_quote_secu_kline_min 立即删除对应的行情数据：1-立即删除（默认，远端上传成功后删除，之后表内再次插入的历史数据按 INCR 增量导出、文件名序号 N 递增）；0-保留数据（不删主表，重复导出直接重写同名 BASE 文件覆盖已有 .mvsv，不生成 INCR）。仅影响 dedup schema，不影响 quote schema 的回测副本')
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
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 20007;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V100042: 20007 配置记录不存在（期望 1 条，实际 %）', v_exists;
    END IF;

    SELECT COALESCE(char_value::text, '') INTO v_value
      FROM sys.sys_config WHERE config_no = 20007;
    IF btrim(v_value) = '' THEN
        RAISE EXCEPTION 'V100042: 20007 的 char_value 为空，必须为 1 或 0';
    END IF;
    IF btrim(v_value) NOT IN ('0', '1') THEN
        RAISE WARNING 'V100042: 20007 当前取值 % 不是 0/1，服务端将按 1（立即删除）处理', v_value;
    END IF;

    SELECT char_value::text INTO v_pre FROM tmp_sys_config_20007_pre;
    IF v_pre IS NOT NULL AND v_pre IS DISTINCT FROM v_value THEN
        RAISE EXCEPTION 'V100042: 20007 既有值被改写（原 %, 现 %）', v_pre, v_value;
    END IF;

    SELECT count(*) INTO v_overwrit
      FROM tmp_sys_config_20007_pre p
      JOIN sys.sys_config c ON c.config_no = p.config_no
     WHERE p.char_value IS DISTINCT FROM c.char_value;
    IF v_overwrit > 0 THEN
        RAISE EXCEPTION 'V100042: 检测到 % 行既有配置被改写', v_overwrit;
    END IF;

    RAISE NOTICE 'V100042: 20007 就绪，当前取值 = %', v_value;
END $$;

COMMIT;
