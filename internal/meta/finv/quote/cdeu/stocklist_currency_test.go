package cdeu

import "testing"

// TestStockListFallbackCurrency 校验采集补币种的兜底规则（需求 10993）。
//
// 依据：主数据表（cdeu.finv_quote_cdeu_usc、public.finv_security）查不到时，
// 按「市场 + 柜台」推导——港股 8 开头的 5 位代码是人民币柜台（R 股）→ CNY，其余 HKD；
// 美股 USD；沪深 CNY。库内 2203 条港股快照验证该规则零例外。
func TestStockListFallbackCurrency(t *testing.T) {
	cases := []struct {
		marketType  string
		marketLabel string
		code        string
		want        string
	}{
		{"1", "HK", "00700", "HKD"},
		{"1", "HK", "00005", "HKD"},
		{"1", "HK", "02840", "HKD"},
		{"1", "HK", "82318", "CNY"},
		{"1", "HK", "80016", "CNY"},
		{"1", "HK", "81299", "CNY"},
		{"2", "US", "NVDA", "USD"},
		{"2", "US", "HSBC", "USD"},
		{"4", "CN", "600519", "CNY"},
		{"4", "CN", "000001", "CNY"},
		{"", "HK", "02328", "HKD"},  // 只有 marketLabel 时也能判定
		{"1", "", "00700", "HKD"},   // 只有 marketType 时也能判定
		{"3", "", "X", ""},          // 未知市场不猜
	}
	for _, tc := range cases {
		if got := stockListFallbackCurrency(tc.marketType, tc.marketLabel, tc.code); got != tc.want {
			t.Errorf("stockListFallbackCurrency(%q,%q,%q) = %q，期望 %q",
				tc.marketType, tc.marketLabel, tc.code, got, tc.want)
		}
	}
}

// TestStockListHKCurrency 校验港股柜台判定边界。
func TestStockListHKCurrency(t *testing.T) {
	cases := map[string]string{
		"80016": "CNY",
		"89999": "CNY",
		"79999": "HKD",
		"0700":  "HKD", // 非 5 位不按 R 柜台处理
		"":      "HKD",
		"AAPL":  "HKD",
	}
	for code, want := range cases {
		if got := stockListHKCurrency(code); got != want {
			t.Errorf("stockListHKCurrency(%q) = %q，期望 %q", code, got, want)
		}
	}
}
