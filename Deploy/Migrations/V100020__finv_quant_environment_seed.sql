-- =====================================================================
-- FinvQuant PostgreSQL V100020：环境与模板种子数据
--
-- 决策（ACANX 2026-08-06，新增需求）：
--   - 默认环境：COMEX 黄金期货回测环境（GCMain）、沪深 ETF 回测环境（示例）；
--   - 内置模板：环境模板（COMEX/沪深）、策略模板（双均线/RSI/布林带/MACD）；
--   - user_id='system' 表示内置全局可见；幂等：ON CONFLICT DO NOTHING。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 默认环境
-- ---------------------------------------------------------------------
-- COMEX 黄金期货回测环境（GCMain，24 小时电子盘：全天时段，行情在即可交易；
-- 如需限定主力时段可在环境管理中调整 trading_sessions）
INSERT INTO finv_quant_environment
    (env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by)
VALUES
    ('E0000000-0000-4000-8000-000000000001', 'ENV-BT-COMEX-GC', 'COMEX 黄金期货回测环境', 'BACKTEST', 'US', 0,
     '{
        "trading_sessions": [{"start": "000000", "end": "235959"}],
        "trading_rules": {"t_plus": 0, "tick_size": 0.1, "contract_multiplier": 100, "limit_up_pct": 0, "limit_down_pct": 0},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001},
        "fill_mode": "NEXT_BAR_OPEN",
        "currency": "USD",
        "preferences": {"date_format": "YYYY-MM-DD", "quote_direction": "RED_UP"}
     }'::jsonb,
     'system', '1', '1', 'ENABLED', 'COMEX 黄金期货（GCMain 主连）回测默认环境（全天交易时段）', 'system')
ON CONFLICT (env_id) DO NOTHING;

-- 沪深 ETF 回测环境（示例：T+1、涨跌停 10%、CNY）
INSERT INTO finv_quant_environment
    (env_id, env_code, env_name, env_type, region, market_code, config, user_id, is_default, allow_backtest, status, description, created_by)
VALUES
    ('E0000000-0000-4000-8000-000000000002', 'ENV-BT-CN-ETF', '沪深 ETF 回测环境', 'BACKTEST', 'CN', 0,
     '{
        "trading_sessions": [{"start": "093000", "end": "113000"}, {"start": "130000", "end": "150000"}],
        "trading_rules": {"t_plus": 1, "tick_size": 0.001, "contract_multiplier": 1, "limit_up_pct": 10, "limit_down_pct": 10},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001},
        "fill_mode": "NEXT_BAR_OPEN",
        "currency": "CNY",
        "preferences": {"date_format": "YYYY-MM-DD", "quote_direction": "RED_UP"}
     }'::jsonb,
     'system', '0', '1', 'ENABLED', '沪深 ETF 回测环境（T+1，涨跌停 10%）', 'system')
ON CONFLICT (env_id) DO NOTHING;

-- ---------------------------------------------------------------------
-- 内置模板
-- ---------------------------------------------------------------------
-- 环境模板：COMEX 黄金回测
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000001', 'TPL-ENV-COMEX-GC', 'COMEX 黄金回测环境模板', 'ENVIRONMENT',
     '{
        "env_code": "ENV-BT-COMEX-GC", "env_name": "COMEX 黄金期货回测环境", "env_type": "BACKTEST", "region": "US",
        "config": {
            "trading_sessions": [{"start": "000000", "end": "235959"}],
            "trading_rules": {"t_plus": 0, "tick_size": 0.1, "contract_multiplier": 100, "limit_up_pct": 0, "limit_down_pct": 0},
            "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001},
            "fill_mode": "NEXT_BAR_OPEN", "currency": "USD"
        }
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：COMEX 黄金回测环境模板（全天交易时段）', 'system')
ON CONFLICT (template_id) DO NOTHING;

-- 策略模板：GCMain 双均线交叉（分批建仓 + 分批减仓示例）
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000002', 'TPL-STRAT-DUALMA', '双均线交叉策略模板', 'STRATEGY',
     '{
        "version": "1", "strategy_type": "RULE_BASED",
        "description": "双均线交叉策略：MA5 上穿 MA20 买入，下穿卖出，3% 止损；分批建仓 + 分批减仓",
        "universe": {"securities": ["GCMain"]},
        "data": {"period": "Min", "price_field": "close", "warmup_bars": 30, "fill_mode": "NEXT_BAR_OPEN"},
        "indicators": [
            {"id": "ma_fast", "type": "MA", "params": {"window": 5, "field": "close"}},
            {"id": "ma_slow", "type": "MA", "params": {"window": 20, "field": "close"}}
        ],
        "signals": {"buy": "cross_up(ma_fast, ma_slow)", "sell": "cross_down(ma_fast, ma_slow)"},
        "rules": {
            "buy":  {"action": "BUY",  "quantity_type": "ALL_IN", "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true},
            "sell": {"action": "SELL", "quantity_type": "ALL",    "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true}
        },
        "risk": {
            "stop_loss_pct": 3, "take_profit_pct": 0, "max_position_pct": 100, "max_positions": 1,
            "max_trades_per_day": 0, "min_interval_bars": 0,
            "builder": {"enabled": true, "target_position_pct": 80, "tranches": 4, "tranche_interval_bars": 240},
            "reduce_tranches": 3,
            "max_trades_per_week": 0, "max_trades_per_month": 0, "max_fee_per_window": 0, "fee_window_days": 0
        },
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001}
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：双均线交叉策略模板（GCMain 示例，分批建仓/减仓）', 'system')
ON CONFLICT (template_id) DO NOTHING;

-- 策略模板：RSI 超买超卖
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000003', 'TPL-STRAT-RSI', 'RSI 超买超卖策略模板', 'STRATEGY',
     '{
        "version": "1", "strategy_type": "RULE_BASED",
        "description": "RSI(14) 低于 30 买入，高于 70 卖出",
        "universe": {"securities": ["GCMain"]},
        "data": {"period": "Min", "price_field": "close", "warmup_bars": 30, "fill_mode": "NEXT_BAR_OPEN"},
        "indicators": [{"id": "rsi14", "type": "RSI", "params": {"window": 14, "field": "close"}}],
        "signals": {"buy": "rsi14 < 30", "sell": "rsi14 > 70"},
        "rules": {
            "buy":  {"action": "BUY",  "quantity_type": "ALL_IN", "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true},
            "sell": {"action": "SELL", "quantity_type": "ALL",    "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true}
        },
        "risk": {"stop_loss_pct": 0, "take_profit_pct": 0, "max_position_pct": 100, "max_positions": 1, "max_trades_per_day": 0, "min_interval_bars": 0},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001}
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：RSI 超买超卖策略模板', 'system')
ON CONFLICT (template_id) DO NOTHING;

-- 策略模板：布林带突破
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000004', 'TPL-STRAT-BOLL', '布林带突破策略模板', 'STRATEGY',
     '{
        "version": "1", "strategy_type": "RULE_BASED",
        "description": "收盘价上穿布林上轨买入，下穿中轨卖出",
        "universe": {"securities": ["GCMain"]},
        "data": {"period": "Min", "price_field": "close", "warmup_bars": 30, "fill_mode": "NEXT_BAR_OPEN"},
        "indicators": [
            {"id": "boll_up", "type": "BOLL", "params": {"window": 20, "k": 2, "field": "close", "output": "upper"}},
            {"id": "boll_mid", "type": "BOLL", "params": {"window": 20, "k": 2, "field": "close", "output": "mid"}}
        ],
        "signals": {"buy": "cross_up(close, boll_up)", "sell": "cross_down(close, boll_mid)"},
        "rules": {
            "buy":  {"action": "BUY",  "quantity_type": "ALL_IN", "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true},
            "sell": {"action": "SELL", "quantity_type": "ALL",    "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true}
        },
        "risk": {"stop_loss_pct": 0, "take_profit_pct": 0, "max_position_pct": 100, "max_positions": 1, "max_trades_per_day": 0, "min_interval_bars": 0},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001}
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：布林带突破策略模板', 'system')
ON CONFLICT (template_id) DO NOTHING;

-- 策略模板：MACD 金叉死叉
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000005', 'TPL-STRAT-MACD', 'MACD 金叉死叉策略模板', 'STRATEGY',
     '{
        "version": "1", "strategy_type": "RULE_BASED",
        "description": "MACD DIF 上穿 DEA（金叉）买入，下穿（死叉）卖出",
        "universe": {"securities": ["GCMain"]},
        "data": {"period": "Min", "price_field": "close", "warmup_bars": 60, "fill_mode": "NEXT_BAR_OPEN"},
        "indicators": [
            {"id": "dif", "type": "MACD", "params": {"fast": 12, "slow": 26, "signal": 9, "field": "close", "output": "dif"}},
            {"id": "dea", "type": "MACD", "params": {"fast": 12, "slow": 26, "signal": 9, "field": "close", "output": "dea"}}
        ],
        "signals": {"buy": "cross_up(dif, dea)", "sell": "cross_down(dif, dea)"},
        "rules": {
            "buy":  {"action": "BUY",  "quantity_type": "ALL_IN", "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true},
            "sell": {"action": "SELL", "quantity_type": "ALL",    "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true}
        },
        "risk": {"stop_loss_pct": 0, "take_profit_pct": 0, "max_position_pct": 100, "max_positions": 1, "max_trades_per_day": 0, "min_interval_bars": 0},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001}
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：MACD 金叉死叉策略模板', 'system')
ON CONFLICT (template_id) DO NOTHING;

-- 环境模板：沪深 ETF 回测
INSERT INTO finv_quant_template
    (template_id, template_code, template_name, template_type, content, user_id, is_builtin, status, description, created_by)
VALUES
    ('t0000000-0000-4000-8000-000000000006', 'TPL-ENV-CN-ETF', '沪深 ETF 回测环境模板', 'ENVIRONMENT',
     '{
        "env_code": "ENV-BT-CN-ETF", "env_name": "沪深 ETF 回测环境", "env_type": "BACKTEST", "region": "CN",
        "config": {
            "trading_sessions": [{"start": "093000", "end": "113000"}, {"start": "130000", "end": "150000"}],
            "trading_rules": {"t_plus": 1, "tick_size": 0.001, "contract_multiplier": 1, "limit_up_pct": 10, "limit_down_pct": 10},
            "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001},
            "fill_mode": "NEXT_BAR_OPEN", "currency": "CNY"
        }
     }'::jsonb,
     'system', '1', 'ENABLED', '内置：沪深 ETF 回测环境模板（T+1，涨跌停 10%）', 'system')
ON CONFLICT (template_id) DO NOTHING;

COMMIT;
