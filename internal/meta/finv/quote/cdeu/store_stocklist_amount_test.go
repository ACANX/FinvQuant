package cdeu

import (
	"net/http"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestStockListAmountUnits 校验金额/股数/成交量的单位换算（需求 10989）。
//
// 关键回归点：moomoo 在未锁定语言时返回**英文单位**（T/B/M/K），
// 中文口径为万亿/亿/万；两种口径必须都换算到「元/股」基准单位，
// 否则万亿级证券会被整条丢弃（市值字段解析失败 → NULL）。
func TestStockListAmountUnits(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"中文万亿", "4.07万亿", "4070000000000", true},
		{"中文亿", "8959.2亿", "895920000000", true},
		{"中文万", "1.5万", "15000", true},
		{"英文T", "4.07T", "4070000000000", true},
		{"英文t", "4.07t", "4070000000000", true},
		{"英文B", "895.92B", "895920000000", true},
		{"英文b", "895.92b", "895920000000", true},
		{"英文M", "1.5M", "1500000", true},
		{"英文m", "1.5m", "1500000", true},
		{"英文K", "12K", "12000", true},
		{"英文k", "12k", "12000", true},
		{"无单位带千分位", "1,234.5", "1234.5", true},
		{"无单位整数", "123", "123", true},
		{"负数", "-2.5B", "-2500000000", true},
		{"空串", "", "", false},
		{"非数字", "N/A", "", false},
		{"仅单位", "亿", "", false},
	}
	for _, tc := range cases {
		got, ok := stockListAmount(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: stockListAmount(%q) = (%q, %v)，期望 (%q, %v)",
				tc.name, tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestStockListSampleItems 校验首页采样串（排查语言/单位口径的证据来源）。
func TestStockListSampleItems(t *testing.T) {
	items := []quoteapiv2.StockListItem{
		{StockCode: "00700", Name: "TENCENT", MarketValue: "4.07T"},
		{StockCode: "00005", Name: "HSBC", MarketValue: "2.51T"},
		{StockCode: "01299", Name: "AIA", MarketValue: "795.2B"},
		{StockCode: "01810", Name: "XIAOMI-W", MarketValue: "741.89B"},
	}
	want := "00700|TENCENT|4.07T 00005|HSBC|2.51T 01299|AIA|795.2B"
	if got := stockListSampleItems(items, 3); got != want {
		t.Errorf("stockListSampleItems = %q，期望 %q", got, want)
	}
	if got := stockListSampleItems(nil, 3); got != "（空）" {
		t.Errorf("空列表采样 = %q，期望「（空）」", got)
	}
	// 超宽名称截断，避免日志刷屏
	long := []quoteapiv2.StockListItem{{StockCode: "X", Name: "ABCDEFGHIJKLMNOPQRSTUVWXYZ", MarketValue: "1B"}}
	if got := stockListSampleItems(long, 1); stockListDisplayWidth(got) > 40 {
		t.Errorf("截断后采样仍过长: %q", got)
	}
}

// TestStockListHeaderTransport 校验 accept-language 注入（语言锁定）。
func TestStockListHeaderTransport(t *testing.T) {
	var seen string
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = req.Header.Get("accept-language")
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: http.Header{}}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (stockListHeaderTransport{base: base}).RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if seen != StockListAcceptLanguage {
		t.Errorf("accept-language = %q，期望 %q", seen, StockListAcceptLanguage)
	}

	// 调用方已设置时不覆盖
	req2, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	req2.Header.Set("accept-language", "en-US")
	if _, err := (stockListHeaderTransport{base: base}).RoundTrip(req2); err != nil {
		t.Fatal(err)
	}
	if seen != "en-US" {
		t.Errorf("已设置的语言被覆盖为 %q", seen)
	}
}

// roundTripFunc 让测试注入轻量 RoundTripper。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
