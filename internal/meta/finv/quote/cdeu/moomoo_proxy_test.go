package cdeu

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestParseMoomooQuoteProxy 单配置项 `{enable}|{base_url}` 的解析口径（需求 11337）。
func TestParseMoomooQuoteProxy(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantEnabled bool
		wantURL     string
		wantActive  bool
	}{
		{"空串=直连", "", false, "", false},
		{"关闭+地址", "0|https://p.example.com", false, "https://p.example.com", false},
		{"启用+地址", "1|https://p.example.com", true, "https://p.example.com", true},
		{"true+地址", "TRUE|https://p.example.com", true, "https://p.example.com", true},
		{"on+地址", "on|https://p.example.com/", true, "https://p.example.com", true},
		{"启用但无地址=不生效", "1|", true, "", false},
		{"启用+空白地址=不生效", "1|   ", true, "", false},
		{"只填地址=视为启用", "https://p.example.com/", true, "https://p.example.com", true},
		{"地址多余空白", " 1 | https://p.example.com/ ", true, "https://p.example.com", true},
	}
	for _, c := range cases {
		got := parseMoomooQuoteProxy(c.raw)
		if got.Enabled != c.wantEnabled || got.BaseURL != c.wantURL || got.Active() != c.wantActive {
			t.Errorf("%s: parse(%q)=%+v active=%v want %v/%q active=%v",
				c.name, c.raw, got, got.Active(), c.wantEnabled, c.wantURL, c.wantActive)
		}
	}
}

// TestMoomooProxyTransport 代理层把 GET 打包成 Worker POST，并把目标响应透传。
func TestMoomooProxyTransport(t *testing.T) {
	var gotPayload moomooProxyRequest
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Worker 应收到 POST，实际 %s", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotPayload); err != nil {
			t.Errorf("Worker 请求体不是合法 JSON: %v", err)
		}
		// 模拟 Worker：真实 gzip 压缩 + content-encoding（贴近中转链路），
		// 由本层 Transport 过滤编码头后把明文交给上层。
		payload := []byte(`{"code":0,"message":"成功","data":{"pagination":{"total":5588},"list":[]}}`)
		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.Header().Set("content-encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		zw := gzip.NewWriter(w)
		_, _ = zw.Write(payload)
		_ = zw.Close()
	}))
	defer worker.Close()

	client := quoteapiv2.New(quoteapiv2.Config{
		Header:     map[string]string{"accept-language": moomooWebAcceptLanguage},
		HTTPClient: &http.Client{Transport: newMoomooProxyTransport(worker.URL)},
	})
	page, err := client.GetStockList(context.Background(), quoteapiv2.GetStockListRequest{
		MarketType: "4", RankType: "3", Page: "0", PageSize: "50", TimestampMillis: "1795093151662",
	})
	if err != nil {
		t.Fatalf("经代理拉取失败: %v", err)
	}
	if page.Pagination.Total != 5588 {
		t.Errorf("解析到的 total=%d want 5588", page.Pagination.Total)
	}

	// Worker 收到的转发配置：GET + 原始完整 URL + 关键头透传 + 关缓存
	if gotPayload.Method != http.MethodGet {
		t.Errorf("target method=%q want GET", gotPayload.Method)
	}
	for _, want := range []string{
		"https://www.moomoo.com/quote-api/quote-v2/get-stock-list?",
		"marketType=4", "pageSize=50", "_=1795093151662",
	} {
		if !strings.Contains(gotPayload.Target, want) {
			t.Errorf("target 缺少 %q，实际 %q", want, gotPayload.Target)
		}
	}
	if got := gotPayload.Headers["accept-language"]; got != moomooWebAcceptLanguage {
		t.Errorf("转发头 accept-language=%q want %q", got, moomooWebAcceptLanguage)
	}
	if got := gotPayload.Headers["quote-token"]; got == "" {
		t.Error("转发头缺少 quote-token（签名头必须透传）")
	}
	if got := gotPayload.Headers["user-agent"]; !strings.Contains(got, "Chrome") {
		t.Errorf("转发头 user-agent 缺少浏览器特征: %q", got)
	}
	if cache, ok := gotPayload.CFOptions["cacheEverything"]; !ok || cache != false {
		t.Errorf("cfOptions.cacheEverything 应为 false，实际 %v", gotPayload.CFOptions)
	}
}

// TestMoomooProxyTransportRejectsPost 带请求体的 POST 明确报错（不猜测性降级）。
func TestMoomooProxyTransportRejectsPost(t *testing.T) {
	transport := newMoomooProxyTransport("https://p.example.com")
	req, _ := http.NewRequest(http.MethodPost, "https://www.moomoo.com/quote-api/quote-v2/option-quote",
		strings.NewReader(`{"a":1}`))
	if _, err := transport.RoundTrip(req); err == nil {
		t.Fatal("POST 请求应被拒绝")
	} else if !strings.Contains(err.Error(), "只支持 GET") {
		t.Errorf("错误信息应说明只支持 GET，实际 %v", err)
	}
}

// TestNewMoomooQuoteClientWithProxy 未启用时等价直连，启用时注入代理 Transport。
func TestNewMoomooQuoteClientWithProxy(t *testing.T) {
	direct := newMoomooQuoteClientWithProxy(MoomooQuoteProxyConfig{})
	if direct.Config().HTTPClient != nil {
		t.Error("未启用代理时不应注入 HTTPClient")
	}
	proxied := newMoomooQuoteClientWithProxy(MoomooQuoteProxyConfig{
		Enabled: true, BaseURL: "https://p.example.com",
	})
	if proxied.Config().HTTPClient == nil {
		t.Fatal("启用代理时应注入 HTTPClient")
	}
	if _, ok := proxied.Config().HTTPClient.Transport.(*moomooProxyTransport); !ok {
		t.Errorf("Transport 类型=%T want *moomooProxyTransport", proxied.Config().HTTPClient.Transport)
	}
	// 启用标志为假或地址为空都不生效
	for _, cfg := range []MoomooQuoteProxyConfig{
		{Enabled: false, BaseURL: "https://p.example.com"},
		{Enabled: true, BaseURL: ""},
	} {
		if newMoomooQuoteClientWithProxy(cfg).Config().HTTPClient != nil {
			t.Errorf("配置 %+v 不应启用代理", cfg)
		}
	}
}
