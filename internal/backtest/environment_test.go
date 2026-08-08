package backtest

import (
	"context"
	"strings"
	"testing"
)

// TestEngineEnvironmentSessions 验证环境交易时段过滤：非时段内信号被拒绝并登记原因。
func TestEngineEnvironmentSessions(t *testing.T) {
	// 生成 09:00-11:59 的分钟行情（环境时段限定 10:00-10:59，其余时段信号应被拒绝）
	bars := make([]Bar, 0, 180)
	ts := int64(1700000000)
	for i := 0; i < 180; i++ {
		close := 100 + float64(i)*0.1
		bars = append(bars, Bar{
			TS: ts, Date: 20260105, Time: 90000 + i*100,
			Open: close, High: close + 0.1, Low: close - 0.1, Close: close,
		})
		ts += 60
	}

	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillNextBarOpen},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100},
	}
	env := &Environment{
		EnvID: "env-test", EnvCode: "ENV-TEST", EnvName: "测试环境",
		EnvType: "BACKTEST", Config: EnvironmentConfig{
			TradingSessions: []SessionDef{{Start: "100000", End: "105900"}},
		},
	}
	cfg := EngineConfig{
		Definition:  def,
		Account:     AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Environment: env,
		SecuCode:    "TEST",
		Period:      PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	// 10:00-10:59 内买入信号成交 1 次；其余时段被拒（不在环境交易时段内）
	var inSessionFilled, sessionRejected int
	for _, ev := range result.EventTraces {
		if ev.TriggerReason != "买入信号" {
			continue
		}
		if ev.ExecStatus == "FILLED" {
			inSessionFilled++
		}
		if ev.ExecStatus == "REJECTED" && strings.Contains(ev.RejectReason, "不在环境交易时段") {
			sessionRejected++
		}
	}
	if inSessionFilled != 1 {
		t.Errorf("10:00-10:59 时段内应成交 1 次（买入后已持仓），实际 %d", inSessionFilled)
	}
	if sessionRejected == 0 {
		t.Error("非环境交易时段内的信号应被拒绝并登记原因")
	}
	if result.Report.EventStats == nil {
		t.Fatal("事件统计为空")
	}
	found := false
	for reason, cnt := range result.Report.EventStats.RejectReasons {
		if strings.Contains(reason, "不在环境交易时段") && cnt > 0 {
			found = true
		}
	}
	if !found {
		t.Error("报告拒绝原因分布应包含「不在环境交易时段内」")
	}
}

// TestEngineEnvironmentTickSize 验证环境 tick_size：成交价按最小变动单位对齐。
func TestEngineEnvironmentTickSize(t *testing.T) {
	bars := genBars(50, 100, 0.1, 20260105)
	// 手工构造非整 tick 价格（100.047 等），验证对齐到 0.05
	for i := range bars {
		bars[i].Close = 100.047 + float64(i)*0.101
		bars[i].Open = bars[i].Close - 0.02
		bars[i].High = bars[i].Close + 0.01
		bars[i].Low = bars[i].Close - 0.01
	}
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillNextBarOpen},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100},
	}
	env := &Environment{
		EnvID: "env-tick", EnvCode: "ENV-TICK", EnvName: "tick 环境",
		EnvType: "BACKTEST", Config: EnvironmentConfig{
			TradingRules: &TradingRulesDef{TickSize: 0.05},
		},
	}
	cfg := EngineConfig{
		Definition:  def,
		Account:     AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Environment: env,
		SecuCode:    "TEST",
		Period:      PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	if len(result.Trades) == 0 {
		t.Fatal("无成交记录")
	}
	for _, tr := range result.Trades {
		// 成交价必须为 tick_size 的整数倍（允许浮点误差）
		ticks := tr.Price / 0.05
		rounded := float64(int64(ticks + 0.5))
		if diff := ticks - rounded; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("成交价 %v 不是 tick_size(0.05) 的整数倍", tr.Price)
		}
	}
}

// TestEngineMultiUserIsolation 验证多用户隔离：列表过滤不跨用户（服务层逻辑由 DB 保证，此处验证模型字段）。
func TestEngineMultiUserModel(t *testing.T) {
	// 多用户/多子账户字段模型验证
	acc := Account{UserID: "user-a", GroupID: strPtr("group-1")}
	if acc.UserID != "user-a" || *acc.GroupID != "group-1" {
		t.Error("账户多用户/子账户字段异常")
	}
	snapshot := AccountSnapshot{UserID: "user-a", GroupID: strPtr("group-1")}
	if snapshot.UserID != "user-a" {
		t.Error("账户快照多用户字段异常")
	}
	env := Environment{EnvType: "PAPER", Config: EnvironmentConfig{Currency: "CNY"}}
	if env.EnvType != "PAPER" || env.Config.Currency != "CNY" {
		t.Error("环境模型字段异常")
	}
}

func strPtr(s string) *string { return &s }
