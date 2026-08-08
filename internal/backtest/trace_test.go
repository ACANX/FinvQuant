package backtest

import (
	"context"
	"testing"
)

// TestEngineTraceChain 验证需求⑨链路追踪：资金流水/持仓变化明细/事件追踪。
func TestEngineTraceChain(t *testing.T) {
	// W 形走势：下跌 → 上涨（买入）→ 下跌（卖出）
	bars := make([]Bar, 0, 450)
	bars = append(bars, genBars(150, 100, -0.05, 20260105)...)
	bars = append(bars, genBars(150, 92.5, 0.1, 20260106)...)
	bars = append(bars, genBars(150, 107.5, -0.05, 20260107)...)

	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 20, FillMode: FillNextBarOpen},
		Indicators: []IndicatorDef{
			{ID: "ma_fast", Type: "MA", Params: map[string]any{"window": 5, "field": "close"}},
			{ID: "ma_slow", Type: "MA", Params: map[string]any{"window": 20, "field": "close"}},
		},
		Signals: SignalsDef{Buy: "cross_up(ma_fast, ma_slow)", Sell: "cross_down(ma_fast, ma_slow)"},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, CommissionRate: 0.0003, MarginRate: 1, MarginMode: "FULL"},
		SecuCode:   "TEST",
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}

	// ① 资金流水：初始注入 + 每笔成交至少 2 条（付款/收款 + 手续费）
	if len(result.Cashflows) == 0 {
		t.Fatal("资金流水为空")
	}
	if result.Cashflows[0].FlowType != "INITIAL_DEPOSIT" || result.Cashflows[0].CashAfter != 100000 {
		t.Errorf("首条流水应为初始资金注入，实际 %+v", result.Cashflows[0])
	}
	hasBuyPay, hasSellRecv, hasFee := false, false, false
	for _, cf := range result.Cashflows {
		switch cf.FlowType {
		case "BUY_PAY":
			hasBuyPay = true
			if cf.Amount >= 0 {
				t.Error("BUY_PAY 金额应为负（支出）")
			}
		case "SELL_RECEIVE":
			hasSellRecv = true
		case "FEE":
			hasFee = true
		}
	}
	if !hasBuyPay || !hasSellRecv || !hasFee {
		t.Errorf("资金流水应包含买入付款/卖出收款/手续费，实际 buy_pay=%v sell_receive=%v fee=%v", hasBuyPay, hasSellRecv, hasFee)
	}
	// 流水连续性：每条流水 cash_after == 下一条 cash_before
	for i := 1; i < len(result.Cashflows); i++ {
		if result.Cashflows[i].CashBefore != result.Cashflows[i-1].CashAfter {
			t.Errorf("流水不连续: [%d].cash_after=%v != [%d].cash_before=%v",
				i-1, result.Cashflows[i-1].CashAfter, i, result.Cashflows[i].CashBefore)
		}
	}

	// ② 持仓变化明细：OPEN → CLOSE（单标的单持仓）
	if len(result.PositionLogs) == 0 {
		t.Fatal("持仓变化明细为空")
	}
	if result.PositionLogs[0].Action != "OPEN" {
		t.Errorf("首次持仓变化应为 OPEN，实际 %s", result.PositionLogs[0].Action)
	}
	last := result.PositionLogs[len(result.PositionLogs)-1]
	if last.Action != "CLOSE" {
		t.Errorf("末次持仓变化应为 CLOSE（平仓），实际 %s", last.Action)
	}

	// ③ 事件追踪：买入信号 PENDING→FILLED；卖出信号 FILLED；委托耗时 >= 0
	if len(result.EventTraces) == 0 {
		t.Fatal("事件追踪为空")
	}
	var buyFilled, sellFilled int
	for _, ev := range result.EventTraces {
		if ev.TriggerReason != "买入信号" && ev.TriggerReason != "卖出信号" {
			t.Errorf("触发原因异常: %s", ev.TriggerReason)
		}
		if ev.ExecStatus == "FILLED" {
			if ev.LatencyBars < 0 || ev.LatencySec < 0 {
				t.Errorf("委托耗时不能为负: bars=%d sec=%d", ev.LatencyBars, ev.LatencySec)
			}
			if ev.TriggerReason == "买入信号" {
				buyFilled++
			} else {
				sellFilled++
			}
			// NEXT_BAR_OPEN：成交时间应晚于触发时间
			if ev.ExecTS <= ev.TriggerTS {
				t.Errorf("成交时间应晚于触发时间: trigger=%d exec=%d", ev.TriggerTS, ev.ExecTS)
			}
		}
	}
	if buyFilled < 1 || sellFilled < 1 {
		t.Errorf("买卖信号应各有成交事件，买=%d 卖=%d", buyFilled, sellFilled)
	}

	// 报告事件统计
	if result.Report.EventStats == nil {
		t.Fatal("报告事件统计为空")
	}
	if result.Report.EventStats.FilledCount != buyFilled+sellFilled {
		t.Errorf("报告成交事件数 %d 与实际 %d 不符", result.Report.EventStats.FilledCount, buyFilled+sellFilled)
	}
	if result.Report.EventStats.AvgLatencyBars <= 0 {
		t.Errorf("平均委托耗时应大于 0，实际 %v", result.Report.EventStats.AvgLatencyBars)
	}
}

// TestEngineEventReject 验证拒绝事件登记：时间点限制/已达最大持仓/无持仓可卖。
func TestEngineEventReject(t *testing.T) {
	bars := genBars(300, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillNextBarOpen},
		Signals:  SignalsDef{Buy: "close > 0", Sell: "close > 0"},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true, MaxPerDay: 1},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		SecuCode:   "TEST",
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	// 信号每根都触发：首根买入成交（MaxPerDay=1），随后买入被拒（超过每日次数）
	// 同时持仓期间买入信号被拒（已达最大持仓）；卖出信号触发时未成交过卖出（一直持仓）→ 挂单成交
	rejected := map[string]int{}
	for _, ev := range result.EventTraces {
		if ev.ExecStatus == "REJECTED" {
			rejected[ev.RejectReason]++
		}
	}
	if len(rejected) == 0 {
		t.Error("应存在拒绝事件")
	}
	// 事件统计中的拒绝原因分布应一致
	if result.Report.EventStats == nil || len(result.Report.EventStats.RejectReasons) == 0 {
		t.Error("报告拒绝原因分布为空")
	}
}
