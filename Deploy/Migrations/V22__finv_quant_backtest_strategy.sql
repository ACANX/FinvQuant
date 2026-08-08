-- =====================================================================
-- FinvQuant PostgreSQL V22：回测策略定义表 finv_quant_backtest_strategy
--
-- 决策（ACANX 2026-08-06）：
--   - 通用量化回测首版：策略采用"结构化定义"模型，definition 列以 JSONB
--     保存完整的、可机器求值的策略定义（标的池/数据周期/指标/信号/规则/风控/成本），
--     通过 schema_version 字段支持定义结构演进（后续版本可平滑迁移）；
--   - 通用性设计：策略不绑定具体证券类型，通过 universe.securities 声明
--     标的池，可覆盖 ETF/股票/场外基金/国内期货/美股期货/黄金/石油商品期货等
--     任意已导入行情数据的证券（finv_security / finv_quote_secu_kline_min）；
--   - 回测开关 allow_backtest：'1' 允许被回测任务引用，'0' 禁止（配置层开关）；
--   - 冗余列 strategy_type / data_period / secu_code 便于列表检索与默认展示，
--     与 definition 内字段保持一致（程序层保证）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

CREATE TABLE finv_quant_backtest_strategy (
    strategy_id        TEXT        PRIMARY KEY,             -- 策略 ID（UUID）
    strategy_code      TEXT        NOT NULL UNIQUE,         -- 策略编码（用户可读，全局唯一）
    strategy_name      TEXT        NOT NULL,                -- 策略名称
    strategy_type      TEXT        NOT NULL DEFAULT 'RULE_BASED'
                       CHECK (strategy_type IN ('RULE_BASED','INDICATOR','MACHINE_LEARNING')), -- 策略类型（初始版本实现 RULE_BASED）
    description        TEXT,                                -- 策略说明
    definition         JSONB       NOT NULL,                -- 结构化策略定义（通用可扩展模型，见 Docs/DevSpec/BacktestStrategySpec.md）
    definition_version INTEGER     NOT NULL DEFAULT 1,      -- 定义结构版本（用于演进兼容）
    data_period        TEXT        NOT NULL DEFAULT 'Min'
                       CHECK (data_period IN ('Min','Hour','Day')),  -- 默认数据周期
    secu_code          TEXT,                                -- 默认标的证券代码（如 GCMain，可空，以 definition.universe 为准）
    user_id            TEXT        NOT NULL DEFAULT 'default', -- 所属用户（多用户隔离，接入认证后与登录态绑定）
    template_id        TEXT,                                -- 来源模板（finv_quant_template.template_id，可空）
    allow_backtest     TEXT        NOT NULL DEFAULT '1'
                       CHECK (allow_backtest IN ('0','1')), -- 回测开关：'1' 允许回测，'0' 禁止
    status             TEXT        NOT NULL DEFAULT 'ENABLED'
                       CHECK (status IN ('DRAFT','ENABLED','DISABLED')), -- 策略状态
    created_by         TEXT,                                -- 创建人
    gmt_create         TIMESTAMPTZ NOT NULL DEFAULT now(),  -- 首次插入时间
    gmt_update         TIMESTAMPTZ NOT NULL DEFAULT now()   -- 最后更新时间（触发器维护）
);

-- 按用户隔离查询
CREATE INDEX idx_finv_quant_backtest_strategy_user
    ON finv_quant_backtest_strategy (user_id, status, allow_backtest);

-- 按类型/开关/状态检索
CREATE INDEX idx_finv_quant_backtest_strategy_type
    ON finv_quant_backtest_strategy (strategy_type, status, allow_backtest);

-- 按标的检索（黄金期货回测验证页默认过滤 GCMain）
CREATE INDEX idx_finv_quant_backtest_strategy_secu
    ON finv_quant_backtest_strategy (secu_code);

CREATE TRIGGER trg_finv_quant_backtest_strategy_gmt_update
    BEFORE UPDATE ON finv_quant_backtest_strategy
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
