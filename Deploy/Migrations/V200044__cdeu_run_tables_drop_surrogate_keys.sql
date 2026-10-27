-- =====================================================================
-- FinvQuant PostgreSQL V100044：CDEU 运行实例表去除自增代理键
--
-- 决策依据：DbSchemaSpec §0 红线 + 用户明确授权（2026-10-27）
--   「先加字段，然后删这两个自增的字段」
--   ⇒ 删除 cdeu_process_run.process_run_id 与 cdeu_job_run.job_run_id
--   cdeu_workflow_run.run_id 保持自增（唯一的运行入口流水号）
--   cdeu_job_step_run.step_run_id 保持自增（本迁移不涉及）
--
-- 前置条件（已在本迁移之前完成）：
--   1) V100043 已给 cdeu_process_run / cdeu_job_run 加 workflow_run_id 并回填；
--   2) 历史脏数据已备份并删除：孤儿 156 条 + 无法归属的批次重复 433 条
--      （备份表 dedup.cdeu_job_run_purge / dedup.cdeu_job_step_run_purge），
--      清理后 cdeu_job_run 208 行、cdeu_job_step_run 208 行，
--      (workflow_run_id, process_id, job_id) 已唯一。
--
-- 新主键（均已在清理后验证唯一）：
--   cdeu_process_run  : (workflow_run_id, process_id)
--   cdeu_job_run      : (workflow_run_id, process_id, job_id)
--   cdeu_job_step_run : 保持 step_run_id 自增主键，job_run_id 改为三元组引用
--
-- 新增列：
--   cdeu_job_run.dynamic_job_id bigint（可空）
--     指向 dedup.cdeu_job_dynamic.dynamic_job_id，记录该次执行对应的动态工作项；
--     非批次 JOB 为 NULL。批次维度由 cdeu_job_dynamic 承载，
--     因此不再需要 batch_key（该列历史上从未写入，本迁移保留列不动）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. DDL 前整表快照（双保险，防止改主键失败后无法回退）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dedup.cdeu_process_run_bak_v100044  AS
SELECT * FROM dedup.cdeu_process_run;

CREATE TABLE IF NOT EXISTS dedup.cdeu_job_run_bak_v100044 AS
SELECT * FROM dedup.cdeu_job_run;

CREATE TABLE IF NOT EXISTS dedup.cdeu_job_step_run_bak_v100044 AS
SELECT * FROM dedup.cdeu_job_step_run;

COMMENT ON TABLE dedup.cdeu_process_run_bak_v100044 IS
    'V100044 删除 process_run_id 前的整表快照（2026-10-27）';
COMMENT ON TABLE dedup.cdeu_job_run_bak_v100044 IS
    'V100044 删除 job_run_id 前的整表快照（2026-10-27）';
COMMENT ON TABLE dedup.cdeu_job_step_run_bak_v100044 IS
    'V100044 改造 job_run_id 引用前的整表快照（2026-10-27）';

-- ---------------------------------------------------------------------
-- 2. cdeu_job_run 补齐 process_id / dynamic_job_id
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_job_run ADD COLUMN IF NOT EXISTS process_id     bigint;
ALTER TABLE dedup.cdeu_job_run ADD COLUMN IF NOT EXISTS dynamic_job_id bigint;

UPDATE dedup.cdeu_job_run jr
   SET process_id = pr.process_id
  FROM dedup.cdeu_process_run pr
 WHERE pr.process_run_id = jr.process_run_id
   AND jr.process_id IS NULL;

ALTER TABLE dedup.cdeu_job_run ALTER COLUMN process_id      SET NOT NULL;
ALTER TABLE dedup.cdeu_job_run ALTER COLUMN workflow_run_id SET NOT NULL;

COMMENT ON COLUMN dedup.cdeu_job_run.process_id IS
    'Process 定义 ID（主键组成列，替代已删除的 process_run_id）';
COMMENT ON COLUMN dedup.cdeu_job_run.dynamic_job_id IS
    '动态工作项 ID，逻辑关联 dedup.cdeu_job_dynamic.dynamic_job_id；非批次 JOB 为 NULL';
COMMENT ON COLUMN dedup.cdeu_job_run.workflow_run_id IS
    '工作流运行 ID（主键组成列）';
COMMENT ON COLUMN dedup.cdeu_job_run.batch_key IS
    '已废弃：批次维度改由 dynamic_job_id 承载；该列历史值为空，不再写入';

-- ---------------------------------------------------------------------
-- 3. cdeu_job_step_run 把 job_run_id 换成三元组引用
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_job_step_run
    ADD COLUMN IF NOT EXISTS workflow_run_id bigint,
    ADD COLUMN IF NOT EXISTS process_id      bigint,
    ADD COLUMN IF NOT EXISTS job_id          bigint;

UPDATE dedup.cdeu_job_step_run sr
   SET workflow_run_id = jr.workflow_run_id,
       process_id      = jr.process_id,
       job_id          = jr.job_id
  FROM dedup.cdeu_job_run jr
 WHERE jr.job_run_id = sr.job_run_id;

ALTER TABLE dedup.cdeu_job_step_run
    ALTER COLUMN workflow_run_id SET NOT NULL,
    ALTER COLUMN process_id      SET NOT NULL,
    ALTER COLUMN job_id          SET NOT NULL;

-- ---------------------------------------------------------------------
-- 4. 删除依赖旧列的索引与唯一约束
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_job_step_run
    DROP CONSTRAINT IF EXISTS cdeu_job_step_run_job_run_id_step_code_key;

DROP INDEX IF EXISTS dedup.idx_cdeu_job_step_run_job;
DROP INDEX IF EXISTS dedup.idx_cdeu_job_run_process;

-- ---------------------------------------------------------------------
-- 5. 删除旧主键与自增列
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_job_step_run DROP CONSTRAINT IF EXISTS cdeu_job_step_run_pkey;
ALTER TABLE dedup.cdeu_job_run      DROP CONSTRAINT IF EXISTS cdeu_job_run_pkey;
ALTER TABLE dedup.cdeu_process_run  DROP CONSTRAINT IF EXISTS cdeu_step_run_pkey;

ALTER TABLE dedup.cdeu_job_step_run DROP COLUMN IF EXISTS job_run_id;
ALTER TABLE dedup.cdeu_job_run      DROP COLUMN IF EXISTS job_run_id;
ALTER TABLE dedup.cdeu_job_run      DROP COLUMN IF EXISTS process_run_id;
ALTER TABLE dedup.cdeu_process_run  DROP COLUMN IF EXISTS process_run_id;

-- 连同遗留序列一起清理（V50 改名后序列名仍为 step_run 前缀）
DROP SEQUENCE IF EXISTS dedup.cdeu_step_run_step_run_id_seq;
DROP SEQUENCE IF EXISTS dedup.cdeu_job_run_job_run_id_seq;

-- ---------------------------------------------------------------------
-- 6. 建立新主键
-- ---------------------------------------------------------------------
-- V50 表名互换遗留：cdeu_workflow_run（原 cdeu_process_run 改名）的主键
-- 仍名为 cdeu_process_run_pkey，会与本次新建的名称冲突，先改名归位。
ALTER TABLE dedup.cdeu_workflow_run
    RENAME CONSTRAINT cdeu_process_run_pkey TO cdeu_workflow_run_pkey;

ALTER TABLE dedup.cdeu_process_run
    ADD CONSTRAINT cdeu_process_run_pkey PRIMARY KEY (workflow_run_id, process_id);

ALTER TABLE dedup.cdeu_job_run
    ADD CONSTRAINT cdeu_job_run_pkey PRIMARY KEY (workflow_run_id, process_id, job_id);

ALTER TABLE dedup.cdeu_job_step_run
    ADD CONSTRAINT cdeu_job_step_run_pkey PRIMARY KEY (step_run_id);

-- ---------------------------------------------------------------------
-- 7. 新的唯一约束与索引
-- ---------------------------------------------------------------------
ALTER TABLE dedup.cdeu_job_step_run
    ADD CONSTRAINT cdeu_job_step_run_key_job_step
    UNIQUE (workflow_run_id, process_id, job_id, step_code);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_step_run_seq
    ON dedup.cdeu_job_step_run (workflow_run_id, process_id, job_id, seq_no);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_process_code
    ON dedup.cdeu_job_run (workflow_run_id, process_id, job_code);

CREATE INDEX IF NOT EXISTS idx_cdeu_job_run_dynamic
    ON dedup.cdeu_job_run (dynamic_job_id)
 WHERE dynamic_job_id IS NOT NULL;

-- ---------------------------------------------------------------------
-- 8. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cnt            bigint;
    v_leftover       int;
BEGIN
    -- 8.1 自增列必须已不存在
    SELECT count(*) INTO v_leftover
      FROM information_schema.columns
     WHERE table_schema = 'dedup'
       AND ((table_name = 'cdeu_process_run'  AND column_name = 'process_run_id')
         OR (table_name = 'cdeu_job_run'      AND column_name IN ('job_run_id', 'process_run_id'))
         OR (table_name = 'cdeu_job_step_run' AND column_name = 'job_run_id'));
    IF v_leftover > 0 THEN
        RAISE EXCEPTION 'V100044: 仍有 % 个自增列未删除', v_leftover;
    END IF;

    -- 8.2 新主键必须存在
    SELECT count(*) INTO v_leftover
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
      JOIN pg_namespace n ON n.oid = t.relnamespace
     WHERE n.nspname = 'dedup' AND c.contype = 'p'
       AND c.conname IN ('cdeu_process_run_pkey', 'cdeu_job_run_pkey',
                         'cdeu_job_step_run_pkey', 'cdeu_workflow_run_pkey');
    IF v_leftover <> 4 THEN
        RAISE EXCEPTION 'V100044: 期望 4 个主键（含 workflow_run），实际 %', v_leftover;
    END IF;

    -- 8.3 三个表的行数必须与备份一致
    SELECT count(*) INTO v_cnt FROM dedup.cdeu_process_run;
    IF v_cnt <> (SELECT count(*) FROM dedup.cdeu_process_run_bak_v100044) THEN
        RAISE EXCEPTION 'V100044: cdeu_process_run 行数异常 %', v_cnt;
    END IF;

    SELECT count(*) INTO v_cnt FROM dedup.cdeu_job_run;
    IF v_cnt <> (SELECT count(*) FROM dedup.cdeu_job_run_bak_v100044) THEN
        RAISE EXCEPTION 'V100044: cdeu_job_run 行数异常 %', v_cnt;
    END IF;

    SELECT count(*) INTO v_cnt FROM dedup.cdeu_job_step_run;
    IF v_cnt <> (SELECT count(*) FROM dedup.cdeu_job_step_run_bak_v100044) THEN
        RAISE EXCEPTION 'V100044: cdeu_job_step_run 行数异常 %', v_cnt;
    END IF;

    -- 8.4 job_step_run 必须全部能挂回 job_run
    SELECT count(*) INTO v_leftover
      FROM dedup.cdeu_job_step_run sr
      LEFT JOIN dedup.cdeu_job_run jr
             ON jr.workflow_run_id = sr.workflow_run_id
            AND jr.process_id      = sr.process_id
            AND jr.job_id          = sr.job_id
     WHERE jr.workflow_run_id IS NULL;
    IF v_leftover > 0 THEN
        RAISE EXCEPTION 'V100044: % 条 job_step_run 无法挂回 job_run', v_leftover;
    END IF;

    RAISE NOTICE 'V100044 完成：process_run=% job_run=% job_step_run=%',
        (SELECT count(*) FROM dedup.cdeu_process_run),
        (SELECT count(*) FROM dedup.cdeu_job_run),
        (SELECT count(*) FROM dedup.cdeu_job_step_run);
END $$;

COMMIT;
