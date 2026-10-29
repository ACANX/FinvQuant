-- =====================================================================
-- FinvQuant PostgreSQL V100045：动态 JOB 改为「每次运行一条实例」
--
-- 决策依据：DbSchemaSpec §0 红线 + 用户明确授权（2026-10-27）
--
-- 问题：
--   cdeu_job_dynamic 原先以
--     UNIQUE (usc, biz_month) WHERE job_category='EXPORT'
--     UNIQUE (file_id)        WHERE job_category='UPLOAD'
--   约束，导致同一个证券月 / 同一个文件在所有运行之间共用一行记录：
--   EnsureDynamicExportJob 会把上一轮运行的记录 UPDATE 成「本轮」，
--   覆盖 workflow_run_id 并重置状态，上一轮运行的历史被抹掉。
--   即「上一次跑的 JOB，下一次跑流程还关联上」。
--
-- 修正：
--   唯一键加入 workflow_run_id，每次流程运行各自持有独立的动态实例：
--     (workflow_run_id, usc, biz_month)   WHERE job_category='EXPORT'
--     (workflow_run_id, file_id)          WHERE job_category='UPLOAD'
--   使用 NULLS NOT DISTINCT（PostgreSQL 15+）保证
--   workflow_run_id 为 NULL（手动单实例执行）时同样按唯一处理。
--
-- 存量数据：
--   现有记录每个 (usc,biz_month) / file_id 只有一条，且已带各自的
--   workflow_run_id（见 V100043 系列回填），新唯一索引不会冲突，全部保留。
--
-- 配套代码变更（同批次提交）：
--   EnsureDynamicExportJob   改为幂等 UPSERT，不再跨运行复用记录；
--   MarkDynamicJobStatus 等  按上下文 runID 定位目标实例，缺失时回退最新一条；
--   GenerateDynamicJobs      ON CONFLICT 目标改为新唯一键；
--   CLEAN_EXPIRED_EXPORT_ARTIFACTS 增加旧运行动态实例的清理。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. DDL 前快照
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_job_dynamic_bak_v100045 AS
SELECT * FROM dedup.cdeu_job_dynamic;

CREATE TABLE IF NOT EXISTS dedup.cdeu_job_dynamic_step_bak_v100045 AS
SELECT * FROM dedup.cdeu_job_dynamic_step;

COMMENT ON TABLE dedup.cdeu_job_dynamic_bak_v100045 IS
    'V100045 调整动态 JOB 唯一键前的整表快照（2026-10-27）';
COMMENT ON TABLE dedup.cdeu_job_dynamic_step_bak_v100045 IS
    'V100045 调整动态 JOB 唯一键前的整表快照（2026-10-27）';

-- ---------------------------------------------------------------------
-- 2. 删除跨运行共用的旧唯一索引
-- ---------------------------------------------------------------------
DROP INDEX IF EXISTS dedup.uq_cdeu_job_dynamic_export;
DROP INDEX IF EXISTS dedup.uq_cdeu_job_dynamic_upload;

-- ---------------------------------------------------------------------
-- 3. 建立「按运行唯一」的新索引
-- ---------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeu_job_dynamic_export_run
    ON dedup.cdeu_job_dynamic (workflow_run_id, usc, biz_month) NULLS NOT DISTINCT
 WHERE job_category = 'EXPORT';

CREATE UNIQUE INDEX IF NOT EXISTS uq_cdeu_job_dynamic_upload_run
    ON dedup.cdeu_job_dynamic (workflow_run_id, file_id) NULLS NOT DISTINCT
 WHERE job_category = 'UPLOAD';

COMMENT ON INDEX dedup.uq_cdeu_job_dynamic_export_run IS
    '同一流程运行内 (usc, biz_month) 唯一；不同运行各自持有独立动态实例';
COMMENT ON INDEX dedup.uq_cdeu_job_dynamic_upload_run IS
    '同一流程运行内 file_id 唯一；不同运行各自持有独立动态实例';

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_total      bigint;
    v_dup        bigint;
    v_old_index  int;
    v_new_index  int;
BEGIN
    SELECT count(*) INTO v_total FROM dedup.cdeu_job_dynamic;

    -- 4.1 新索引必须存在、旧索引必须消失
    SELECT count(*) INTO v_new_index
      FROM pg_indexes
     WHERE schemaname = 'dedup'
       AND indexname IN ('uq_cdeu_job_dynamic_export_run', 'uq_cdeu_job_dynamic_upload_run');

    SELECT count(*) INTO v_old_index
      FROM pg_indexes
     WHERE schemaname = 'dedup'
       AND indexname IN ('uq_cdeu_job_dynamic_export', 'uq_cdeu_job_dynamic_upload');

    IF v_new_index <> 2 THEN
        RAISE EXCEPTION 'V100045: 新唯一索引未全部创建（% / 2）', v_new_index;
    END IF;
    IF v_old_index <> 0 THEN
        RAISE EXCEPTION 'V100045: 旧唯一索引仍有 % 个未删除', v_old_index;
    END IF;

    -- 4.2 行数不变（本迁移不删除任何数据）
    IF v_total <> (SELECT count(*) FROM dedup.cdeu_job_dynamic_bak_v100045) THEN
        RAISE EXCEPTION 'V100045: cdeu_job_dynamic 行数异常 %', v_total;
    END IF;

    -- 4.3 新唯一键下不得有重复（索引已保证，此处显式复核）
    SELECT count(*) INTO v_dup FROM (
        SELECT 1
          FROM dedup.cdeu_job_dynamic
         WHERE job_category = 'EXPORT'
         GROUP BY workflow_run_id, usc, biz_month
        HAVING count(*) > 1
    ) s;
    IF v_dup > 0 THEN
        RAISE EXCEPTION 'V100045: EXPORT 动态实例仍有 % 组重复', v_dup;
    END IF;

    SELECT count(*) INTO v_dup FROM (
        SELECT 1
          FROM dedup.cdeu_job_dynamic
         WHERE job_category = 'UPLOAD'
         GROUP BY workflow_run_id, file_id
        HAVING count(*) > 1
    ) s;
    IF v_dup > 0 THEN
        RAISE EXCEPTION 'V100045: UPLOAD 动态实例仍有 % 组重复', v_dup;
    END IF;

    RAISE NOTICE 'V100045 完成：动态实例共 % 行，唯一键已按 (workflow_run_id, 业务键) 划分', v_total;
END $$;

COMMIT;
