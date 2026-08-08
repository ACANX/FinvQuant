-- =====================================================================
-- FinvQuant PostgreSQL V100023：回测任务删除任务与审计日志
--
-- 背景（ACANX 需求）：回测任务可能产生大量无参考价值记录（如 bug 测试
--   产生的净值曲线/成交/资金流水/持仓/事件追踪明细），需按回测任务 ID
--   删除任务及其关联记录（策略/账户/环境/模板保留）。
--
-- 设计：
--   - finv_quant_backtest_del_task：删除任务（异步执行，状态/进度/错误/
--     各表删除行数），del_task_id 为 UUID 大写（与 run_id 等主键一致）；
--   - finv_quant_backtest_del_log：删除审计日志（只追加不可改），按
--     del_task_id 可查完整留痕（创建/每表删除行数/成功/失败）；
--   - finv_quant_backtest_run_archive：已删除任务归档表（"曾经存在的证明"），
--     删除任务时先把 run 元信息（任务号/策略/账户/标的/区间/状态/错误信息/
--     创建与结束时间）归档留痕，再物理删除明细与主表；失败/接入任务等
--     被删任务的执行记录在此可查，不会随删除而消失。
--
-- 幂等：CREATE TABLE IF NOT EXISTS，重复执行安全。
-- =====================================================================

-- 删除任务表
CREATE TABLE IF NOT EXISTS finv_quant_backtest_del_task (
    del_task_id    TEXT        PRIMARY KEY,             -- 删除任务 ID（UUID 大写）
    run_id         TEXT        NOT NULL,                -- 待删除的回测任务 ID
    status         TEXT        NOT NULL DEFAULT 'PENDING', -- PENDING/RUNNING/SUCCEEDED/FAILED
    progress       INTEGER     NOT NULL DEFAULT 0,      -- 进度（0-100）
    error_message  TEXT,                                -- 失败原因（可空）
    deleted_counts JSONB,                               -- 各表删除行数 {表名: 行数}
    created_by     TEXT        NOT NULL DEFAULT 'console',
    gmt_create     TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_bt_del_task_run ON finv_quant_backtest_del_task (run_id);

-- 删除审计日志表（只追加）
CREATE TABLE IF NOT EXISTS finv_quant_backtest_del_log (
    log_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    del_task_id  TEXT        NOT NULL,                  -- 关联删除任务
    run_id       TEXT        NOT NULL,                  -- 关联回测任务
    seq          INTEGER     NOT NULL,                  -- 日志序号（按序递增）
    action       TEXT        NOT NULL,                  -- TASK_CREATED/DELETING_TABLE/TASK_SUCCEEDED/TASK_FAILED
    detail       TEXT,                                  -- 详情（如表名+删除行数/错误信息）
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_bt_del_log_task ON finv_quant_backtest_del_log (del_task_id);
CREATE INDEX IF NOT EXISTS idx_bt_del_log_run ON finv_quant_backtest_del_log (run_id);

-- 已删除任务归档表（"曾经存在的证明"）：删除任务时先把 run 元信息归档留痕，
-- 再物理删除明细与主表。失败/已结束任务的执行记录（任务号/策略/状态/错误等）
-- 在此可查，不随删除而消失。
CREATE TABLE IF NOT EXISTS finv_quant_backtest_run_archive (
    archive_id      TEXT        PRIMARY KEY,             -- 归档记录 ID（UUID 大写）
    run_id          TEXT        NOT NULL,                -- 原回测任务 ID（删除后仍可追溯）
    run_no          BIGINT      NOT NULL,                -- 原任务号（保留作为证明）
    strategy_id     TEXT,                                -- 关联策略
    strategy_name   TEXT,                                -- 策略名称
    account_id      TEXT,                                -- 关联账户
    account_name    TEXT,
    secu_code       TEXT,
    period          TEXT,
    start_date      INTEGER,
    end_date        INTEGER,
    status          TEXT        NOT NULL,                -- 删除前任务状态（SUCCEEDED/FAILED/CANCELLED）
    error_message   TEXT,                                -- 原任务错误信息（失败原因留痕）
    report_json     TEXT,                                -- 原任务报告 JSONB（若存在，转文本归档）
    deleted_at      TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 删除时间
    deleted_by      TEXT        NOT NULL,                -- 删除操作人
    del_task_id     TEXT        NOT NULL                 -- 关联删除任务
);
CREATE INDEX IF NOT EXISTS idx_bt_run_archive_run ON finv_quant_backtest_run_archive (run_id);
CREATE INDEX IF NOT EXISTS idx_bt_run_archive_no ON finv_quant_backtest_run_archive (run_no);
CREATE INDEX IF NOT EXISTS idx_bt_run_archive_del ON finv_quant_backtest_run_archive (del_task_id);
