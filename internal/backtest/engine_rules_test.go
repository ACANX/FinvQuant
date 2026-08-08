package backtest

import (
	"context"
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// 评审修复验证：ref 前视漏洞 / 标识符交叉校验 / 表达式深度限制
// ---------------------------------------------------------------------

// TestExprRefNegativeGuard 验证 ref 负偏移前视漏洞已修复：
// 1) 编译期常量负偏移直接报错；2) 运行期算术构造的负偏移返回 NaN（读不到未来 bar）。
func TestExprRefNegativeGuard(t *testing.T) {
	// 词法不支持 -5 字面量，但算术可构造：ref(close, 0-5)
	if _, err := CompileExpr("ref(close, 0-5) > 0"); err == nil {
		t.Error("编译期应拒绝常量负偏移 ref(close, 0-5)")
	} else if !strings.Contains(err.Error(), "必须 >= 0") {
		t.Errorf("错误信息应提示偏移必须 >= 0，实际: %v", err)
	}
	if _, err := CompileExpr("ref(close, -5) > 0"); err == nil {
		t.Error("编译期应拒绝负数字面量 ref(close, -5)")
	}
	// 非负常量合法
	if _, err := CompileExpr("ref(close, 5) > 0"); err != nil {
		t.Errorf("ref(close, 5) 应合法: %v", err)
	}

	// 运行期防御：动态负偏移（如 rsi14 - 30 < 0 时）返回 NaN，比较恒 false
	closeV := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ctx := &EvalContext{At: 2, Fields: map[string][]float64{"close": closeV}}
	fn, err := CompileExpr("ref(close, 0-5) == 8")
	if err == nil {
		// 若编译期已拦截，此分支不应执行；此处兜底验证运行期行为
		hit, evalErr := fn(ctx)
		if evalErr != nil {
			t.Fatalf("求值错误: %v", evalErr)
		}
		if hit {
			t.Error("运行期 ref 负偏移读到未来 bar（前视漏洞）")
		}
	}
}

// TestExprDepthLimit 验证表达式嵌套深度限制（防递归栈溢出）。
func TestExprDepthLimit(t *testing.T) {
	deep := strings.Repeat("(", maxExprDepth+2) + "close" + strings.Repeat(")", maxExprDepth+2)
	if _, err := CompileExpr(deep + " > 0"); err == nil {
		t.Error("超深嵌套表达式应被拒绝")
	} else if !strings.Contains(err.Error(), "深度") {
		t.Errorf("错误信息应提示深度超限，实际: %v", err)
	}
}

// TestValidateSignalIdentifiers 验证保存期标识符交叉校验（拦截“指标名写错”）。
func TestValidateSignalIdentifiers(t *testing.T) {
	st := &Strategy{
		StrategyCode: "TEST-1", StrategyName: "测试",
		Definition: StrategyDefinition{
			Universe: UniverseDef{Securities: []string{"GCMain"}},
			Data:     DataDef{Period: PeriodMin},
			Indicators: []IndicatorDef{
				{ID: "ma_fast", Type: "MA", Params: map[string]any{"window": 5}},
			},
			Signals: SignalsDef{Buy: "cross_up(ma_fast, ma_slow)"}, // ma_slow 未声明
		},
	}
	err := validateStrategy(st)
	if err == nil {
		t.Fatal("引用未声明指标应校验失败")
	}
	if !strings.Contains(err.Error(), "ma_slow") {
		t.Errorf("错误信息应指明未声明标识符，实际: %v", err)
	}
}

// TestValidateStrategyBothSignalsEmpty 验证买卖信号至少一个非空。
func TestValidateStrategyBothSignalsEmpty(t *testing.T) {
	st := &Strategy{
		StrategyCode: "TEST-2", StrategyName: "测试",
		Definition: StrategyDefinition{
			Universe: UniverseDef{Securities: []string{"GCMain"}},
			Data:     DataDef{Period: PeriodMin},
			Signals:  SignalsDef{}, // 买卖信号全空
		},
	}
	if err := validateStrategy(st); err == nil {
		t.Error("买卖信号全空应校验失败")
	} else if !strings.Contains(err.Error(), "至少需要一个非空信号") {
		t.Errorf("错误信息不符: %v", err)
	}
}

// TestValidateQuantityDirection 验证数量模式方向语义（买入禁 ALL、卖出禁 ALL_IN）。
func TestValidateQuantityDirection(t *testing.T) {
	base := func() *Strategy {
		return &Strategy{
			StrategyCode: "TEST-3", StrategyName: "测试",
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
	st.Definition.Rules.Buy.QuantityType = "ALL" // 买入用清仓语义
	if err := validateStrategy(st); err == nil {
		t.Error("买入 quantity_type=ALL 应校验失败")
	}
	st = base()
	st.Definition.Rules.Sell.QuantityType = "ALL_IN" // 卖出用全额买入语义
	if err := validateStrategy(st); err == nil {
		t.Error("卖出 quantity_type=ALL_IN 应校验失败")
	}
}

// ---------------------------------------------------------------------
// 成本覆盖链 / 环境规则 / 数量模式
// ---------------------------------------------------------------------

// TestResolveCostsChain 验证成本覆盖链：环境 > 任务 > 策略 > 账户。
func TestResolveCostsChain(t *testing.T) {
	acc := AccountSnapshot{CommissionRate: 0.001, SlippagePct: 0.0005}
	env := &Environment{Config: EnvironmentConfig{Cost: &CostDef{CommissionRate: 0.0001, SlippagePct: 0.00002}}}
	stratCost := &CostDef{CommissionRate: 0.0002, SlippagePct: 0.00003}
	taskC := 0.0004
	taskS := 0.00004
	opts := RunOptions{CommissionRate: &taskC, SlippagePct: &taskS}

	// 全链路：环境最高
	cfg := EngineConfig{Account: acc, Environment: env, Definition: StrategyDefinition{Cost: stratCost}, Options: opts}
	comm, slip := resolveCosts(cfg)
	if comm != 0.0001 || slip != 0.00002 {
		t.Errorf("环境应最高优先级，实际 commission=%v slippage=%v", comm, slip)
	}
	// 无环境：任务 > 策略 > 账户
	cfg.Environment = nil
	comm, slip = resolveCosts(cfg)
	if comm != 0.0004 || slip != 0.00004 {
		t.Errorf("无环境时任务应最高，实际 commission=%v slippage=%v", comm, slip)
	}
	// 仅账户
	cfg.Environment, cfg.Definition.Cost, cfg.Options = nil, nil, RunOptions{}
	comm, slip = resolveCosts(cfg)
	if comm != 0.001 || slip != 0.0005 {
		t.Errorf("仅账户时取账户配置，实际 commission=%v slippage=%v", comm, slip)
	}
}

// TestSellQuantityModes 验证卖出数量模式：FIXED / PERCENT / AMOUNT / ALL。
func TestSellQuantityModes(t *testing.T) {
	state := &accountState{position: 1000, avgCost: 100}
	price := 110.0
	engine := &Engine{}

	cases := []struct {
		name     string
		rule     RuleDef
		expected float64
	}{
		{"ALL 清仓", RuleDef{QuantityType: "ALL"}, 1000},
		{"FIXED 300", RuleDef{QuantityType: "FIXED", Quantity: 300}, 300},
		{"FIXED 超持仓", RuleDef{QuantityType: "FIXED", Quantity: 5000}, 1000},
		{"PERCENT 50", RuleDef{QuantityType: "PERCENT", Quantity: 50}, 500},
		{"AMOUNT 55000", RuleDef{QuantityType: "AMOUNT", Quantity: 55000}, 500},
		{"AMOUNT 超持仓", RuleDef{QuantityType: "AMOUNT", Quantity: 99999999}, 1000},
		{"缺省清仓", RuleDef{}, 1000},
		{"ALL_IN 按清仓兜底", RuleDef{QuantityType: "ALL_IN"}, 1000},
	}
	for _, tc := range cases {
		got := engine.sellQuantity(Bar{}, state, price, tc.rule, 1)
		if math.Abs(got-tc.expected) > 1e-6 {
			t.Errorf("%s: 期望 %v 实际 %v", tc.name, tc.expected, got)
		}
	}
}

// TestEngineTPlusRestriction 验证 T+N 交收限制生效（T+1 当日买入不可卖出）。
func TestEngineTPlusRestriction(t *testing.T) {
	bars := genBars(50, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0},
		Signals:  SignalsDef{Buy: "close > 0", Sell: "close > 0"}, // 每根都触发买卖信号
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
	}
	env := &Environment{Config: EnvironmentConfig{TradingRules: &TradingRulesDef{TPlus: 1}}}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Environment: env,
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	// T+1：买入后当日无法卖出 → 卖出信号应被拒绝
	foundReject := false
	for _, ev := range result.EventTraces {
		if ev.TriggerReason == "卖出信号" && ev.ExecStatus == "REJECTED" && strings.Contains(ev.RejectReason, "T+1") {
			foundReject = true
		}
	}
	if !foundReject {
		t.Error("T+1 环境下当日卖出信号应产生 T+1 拒绝事件")
	}
	if result.Report.SellCount > 0 {
		t.Error("T+1 环境下当日不应有任何卖出成交")
	}
}

// TestEngineLimitUpDown 验证涨跌停限制生效（涨停无法买入）。
// 构造两日行情：Day1 收 ~105，Day2 高开跳空至 ~111（相对 Day1 收盘涨幅 > 涨停 0.5%）→ 买入被拒。
func TestEngineLimitUpDown(t *testing.T) {
	bars := make([]Bar, 0, 70)
	bars = append(bars, genBars(50, 100, 0.1, 20260105)...) // Day1: 100.1 → 105.0
	bars = append(bars, genBars(20, 105, 0.5, 20260106)...) // Day2: 105.5 → 115.0
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0},
		Signals:  SignalsDef{Buy: "close > 110"}, // 仅在 Day2 触发
		Rules:    RulesDef{Buy: RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true}},
	}
	// 涨停 0.5%：Day2 相对 Day1 收盘涨幅远超 → 挂单成交价触及涨停价 → 拒绝
	env := &Environment{Config: EnvironmentConfig{TradingRules: &TradingRulesDef{LimitUpPct: 0.5}}}
	cfg := EngineConfig{
		Definition:  def,
		Account:     AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Environment: env,
		Period:      PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	foundLimitReject := false
	for _, ev := range result.EventTraces {
		if strings.Contains(ev.RejectReason, "涨停") {
			foundLimitReject = true
		}
	}
	if !foundLimitReject {
		t.Error("涨停限制下应产生「涨停无法买入」拒绝事件")
	}
	if result.Report.BuyCount != 0 {
		t.Errorf("涨停限制下不应有买入成交，实际 %d", result.Report.BuyCount)
	}
}

// TestEngineCurrentCloseFill 验证 CURRENT_CLOSE 撮合模式（信号 bar 收盘立即成交）。
func TestEngineCurrentCloseFill(t *testing.T) {
	bars := genBars(30, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillCurrentClose},
		Signals:  SignalsDef{Buy: "close > 99"},
		Rules:    RulesDef{Buy: RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true}},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	if result.Report.BuyCount != 1 {
		t.Errorf("CURRENT_CLOSE 模式应成交 1 笔买入，实际 %d", result.Report.BuyCount)
	}
	for _, ev := range result.EventTraces {
		if ev.ExecStatus == "FILLED" && ev.LatencyBars != 0 {
			t.Errorf("CURRENT_CLOSE 成交事件 latency 应为 0，实际 %d", ev.LatencyBars)
		}
	}
}

// TestEventTraceOrderAndAlive 验证事件追踪含委托下单时间与事件存活时间（FR-10 ⑤⑦）。
func TestEventTraceOrderAndAlive(t *testing.T) {
	bars := genBars(30, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0},
		Signals:  SignalsDef{Buy: "close > 99"},
		Rules:    RulesDef{Buy: RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true}},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	found := false
	for _, ev := range result.EventTraces {
		if ev.ExecStatus == "FILLED" {
			found = true
			if ev.OrderTS != ev.TriggerTS {
				t.Errorf("委托下单时间应等于触发时间（NEXT_BAR_OPEN 语义），实际 %d vs %d", ev.OrderTS, ev.TriggerTS)
			}
			if ev.AliveSec != ev.ExecTS-ev.TriggerTS {
				t.Errorf("事件存活时间应=结束-触发，实际 %d vs %d", ev.AliveSec, ev.ExecTS-ev.TriggerTS)
			}
			if ev.LatencyBars <= 0 {
				t.Errorf("NEXT_BAR_OPEN 成交事件委托耗时应 >0，实际 %d", ev.LatencyBars)
			}
		}
	}
	if !found {
		t.Error("未找到 FILLED 事件")
	}
}

// TestAggregateBarsSessionTime 验证聚合 bar 保留周期首根时间（交易时段判定正确）。
func TestAggregateBarsSessionTime(t *testing.T) {
	bars := make([]Bar, 0, 60)
	ts := int64(1700000000)
	for i := 0; i < 60; i++ {
		bars = append(bars, Bar{
			TS: ts, Date: 20260105, Time: 100000 + i*100,
			Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 100, Turnover: 10000,
		})
		ts += 60
	}
	hourly, err := AggregateBars(bars, PeriodHour)
	if err != nil {
		t.Fatal(err)
	}
	if len(hourly) != 1 {
		t.Fatalf("期望 1 根小时线，实际 %d", len(hourly))
	}
	if hourly[0].Time != 100000 {
		t.Errorf("聚合 bar 时间应取周期首根（100000），实际 %d", hourly[0].Time)
	}
}

// TestDownsampleEquity 验证曲线降采样（保留首尾、点数上限）。
func TestDownsampleEquity(t *testing.T) {
	pts := make([]EquityPoint, 50000)
	for i := range pts {
		pts[i].Seq = i + 1
		pts[i].Equity = float64(i)
	}
	out := downsampleEquity(pts, maxEquityPoints)
	if len(out) > maxEquityPoints {
		t.Errorf("降采样后点数 %d 超过上限 %d", len(out), maxEquityPoints)
	}
	if out[0].Seq != 1 || out[len(out)-1].Seq != 50000 {
		t.Error("降采样应保留首尾点")
	}
	if len(downsampleEquity(pts[:100], maxEquityPoints)) != 100 {
		t.Error("未超上限时不应降采样")
	}
}

// TestRSIWilderOffset 验证 RSI 首个有效值位置（与主流 Wilder 实现对齐）。
func TestRSIWilderOffset(t *testing.T) {
	closeV := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	fields := map[string][]float64{"close": closeV}
	rsi, err := buildIndicator(IndicatorDef{ID: "r", Type: "RSI", Params: map[string]any{"window": 5, "field": "close"}}, fields)
	if err != nil {
		t.Fatal(err)
	}
	v := rsi.Values()
	// 索引 0..4 未就绪，索引 5（window）首个有效
	for i := 0; i < 5; i++ {
		if !math.IsNaN(v[i]) {
			t.Errorf("RSI 索引 %d 应为 NaN（未就绪），实际 %v", i, v[i])
		}
	}
	if math.IsNaN(v[5]) {
		t.Error("RSI 索引 5（window）应为首个有效值")
	}
	if v[9] != 100 {
		t.Errorf("单边上涨 RSI 应为 100，实际 %v", v[9])
	}
	if rsi.Type() != "RSI" {
		t.Errorf("指标 Type() 应返回真实类型 RSI，实际 %q", rsi.Type())
	}
}
