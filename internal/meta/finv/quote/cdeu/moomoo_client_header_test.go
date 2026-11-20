package cdeu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// captureStockListHeaders 起一个假站点，返回构造好的客户端与「最后一次请求的头」取值函数。
func captureStockListHeaders(t *testing.T, marketType string) (quoteapiv2.StockListPage, http.Header) {
	t.Helper()
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"pagination":{"page":0,"pageSize":50,"pageCount":1,"total":1},` +
			`"list":[{"stockId":1,"name":"工商银行","stockCode":"601398","marketLabel":"SH","instrumentType":3}]}}`))
	}))
	t.Cleanup(server.Close)

	client := quoteapiv2.New(quoteapiv2.Config{
		BaseURL: server.URL,
		Header: map[string]string{
			"accept-language": moomooWebAcceptLanguage,
		},
	})
	page, err := client.GetStockList(context.Background(), quoteapiv2.GetStockListRequest{
		MarketType:      marketType,
		RankType:        "3",
		Page:            "0",
		PageSize:        "50",
		TimestampMillis: "1795093151662",
	})
	if err != nil {
		t.Fatalf("请求假站点失败: %v", err)
	}
	return page, captured
}

// TestMoomooClientBrowserHeaders 浏览器的四个关键特征头必须齐全（需求 11336）。
//
// 背景：站点按 UA / Referer / Accept / Accept-Language 判定浏览器请求，
// 缺失（尤其 UA、Referer）会被 302 到 /403 风控页，客户端拿到 HTML 后只报
// `invalid character '<'`，看不出是被限流（需求 11335）。
func TestMoomooClientBrowserHeaders(t *testing.T) {
	page, header := captureStockListHeaders(t, "4")
	if len(page.Items) != 1 || page.Items[0].StockCode != "601398" {
		t.Fatalf("假站点数据未正确解析: %+v", page.Items)
	}

	ua := header.Get("User-Agent")
	if !strings.Contains(ua, "Mozilla") || !strings.Contains(ua, "Chrome") {
		t.Errorf("User-Agent 应为浏览器 UA，实际 %q", ua)
	}
	if got := header.Get("Accept-Language"); got != moomooWebAcceptLanguage {
		t.Errorf("Accept-Language=%q want %q", got, moomooWebAcceptLanguage)
	}
	if got := header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept=%q want application/json", got)
	}
	wantReferer := "https://www.moomoo.com/hans/quote/cn/stock-list/all-cn-stocks/top-market-cap"
	if got := header.Get("Referer"); got != wantReferer {
		t.Errorf("Referer=%q want %q", got, wantReferer)
	}
	cookie := header.Get("Cookie")
	for _, want := range []string{"csrfToken=", "locale=zh-cn"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("Cookie 缺少 %q，实际 %q", want, cookie)
		}
	}
	if header.Get("Quote-Token") == "" {
		t.Error("quote-token 不能为空（GET 请求签名头）")
	}
}

// TestMoomooClientRefererByMarket 不同市场带各自的中文站列表页 Referer（决定名称语言与数值单位）。
func TestMoomooClientRefererByMarket(t *testing.T) {
	cases := map[string]string{
		"1": "https://www.moomoo.com/hans/quote/hk/stock-list/all-hk-stocks/top-market-cap",
		"2": "https://www.moomoo.com/hans/quote/us/stock-list/all-us-stocks/top-market-cap",
		"4": "https://www.moomoo.com/hans/quote/cn/stock-list/all-cn-stocks/top-market-cap",
	}
	for marketType, want := range cases {
		_, header := captureStockListHeaders(t, marketType)
		if got := header.Get("Referer"); got != want {
			t.Errorf("marketType=%s Referer=%q want %q", marketType, got, want)
		}
	}
}

// TestNewMoomooQuoteClientSetsAcceptLanguage 全仓库统一入口带 Accept-Language。
//
// 用假站点验证工厂函数本身（其余用例手工构造 Config，覆盖不到工厂）。
func TestNewMoomooQuoteClientSetsAcceptLanguage(t *testing.T) {
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"pagination":{},"list":[]}}`))
	}))
	defer server.Close()

	client := newMoomooQuoteClient()
	if got := client.Config().Header["accept-language"]; got != moomooWebAcceptLanguage {
		t.Errorf("工厂函数未设 accept-language，实际 %q", got)
	}
	// BaseURL 指向假站点后再真正发一次请求，确认头确实发出去了。
	patched := quoteapiv2.New(quoteapiv2.Config{
		BaseURL: server.URL,
		Header:  client.Config().Header,
	})
	if _, err := patched.GetStockList(context.Background(), quoteapiv2.GetStockListRequest{
		MarketType: "4", Page: "0", PageSize: "50", TimestampMillis: "1795093151662",
	}); err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if got := captured.Get("Accept-Language"); got != moomooWebAcceptLanguage {
		t.Errorf("实际请求 Accept-Language=%q want %q", got, moomooWebAcceptLanguage)
	}
	if ua := captured.Get("User-Agent"); !strings.Contains(ua, "Chrome") {
		t.Errorf("实际请求 User-Agent 缺少 Chrome 特征: %q", ua)
	}
}
