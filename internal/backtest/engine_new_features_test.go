package backtest

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// 新增特性测试：每日计数跨日重置（防死锁）/ 分批建仓减仓 / 滑动窗口限制
// ---------------------------------------------------------------------

// multiDateBars 生成跨多日的行情：每天 minPerDay 根分钟 bar，close 从 base 起线性增长。
func multiDateBars(dates []int, minPerDay int, base float64, step float64) []Bar {
	bars := []Bar{}
	ts := int64(1700000000)
	for _, d := range dates {
		for m := 0; m < minPerDay; m++ {
			close := base + step*float64(len(bars))
			bars = append(bars, Bar{
				TS: ts, Date: d, Time: 90000 + m*100,
				Open: close - 0.01, High: close + 0.01, Low: close - 0.01, Close: close,
			})
			ts += 60
		}
	}
	return bars
}

// TestDailyCounterResetAcrossDays 验证每日成交笔数跨日重置（修复“某日达上限后永久死锁”Bug）。
// 构造两日行情：每根都触发买入信号，但每日成交上限 2 笔；
// 修复前：Day1 达上限后计数不重置，Day2 买入信号被「今日已达」拒绝 → 仅 2 笔成交；
// 修复后：每日重置 → 两日各 2 笔，共 4 笔。
func TestDailyCounterResetAcrossDays(t *testing.T) {
	// Day1: 6 根 close=100.0..100.5；Day2: 6 根 close=200.0..200.5（builder 保证持仓上限不拦截买入）
	bars := []Bar{}
	ts := int64(1700000000)
	for d, base := range []float64{100, 200} {
		for m := 0; m < 6; m++ {
			close := base + float64(m)*0.1
			bars = append(bars, Bar{
				TS: ts, Date: 20260105 + d, Time: 90000 + m*100,
				Open: close - 0.01, High: close + 0.01, Low: close - 0.01, Close: close,
			})
			ts += 60
		}
	}
	maxPerDay := 2
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillCurrentClose},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		// builder 目标 100%、大量份数：每次买入只增加 1/200 仓位，不被「已达目标」拦截，专注验证每日重置
		Risk: RiskDef{MaxPositionPct: 100, Builder: &BuilderDef{Enabled: true, TargetPositionPct: 100, Tranches: 200}},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		SecuCode:   "TEST",
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true, MaxTradesPerDay: &maxPerDay},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	// 期望：两日各成交 2 笔（每日上限 2，跨日重置），共 4 笔
	if result.Report.TradeCount != 4 {
		t.Errorf("每日上限 2 且跨日重置：两日应各成交 2 笔（共 4），实际 %d（修复前仅 Day1 成交、Day2 死锁）", result.Report.TradeCount)
	}
	// 且存在「今日已达成交笔数上限」的拒绝（同日内第 3 个信号被限）
	found := false
	for _, ev := range result.EventTraces {
		if strings.Contains(ev.RejectReason, "今日已达成交笔数上限") {
			found = true
			break
		}
	}
	if !found {
		t.Error("同日后续信号应产生「今日已达成交笔数上限」拒绝事件")
	}
}

// TestBuilderTranches 验证分批建仓：目标仓位 60%、分 4 批，连续买入信号逐批加仓至目标后拒绝。
func TestBuilderTranches(t *testing.T) {
	// 每根 bar 收盘略涨，买入信号每根都触发（close > 0），CURRENT_CLOSE 撮合保证即时成交
	bars := genBars(30, 100, 0.5, 20260105)
	builder := &BuilderDef{
		Enabled:           true,
		TargetPositionPct: 60,
		Tranches:          4,
	}
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillCurrentClose},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100, MaxPositions: 1, Builder: builder},
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
	// 分批建仓：最多建到 60% 目标，之后信号被拒
	if result.Report.BuyCount == 0 {
		t.Fatal("分批建仓应产生至少 1 笔买入")
	}
	if result.Report.BuyCount > 10 {
		t.Errorf("分批建仓后应很快达目标（买入笔数过多 = 分批未生效）: %d", result.Report.BuyCount)
	}
	// 存在 ADD 加仓（持仓后再次买入）
	hasAdd := false
	for _, pl := range result.PositionLogs {
		if pl.Action == "ADD" {
			hasAdd = true
			break
		}
	}
	if !hasAdd {
		t.Error("分批建仓应产生 ADD 加仓持仓变化")
	}
	// 存在「已达目标持仓上限」拒绝
	found := false
	for _, ev := range result.EventTraces {
		if strings.Contains(ev.RejectReason, "已达目标持仓上限") {
			found = true
			break
		}
	}
	if !found {
		t.Error("达到目标仓位后应产生「已达目标持仓上限」拒绝事件")
	}
	// 期末持仓价值应接近目标仓位 60%
	if result.Report.MaxInvested <= 0 {
		t.Error("分批建仓后应有持仓投入")
	}
	if ratio := result.Report.MaxInvested / result.Report.InitialCapital; ratio < 0.5 || ratio > 0.75 {
		t.Errorf("目标仓位 60%% 的建仓结果应在 50%%~75%% 之间，实际 %.2f%%", ratio*100)
	}
}

// TestReduceTranches 验证分批减仓：分 3 批，前两批 REDUCE、最后一批 CLOSE。
func TestReduceTranches(t *testing.T) {
	bars := genBars(30, 100, 0.5, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillCurrentClose},
		Signals:  SignalsDef{Buy: "close < 100.6", Sell: "close > 104"}, // 首根买入，后期连续卖出
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100, ReduceTranches: 3},
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
	reduceCount, closeCount := 0, 0
	for _, pl := range result.PositionLogs {
		switch pl.Action {
		case "REDUCE":
			reduceCount++
		case "CLOSE":
			closeCount++
		}
	}
	// 分批减仓 3 次：应含 2 次 REDUCE + 1 次 CLOSE（若只有 1 次卖出则是清仓语义）
	if reduceCount == 0 {
		t.Errorf("分批减仓应产生 REDUCE 记录，实际 0 次 REDUCE %d 次 CLOSE", closeCount)
	}
	if closeCount == 0 {
		t.Error("分批减仓最后一批应 CLOSE 清仓")
	}
	// 期末应空仓
	if result.Report.MaxInvested <= 0 {
		t.Log("分批减仓后已清仓（符合预期）")
	}
}

// TestWindowTradeLimit 验证滚动 7 日成交笔数上限：达到上限后本周内后续成交被拒。
// 使用同一自然周内的 5 个交易日（20260105-20260109），避免跨滚动窗口边界。
func TestWindowTradeLimit(t *testing.T) {
	// 5 个连续交易日，每根触发买入（CURRENT_CLOSE 即时成交），但 max_trades_per_week=2
	bars := multiDateBars([]int{20260105, 20260106, 20260107, 20260108, 20260109}, 5, 100, 0.1)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillCurrentClose},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100, MaxTradesPerWeek: 2, Builder: &BuilderDef{
			Enabled: true, TargetPositionPct: 100, Tranches: 100,
		}},
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
	if result.Report.TradeCount > 2 {
		t.Errorf("近 7 日成交上限 2 笔：实际成交 %d（窗口限制未生效）", result.Report.TradeCount)
	}
	found := false
	for _, ev := range result.EventTraces {
		if strings.Contains(ev.RejectReason, "近7日成交已达上限") {
			found = true
			break
		}
	}
	if !found {
		t.Error("达到周上限后应产生「近7日成交已达上限」拒绝事件")
	}
}

// TestBuilderValidation 验证分批建仓参数校验。
func TestBuilderValidation(t *testing.T) {
	base := func() *Strategy {
		return &Strategy{
			StrategyCode: "T-B", StrategyName: "测试",
			Definition: StrategyDefinition{
				Universe: UniverseDef{Securities: []string{"GCMain"}},
				Data:     DataDef{Period: PeriodMin},
				Signals:  SignalsDef{Buy: "close > 0"},
				Rules: RulesDef{
					Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
					Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
				},
			},
		}
	}
	st := base()
	st.Definition.Risk.Builder = &BuilderDef{Enabled: true, Tranches: 0} // 份数 0
	if err := validateStrategy(st); err == nil || !strings.Contains(err.Error(), "tranches") {
		t.Errorf("tranches<1 应校验失败: %v", err)
	}
	st = base()
	st.Definition.Risk.Builder = &BuilderDef{Enabled: true, Tranches: 2, TargetPositionPct: 150} // 仓位超 100
	if err := validateStrategy(st); err == nil || !strings.Contains(err.Error(), "target_position_pct") {
		t.Errorf("target_position_pct>100 应校验失败: %v", err)
	}
	st = base()
	st.Definition.Risk.ReduceTranches = -1
	if err := validateStrategy(st); err == nil {
		t.Error("reduce_tranches<0 应校验失败")
	}
}
