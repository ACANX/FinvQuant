-- =====================================================================
-- FinvQuant PostgreSQL V300158：关闭 USC 映射每日同步的自动调度（需求 11072 补充）
--
-- 背景（用户 2026-11-10 决定）：
--   V300157 把「USC 映射规则每日同步」的 cron 初始值设为 `0 30 3 * * *`（每天 03:30）。
--   实测（事务内 ROLLBACK 预演）表明：按当前规则短路语义首次应用会**改写
--   `futu.finv_futu_secu_key.usc` 共 16,848 行**（AU / CA / JP / BMS / SG 等市场
--   的天然唯一码从 `CA.XXX` 这类全码退回裸 `XXX`）。这属于跨表主键级变更，
--   需要先与「存量数据对齐方案」一起确认，不能由定时任务在夜间无人值守地自动落库。
--
-- 因此本迁移把 `sys.sys_config 20015` 的 `str_value` 置空 = 关闭自动同步：
--   - 空串或非法 cron 时，运行器不解析调度、永不自动触发（只记一次日志）；
--   - 手动「立即应用」仍然可用（维护页 → 预演 → 应用，带预演令牌校验）；
--   - 对齐方案确认后，由运维在系统配置里填回 cron 即可恢复每日自动同步（无需改代码）。
--
-- 不覆盖人工配置：只有当前值仍是 V300157 的种子默认值 `0 30 3 * * *` 时才清空；
--   若运维已自行改过（值既不是空串也不是种子默认），本迁移不动它，只在 remark 追加说明。
--
-- 幂等：UPDATE 带「值等于种子默认」条件，重跑影响 0 行；末尾收敛校验。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移前快照（用于校验未被误改）
-- ---------------------------------------------------------------------
CREATE TEMP TABLE tmp_usc_cron_20015_pre ON COMMIT DROP AS
SELECT config_no, COALESCE(str_value, '') AS str_value, COALESCE(remark, '') AS remark
  FROM sys.sys_config
 WHERE config_no = 20015;

-- ---------------------------------------------------------------------
-- 2. 关闭自动同步：仅当仍是 V300157 的种子默认值时清空
-- ---------------------------------------------------------------------
UPDATE sys.sys_config
   SET str_value  = '',
       gmt_modify = CURRENT_TIMESTAMP(6),
       modify_by  = 'migration',
       remark     = CASE
                      WHEN COALESCE(remark, '') LIKE '%上线初期关闭自动同步%' THEN remark
                      ELSE trim(both ' ' FROM COALESCE(remark, '') ||
                           ' | 2026-11-10 上线初期关闭自动同步：按规则首次应用会改写约 1.68 万行存量 usc（需求 11072），' ||
                           '待存量对齐方案确认后由运维填回 cron')
                    END
 WHERE config_no = 20015
   AND btrim(COALESCE(str_value, '')) = '0 30 3 * * *';

-- 若已被运维改成别的表达式，也只补一句说明，不改值
UPDATE sys.sys_config
   SET remark = CASE
                  WHEN COALESCE(remark, '') LIKE '%上线初期关闭自动同步%' THEN remark
                  ELSE trim(both ' ' FROM COALESCE(remark, '') ||
                       ' | 2026-11-10 说明：自动同步默认应保持关闭，直到存量 usc 对齐方案确认（需求 11072）')
                END
 WHERE config_no = 20015
   AND btrim(COALESCE(str_value, '')) NOT IN ('', '0 30 3 * * *');

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_exists INT;
    v_value  TEXT;
    v_pre    TEXT;
    v_remark TEXT;
BEGIN
    SELECT count(*) INTO v_exists FROM sys.sys_config WHERE config_no = 20015;
    IF v_exists <> 1 THEN
        RAISE EXCEPTION 'V300158 校验失败：系统配置 20015 不存在（应由 V300157 创建）';
    END IF;

    SELECT COALESCE(str_value, ''), COALESCE(remark, '') INTO v_value, v_remark
      FROM sys.sys_config WHERE config_no = 20015;
    SELECT COALESCE(str_value, '') INTO v_pre FROM tmp_usc_cron_20015_pre;

    IF btrim(v_pre) = '0 30 3 * * *' THEN
        IF btrim(v_value) <> '' THEN
            RAISE EXCEPTION 'V300158 校验失败：20015 应已置空，实际 = [%]', v_value;
        END IF;
    ELSE
        IF btrim(v_value) <> btrim(v_pre) THEN
            RAISE EXCEPTION 'V300158 校验失败：20015 是人工配置（迁移前 [%]），不得改动，实际 = [%]', v_pre, v_value;
        END IF;
    END IF;

    IF v_remark NOT LIKE '%上线初期关闭自动同步%' AND v_remark NOT LIKE '%自动同步默认应保持关闭%' THEN
        RAISE EXCEPTION 'V300158 校验失败：20015 的 remark 未记录关闭原因';
    END IF;

    RAISE NOTICE 'V300158 完成：20015 当前值 = [%]（空 = 关闭自动同步，手动应用与预演仍可用）', v_value;
END $$;

COMMIT;
