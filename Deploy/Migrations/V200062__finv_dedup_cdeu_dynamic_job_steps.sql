-- =====================================================================
-- FinvQuant PostgreSQL V62：动态导出 JOB 的串行 Step
--
-- 每个 cdeu_dynamic_job 固定拆为：
--   STEP_LOAD → STEP_EXPORT → STEP_REGISTER → STEP_CLEANUP
-- 运行时按实际导出阶段更新 Step 状态，前端据此展示 JOB 当前步骤。
-- =====================================================================

CREATE TABLE IF NOT EXISTS dedup.cdeu_dynamic_job_step (
    dynamic_step_id bigserial PRIMARY KEY,
    dynamic_job_id  bigint NOT NULL,
    step_code       text NOT NULL,
    name            text NOT NULL,
    seq_no          int NOT NULL,
    status          text NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING','RUNNING','SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')),
    started_at      timestamptz,
    finished_at     timestamptz,
    error           text,
    gmt_create      timestamptz NOT NULL DEFAULT now(),
    gmt_update      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dynamic_job_id, step_code),
    UNIQUE (dynamic_job_id, seq_no)
);

CREATE INDEX IF NOT EXISTS idx_cdeu_dynamic_job_step_job
    ON dedup.cdeu_dynamic_job_step (dynamic_job_id, seq_no);

COMMENT ON TABLE dedup.cdeu_dynamic_job_step IS '动态单证券导出 JOB 内的串行 Step：LOAD / EXPORT / REGISTER / CLEANUP';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.dynamic_step_id IS '动态 Step 主键';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.dynamic_job_id IS '所属动态 JOB ID';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.step_code IS 'Step 编码：STEP_LOAD / STEP_EXPORT / STEP_REGISTER / STEP_CLEANUP';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.name IS 'Step 中文名称';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.seq_no IS 'JOB 内串行顺序';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.status IS 'Step 运行状态';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.started_at IS '开始时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.finished_at IS '结束时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.error IS '失败原因';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.gmt_create IS '创建时间';
COMMENT ON COLUMN dedup.cdeu_dynamic_job_step.gmt_update IS '更新时间';

-- 为已有动态 JOB 补齐四个 Step（初始 PENDING）。
INSERT INTO dedup.cdeu_dynamic_job_step
    (dynamic_job_id, step_code, name, seq_no, status)
SELECT dj.dynamic_job_id, v.step_code, v.name, v.seq_no, 'PENDING'
  FROM dedup.cdeu_dynamic_job dj
 CROSS JOIN (VALUES
    ('STEP_LOAD', '装载', 1),
    ('STEP_EXPORT', '文件生成', 2),
    ('STEP_REGISTER', '文件登记', 3),
    ('STEP_CLEANUP', '中转清理', 4)
 ) AS v(step_code, name, seq_no)
ON CONFLICT (dynamic_job_id, step_code) DO NOTHING;
