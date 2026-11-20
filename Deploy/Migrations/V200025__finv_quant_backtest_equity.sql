-- =====================================================================
-- FinvQuant PostgreSQL V25：回测净值/曲线数据表 finv_quant_backtest_equity
--
-- 决策（ACANX 2026-08-06）：
--   - 按"报告时间精度"（report_precision：Day/Hour/Min）逐点落库，
--     前端按 run_id 拉取即可绘制：账户余额/收益率/收益额/持仓金额四条曲线
--     （对应需求报告 ①③⑦⑧ 项的结构化数据）；
--   - equity   = cash + position_value（总资产，持仓换算成现金口径）；
--   - profit   = equity - initial_capital（累计盈亏额）；
--   - roi      = profit / initial_capital * 100（累计收益率 %）；
--   - drawdown = 当前净值相对历史峰值的回撤 %（负数或正数按约定：存正数表示回撤幅度，
--     前端按需取负展示；此处存正值 pct，如 5.23 表示回撤 5.23%）；
--   - 同一 run 内 ts 唯一（MINUTE 精度也满足：同一秒只记录一个点）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

CREATE TABLE finv_quant_backtest_equity (
    id               BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id           TEXT         NOT NULL,                 -- 关联回测任务
    seq              INTEGER      NOT NULL DEFAULT 0,       -- 序列号（报告点顺序 1..N）
    ts               BIGINT       NOT NULL CHECK (ts >= 0), -- 报告点时间（UTC 秒）
    date             INTEGER      DEFAULT 0,                -- 交易日期 yyyymmdd
    "time"           INTEGER      DEFAULT 0,                -- 交易时间 hhmmss
    equity           NUMERIC(20,6) NOT NULL,                -- 总资产（现金+持仓市值）
    cash             NUMERIC(20,6) NOT NULL,                -- 现金余额
    position_value   NUMERIC(20,6) NOT NULL DEFAULT 0,      -- 持仓市值
    position_qty     NUMERIC(20,6) NOT NULL DEFAULT 0,      -- 持仓数量
    profit           NUMERIC(20,6) NOT NULL DEFAULT 0,      -- 累计盈亏额
    roi              NUMERIC(20,6) NOT NULL DEFAULT 0,      -- 累计收益率 %
    drawdown         NUMERIC(20,6) NOT NULL DEFAULT 0,      -- 回撤幅度 %
    UNIQUE (run_id, ts)
);

CREATE INDEX idx_finv_quant_backtest_equity_run
    ON finv_quant_backtest_equity (run_id, seq);

COMMIT;
