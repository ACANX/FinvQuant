package cdeu

import "testing"

// TestStockListFallbackCurrency 校验采集补币种的兜底规则（需求 10993）。
//
// 依据：主数据表（cdeu.finv_quote_cdeu_usc、public.finv_security）查不到时，
// 按 market_label + 代码推导——HK 的 8 开头 5 位代码是人民币柜台（R 股）→ CNY，其余 HKD；
// US → USD；SH/SZ/BJ → CNY。库内 2203 条港股快照验证该规则零例外。
func TestStockListFallbackCurrency(t *testing.T) {
	cases := []struct {
		marketLabel string
		code        string
		want        string
	}{
		{"HK", "00700", "HKD"},
		{"HK", "00005", "HKD"},
		{"HK", "02840", "HKD"},
		{"HK", "82318", "CNY"},
		{"HK", "80016", "CNY"},
		{"HK", "81299", "CNY"},
		{"hk", "82318", "CNY"}, // 大小写不敏感
		{"US", "NVDA", "USD"},
		{"US", "HSBC", "USD"},
		{"SH", "600519", "CNY"},
		{"SZ", "000001", "CNY"},
		{"BJ", "830799", "CNY"},
		{"CN", "600519", "CNY"}, // 兼容历史数据
		{"", "00700", ""},       // market_label 缺失不猜
	}
	for _, tc := range cases {
		if got := stockListFallbackCurrency(tc.marketLabel, tc.code); got != tc.want {
			t.Errorf("stockListFallbackCurrency(%q,%q) = %q，期望 %q",
				tc.marketLabel, tc.code, got, tc.want)
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
