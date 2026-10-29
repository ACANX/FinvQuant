-- =====================================================================
-- FinvQuant PostgreSQL V100043：CDEU 运行实例表补充 workflow_run_id
--
-- 决策依据：DbSchemaSpec §0（DDL 红线，本次为用户明确授权的加列变更）
--
-- 目标：
--   给 cdeu_process_run / cdeu_job_run 增加冗余列 workflow_run_id，
--   使「按一次 workflow 运行筛选 JOB」不再需要 JOIN 三层：
--     cdeu_job_run → cdeu_process_run → cdeu_phase_run → workflow_run_id
--   改为一句话：
--     SELECT * FROM dedup.cdeu_job_run WHERE workflow_run_id = $1
--
-- 为什么本迁移不删除 process_run_id / job_run_id（自增代理键）：
--   1) cdeu_job_run 的 (workflow_run_id, process_id, job_id) 并不唯一：
--      一次 process_run 内同一 job 会按证券月重复执行 —— 实测
--      process_run_id=19158 下 job_id=31/32/33 各有 51 条记录。
--   2) 用于区分批次的 batch_key 列（注释示例 'US.AAPL:202604'）
--      在 797 行中全部为 NULL，从未被写入，因此无法承担业务唯一键职责。
--   3) 因此在补齐业务唯一键之前，自增主键仍是唯一可用的行标识。
--   删代理键需另行决策（补齐 batch_key 或改用 dynamic_job_id）。
--
-- 回填规则：
--   cdeu_process_run.workflow_run_id ← cdeu_phase_run.workflow_run_id（父行必存在）
--   cdeu_job_run.workflow_run_id     ← cdeu_process_run.workflow_run_id
--
-- 已知数据缺口（不在本迁移中处理）：
--   存在 156 条 cdeu_job_run 的 process_run_id 指向已被删除的父行
--   （引用区间 3674~15999，而现存 cdeu_process_run 起始于 16145）。
--   这些孤儿记录无法推导 workflow_run_id，回填后保持 NULL，
--   其处置（删除或标记）需另行决策，本迁移不删除任何数据。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 加列（可空，回填后再由后续迁移决定是否收紧为 NOT NULL）
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_process_run
    ADD COLUMN IF NOT EXISTS workflow_run_id bigint;

ALTER TABLE dedup.cdeu_job_run
    ADD COLUMN IF NOT EXISTS workflow_run_id bigint;

COMMENT ON COLUMN dedup.cdeu_process_run.workflow_run_id IS
    '工作流运行 ID（冗余列，来源于 cdeu_phase_run.workflow_run_id，用于免 JOIN 按运行筛选）';
COMMENT ON COLUMN dedup.cdeu_job_run.workflow_run_id IS
    '工作流运行 ID（冗余列，来源于 cdeu_process_run.workflow_run_id，用于免 JOIN 按运行筛选）';

-- ---------------------------------------------------------------------
-- 2. 回填
-- ---------------------------------------------------------------------
UPDATE dedup.cdeu_process_run pr
   SET workflow_run_id = phr.workflow_run_id
  FROM dedup.cdeu_phase_run phr
 WHERE phr.phase_run_id = pr.phase_run_id
   AND pr.workflow_run_id IS DISTINCT FROM phr.workflow_run_id;

UPDATE dedup.cdeu_job_run jr
   SET workflow_run_id = pr.workflow_run_id
  FROM dedup.cdeu_process_run pr
 WHERE pr.process_run_id = jr.process_run_id
   AND pr.workflow_run_id IS NOT NULL
   AND jr.workflow_run_id IS DISTINCT FROM pr.workflow_run_id;

-- ---------------------------------------------------------------------
-- 3. 索引：支撑「按运行筛选」与「运行 + 业务编码」两种查法
-- ---------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_cdeu_process_run_workflow
    ON dedup.cdeu_process_run (workflow_run_id, process_id);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_workflow
    ON dedup.cdeu_job_run (workflow_run_id, job_code);

-- ---------------------------------------------------------------------
-- 4. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_process_total     bigint;
    v_process_filled    bigint;
    v_process_mismatch  bigint;
    v_job_total         bigint;
    v_job_filled        bigint;
    v_job_orphan        bigint;
    v_job_mismatch      bigint;
BEGIN
    -- 4.1 cdeu_process_run：必须 100% 回填，且与父表一致
    SELECT count(*), count(workflow_run_id) INTO v_process_total, v_process_filled
      FROM dedup.cdeu_process_run;

    SELECT count(*) INTO v_process_mismatch
      FROM dedup.cdeu_process_run pr
      JOIN dedup.cdeu_phase_run phr ON phr.phase_run_id = pr.phase_run_id
     WHERE pr.workflow_run_id IS DISTINCT FROM phr.workflow_run_id;

    IF v_process_total <> v_process_filled THEN
        RAISE EXCEPTION 'V100043: cdeu_process_run 回填不完整，total=% filled=%',
            v_process_total, v_process_filled;
    END IF;

    IF v_process_mismatch > 0 THEN
        RAISE EXCEPTION 'V100043: cdeu_process_run 有 % 行与父表 workflow_run_id 不一致',
            v_process_mismatch;
    END IF;

    -- 4.2 cdeu_job_run：允许孤儿为 NULL，但已回填的必须与父表一致
    SELECT count(*), count(workflow_run_id) INTO v_job_total, v_job_filled
      FROM dedup.cdeu_job_run;

    SELECT count(*) INTO v_job_orphan
      FROM dedup.cdeu_job_run jr
      LEFT JOIN dedup.cdeu_process_run pr ON pr.process_run_id = jr.process_run_id
     WHERE pr.process_run_id IS NULL;

    SELECT count(*) INTO v_job_mismatch
      FROM dedup.cdeu_job_run jr
      JOIN dedup.cdeu_process_run pr ON pr.process_run_id = jr.process_run_id
     WHERE jr.workflow_run_id IS DISTINCT FROM pr.workflow_run_id;

    IF v_job_mismatch > 0 THEN
        RAISE EXCEPTION 'V100043: cdeu_job_run 有 % 行与父表 workflow_run_id 不一致',
            v_job_mismatch;
    END IF;

    IF v_job_total - v_job_filled <> v_job_orphan THEN
        RAISE EXCEPTION 'V100043: cdeu_job_run 空值行数(%) 与孤儿行数(%) 不符',
            v_job_total - v_job_filled, v_job_orphan;
    END IF;

    RAISE NOTICE 'V100043: cdeu_process_run %/% 已回填；cdeu_job_run %/% 已回填，孤儿 % 行保持 NULL',
        v_process_filled, v_process_total, v_job_filled, v_job_total, v_job_orphan;
END $$;

COMMIT;
