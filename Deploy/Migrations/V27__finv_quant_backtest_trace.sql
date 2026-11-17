-- =====================================================================
-- FinvQuant PostgreSQL V27：回测链路追踪明细表（需求⑨ 持仓变动详细情况链路追踪分析）
--
-- 决策（ACANX 2026-08-06）：
--   - 需求⑨要求三个维度：
--       1. 资金流水明细（finv_quant_backtest_cashflow）：初始资金注入/买入付款/
--          卖出收款/手续费/保证金占用与释放，记录每次资金变动前后余额；
--       2. 持仓变化明细（finv_quant_backtest_position_log）：开仓/加仓/减仓/平仓，
--          记录变动前后数量与加权成本；
--       3. 交易事件追踪（finv_quant_backtest_event_trace）：每个交易事件触发的
--          结果追踪——触发原因（买入信号/卖出信号/止损/止盈）、成交结果与否
--          （FILLED/REJECTED/PENDING/EXPIRED）、委托耗时（触发→成交的 bar 数与
--          秒数）、未能成交的原因（资金不足/超过每日笔数上限/最小间隔不足/
--          仓位已满/无持仓/时间点限制/规则次数限制等）；
--   - 表名遵循量化模块统一前缀 finv_quant_（与 finv_quant_backtest_* 系列一致）；
--   - run_id 均关联 finv_quant_backtest_run，无物理外键（项目惯例：程序层控制）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 资金流水明细
-- ---------------------------------------------------------------------
CREATE TABLE finv_quant_backtest_cashflow (
    cashflow_id  BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id       TEXT         NOT NULL,                     -- 关联回测任务
    seq          INTEGER      NOT NULL DEFAULT 0,           -- 流水顺序号
    ts           BIGINT       NOT NULL CHECK (ts >= 0),     -- 发生时间（UTC 秒）
    date         INTEGER      DEFAULT 0,                    -- 发生日期 yyyymmdd
    "time"       INTEGER      DEFAULT 0,                    -- 发生时间 hhmmss
    flow_type    TEXT         NOT NULL,                     -- 类型：INITIAL_DEPOSIT/BUY_PAY/SELL_RECEIVE/FEE/MARGIN_HOLD/MARGIN_RELEASE
    amount       NUMERIC(20,6) NOT NULL,                    -- 变动金额（负=支出，正=收入）
    cash_before  NUMERIC(20,6) NOT NULL,                    -- 变动前现金
    cash_after   NUMERIC(20,6) NOT NULL,                    -- 变动后现金
    trade_id     BIGINT       DEFAULT 0,                    -- 关联成交记录（finv_quant_backtest_trade.trade_id，无则 0）
    remark       TEXT                                       -- 备注
);
CREATE INDEX idx_finv_quant_backtest_cashflow_run
    ON finv_quant_backtest_cashflow (run_id, seq);

-- ---------------------------------------------------------------------
-- 2. 持仓变化明细
-- ---------------------------------------------------------------------
CREATE TABLE finv_quant_backtest_position_log (
    log_id          BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id          TEXT         NOT NULL,                  -- 关联回测任务
    seq             INTEGER      NOT NULL DEFAULT 0,        -- 明细顺序号
    ts              BIGINT       NOT NULL CHECK (ts >= 0),  -- 发生时间（UTC 秒）
    date            INTEGER      DEFAULT 0,
    "time"          INTEGER      DEFAULT 0,
    action          TEXT         NOT NULL
                    CHECK (action IN ('OPEN','ADD','REDUCE','CLOSE')), -- 开仓/加仓/减仓/平仓
    price           NUMERIC(20,6) NOT NULL,                 -- 成交价
    qty             NUMERIC(20,6) NOT NULL,                 -- 变动数量（正=增加，负=减少）
    position_before NUMERIC(20,6) NOT NULL DEFAULT 0,       -- 变动前持仓
    position_after  NUMERIC(20,6) NOT NULL DEFAULT 0,       -- 变动后持仓
    avg_cost_before NUMERIC(20,6) DEFAULT 0,                -- 变动前加权成本
    avg_cost_after  NUMERIC(20,6) DEFAULT 0,                -- 变动后加权成本
    trade_id        BIGINT       DEFAULT 0,                 -- 关联成交记录
    remark          TEXT                                    -- 备注（如触发信号）
);
CREATE INDEX idx_finv_quant_backtest_poslog_run
    ON finv_quant_backtest_position_log (run_id, seq);

-- ---------------------------------------------------------------------
-- 3. 交易事件追踪（触发原因/成交结果/委托耗时/未成交原因）
-- ---------------------------------------------------------------------
CREATE TABLE finv_quant_backtest_event_trace (
    event_id       BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id         TEXT         NOT NULL,                   -- 关联回测任务
    seq            INTEGER      NOT NULL DEFAULT 0,         -- 事件顺序号
    action         TEXT         NOT NULL CHECK (action IN ('BUY','SELL')), -- 交易方向
    trigger_reason TEXT         NOT NULL,                   -- 触发原因：买入信号/卖出信号/止损/止盈
    trigger_ts     BIGINT       NOT NULL CHECK (trigger_ts >= 0),  -- 触发时点（UTC 秒）
    trigger_date   INTEGER      DEFAULT 0,                  -- 触发日期 yyyymmdd
    trigger_time   INTEGER      DEFAULT 0,                  -- 触发时间 hhmmss
    order_ts       BIGINT       DEFAULT 0,                  -- ⑤ 委托下单时间（信号确认后立即下单，= 触发时点）
    order_date     INTEGER      DEFAULT 0,                  -- 委托下单日期 yyyymmdd
    order_time     INTEGER      DEFAULT 0,                  -- 委托下单时间 hhmmss
    exec_status    TEXT         NOT NULL DEFAULT 'PENDING'
                   CHECK (exec_status IN ('PENDING','FILLED','REJECTED','EXPIRED')), -- 事件结果：挂单/成交/拒绝/过期
    exec_ts        BIGINT       DEFAULT 0,                  -- 成交/处理时点（0=未处理）
    exec_date      INTEGER      DEFAULT 0,
    exec_time      INTEGER      DEFAULT 0,
    latency_bars   INTEGER      DEFAULT 0,                  -- ⑥ 委托耗时（bar 数：触发→成交）
    latency_sec    BIGINT       DEFAULT 0,                  -- ⑥ 委托耗时（秒）
    alive_sec      BIGINT       DEFAULT 0,                  -- ⑦ 事件存活时间（结束时间-触发时间，秒）
    reject_reason  TEXT         DEFAULT '',                 -- ⑧ 未能成交的原因（REJECTED/EXPIRED 时）
    price          NUMERIC(20,6) DEFAULT 0,                 -- 拟成交价/成交价
    qty            NUMERIC(20,6) DEFAULT 0,                 -- 拟成交数量/成交数量
    trade_id       BIGINT       DEFAULT 0                   -- 关联成交记录（FILLED 时）
);
CREATE INDEX idx_finv_quant_backtest_event_run
    ON finv_quant_backtest_event_trace (run_id, seq);

COMMIT;
