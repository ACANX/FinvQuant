package cdeu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件覆盖「期货列表」自建 HTTP 拉取与宽松解析（需求 11274 修正）。
//
// 关键回归点：线上响应 `stockId` 是**字符串**，上游 FuturesListItem.StockID 是 int64，
// 直接用上游解析会报 cannot unmarshal string into ... int64（2026-11-18 的线上故障）。

// futureListRawSample 线上真实形状的样本：stockId 是字符串、pagination 是数字、
// fields 的值混合数字与字符串。
const futureListRawSample = `{
    "code": 0,
    "message": "成功",
    "data": {
        "pagination": {"page": 0, "pageCount": 1, "pageSize": 40, "total": 14},
        "list": [
            {
                "stockId": "70000294",
                "priceDirect": "down",
                "fieldList": [
                    {"fieldId": 1, "fieldName": "合约代码", "text": "GCmain", "contentType": 4},
                    {"fieldId": 3, "fieldName": "价格", "value": "4413800000000", "precision": 1, "contentType": 2}
                ],
                "fields": {"1": "GCmain", "2": "黄金", "3": 4413.8, "4": "-0.25%", "19": "US"}
            }
        ]
    }
}`

// newFetcherForServer 用 httptest 服务器构造 fetcher（BaseURL 指向本地）。
func newFetcherForServer(t *testing.T, handler http.HandlerFunc) (*FutureListHTTPFetcher, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	// 让 referer 选择可预测：配置里显式给默认来源页，避免依赖上游表。
	fetcher := newFutureListHTTPFetcher(quoteapiv2.Config{BaseURL: server.URL})
	return fetcher, server
}

// TestFutureListHTTPFetcherParsesStringStockID 字符串 stockId 能解析，且签名/请求头齐备。
func TestFutureListHTTPFetcherParsesStringStockID(t *testing.T) {
	var gotQuery, gotToken, gotReferer, gotCookie string
	fetcher, server := newFetcherForServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != quoteapiv2.DefaultFuturesListPath {
			t.Errorf("路径期望 %s，实际 %s", quoteapiv2.DefaultFuturesListPath, r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("quote-token")
		gotReferer = r.Header.Get("referer")
		gotCookie = r.Header.Get("cookie")
		_, _ = w.Write([]byte(futureListRawSample))
	})
	defer server.Close()

	page, err := fetcher.GetFuturesList(context.Background(), quoteapiv2.GetFuturesListRequest{
		MarketType:      "0",
		FuturesType:     "5",
		Page:            "0",
		PageSize:        "40",
		TimestampMillis: "1794933913088",
	})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}

	// 参数键序：marketType / futuresType / page / pageSize / _（签名输入顺序）
	if !strings.HasPrefix(gotQuery, "marketType=0&futuresType=5&page=0&pageSize=40&_=1794933913088") {
		t.Fatalf("查询串不符: %s", gotQuery)
	}
	// 签名必须存在且是 10 位十六进制（上游 SignParams 的输出形态）
	if len(gotToken) != 10 {
		t.Fatalf("quote-token 形态不符: %q", gotToken)
	}
	if !strings.Contains(gotCookie, "csrfToken=") || !strings.Contains(gotCookie, "locale=") {
		t.Fatalf("cookie 缺 csrfToken/locale: %q", gotCookie)
	}
	if !strings.Contains(gotReferer, "moomoo.com") {
		t.Fatalf("referer 应为 moomoo 站点页: %q", gotReferer)
	}

	if page.Pagination.Total != 14 || page.Pagination.PageCount != 1 || page.Pagination.PageSize != 40 {
		t.Fatalf("分页解析不符: %+v", page.Pagination)
	}
	if len(page.Items) != 1 {
		t.Fatalf("条目数 = %d，期望 1", len(page.Items))
	}
	item := page.Items[0]
	if item.StockID != 70000294 {
		t.Fatalf("字符串 stockId 应解析为 70000294，实际 %d", item.StockID)
	}
	if item.PriceDirect != "down" {
		t.Fatalf("priceDirect 不符: %q", item.PriceDirect)
	}
	if item.Fields["1"] != "GCmain" || item.Fields["19"] != "US" {
		t.Fatalf("fields 文本值不符: %+v", item.Fields)
	}
	// fields 里的数字值要归一为文本（"3": 4413.8 → "4413.8"）
	if item.Fields["3"] != "4413.8" {
		t.Fatalf("fields 数字值应转文本 4413.8，实际 %q", item.Fields["3"])
	}
	if len(item.FieldList) != 2 || item.FieldList[0].FieldID != 1 || item.FieldList[1].Precision != 1 {
		t.Fatalf("fieldList 解析不符: %+v", item.FieldList)
	}
}

// TestFutureListHTTPFetcherNumericStockID 数字 stockId 同样可解析（环境差异兼容）。
func TestFutureListHTTPFetcherNumericStockID(t *testing.T) {
	body := strings.Replace(futureListRawSample, `"stockId": "70000294"`, `"stockId": 70000294`, 1)
	fetcher, server := newFetcherForServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	defer server.Close()

	page, err := fetcher.GetFuturesList(context.Background(), quoteapiv2.GetFuturesListRequest{
		MarketType: "0", FuturesType: "5", Page: "0", PageSize: "40",
	})
	if err != nil {
		t.Fatalf("拉取失败: %v", err)
	}
	if page.Items[0].StockID != 70000294 {
		t.Fatalf("数字 stockId 解析失败: %+v", page.Items[0])
	}
}

// TestFutureListHTTPFetcherBusinessError 业务码非 0 时返回 APIError（带 code 与 message）。
func TestFutureListHTTPFetcherBusinessError(t *testing.T) {
	fetcher, server := newFetcherForServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":500,"message":"Params Error","data":null}`))
	})
	defer server.Close()

	_, err := fetcher.GetFuturesList(context.Background(), quoteapiv2.GetFuturesListRequest{
		MarketType: "0", FuturesType: "9", Page: "0", PageSize: "40",
	})
	if err == nil {
		t.Fatal("业务码 500 时应返回错误")
	}
	if !quoteapiv2.IsParamsError(err) {
		t.Fatalf("应识别为参数错误，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "Params Error") {
		t.Fatalf("错误信息应带服务端 message，实际 %v", err)
	}
}

// TestFutureListHTTPFetcherHTTPError 非 200 返回 APIError（带 HTTP 状态）。
func TestFutureListHTTPFetcherHTTPError(t *testing.T) {
	fetcher, server := newFetcherForServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})
	defer server.Close()

	_, err := fetcher.GetFuturesList(context.Background(), quoteapiv2.GetFuturesListRequest{
		MarketType: "0", FuturesType: "5", Page: "0", PageSize: "40",
	})
	if err == nil {
		t.Fatal("HTTP 403 时应返回错误")
	}
	apiErr, ok := err.(*quoteapiv2.APIError)
	if !ok || apiErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("应返回 HTTPStatus=403 的 APIError，实际 %#v", err)
	}
}

// TestFutureListHTTPFetcherRejectsBadParams 必填参数与页码校验在本地就拦下（不发请求）。
func TestFutureListHTTPFetcherRejectsBadParams(t *testing.T) {
	fetcher := newFutureListHTTPFetcher(quoteapiv2.Config{})
	cases := []quoteapiv2.GetFuturesListRequest{
		{FuturesType: "5"},
		{MarketType: "0"},
		{MarketType: "0", FuturesType: "5", Page: "-1"},
		{MarketType: "0", FuturesType: "5", PageSize: "0"},
	}
	for _, req := range cases {
		if _, err := fetcher.GetFuturesList(context.Background(), req); err == nil {
			t.Fatalf("非法参数应被拒绝: %+v", req)
		}
	}
}

// TestFutureFlexText 宽松文本：字符串/数字/布尔/null 都归一为文本。
func TestFutureFlexText(t *testing.T) {
	cases := []struct{ raw, want string }{
		{`"70000294"`, "70000294"},
		{`70000294`, "70000294"},
		{`4413.80`, "4413.80"},
		{`true`, "true"},
		{`null`, ""},
		{`""`, ""},
	}
	for _, c := range cases {
		var value futureFlexText
		if err := value.UnmarshalJSON([]byte(c.raw)); err != nil {
			t.Fatalf("raw=%s 解析失败: %v", c.raw, err)
		}
		if string(value) != c.want {
			t.Fatalf("raw=%s 期望 %q，实际 %q", c.raw, c.want, string(value))
		}
	}
}
