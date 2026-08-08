-- =====================================================================
-- FinvQuant PostgreSQL V23：回测账户表 finv_quant_backtest_account
--
-- 决策（ACANX 2026-08-06）：
--   - 回测账户 = 回测运行的"初始资金 + 交易成本 + 保证金模式"基线配置；
--   - initial_capital 初始启动资金（账户币种计价）；
--   - commission_rate 手续费率（按成交金额比例，单边）；slippage_pct 滑点
--     （按成交价比例，单边）——策略 definition.cost 可在任务级覆盖（优先策略）；
--   - margin_mode / margin_rate 为期货保证金模式预留：初始版本按全额模式
--     （margin_rate=1）撮合，后续支持 'FUTURES' 保证金杠杆模式；
--   - currency_type 对齐 finv_currency.currency_type（如 USD / CNY）；
--   - 账户为"仿真/回测"用途，与实盘账户（后续实盘交易模块）物理隔离。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

CREATE TABLE finv_quant_backtest_account (
    account_id        TEXT         PRIMARY KEY,             -- 账户 ID（UUID）
    account_code      TEXT         NOT NULL UNIQUE,         -- 账户编码（用户可读，全局唯一）
    account_name      TEXT         NOT NULL,                -- 账户名称
    user_id           TEXT         NOT NULL DEFAULT 'default', -- 所属用户（多用户隔离）
    group_id          TEXT,                                 -- 子账户分组/主账户归属（单用户多子账户，NULL=主账户）
    env_id            TEXT,                                 -- 默认关联环境（finv_quant_environment.env_id，可空）
    initial_capital   NUMERIC(20,6) NOT NULL
                      CHECK (initial_capital > 0),          -- 初始启动资金
    currency_type     TEXT         NOT NULL DEFAULT 'USD',  -- 计价货币（对齐 finv_currency.currency_type）
    commission_rate   NUMERIC(12,8) NOT NULL DEFAULT 0
                      CHECK (commission_rate >= 0),         -- 手续费率（按成交金额比例）
    slippage_pct      NUMERIC(12,8) NOT NULL DEFAULT 0
                      CHECK (slippage_pct >= 0),            -- 滑点（按成交价比例）
    margin_mode       TEXT         NOT NULL DEFAULT 'FULL'
                      CHECK (margin_mode IN ('FULL','FUTURES')), -- 保证金模式：FULL 全额 / FUTURES 期货保证金（预留）
    margin_rate       NUMERIC(12,8) NOT NULL DEFAULT 1
                      CHECK (margin_rate > 0 AND margin_rate <= 1), -- 保证金比例（FULL=1）
    allow_backtest    TEXT         NOT NULL DEFAULT '1'
                      CHECK (allow_backtest IN ('0','1')),  -- 回测开关
    status            TEXT         NOT NULL DEFAULT 'ENABLED'
                      CHECK (status IN ('DRAFT','ENABLED','DISABLED')),
    remark            TEXT,                                -- 备注
    created_by        TEXT,
    gmt_create        TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 按用户/分组查询（多用户多子账户）
CREATE INDEX idx_finv_quant_backtest_account_user
    ON finv_quant_backtest_account (user_id, status, allow_backtest);
CREATE INDEX idx_finv_quant_backtest_account_group
    ON finv_quant_backtest_account (group_id);

CREATE INDEX idx_finv_quant_backtest_account_status
    ON finv_quant_backtest_account (status, allow_backtest);

CREATE TRIGGER trg_finv_quant_backtest_account_gmt_update
    BEFORE UPDATE ON finv_quant_backtest_account
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
