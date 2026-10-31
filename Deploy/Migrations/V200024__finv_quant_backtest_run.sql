-- =====================================================================
-- FinvQuant PostgreSQL V24：回测任务表 finv_quant_backtest_run
--
-- 决策（ACANX 2026-08-06）：
--   - 一条回测任务 = 一次独立回放：策略快照 + 账户快照 + 标的时间区间 +
--     回测配置（周期/报告精度/回测开关/限制覆盖），运行结束后 report 保存
--     汇总指标（JSONB），曲线与成交明细分别落 finv_quant_backtest_equity /
--     finv_quant_backtest_trade，支持持久化保存与回看；
--   - 快照列（strategy_snapshot / account_snapshot / options）保证任务结果
--     可复现：策略/账户后续修改不影响历史任务；
--   - status 状态机：PENDING → RUNNING → SUCCEEDED / FAILED / CANCELLED；
--   - start_ts/end_ts 为 UTC 秒（对齐 finv_quote_secu_kline_min.ts），
--     start_date/end_date 为 yyyymmdd 便于按交易日过滤与展示；
--   - progress 0~100 供前端轮询进度条展示。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

CREATE TABLE finv_quant_backtest_run (
    run_id              TEXT        PRIMARY KEY,            -- 任务 ID（UUID）
    run_no              BIGINT      GENERATED ALWAYS AS IDENTITY, -- 任务序号（展示用，单调递增）
    user_id             TEXT        NOT NULL DEFAULT 'default', -- 所属用户（多用户隔离）
    strategy_id         TEXT        NOT NULL,               -- 引用策略（无物理外键，程序层控制）
    strategy_code       TEXT        NOT NULL,               -- 策略编码快照（冗余，便于列表展示）
    strategy_name       TEXT        NOT NULL,               -- 策略名称快照
    strategy_snapshot   JSONB       NOT NULL,               -- 策略定义快照（definition 全量）
    account_id          TEXT        NOT NULL,               -- 引用账户
    account_code        TEXT        NOT NULL,               -- 账户编码快照
    account_name        TEXT        NOT NULL,               -- 账户名称快照
    account_snapshot    JSONB       NOT NULL,               -- 账户配置快照（初始资金/成本/保证金模式）
    env_id              TEXT,                               -- 关联环境（finv_quant_environment.env_id，可空）
    env_snapshot        JSONB,                              -- 环境配置快照（交易时段/规则/成本，保证任务可复现）
    secu_code           TEXT        NOT NULL,               -- 回测标的证券代码（如 GCMain）
    market_code         INTEGER     NOT NULL DEFAULT 0,     -- 市场代码（对齐 finv_security.market_code）
    period              TEXT        NOT NULL DEFAULT 'Min'
                        CHECK (period IN ('Min','Hour','Day')),     -- 回测数据周期
    report_precision    TEXT        NOT NULL DEFAULT 'Day'
                        CHECK (report_precision IN ('Min','Hour','Day')), -- 报告时间精度
    start_ts            BIGINT      NOT NULL CHECK (start_ts >= 0),   -- 回测起始（UTC 秒）
    end_ts              BIGINT      NOT NULL CHECK (end_ts >= 0),     -- 回测结束（UTC 秒）
    start_date          INTEGER     NOT NULL DEFAULT 0,     -- 起始交易日 yyyymmdd（冗余展示）
    end_date            INTEGER     NOT NULL DEFAULT 0,     -- 结束交易日 yyyymmdd
    options             JSONB       NOT NULL DEFAULT '{}',  -- 回测配置快照（回测开关/限制覆盖等）
    status              TEXT        NOT NULL DEFAULT 'PENDING'
                        CHECK (status IN ('PENDING','RUNNING','SUCCEEDED','FAILED','CANCELLED')), -- 任务状态
    progress            INTEGER     NOT NULL DEFAULT 0
                        CHECK (progress BETWEEN 0 AND 100), -- 执行进度 0~100
    error_message       TEXT,                               -- 失败原因（FAILED 时）
    report              JSONB,                              -- 汇总回测报告（指标，SUCCEEDED 后写入）
    started_at          TIMESTAMPTZ,                        -- 开始执行时间
    finished_at         TIMESTAMPTZ,                        -- 结束时间（成功/失败/取消）
    created_by          TEXT,
    gmt_create          TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按用户/状态查询（多用户隔离 + 回测分析页默认视图）
CREATE INDEX idx_finv_quant_backtest_run_user
    ON finv_quant_backtest_run (user_id, status, gmt_create DESC);

-- 按状态查询（回测分析页默认按状态/时间倒序）
CREATE INDEX idx_finv_quant_backtest_run_status
    ON finv_quant_backtest_run (status, gmt_create DESC);

-- 按标的 + 时间查询（黄金期货回测验证页）
CREATE INDEX idx_finv_quant_backtest_run_secu
    ON finv_quant_backtest_run (secu_code, gmt_create DESC);

-- 按策略查询
CREATE INDEX idx_finv_quant_backtest_run_strategy
    ON finv_quant_backtest_run (strategy_id, gmt_create DESC);

CREATE TRIGGER trg_finv_quant_backtest_run_gmt_update
    BEFORE UPDATE ON finv_quant_backtest_run
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
