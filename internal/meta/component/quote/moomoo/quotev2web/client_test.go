package quotev2web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

// klineSample 1108-Kline.md §3 文档样本（含 o/c/h/l 为字符串、v/t/lc 为数值的类型混用）。
const klineSample = `{
    "code": 0,
    "message": "成功",
    "data": {
        "list": [
            {"k": 157870800, "o": "187.5", "c": "178.2", "h": "187.5", "l": "176",
             "v": 0, "t": 0, "r": null, "lc": 187.5, "cp": "-9.3"}
        ]
    }
}`

// TestKlineRequestAndParse 校验请求装配（路径/键序/token/请求头）与响应解析。
func TestKlineRequestAndParse(t *testing.T) {
	var gotQuery map[string]string
	var gotOrder []string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != DefaultKlinePath {
			t.Errorf("路径期望 %s，实际 %s", DefaultKlinePath, r.URL.Path)
		}
		gotHeaders = r.Header.Clone()
		gotOrder = nil
		gotQuery = map[string]string{}
		for _, pair := range strings.Split(r.URL.RawQuery, "&") {
			kv := strings.SplitN(pair, "=", 2)
			gotOrder = append(gotOrder, kv[0])
			gotQuery[kv[0]] = kv[1]
		}
		// 服务端校验 token：用请求里出现的键序重算，应一致（等价于真实网关的校验）。
		params := make([]Param, 0, len(gotOrder))
		for _, name := range gotOrder {
			params = append(params, Param{Name: name, Value: gotQuery[name]})
		}
		if want := SignParams(params); r.Header.Get("quote-token") != want {
			t.Errorf("quote-token 不符：期望 %s，实际 %s", want, r.Header.Get("quote-token"))
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(klineSample))
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		CSRF:    "abcdef0123",
		Now:     func() time.Time { return time.UnixMilli(1780507278122) },
	})
	bars, err := client.Kline(context.Background(), KlineRequest{
		StockID:           "70000294",
		MarketType:        "2",
		MarketCode:        "70",
		InstrumentType:    "9",
		SubInstrumentType: "9008",
	})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}

	// 键序：stockId, marketType, type, marketCode, instrumentType, subInstrumentType, _
	wantOrder := []string{"stockId", "marketType", "type", "marketCode", "instrumentType", "subInstrumentType", "_"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("查询键序不符：期望 %v，实际 %v", wantOrder, gotOrder)
	}
	if gotQuery["type"] != KlineTypeDay {
		t.Fatalf("type 期望 %s，实际 %s", KlineTypeDay, gotQuery["type"])
	}
	if gotQuery["_"] != "1780507278122" {
		t.Fatalf("时间戳未按注入时钟生成：%s", gotQuery["_"])
	}
	if gotQuery["type"] != "2" || gotQuery["subInstrumentType"] != "9008" {
		t.Fatalf("参数值不符：%v", gotQuery)
	}
	// token 必须等于文档复现向量（同一组参数 + 时间戳）。
	if got := gotHeaders.Get("quote-token"); got != "45671b44c1" {
		t.Fatalf("quote-token 期望 45671b44c1，实际 %s", got)
	}
	if got := gotHeaders.Get("futu-x-csrf-token"); got != "abcdef0123" {
		t.Fatalf("csrf 头不符：%s", got)
	}
	if cookie := gotHeaders.Get("cookie"); !strings.Contains(cookie, "csrfToken=abcdef0123") ||
		!strings.Contains(cookie, "locale=zh-cn") {
		t.Fatalf("cookie 不符：%s", cookie)
	}

	if len(bars) != 1 {
		t.Fatalf("解析根数期望 1，实际 %d", len(bars))
	}
	bar := bars[0]
	if bar.Time != 157870800 || bar.Open != "187.5" || bar.Close != "178.2" ||
		bar.High != "187.5" || bar.Low != "176" {
		t.Fatalf("OHLC 解析不符：%+v", bar)
	}
	if bar.LastClose != 187.5 || bar.ChangePrice != "-9.3" || bar.Volume != 0 || bar.Turnover != 0 {
		t.Fatalf("数值字段解析不符：%+v", bar)
	}
	if bar.ChangeRatio != nil {
		t.Fatalf("r=null 时应保持 nil，实际 %v", *bar.ChangeRatio)
	}
}

// TestKlineTokenRejected 业务码非 0 应返回带 code 的错误。
func TestKlineTokenRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":-403,"message":"非法请求","data":null}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	_, err := client.Kline(context.Background(), KlineRequest{StockID: "70000294"})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if !ErrTokenRejected(err) {
		t.Fatalf("应识别为 token 被拒（-403），实际 %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != CodeTokenInvalid {
		t.Fatalf("错误类型不符：%v", err)
	}
}

// TestKlineEmptyStockID 缺少 stockId 时本地直接报错（不发请求）。
func TestKlineEmptyStockID(t *testing.T) {
	client := New(Config{BaseURL: "http://127.0.0.1:1"})
	if _, err := client.Kline(context.Background(), KlineRequest{}); err == nil {
		t.Fatal("stockId 为空应报错")
	}
}

// TestFlexNum JSON 数字与字符串混用都应解析成功。
func TestFlexNum(t *testing.T) {
	var payload klinePayload
	body := `{"code":0,"data":{"list":[{"k":1,"o":187.5,"c":"178.2","v":"12.5","lc":187.5,"cp":-9.3}]}}`
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	bars := payload.Data.Bars()
	if len(bars) != 1 || bars[0].Open != "187.5" || bars[0].Close != "178.2" ||
		bars[0].Volume != 12.5 || bars[0].ChangePrice != "-9.3" {
		t.Fatalf("混用类型解析不符：%+v", bars)
	}
}
