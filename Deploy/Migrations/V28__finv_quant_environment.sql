-- =====================================================================
-- FinvQuant PostgreSQL V28：回测环境与模板表
--
-- 决策（ACANX 2026-08-06，新增需求）：
--   - 环境（finv_quant_environment）：回测/模拟盘/仿真/实盘交易环境的配置差异、
--     不同市场（COMEX 黄金 vs 沪深 ETF 等）的交易约束与交易规则差异、不同地区的
--     习惯偏好差异；相同部分复用、差异部分自定义；回测任务创建时指定环境并保存
--     环境快照，支持动态切换环境，程序自适应环境配置；
--   - 环境 config JSONB 结构（约定，见 Docs/DevSpec/BacktestStrategySpec.md）：
--       {
--         "trading_sessions": [{"start":"082000","end":"133000"}],  // 交易时段 hhmmss
--         "trading_rules": {"t_plus": 0, "tick_size": 0.1, "contract_multiplier": 100,
--                            "limit_up_pct": 10, "limit_down_pct": 10},
--         "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001},  // 成本基准（环境>任务>策略>账户）
--         "fill_mode": "NEXT_BAR_OPEN",
--         "currency": "USD",
--         "preferences": {"date_format": "YYYY-MM-DD", "quote_direction": "RED_UP"}
--       }
--   - 模板（finv_quant_template）：策略模板/账户模板/环境模板三类，内置+自定义；
--     策略/账户/环境创建时可引用模板（template_id），相同部分复用、差异部分自定义；
--   - 多用户：环境/模板均带 user_id（默认 default，内置模板 user_id='system' 全局可见）。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 环境表
-- ---------------------------------------------------------------------
CREATE TABLE finv_quant_environment (
    env_id        TEXT        PRIMARY KEY,                  -- 环境 ID（UUID）
    env_code      TEXT        NOT NULL UNIQUE,              -- 环境编码（如 ENV-BT-COMEX-GC）
    env_name      TEXT        NOT NULL,                     -- 环境名称
    env_type      TEXT        NOT NULL DEFAULT 'BACKTEST'
                  CHECK (env_type IN ('BACKTEST','PAPER','SIMULATION','LIVE')), -- 环境类型：回测/模拟盘/仿真/实盘
    region        TEXT        DEFAULT '',                   -- 地区/区域（CN/US/HK...）
    market_code   INTEGER     NOT NULL DEFAULT 0,           -- 关联市场（对齐 finv_market.market_code，0=通用）
    config        JSONB       NOT NULL DEFAULT '{}',        -- 环境配置（交易时段/规则/成本/撮合/偏好）
    user_id       TEXT        NOT NULL DEFAULT 'default',   -- 所属用户（'system'=内置全局）
    is_default    TEXT        NOT NULL DEFAULT '0'
                  CHECK (is_default IN ('0','1')),          -- 是否默认环境（同一 user+type 仅一个默认）
    allow_backtest TEXT       NOT NULL DEFAULT '1'
                  CHECK (allow_backtest IN ('0','1')),      -- 回测开关
    status        TEXT        NOT NULL DEFAULT 'ENABLED'
                  CHECK (status IN ('DRAFT','ENABLED','DISABLED')),
    description   TEXT,                                     -- 说明
    created_by    TEXT,
    gmt_create    TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_finv_quant_environment_user
    ON finv_quant_environment (user_id, env_type, status);
CREATE INDEX idx_finv_quant_environment_market
    ON finv_quant_environment (market_code, env_type);

CREATE TRIGGER trg_finv_quant_environment_gmt_update
    BEFORE UPDATE ON finv_quant_environment
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 2. 模板表（策略/账户/环境三类模板）
-- ---------------------------------------------------------------------
CREATE TABLE finv_quant_template (
    template_id    TEXT        PRIMARY KEY,                 -- 模板 ID（UUID）
    template_code  TEXT        NOT NULL UNIQUE,             -- 模板编码（如 TPL-STRAT-DUALMA）
    template_name  TEXT        NOT NULL,                    -- 模板名称
    template_type  TEXT        NOT NULL
                   CHECK (template_type IN ('STRATEGY','ACCOUNT','ENVIRONMENT')), -- 模板类型
    content        JSONB       NOT NULL,                    -- 模板内容（STRATEGY=策略定义；ACCOUNT=账户配置；ENVIRONMENT=环境配置）
    user_id        TEXT        NOT NULL DEFAULT 'system',   -- 所属用户（'system'=内置模板全局可见）
    is_builtin     TEXT        NOT NULL DEFAULT '0'
                   CHECK (is_builtin IN ('0','1')),         -- 是否内置模板
    status         TEXT        NOT NULL DEFAULT 'ENABLED'
                   CHECK (status IN ('DRAFT','ENABLED','DISABLED')),
    description    TEXT,
    created_by     TEXT,
    gmt_create     TIMESTAMPTZ NOT NULL DEFAULT now(),
    gmt_update     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_finv_quant_template_user
    ON finv_quant_template (user_id, template_type, status);
CREATE INDEX idx_finv_quant_template_type
    ON finv_quant_template (template_type, is_builtin);

-- 环境默认唯一：同用户 + 同环境类型仅允许一个默认（评审落实，配合 SaveEnvironment 事务清理）
CREATE UNIQUE INDEX uq_finv_quant_environment_default
    ON finv_quant_environment (user_id, env_type)
    WHERE is_default = '1';

CREATE TRIGGER trg_finv_quant_template_gmt_update
    BEFORE UPDATE ON finv_quant_template
    FOR EACH ROW EXECUTE FUNCTION vq_set_gmt_update();

COMMIT;
