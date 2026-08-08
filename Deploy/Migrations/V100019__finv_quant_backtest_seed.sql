-- =====================================================================
-- FinvQuant PostgreSQL V100019：回测种子数据（默认账户 + 示例策略）
--
-- 决策（ACANX 2026-08-06）：
--   - 数据种子段（V100000+）：为回测模块提供开箱即用的默认配置；
--   - 默认账户：黄金期货回测账户（初始资金 100000 USD，手续费率 0.03%，
--     滑点 0.01%，全额保证金模式）；
--   - 示例策略：GCMain 双均线交叉策略（5/20 分钟线，快线上穿买入、
--     下穿卖出，3% 止损），演示通用结构化策略定义模型；
--   - 幂等：ON CONFLICT DO NOTHING，重复执行不产生重复数据。
-- =====================================================================

BEGIN;

-- 默认回测账户
INSERT INTO finv_quant_backtest_account
    (account_id, account_code, account_name, initial_capital, currency_type,
     commission_rate, slippage_pct, margin_mode, margin_rate, allow_backtest, status, remark, created_by)
VALUES
    ('A0000000-0000-4000-8000-000000000001', 'ACCT-GOLD-001', '黄金期货回测账户', 100000, 'USD',
     0.0003, 0.0001, 'FULL', 1, '1', 'ENABLED', '默认回测账户（GCMain 黄金期货主连）', 'system')
ON CONFLICT (account_id) DO NOTHING;

-- 示例策略：GCMain 双均线交叉（黄金期货合约回测验证首版演示）
INSERT INTO finv_quant_backtest_strategy
    (strategy_id, strategy_code, strategy_name, strategy_type, description,
     definition, definition_version, data_period, secu_code, allow_backtest, status, created_by)
VALUES
    ('B0000000-0000-4000-8000-000000000001', 'STRAT-DUALMA-GC', 'GCMain 双均线交叉策略', 'RULE_BASED',
     '双均线交叉策略（示例）：快线 MA5 上穿慢线 MA20 时全仓买入，下穿时清仓卖出；3% 止损。演示通用结构化策略定义模型（指标/信号/规则/风控/成本）。',
     '{
        "version": "1",
        "strategy_type": "RULE_BASED",
        "description": "双均线交叉策略：MA5 上穿 MA20 买入，下穿卖出，3% 止损",
        "universe": {"securities": ["GCMain"]},
        "data": {"period": "Min", "price_field": "close", "warmup_bars": 30, "fill_mode": "NEXT_BAR_OPEN"},
        "indicators": [
            {"id": "ma_fast", "type": "MA", "params": {"window": 5, "field": "close"}},
            {"id": "ma_slow", "type": "MA", "params": {"window": 20, "field": "close"}}
        ],
        "signals": {
            "buy": "cross_up(ma_fast, ma_slow)",
            "sell": "cross_down(ma_fast, ma_slow)"
        },
        "rules": {
            "buy":  {"action": "BUY",  "quantity_type": "ALL_IN", "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true},
            "sell": {"action": "SELL", "quantity_type": "ALL",    "quantity": 0, "max_per_day": 0, "max_per_run": 0, "allowed_times": [], "allow": true}
        },
        "risk": {"stop_loss_pct": 3, "take_profit_pct": 0, "max_position_pct": 100, "max_positions": 1, "max_trades_per_day": 0, "min_interval_bars": 0},
        "cost": {"commission_rate": 0.0003, "slippage_pct": 0.0001}
     }'::jsonb,
     1, 'Min', 'GCMain', '1', 'ENABLED', 'system')
ON CONFLICT (strategy_id) DO NOTHING;

COMMIT;
