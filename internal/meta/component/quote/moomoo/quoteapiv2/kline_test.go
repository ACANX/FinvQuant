package quoteapiv2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// klineSample 响应样本：o/h/l/c/cp 为字符串，k/v/t/lc 为数值，r 为 null，
// 类型混用是接口本身的行为，解析需要同时兼容。
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

// TestGetKLineRequestAndParse 校验请求装配（路径、键序、token、请求头）与响应解析。
func TestGetKLineRequestAndParse(t *testing.T) {
	var (
		gotOrder   []string
		gotQuery   map[string]string
		gotHeaders http.Header
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != DefaultKLinePath {
			t.Errorf("路径期望 %s，实际 %s", DefaultKLinePath, r.URL.Path)
		}
		gotHeaders = r.Header.Clone()
		gotOrder, gotQuery = splitQuery(r.URL.RawQuery)

		// 服务端按收到的键序重算 token，等价于网关的校验方式。
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

	client := newTestClient(t, server, nil)
	bars, err := client.GetKLine(context.Background(), GetKLineRequest{
		StockID:           "70000294",
		MarketType:        "2",
		MarketCode:        "70",
		InstrumentType:    "9",
		SubInstrumentType: "9008",
	})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}

	wantOrder := []string{"stockId", "marketType", "type", "marketCode", "instrumentType", "subInstrumentType", "_"}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("查询键序不符：期望 %v，实际 %v", wantOrder, gotOrder)
	}
	if gotQuery["type"] != KLineTypeDay {
		t.Fatalf("type 期望 %s，实际 %s", KLineTypeDay, gotQuery["type"])
	}
	if gotQuery["_"] != "1780507278122" {
		t.Fatalf("时间戳未按注入时钟生成：%s", gotQuery["_"])
	}
	if gotQuery["stockId"] != "70000294" || gotQuery["subInstrumentType"] != "9008" {
		t.Fatalf("参数值不符：%v", gotQuery)
	}
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
	if ua := gotHeaders.Get("user-agent"); ua != DefaultUserAgent {
		t.Fatalf("user-agent 不符：%s", ua)
	}
	if referer := gotHeaders.Get("referer"); referer != DefaultReferer {
		t.Fatalf("referer 不符：%s", referer)
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
		t.Fatalf("r 为 null 时应保持 nil，实际 %v", *bar.ChangeRatio)
	}
}

// TestGetKLineExplicitParams 显式传入周期与时间戳时应原样使用，不被默认值覆盖。
func TestGetKLineExplicitParams(t *testing.T) {
	var gotQuery map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, gotQuery = splitQuery(r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"code":0,"data":{"list":[]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	bars, err := client.GetKLine(context.Background(), GetKLineRequest{
		StockID:         " 1234 ",
		KLineType:       "7",
		TimestampMillis: "1700000000000",
	})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if len(bars) != 0 {
		t.Fatalf("空列表应返回 0 根，实际 %d", len(bars))
	}
	if gotQuery["type"] != "7" {
		t.Fatalf("周期未透传：%s", gotQuery["type"])
	}
	if gotQuery["_"] != "1700000000000" {
		t.Fatalf("时间戳未透传：%s", gotQuery["_"])
	}
	if gotQuery["stockId"] != "1234" {
		t.Fatalf("stockId 未去空白：%q", gotQuery["stockId"])
	}
}

// TestGetKLineTokenRejected 业务码非 0 时应返回可判定的 APIError。
func TestGetKLineTokenRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":-403,"message":"非法请求","data":null}`))
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	_, err := client.GetKLine(context.Background(), GetKLineRequest{StockID: "70000294"})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if !IsTokenRejected(err) {
		t.Fatalf("应识别为 token 被拒（-403），实际 %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("错误应可断言为 *APIError，实际 %T", err)
	}
	if apiErr.Code != CodeTokenInvalid || apiErr.Message != "非法请求" {
		t.Fatalf("错误内容不符：%+v", apiErr)
	}
	if apiErr.Path != DefaultKLinePath || apiErr.URL == "" {
		t.Fatalf("错误应带上路径与地址：%+v", apiErr)
	}
	if !strings.Contains(err.Error(), "code=-403") {
		t.Fatalf("错误信息应包含业务码：%s", err.Error())
	}
}

// TestGetKLineOtherBusinessCode 其它业务码不应被判定为 token 被拒。
func TestGetKLineOtherBusinessCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":-1,"message":"系统繁忙","data":null}`))
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	_, err := client.GetKLine(context.Background(), GetKLineRequest{StockID: "123"})
	if err == nil || IsTokenRejected(err) {
		t.Fatalf("应返回普通业务错误，实际 %v", err)
	}
}

// TestGetKLineHTTPError 非 200 应返回带状态码的错误。
func TestGetKLineHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	_, err := client.GetKLine(context.Background(), GetKLineRequest{StockID: "123"})
	if err == nil {
		t.Fatal("应返回错误")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("应返回 HTTPStatus=502 的错误，实际 %v", err)
	}
	if !strings.Contains(apiErr.Body, "bad gateway") {
		t.Fatalf("错误应保留响应体：%+v", apiErr)
	}
}

// TestGetKLineParseError 响应不是合法 JSON 时应给出解析失败错误。
func TestGetKLineParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	_, err := client.GetKLine(context.Background(), GetKLineRequest{StockID: "123"})
	if err == nil || !strings.Contains(err.Error(), "响应解析失败") {
		t.Fatalf("应返回解析失败错误，实际 %v", err)
	}
	if IsTokenRejected(err) {
		t.Fatal("解析失败不应被判定为 token 被拒")
	}
}

// TestGetKLineEmptyStockID 缺少 stockId 时本地直接报错，不发请求。
func TestGetKLineEmptyStockID(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client := newTestClient(t, server, nil)
	if _, err := client.GetKLine(context.Background(), GetKLineRequest{StockID: "   "}); err == nil {
		t.Fatal("stockId 为空应报错")
	}
	if called {
		t.Fatal("stockId 为空时不应发起请求")
	}
}

// TestGetKLineContextCanceled 上下文取消后应返回错误。
func TestGetKLineContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"code":0,"data":{"list":[]}}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newTestClient(t, server, nil)
	if _, err := client.GetKLine(ctx, GetKLineRequest{StockID: "123"}); err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}
