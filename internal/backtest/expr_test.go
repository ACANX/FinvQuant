package backtest

import (
	"math"
	"testing"
)

// 构造测试上下文：两个指标序列 + 字段。
func testCtx() *EvalContext {
	maFast := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	maSlow := []float64{5, 5, 5, 5, 5, 5, 5, 5}
	closeV := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	return &EvalContext{
		At:         7,
		Fields:     map[string][]float64{"close": closeV, "open": closeV},
		Indicators: map[string][]float64{"ma_fast": maFast, "ma_slow": maSlow},
	}
}

func TestExprComparison(t *testing.T) {
	ctx := testCtx()
	cases := []struct {
		expr string
		want bool
	}{
		{"ma_fast > ma_slow", true},
		{"ma_fast < ma_slow", false},
		{"ma_fast >= ma_slow", true},
		{"ma_fast == ma_slow", false},
		{"ma_fast != ma_slow", true},
		{"ma_fast > 7", true},
		{"close > 7 AND ma_fast > 7", true},
		{"close < 7 OR ma_fast > 7", true},
		{"NOT (close < 7)", true},
		{"(ma_fast - ma_slow) > 2", true},
		{"ma_fast / 2 > 3", true},
		{"abs(ma_fast - 10) > 1", true},
		{"close * 2 == 16", true},
	}
	for _, c := range cases {
		fn, err := CompileExpr(c.expr)
		if err != nil {
			t.Errorf("编译 %q 失败: %v", c.expr, err)
			continue
		}
		got, err := fn(ctx)
		if err != nil {
			t.Errorf("求值 %q 失败: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("表达式 %q = %v, 期望 %v", c.expr, got, c.want)
		}
	}
}

func TestExprCross(t *testing.T) {
	// ma_fast 上穿 ma_slow 发生在 index=3（3>4? no）——构造：fast 在 idx3 上穿 slow
	fast := []float64{6, 5, 4, 5, 6, 7}
	slow := []float64{5, 5, 5, 5, 5, 5}
	ctx := &EvalContext{
		At:         4,
		Fields:     map[string][]float64{"close": fast},
		Indicators: map[string][]float64{"fast": fast, "slow": slow},
	}
	fn, err := CompileExpr("cross_up(fast, slow)")
	if err != nil {
		t.Fatal(err)
	}
	// index 4：prev fast=5 <= slow=5, cur fast=6 > 5 → true
	got, _ := fn(ctx)
	if !got {
		t.Errorf("cross_up 应在 index=4 为 true")
	}
	// index 5：prev fast=6 > 5 → false
	ctx.At = 5
	got, _ = fn(ctx)
	if got {
		t.Errorf("cross_up 应在 index=5 为 false")
	}

	// cross_down
	fn2, _ := CompileExpr("cross_down(fast, slow)")
	ctx.At = 2 // prev fast=5 <= 5? fast[1]=5 <= slow[1]=5 且 fast[2]=4 < 5 → true
	got, _ = fn2(ctx)
	if !got {
		t.Errorf("cross_down 应在 index=2 为 true")
	}
}

func TestExprRefAndRange(t *testing.T) {
	ctx := testCtx()
	fn, err := CompileExpr("ref(ma_fast, 2) == 6")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := fn(ctx)
	if !got {
		t.Errorf("ref(ma_fast, 2) 在 index=7 应为 6")
	}

	fn2, err := CompileExpr("highest(ma_fast, 3) == 8")
	if err != nil {
		t.Fatal(err)
	}
	got, _ = fn2(ctx)
	if !got {
		t.Errorf("highest(ma_fast, 3) 应为 8")
	}

	fn3, err := CompileExpr("lowest(ma_fast, 3) == 6")
	if err != nil {
		t.Fatal(err)
	}
	got, _ = fn3(ctx)
	if !got {
		t.Errorf("lowest(ma_fast, 3) 应为 6")
	}
}

func TestExprNaN(t *testing.T) {
	// NaN 参与比较恒为 false
	series := []float64{math.NaN(), math.NaN(), math.NaN()}
	ctx := &EvalContext{
		At:         2,
		Fields:     map[string][]float64{"close": series},
		Indicators: map[string][]float64{"ma": series},
	}
	fn, err := CompileExpr("ma > 1")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := fn(ctx)
	if got {
		t.Errorf("NaN 比较应返回 false")
	}
}

func TestExprSyntaxError(t *testing.T) {
	cases := []string{
		"",
		"ma_fast >",
		"(ma_fast",
		"ma_fast >> 1",
		"unknown_func(a)",
		"ma_fast > 1 AND",
	}
	for _, expr := range cases {
		if expr == "" {
			continue
		}
		if _, err := CompileExpr(expr); err == nil {
			t.Errorf("表达式 %q 应编译失败", expr)
		}
	}
}
