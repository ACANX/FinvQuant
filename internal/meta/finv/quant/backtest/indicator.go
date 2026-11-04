package backtest

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------
// 指标计算（向量化）：输入与行情 bar 等长的字段数组，输出等长结果序列，
// 未就绪位置为 NaN。信号表达式引用指标 ID 时取当前 bar 位置的值。
// ---------------------------------------------------------------------

// Indicator 指标接口。
type Indicator interface {
	ID() string
	Type() string
	// Values 返回与输入对齐的完整序列（NaN = 未就绪）。
	Values() []float64
}

// buildIndicator 按定义构建指标（未识别的类型返回错误，保证策略定义可校验）。
func buildIndicator(def IndicatorDef, fieldValues map[string][]float64) (Indicator, error) {
	field := paramString(def.Params, "field", "close")
	values, ok := fieldValues[field]
	if !ok {
		return nil, fmt.Errorf("指标 %s 引用未知字段 %q（可用: open/high/low/close/volume/turnover）", def.ID, field)
	}
	window := paramInt(def.Params, "window", 14)
	if window < 1 {
		window = 1
	}
	switch def.Type {
	case "MA":
		return &maIndicator{id: def.ID, typ: "MA", values: movingAverage(values, window)}, nil
	case "EMA":
		return &maIndicator{id: def.ID, typ: "EMA", values: exponentialMovingAverage(values, window)}, nil
	case "RSI":
		return &maIndicator{id: def.ID, typ: "RSI", values: rsi(values, window)}, nil
	case "MACD":
		fast := paramInt(def.Params, "fast", 12)
		slow := paramInt(def.Params, "slow", 26)
		signal := paramInt(def.Params, "signal", 9)
		if fast <= 0 || slow <= 0 || signal <= 0 {
			return nil, fmt.Errorf("指标 %s MACD 参数必须为正数", def.ID)
		}
		dif, dea, hist := macd(values, fast, slow, signal)
		output := paramString(def.Params, "output", "dif")
		var out []float64
		switch output {
		case "dif":
			out = dif
		case "dea":
			out = dea
		case "hist":
			out = hist
		default:
			return nil, fmt.Errorf("指标 %s MACD output 仅支持 dif/dea/hist", def.ID)
		}
		return &maIndicator{id: def.ID, typ: "MACD", values: out}, nil
	case "BOLL":
		k := paramFloat(def.Params, "k", 2)
		mid, upper, lower := bollinger(values, window, k)
		output := paramString(def.Params, "output", "mid")
		var out []float64
		switch output {
		case "mid":
			out = mid
		case "upper":
			out = upper
		case "lower":
			out = lower
		default:
			return nil, fmt.Errorf("指标 %s BOLL output 仅支持 mid/upper/lower", def.ID)
		}
		return &maIndicator{id: def.ID, typ: "BOLL", values: out}, nil
	case "ATR":
		high := fieldValues["high"]
		low := fieldValues["low"]
		closeV := fieldValues["close"]
		return &maIndicator{id: def.ID, typ: "ATR", values: atr(high, low, closeV, window)}, nil
	case "STDDEV":
		return &maIndicator{id: def.ID, typ: "STDDEV", values: rollingStddev(values, window)}, nil
	case "HHV", "LLV":
		// 参数键统一为 field（评审：原实现校验取 field、计算读 source，键不一致静默算错路径），
		// 兼容历史 source 键。
		srcField := paramString(def.Params, "field", "")
		if srcField == "" {
			srcField = paramString(def.Params, "source", "")
		}
		if srcField == "" {
			if def.Type == "HHV" {
				srcField = "high"
			} else {
				srcField = "low"
			}
		}
		src, ok := fieldValues[srcField]
		if !ok {
			return nil, fmt.Errorf("指标 %s 引用未知字段 %q（可用: open/high/low/close/volume/turnover）", def.ID, srcField)
		}
		if def.Type == "HHV" {
			return &maIndicator{id: def.ID, typ: "HHV", values: rollingMax(src, window)}, nil
		}
		return &maIndicator{id: def.ID, typ: "LLV", values: rollingMin(src, window)}, nil
	default:
		return nil, fmt.Errorf("不支持的指标类型 %q（支持: MA/EMA/RSI/MACD/BOLL/ATR/STDDEV/HHV/LLV）", def.Type)
	}
}

// maIndicator 通用结果容器（values 已就绪，typ 记录真实指标类型）。
type maIndicator struct {
	id     string
	typ    string
	values []float64
}

func (m *maIndicator) ID() string        { return m.id }
func (m *maIndicator) Type() string      { return m.typ }
func (m *maIndicator) Values() []float64 { return m.values }

// ---------------------------------------------------------------------
// 基础计算函数
// ---------------------------------------------------------------------

func movingAverage(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	sum := 0.0
	for i := 0; i < len(values); i++ {
		sum += values[i]
		if i >= window {
			sum -= values[i-window]
		}
		if i >= window-1 {
			out[i] = sum / float64(window)
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

func exponentialMovingAverage(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	alpha := 2.0 / float64(window+1)
	prev := math.NaN()
	for i, v := range values {
		if math.IsNaN(prev) {
			prev = v
			out[i] = v
			continue
		}
		prev = alpha*v + (1-alpha)*prev
		out[i] = prev
	}
	return out
}

func rsi(values []float64, window int) []float64 {
	// Wilder RSI：首个有效值位于索引 window（前 window 根变化量简单平均，与主流实现对齐）。
	// 评审意见：原实现首个有效值差 1 bar（window+1 才出值），已修正。
	out := make([]float64, len(values))
	for i := range out {
		out[i] = math.NaN() // 未就绪位置恒为 NaN
	}
	if len(values) <= window {
		return out
	}
	avgGain, avgLoss := 0.0, 0.0
	for i := 1; i <= window; i++ {
		change := values[i] - values[i-1]
		if change > 0 {
			avgGain += change / float64(window)
		} else {
			avgLoss += -change / float64(window)
		}
	}
	if avgLoss == 0 {
		out[window] = 100
	} else {
		out[window] = 100 - 100/(1+avgGain/avgLoss)
	}
	for i := window + 1; i < len(values); i++ {
		change := values[i] - values[i-1]
		gain, loss := 0.0, 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}
		avgGain = (avgGain*float64(window-1) + gain) / float64(window)
		avgLoss = (avgLoss*float64(window-1) + loss) / float64(window)
		if avgLoss == 0 {
			out[i] = 100
		} else {
			out[i] = 100 - 100/(1+avgGain/avgLoss)
		}
	}
	return out
}

func macd(values []float64, fast, slow, signal int) ([]float64, []float64, []float64) {
	emaFast := exponentialMovingAverage(values, fast)
	emaSlow := exponentialMovingAverage(values, slow)
	dif := make([]float64, len(values))
	for i := range values {
		dif[i] = emaFast[i] - emaSlow[i]
	}
	dea := exponentialMovingAverage(dif, signal)
	hist := make([]float64, len(values))
	for i := range values {
		hist[i] = (dif[i] - dea[i]) * 2
	}
	return dif, dea, hist
}

func bollinger(values []float64, window int, k float64) ([]float64, []float64, []float64) {
	mid := movingAverage(values, window)
	std := rollingStddev(values, window)
	upper := make([]float64, len(values))
	lower := make([]float64, len(values))
	for i := range values {
		if math.IsNaN(mid[i]) {
			upper[i], lower[i] = math.NaN(), math.NaN()
			continue
		}
		upper[i] = mid[i] + k*std[i]
		lower[i] = mid[i] - k*std[i]
	}
	return mid, upper, lower
}

func atr(high, low, closeV []float64, window int) []float64 {
	n := len(high)
	if n == 0 {
		return nil
	}
	tr := make([]float64, n)
	tr[0] = high[0] - low[0]
	for i := 1; i < n; i++ {
		prevClose := closeV[i-1]
		tr[i] = math.Max(high[i]-low[i], math.Max(math.Abs(high[i]-prevClose), math.Abs(low[i]-prevClose)))
	}
	return exponentialMovingAverage(tr, window)
}

func rollingStddev(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	for i := 0; i < len(values); i++ {
		if i < window-1 {
			out[i] = math.NaN()
			continue
		}
		var sum, sumSq float64
		for j := i - window + 1; j <= i; j++ {
			sum += values[j]
			sumSq += values[j] * values[j]
		}
		mean := sum / float64(window)
		variance := sumSq/float64(window) - mean*mean
		if variance < 0 {
			variance = 0
		}
		out[i] = math.Sqrt(variance)
	}
	return out
}

func rollingMax(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	for i := 0; i < len(values); i++ {
		if i < window-1 {
			out[i] = math.NaN()
			continue
		}
		m := math.Inf(-1)
		for j := i - window + 1; j <= i; j++ {
			if values[j] > m {
				m = values[j]
			}
		}
		out[i] = m
	}
	return out
}

func rollingMin(values []float64, window int) []float64 {
	out := make([]float64, len(values))
	for i := 0; i < len(values); i++ {
		if i < window-1 {
			out[i] = math.NaN()
			continue
		}
		m := math.Inf(1)
		for j := i - window + 1; j <= i; j++ {
			if values[j] < m {
				m = values[j]
			}
		}
		out[i] = m
	}
	return out
}

// ---------------------------------------------------------------------
// 参数工具
// ---------------------------------------------------------------------

func paramInt(params map[string]any, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	switch v := params[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var out int
		if _, err := fmt.Sscanf(v, "%d", &out); err == nil {
			return out
		}
	}
	return fallback
}

func paramFloat(params map[string]any, key string, fallback float64) float64 {
	if params == nil {
		return fallback
	}
	switch v := params[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		var out float64
		if _, err := fmt.Sscanf(v, "%f", &out); err == nil {
			return out
		}
	}
	return fallback
}

func paramString(params map[string]any, key, fallback string) string {
	if params == nil {
		return fallback
	}
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
