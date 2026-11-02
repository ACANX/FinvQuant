package quoteapiv2

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixedNow 固定时钟，便于断言时间戳与签名。
func fixedNow() time.Time { return time.UnixMilli(1780507278122) }

// newTestClient 创建一个指向测试服务器的客户端。
func newTestClient(t *testing.T, server *httptest.Server, mutate func(*Config)) *Client {
	t.Helper()
	cfg := Config{BaseURL: server.URL, CSRF: "abcdef0123", Now: fixedNow}
	if mutate != nil {
		mutate(&cfg)
	}
	return New(cfg)
}

// splitQuery 按出现顺序拆分查询串，返回键序与键值表。
func splitQuery(raw string) ([]string, map[string]string) {
	order := make([]string, 0, 8)
	values := make(map[string]string, 8)
	for _, pair := range strings.Split(raw, "&") {
		kv := strings.SplitN(pair, "=", 2)
		order = append(order, kv[0])
		if len(kv) == 2 {
			values[kv[0]] = kv[1]
		}
	}
	return order, values
}

// TestConfigDefaults 零值配置应补齐为协议默认值，并规整 URL 与路径。
func TestConfigDefaults(t *testing.T) {
	cfg := New(Config{}).Config()
	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("BaseURL 期望 %s，实际 %s", DefaultBaseURL, cfg.BaseURL)
	}
	if cfg.KLinePath != DefaultKLinePath {
		t.Fatalf("KLinePath 期望 %s，实际 %s", DefaultKLinePath, cfg.KLinePath)
	}
	if cfg.Locale != DefaultLocale {
		t.Fatalf("Locale 期望 %s，实际 %s", DefaultLocale, cfg.Locale)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Fatalf("Timeout 期望 %s，实际 %s", DefaultTimeout, cfg.Timeout)
	}
	if cfg.CSRF == "" {
		t.Fatal("CSRF 应有占位值")
	}
	if cfg.Now == nil {
		t.Fatal("Now 应有默认值")
	}

	// 自定义路径缺前导斜杠应补上，BaseURL 结尾斜杠应去掉。
	custom := New(Config{
		BaseURL:   "https://example.com/",
		KLinePath: "quote-api/quote-v2/get-kline",
	}).Config()
	if custom.BaseURL != "https://example.com" {
		t.Fatalf("BaseURL 结尾斜杠未去掉：%s", custom.BaseURL)
	}
	if custom.KLinePath != "/quote-api/quote-v2/get-kline" {
		t.Fatalf("KLinePath 前导斜杠未补上：%s", custom.KLinePath)
	}
}

// TestSignVectors 复现 MoomooQuoteV2WebSignature.md §6 的公开向量。
func TestSignVectors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"4A 空输入降级 quote", "", "58af97268c"},
		{"4A literal quote", "quote", "58af97268c"},
		{"4B 序列化校验", `{"a":"1","b":"2"}`, "39adef0837"},
		{
			"3A get-kline 黄金期货主连",
			`{"stockId":"70000294","marketType":"2","type":"2","marketCode":"70","instrumentType":"9","subInstrumentType":"9008","_":"1780507278122"}`,
			"45671b44c1",
		},
		{
			"3B 同 3A 改时间戳",
			`{"stockId":"70000294","marketType":"2","type":"2","marketCode":"70","instrumentType":"9","subInstrumentType":"9008","_":"1780600000000"}`,
			"7feb1b1665",
		},
		{
			"1A 新闻列表",
			`{"stock_id":"76033806042898","market_type":"1","type":"0","subType":"0","_":"1781592384789"}`,
			"055b0960ec",
		},
	}
	for _, tc := range cases {
		if got := Sign(tc.input); got != tc.want {
			t.Errorf("%s：token 期望 %s，实际 %s", tc.name, tc.want, got)
		}
	}
}

// TestSignParams 验证参数列表 → 签名输入 → token 的整条链路（含键序保真）。
func TestSignParams(t *testing.T) {
	params := Params(
		"stockId", "70000294",
		"marketType", "2",
		"type", "2",
		"marketCode", "70",
		"instrumentType", "9",
		"subInstrumentType", "9008",
		"_", "1780507278122",
	)
	want := `{"stockId":"70000294","marketType":"2","type":"2","marketCode":"70","instrumentType":"9","subInstrumentType":"9008","_":"1780507278122"}`
	if got := SignInput(params); got != want {
		t.Fatalf("签名输入不符：\n期望 %s\n实际 %s", want, got)
	}
	if got := SignParams(params); got != "45671b44c1" {
		t.Fatalf("token 期望 45671b44c1，实际 %s", got)
	}
	// 键序敏感：调换 marketType/type 应得到不同 token。
	swapped := Params(
		"stockId", "70000294",
		"type", "2",
		"marketType", "2",
		"marketCode", "70",
		"instrumentType", "9",
		"subInstrumentType", "9008",
		"_", "1780507278122",
	)
	if SignParams(swapped) == SignParams(params) {
		t.Fatal("键序变化后 token 不应相同")
	}
	// 查询串键序必须与参数顺序一致（不能被字典序重排）。
	query := SignedQuery(params)
	if !strings.HasPrefix(query, "stockId=70000294&marketType=2&type=2&marketCode=70") {
		t.Fatalf("查询串键序不符：%s", query)
	}
	if strings.Index(query, "_=") < strings.Index(query, "subInstrumentType=") {
		t.Fatalf("_ 参数应位于最后：%s", query)
	}
}

// TestAPIErrorError 三类失败都应带上可读描述。
func TestAPIErrorError(t *testing.T) {
	business := &APIError{Path: "/p", URL: "http://x/p", Code: -1, Message: "系统繁忙"}
	if got := business.Error(); !strings.Contains(got, "code=-1") || !strings.Contains(got, "系统繁忙") {
		t.Fatalf("业务码错误描述不符：%s", got)
	}
	httpErr := &APIError{Path: "/p", URL: "http://x/p", HTTPStatus: 502}
	if got := httpErr.Error(); !strings.Contains(got, "HTTP 错误：502") {
		t.Fatalf("HTTP 错误描述不符：%s", got)
	}
	plain := &APIError{Path: "/p", URL: "http://x/p", Message: "解析失败"}
	if got := plain.Error(); !strings.Contains(got, "请求失败：解析失败") {
		t.Fatalf("普通错误描述应带上原因：%s", got)
	}
}

// TestFlexNum JSON 数字、字符串与 null 都应正确解析。
func TestFlexNum(t *testing.T) {
	var payload klinePayload
	body := `{"code":0,"data":{"list":[{"k":1,"o":187.5,"c":"178.2","v":"12.5","r":0.03,"lc":187.5,"cp":-9.3}]}}`
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	bars := payload.Data.Bars()
	if len(bars) != 1 {
		t.Fatalf("期望 1 根，实际 %d", len(bars))
	}
	bar := bars[0]
	if bar.Open != "187.5" || bar.Close != "178.2" || bar.Volume != 12.5 ||
		bar.ChangePrice != "-9.3" || bar.LastClose != 187.5 {
		t.Fatalf("混用类型解析不符：%+v", bar)
	}
	if bar.ChangeRatio == nil || *bar.ChangeRatio != 0.03 {
		t.Fatalf("涨跌幅解析不符：%+v", bar.ChangeRatio)
	}

	var empty FlexNum
	if err := json.Unmarshal([]byte("null"), &empty); err != nil {
		t.Fatalf("null 解析失败: %v", err)
	}
	if empty.String() != "" || empty.Float() != 0 {
		t.Fatalf("null 应解析为空串且数值为 0，实际 %q / %v", empty.String(), empty.Float())
	}
	if got := FlexNum("not-a-number").Float(); got != 0 {
		t.Fatalf("非法数值应退回 0，实际 %v", got)
	}
}

// TestTruncate 超长响应体应被截断，避免错误信息膨胀。
func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc..." {
		t.Fatalf("截断结果不符：%s", got)
	}
	if got := truncate("abc", 3); got != "abc" {
		t.Fatalf("未超长时不应改变内容：%s", got)
	}
}
