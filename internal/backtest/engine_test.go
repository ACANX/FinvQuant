package backtest

import (
	"context"
	"math"
	"testing"
)

// genBars 生成合成行情：收盘价从 base 起每根线性变化 step（可正可负），
// OHLC 关系始终正确：open=前收，high=max(open,close)+|step|，low=min(open,close)-|step|。
func genBars(n int, base float64, step float64, date int) []Bar {
	bars := make([]Bar, n)
	prevClose := base
	for i := 0; i < n; i++ {
		close := base + step*float64(i+1)
		open := prevClose
		high := math.Max(open, close) + math.Abs(step)
		low := math.Min(open, close) - math.Abs(step)
		bars[i] = Bar{
			TS:    int64(1700000000 + i*60),
			Date:  date,
			Time:  int(10000 + i),
			Open:  open,
			High:  high,
			Low:   low,
			Close: close,
		}
		prevClose = close
	}
	return bars
}

func TestEngineDualMA(t *testing.T) {
	// W 形走势：下跌 → 上涨（快线上穿慢线买入）→ 下跌（快线下穿卖出）
	bars := make([]Bar, 0, 450)
	bars = append(bars, genBars(150, 100, -0.05, 20260105)...)
	bars = append(bars, genBars(150, 92.5, 0.1, 20260106)...)
	bars = append(bars, genBars(150, 107.5, -0.05, 20260107)...)

	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, PriceField: "close", WarmupBars: 20, FillMode: FillNextBarOpen},
		Indicators: []IndicatorDef{
			{ID: "ma_fast", Type: "MA", Params: map[string]any{"window": 5, "field": "close"}},
			{ID: "ma_slow", Type: "MA", Params: map[string]any{"window": 20, "field": "close"}},
		},
		Signals: SignalsDef{
			Buy:  "cross_up(ma_fast, ma_slow)",
			Sell: "cross_down(ma_fast, ma_slow)",
		},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{MaxPositionPct: 100, MaxPositions: 1},
	}

	cfg := EngineConfig{
		Definition:      def,
		Account:         AccountSnapshot{InitialCapital: 100000, CommissionRate: 0.0003, MarginRate: 1, MarginMode: "FULL"},
		SecuCode:        "TEST",
		Period:          PeriodMin,
		ReportPrecision: PeriodDay,
		Options:         RunOptions{EnableBacktest: true},
	}
	result, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err != nil {
		t.Fatalf("回测执行失败: %v", err)
	}
	if result.Report == nil {
		t.Fatal("报告为空")
	}
	if result.Report.TradeCount < 2 {
		t.Errorf("期望至少 2 笔成交（买+卖），实际 %d", result.Report.TradeCount)
	}
	if result.Report.FinalEquity <= result.Report.InitialCapital {
		t.Errorf("W 形底部买入顶部卖出应盈利，期末 %v <= 初始 %v", result.Report.FinalEquity, result.Report.InitialCapital)
	}
	if len(result.EquityPoints) == 0 {
		t.Error("净值曲线为空")
	}
	if result.Report.MaxInvested <= 0 || result.Report.AvgInvested <= 0 {
		t.Error("最大/平均投入金额应为正")
	}
	if result.Report.MaxDrawdownPct < 0 {
		t.Error("最大回撤不应为负")
	}
}

func TestEngineAllInAndStopLoss(t *testing.T) {
	// 单边下跌 + 止损：第一根买入，随后触发止损平仓，亏损受控
	bars := genBars(300, 100, -0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillNextBarOpen},
		// 只在首根触发买入（close > 99.5），止损后不再入场，验证单次止损
		Signals: SignalsDef{Buy: "close > 99.5", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{StopLossPct: 5, MaxPositionPct: 100},
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
	foundSell := false
	for _, tr := range result.Trades {
		if tr.Action == ActionSell {
			foundSell = true
			if tr.Signal != "止损" {
				t.Errorf("预期止损触发，实际信号 %q", tr.Signal)
			}
		}
	}
	if !foundSell {
		t.Error("未找到止损卖出记录")
	}
	if result.Report.TotalReturnPct < -8 {
		t.Errorf("止损后亏损应受控（约 -5%%），实际 %.2f%%", result.Report.TotalReturnPct)
	}
	if result.Report.SellCount != 1 || result.Report.BuyCount != 1 {
		t.Errorf("期望买 1 卖 1，实际买 %d 卖 %d", result.Report.BuyCount, result.Report.SellCount)
	}
}

func TestEngineTakeProfit(t *testing.T) {
	// 单边上涨 + 止盈：应触发止盈平仓，盈利约 +5%
	bars := genBars(300, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin, WarmupBars: 0, FillMode: FillNextBarOpen},
		Signals:  SignalsDef{Buy: "close > 0", Sell: ""},
		Rules: RulesDef{
			Buy:  RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true},
			Sell: RuleDef{Action: ActionSell, QuantityType: "ALL", Allow: true},
		},
		Risk: RiskDef{TakeProfitPct: 5, MaxPositionPct: 100},
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
	found := false
	for _, tr := range result.Trades {
		if tr.Action == ActionSell && tr.Signal == "止盈" {
			found = true
		}
	}
	if !found {
		t.Error("未找到止盈卖出记录")
	}
	if result.Report.TotalReturnPct < 3 {
		t.Errorf("止盈后应盈利约 +5%%，实际 %.2f%%", result.Report.TotalReturnPct)
	}
}

func TestEngineBacktestSwitchOff(t *testing.T) {
	bars := genBars(50, 100, 0.1, 20260105)
	def := StrategyDefinition{
		Version:  "1",
		Universe: UniverseDef{Securities: []string{"TEST"}},
		Data:     DataDef{Period: PeriodMin},
		Signals:  SignalsDef{Buy: "close > 0"},
		Rules:    RulesDef{Buy: RuleDef{Action: ActionBuy, QuantityType: "ALL_IN", Allow: true}},
	}
	cfg := EngineConfig{
		Definition: def,
		Account:    AccountSnapshot{InitialCapital: 100000, MarginRate: 1},
		Period:     PeriodMin, ReportPrecision: PeriodDay,
		Options: RunOptions{EnableBacktest: false}, // 回测开关关闭
	}
	_, err := NewEngine(cfg, nil).Run(context.Background(), bars)
	if err == nil {
		t.Error("回测开关关闭时应拒绝执行")
	}
}

func TestAggregateBars(t *testing.T) {
	// 构造 120 根分钟线：前 60 根 10:00-10:59，后 60 根 11:00-11:59
	bars := make([]Bar, 0, 120)
	ts := int64(1700000000)
	for i := 0; i < 120; i++ {
		minute := i % 60
		hour := 10 + i/60
		close := 100 + float64(i)*0.1
		bars = append(bars, Bar{
			TS: ts, Date: 20260105, Time: hour*10000 + minute*100,
			Open: close - 0.1, High: close + 0.1, Low: close - 0.1, Close: close,
			Volume: 100, Turnover: close * 100,
		})
		ts += 60
	}
	hourly, err := AggregateBars(bars, PeriodHour)
	if err != nil {
		t.Fatal(err)
	}
	if len(hourly) != 2 {
		t.Fatalf("120 分钟应聚合为 2 根小时线，实际 %d", len(hourly))
	}
	if hourly[0].Open != bars[0].Open || hourly[0].Close != bars[59].Close {
		t.Error("小时线开收盘价错误")
	}
	if hourly[0].Volume != 100*60 {
		t.Errorf("小时线成交量应求和为 %v，实际 %v", 100.0*60, hourly[0].Volume)
	}
	daily, err := AggregateBars(bars, PeriodDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 1 {
		t.Errorf("同日 120 分钟应聚合为 1 根日线，实际 %d", len(daily))
	}
	if daily[0].High != bars[119].High || daily[0].Low != bars[0].Low {
		t.Error("日线高低价错误")
	}
}

func TestIndicators(t *testing.T) {
	closeV := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	fields := map[string][]float64{"close": closeV, "high": closeV, "low": closeV}

	ma, err := buildIndicator(IndicatorDef{ID: "m", Type: "MA", Params: map[string]any{"window": 3, "field": "close"}}, fields)
	if err != nil {
		t.Fatal(err)
	}
	v := ma.Values()
	if !math.IsNaN(v[1]) {
		t.Errorf("MA3 第 2 个值应为 NaN，实际 %v", v[1])
	}
	if v[2] != 2 || v[9] != 9 {
		t.Errorf("MA3 值错误: v[2]=%v v[9]=%v", v[2], v[9])
	}

	rsi, err := buildIndicator(IndicatorDef{ID: "r", Type: "RSI", Params: map[string]any{"window": 5, "field": "close"}}, fields)
	if err != nil {
		t.Fatal(err)
	}
	rv := rsi.Values()
	if !math.IsNaN(rv[4]) {
		t.Errorf("RSI 前 5 个值应为 NaN，实际 %v", rv[4])
	}
	if rv[9] != 100 {
		t.Errorf("单边上涨 RSI 应为 100，实际 %v", rv[9])
	}

	ema, err := buildIndicator(IndicatorDef{ID: "e", Type: "EMA", Params: map[string]any{"window": 3, "field": "close"}}, fields)
	if err != nil {
		t.Fatal(err)
	}
	if ema.Values()[0] != 1 {
		t.Errorf("EMA 首值应为 1，实际 %v", ema.Values()[0])
	}

	if _, err := buildIndicator(IndicatorDef{ID: "x", Type: "UNKNOWN", Params: map[string]any{}}, fields); err == nil {
		t.Error("未知指标类型应报错")
	}
}
