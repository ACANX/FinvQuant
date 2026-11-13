package backtest

import (
	"fmt"
	"strings"
)

// validateStrategy 校验策略必填项与定义结构。
func validateStrategy(st *Strategy) error {
	if strings.TrimSpace(st.StrategyCode) == "" {
		return fmt.Errorf("strategy_code（策略编码）必填")
	}
	if strings.TrimSpace(st.StrategyName) == "" {
		return fmt.Errorf("strategy_name（策略名称）必填")
	}
	if st.StrategyType == "" {
		st.StrategyType = StrategyTypeRuleBased
	}
	if st.StrategyType != StrategyTypeRuleBased {
		return fmt.Errorf("当前版本仅支持 RULE_BASED 策略类型（%s 暂未实现）", st.StrategyType)
	}
	def := st.Definition
	if len(def.Universe.Securities) == 0 {
		return fmt.Errorf("definition.universe.securities（标的池）至少需要一个证券代码")
	}
	for _, code := range def.Universe.Securities {
		if strings.TrimSpace(code) == "" {
			return fmt.Errorf("definition.universe.securities 中存在空证券代码")
		}
	}
	if def.Data.Period != "" && def.Data.Period != PeriodMin && def.Data.Period != PeriodHour && def.Data.Period != PeriodDay {
		return fmt.Errorf("definition.data.period 仅支持 Min/Hour/Day")
	}

	// 指标 ID 唯一性 + 指标定义合法性（不加载数据，仅类型校验）
	seen := map[string]bool{}
	for _, ind := range def.Indicators {
		if strings.TrimSpace(ind.ID) == "" {
			return fmt.Errorf("指标定义缺少 id")
		}
		if seen[ind.ID] {
			return fmt.Errorf("指标 id 重复: %s", ind.ID)
		}
		seen[ind.ID] = true
		switch ind.Type {
		case "MA", "EMA", "RSI", "MACD", "BOLL", "ATR", "STDDEV", "HHV", "LLV":
		default:
			return fmt.Errorf("不支持的指标类型 %q（支持: MA/EMA/RSI/MACD/BOLL/ATR/STDDEV/HHV/LLV）", ind.Type)
		}
	}

	// 信号：至少一个方向非空（buy/sell 全空 = 无任何交易逻辑，拒绝保存）
	if strings.TrimSpace(def.Signals.Buy) == "" && strings.TrimSpace(def.Signals.Sell) == "" {
		return fmt.Errorf("signals.buy / signals.sell 至少需要一个非空信号表达式（否则策略无交易逻辑）")
	}

	// 信号表达式：语法 + 函数/参数/偏移常量/深度校验 + 标识符交叉校验（拦“指标名写错”）
	for name, expr := range map[string]string{"买入": def.Signals.Buy, "卖出": def.Signals.Sell} {
		if strings.TrimSpace(expr) == "" {
			continue
		}
		ids, err := CompileExprIdentifiers(expr)
		if err != nil {
			return fmt.Errorf("%s信号表达式错误: %w", name, err)
		}
		if err := validateSignalIdentifiers(name, ids, seen); err != nil {
			return err
		}
	}

	// 规则校验
	for name, rule := range map[string]RuleDef{"buy": def.Rules.Buy, "sell": def.Rules.Sell} {
		if rule.QuantityType != "" {
			switch rule.QuantityType {
			case "ALL_IN", "ALL", "FIXED", "PERCENT", "AMOUNT":
			default:
				return fmt.Errorf("rules.%s.quantity_type 不支持 %q", name, rule.QuantityType)
			}
		}
		// 数量模式方向语义：买入不能用 ALL（清仓），卖出不能用 ALL_IN（全额买入语义）
		if name == "buy" && rule.QuantityType == "ALL" {
			return fmt.Errorf("rules.buy.quantity_type 不能为 ALL（ALL 为清仓语义，仅卖出方向可用）；全额买入请用 ALL_IN")
		}
		if name == "sell" && rule.QuantityType == "ALL_IN" {
			return fmt.Errorf("rules.sell.quantity_type 不能为 ALL_IN（全额买入语义，仅买入方向可用）；清仓请用 ALL")
		}
		if (rule.QuantityType == "FIXED" || rule.QuantityType == "AMOUNT" || rule.QuantityType == "PERCENT") && rule.Quantity <= 0 {
			return fmt.Errorf("rules.%s.quantity_type=%s 时 quantity 必须大于 0", name, rule.QuantityType)
		}
	}
	if def.Risk.StopLossPct < 0 || def.Risk.TakeProfitPct < 0 {
		return fmt.Errorf("risk.stop_loss_pct / take_profit_pct 不能为负数")
	}
	if def.Risk.MaxPositionPct < 0 || def.Risk.MaxPositionPct > 100 {
		return fmt.Errorf("risk.max_position_pct 必须在 0~100 之间")
	}
	// 分批建仓校验
	if b := def.Risk.Builder; b != nil {
		if b.Tranches < 1 {
			return fmt.Errorf("risk.builder.tranches（分批建仓份数）必须 >= 1")
		}
		if b.TargetPositionPct < 0 || b.TargetPositionPct > 100 {
			return fmt.Errorf("risk.builder.target_position_pct 必须在 0~100 之间")
		}
		if b.TrancheIntervalBars < 0 {
			return fmt.Errorf("risk.builder.tranche_interval_bars 不能为负数")
		}
	}
	if def.Risk.ReduceTranches < 0 {
		return fmt.Errorf("risk.reduce_tranches（分批减仓份数）不能为负数")
	}
	// 滑动窗口限制校验
	if def.Risk.MaxTradesPerWeek < 0 || def.Risk.MaxTradesPerMonth < 0 || def.Risk.MaxFeePerWindow < 0 || def.Risk.FeeWindowDays < 0 {
		return fmt.Errorf("risk.max_trades_per_week / max_trades_per_month / max_fee_per_window / fee_window_days 不能为负数")
	}
	for name, rule := range map[string]RuleDef{"buy": def.Rules.Buy, "sell": def.Rules.Sell} {
		if rule.MaxPerWeek < 0 || rule.MaxPerMonth < 0 || rule.MaxFeePerWindow < 0 || rule.FeeWindowDays < 0 {
			return fmt.Errorf("rules.%s 的 max_per_week / max_per_month / max_fee_per_window / fee_window_days 不能为负数", name)
		}
	}
	return nil
}

// validateSignalIdentifiers 校验信号表达式引用的标识符均已在 indicators 声明或属于字段白名单。
func validateSignalIdentifiers(signalName string, ids map[string]bool, indicatorIDs map[string]bool) error {
	for id := range ids {
		if indicatorIDs[id] {
			continue
		}
		switch id {
		case "open", "high", "low", "close", "volume", "turnover":
			continue
		}
		return fmt.Errorf("%s信号表达式引用了未声明的标识符 %q（须为 indicators 中声明的指标 id，或字段 open/high/low/close/volume/turnover）", signalName, id)
	}
	return nil
}

// validateAccount 校验回测账户必填项。
func validateAccount(acc *Account) error {
	if strings.TrimSpace(acc.AccountCode) == "" {
		return fmt.Errorf("account_code（账户编码）必填")
	}
	if strings.TrimSpace(acc.AccountName) == "" {
		return fmt.Errorf("account_name（账户名称）必填")
	}
	if acc.InitialCapital <= 0 {
		return fmt.Errorf("initial_capital（初始启动资金）必须大于 0")
	}
	if acc.CommissionRate < 0 || acc.SlippagePct < 0 {
		return fmt.Errorf("commission_rate / slippage_pct 不能为负数")
	}
	if acc.MarginMode != "" && acc.MarginMode != "FULL" && acc.MarginMode != "FUTURES" {
		return fmt.Errorf("margin_mode 仅支持 FULL/FUTURES")
	}
	return nil
}

// validateEnvironment 校验环境必填项与配置。
func validateEnvironment(env *Environment) error {
	if strings.TrimSpace(env.EnvCode) == "" {
		return fmt.Errorf("env_code（环境编码）必填")
	}
	if strings.TrimSpace(env.EnvName) == "" {
		return fmt.Errorf("env_name（环境名称）必填")
	}
	switch env.EnvType {
	case "", "BACKTEST", "PAPER", "SIMULATION", "LIVE":
	default:
		return fmt.Errorf("env_type 仅支持 BACKTEST/PAPER/SIMULATION/LIVE")
	}
	for _, s := range env.Config.TradingSessions {
		if len(s.Start) != 6 || len(s.End) != 6 {
			return fmt.Errorf("trading_sessions 起止时间需为 hhmmss（如 093000）")
		}
	}
	if env.Config.TradingRules != nil && env.Config.TradingRules.TickSize < 0 {
		return fmt.Errorf("trading_rules.tick_size 不能为负数")
	}
	return nil
}

// validateTemplate 校验模板必填项。
func validateTemplate(tmpl *Template) error {
	if strings.TrimSpace(tmpl.TemplateCode) == "" {
		return fmt.Errorf("template_code（模板编码）必填")
	}
	if strings.TrimSpace(tmpl.TemplateName) == "" {
		return fmt.Errorf("template_name（模板名称）必填")
	}
	switch tmpl.TemplateType {
	case "STRATEGY", "ACCOUNT", "ENVIRONMENT":
	default:
		return fmt.Errorf("template_type 仅支持 STRATEGY/ACCOUNT/ENVIRONMENT")
	}
	return nil
}
