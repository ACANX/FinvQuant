package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/indicator"
)

// 证券指标只读查询（需求 11249/11250）的 Handler 单测：用假 Reader，不连库。

// fakeIndicatorReader 记录收到的 filter，并按配置返回结果或错误。
type fakeIndicatorReader struct {
	columns []indicator.IndicatorColumn
	total   int64
	list    []map[string]string
	err     error
	// got 最后一次收到的分页条件（用于断言参数解析结果）
	got indicator.IndicatorPageFilter
}

func (f *fakeIndicatorReader) Columns(context.Context) ([]indicator.IndicatorColumn, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.columns, nil
}

func (f *fakeIndicatorReader) Page(_ context.Context, filter indicator.IndicatorPageFilter) (int64, []map[string]string, error) {
	f.got = filter
	if f.err != nil {
		return 0, nil, f.err
	}
	return f.total, f.list, nil
}

// decodeIndicatorEnvelope 解析响应信封里的 data 节点。
func decodeIndicatorEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON：%v（%s）", err, recorder.Body.String())
	}
	if body.Code != 0 {
		t.Fatalf("code = %d，期望 0（%s）", body.Code, recorder.Body.String())
	}
	return body.Data
}

func TestSecuIndicatorColumnsRoute(t *testing.T) {
	reader := &fakeIndicatorReader{columns: []indicator.IndicatorColumn{
		{Name: "usc", Title: "证券统一编码", DataType: "text", Sortable: true},
		{Name: "pe_ttm", Title: "市盈率 TTM", DataType: "numeric", Sortable: true},
	}}
	handler := NewSecuIndicator(reader)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	handler.Columns(c)

	data := decodeIndicatorEnvelope(t, recorder)
	if data["table"] != indicator.IndicatorTable {
		t.Fatalf("table = %v，期望 %s", data["table"], indicator.IndicatorTable)
	}
	if total, _ := data["total"].(float64); int(total) != 2 {
		t.Fatalf("total = %v，期望 2", data["total"])
	}
}

func TestSecuIndicatorPageParsesFilter(t *testing.T) {
	reader := &fakeIndicatorReader{total: 1, list: []map[string]string{
		{"usc": "00700", "ftmm_symbol": "HK.00700", "pe_ttm": "19.5000000000"},
	}}
	handler := NewSecuIndicator(reader)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/x?keyword=00700&market=hk&columns=pe_ttm,,name,pe_ttm&orderBy=pe_ttm&order=asc&page=3&pageSize=20", nil)
	handler.Page(c)

	decodeIndicatorEnvelope(t, recorder)
	got := reader.got
	if got.Keyword != "00700" || got.Market != "hk" {
		t.Fatalf("筛选项解析不符: %+v", got)
	}
	// 列拆分：去空、去重、保持请求顺序（身份列由 Reader 侧补齐，Handler 不重复处理）
	if len(got.Columns) != 2 || got.Columns[0] != "pe_ttm" || got.Columns[1] != "name" {
		t.Fatalf("columns 解析不符: %v", got.Columns)
	}
	if got.OrderBy != "pe_ttm" || got.Desc {
		t.Fatalf("排序解析不符: %+v", got)
	}
	if got.Limit != 20 || got.Offset != 40 {
		t.Fatalf("分页解析不符: limit=%d offset=%d", got.Limit, got.Offset)
	}
}

func TestSecuIndicatorPageRejectsBadParams(t *testing.T) {
	cases := []string{
		"page=0",
		"pageSize=0",
		"pageSize=" + "501",
		"page=abc",
		"order=bogus",
	}
	for _, query := range cases {
		reader := &fakeIndicatorReader{}
		handler := NewSecuIndicator(reader)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/x?"+query, nil)
		handler.Page(c)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%q 应返回 400，实际 %d", query, recorder.Code)
		}
	}
}

func TestSecuIndicatorPageMapsInvalidFilterTo400(t *testing.T) {
	reader := &fakeIndicatorReader{err: indicator.ErrQueryInvalid}
	handler := NewSecuIndicator(reader)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?orderBy=drop_table", nil)
	handler.Page(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("ErrQueryInvalid 应映射 400，实际 %d（%s）", recorder.Code, recorder.Body.String())
	}
}

func TestSecuIndicatorPageMapsOtherErrorsTo500(t *testing.T) {
	reader := &fakeIndicatorReader{err: errors.New("连接池耗尽")}
	handler := NewSecuIndicator(reader)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	handler.Page(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("其它错误应 500，实际 %d", recorder.Code)
	}
}

func TestSplitIndicatorColumns(t *testing.T) {
	if got := splitIndicatorColumns("  "); got != nil {
		t.Fatalf("空串应返回 nil，实际 %v", got)
	}
	got := splitIndicatorColumns("a, b ,,a,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("拆分结果不符: %v", got)
	}
}
