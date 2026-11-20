-- =====================================================================
-- FinvQuant PostgreSQL V26：回测成交记录表 finv_quant_backtest_trade
--
-- 决策（ACANX 2026-08-06）：
--   - 记录每次撮合成交（BUY/SELL），含成交价/数量/金额/手续费/成交后现金与
--     持仓（便于回看任意时点账户状态）；
--   - profit 仅 SELL（平仓）时记录该笔平仓盈亏（相对该仓位加权买入成本）；
--   - signal 记录触发原因（信号表达式命中/止损/止盈/规则名），便于报告归因；
--   - 数量 qty 用 NUMERIC(20,6) 兼容期货手数小数与股票股数整数。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

CREATE TABLE finv_quant_backtest_trade (
    trade_id       BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id         TEXT         NOT NULL,                   -- 关联回测任务
    seq            INTEGER      NOT NULL DEFAULT 0,         -- 引擎内成交顺序号（明细表 trade_id 关联用）
    ts             BIGINT       NOT NULL CHECK (ts >= 0),   -- 成交时间（UTC 秒）
    date           INTEGER      DEFAULT 0,                  -- 成交日期 yyyymmdd
    "time"         INTEGER      DEFAULT 0,                  -- 成交时间 hhmmss
    action         TEXT         NOT NULL CHECK (action IN ('BUY','SELL')), -- 买卖方向
    price          NUMERIC(20,6) NOT NULL,                  -- 成交价（含滑点）
    qty            NUMERIC(20,6) NOT NULL CHECK (qty > 0),  -- 成交数量
    amount         NUMERIC(20,6) NOT NULL,                  -- 成交金额（价*量）
    fee            NUMERIC(20,6) NOT NULL DEFAULT 0,        -- 手续费
    profit         NUMERIC(20,6) DEFAULT 0,                 -- 平仓盈亏（SELL 时）
    position_after NUMERIC(20,6) NOT NULL DEFAULT 0,        -- 成交后持仓数量
    cash_after     NUMERIC(20,6) NOT NULL,                  -- 成交后现金余额
    signal         TEXT         DEFAULT '',                 -- 触发信号/原因
    remark         TEXT                                     -- 备注
);

CREATE INDEX idx_finv_quant_backtest_trade_run
    ON finv_quant_backtest_trade (run_id, ts);

COMMIT;
