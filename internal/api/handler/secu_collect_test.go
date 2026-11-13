package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 采集参数查询 / 维护（需求 11254）的 Handler 单测：用假 Store，不连库。

type fakeSecuFtmmCollectStore struct {
	total  int64
	list   []cdeu.SecuFtmmCollectRow
	err    error
	update bool
	// 最后一次收到的入参
	gotFilter cdeu.SecuFtmmCollectFilter
	gotPatch  cdeu.SecuFtmmCollectPatch
}

func (f *fakeSecuFtmmCollectStore) ListSecuFtmmCollect(_ context.Context, filter cdeu.SecuFtmmCollectFilter) (int64, []cdeu.SecuFtmmCollectRow, error) {
	f.gotFilter = filter
	if f.err != nil {
		return 0, nil, f.err
	}
	return f.total, f.list, nil
}

func (f *fakeSecuFtmmCollectStore) UpdateSecuFtmmCollect(_ context.Context, patch cdeu.SecuFtmmCollectPatch) (bool, error) {
	f.gotPatch = patch
	if f.err != nil {
		return false, f.err
	}
	return f.update, nil
}

func newCollectCtx(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	if body == "" {
		c.Request = httptest.NewRequest(method, target, nil)
	} else {
		c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
	}
	return c, recorder
}

func TestSecuFtmmCollectPageParsesFilter(t *testing.T) {
	store := &fakeSecuFtmmCollectStore{total: 1, list: []cdeu.SecuFtmmCollectRow{{USC: "00700"}}}
	handler := NewSecuFtmmCollect(store)

	c, recorder := newCollectCtx(http.MethodGet,
		"/x?keyword=%2000700%20&marketLabel=hk&instrument=STOCK&securityType=Stock&flagEnable=1&onlyMissing=1&orderBy=usc&order=desc&page=3&pageSize=20", "")
	handler.Page(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d（%s）", recorder.Code, recorder.Body.String())
	}
	got := store.gotFilter
	if got.Keyword != "00700" || got.MarketLabel != "hk" || got.Instrument != "STOCK" {
		t.Fatalf("筛选解析不符: %+v", got)
	}
	if got.SecurityType != "Stock" || got.FlagEnable != "1" || !got.USCMissing {
		t.Fatalf("筛选解析不符: %+v", got)
	}
	if got.OrderBy != "usc" || !got.Desc || got.Limit != 20 || got.Offset != 40 {
		t.Fatalf("分页/排序解析不符: %+v", got)
	}
}

func TestSecuFtmmCollectPageRejectsBadParams(t *testing.T) {
	for _, query := range []string{"page=0", "pageSize=0", "pageSize=501", "order=bogus"} {
		handler := NewSecuFtmmCollect(&fakeSecuFtmmCollectStore{})
		c, recorder := newCollectCtx(http.MethodGet, "/x?"+query, "")
		handler.Page(c)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%q 应 400，实际 %d", query, recorder.Code)
		}
	}
}

func TestSecuFtmmCollectPageMapsInvalidOrderBy(t *testing.T) {
	handler := NewSecuFtmmCollect(&fakeSecuFtmmCollectStore{err: cdeu.ErrSecuFtmmCollectInvalid})
	c, recorder := newCollectCtx(http.MethodGet, "/x?orderBy=security_name", "")
	handler.Page(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("入参非法应 400，实际 %d", recorder.Code)
	}
}

func TestSecuFtmmCollectUpdateSuccess(t *testing.T) {
	store := &fakeSecuFtmmCollectStore{update: true}
	handler := NewSecuFtmmCollect(store)

	body := `{"usc":"00700","fields":{"futu_symbol":"00700","market_label":"hk","flag_enable":"1","remark":"人工修正"}}`
	c, recorder := newCollectCtx(http.MethodPost, "/x", body)
	handler.Update(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d（%s）", recorder.Code, recorder.Body.String())
	}
	patch := store.gotPatch
	if patch.USC != "00700" || patch.FutuSymbol == nil || *patch.FutuSymbol != "00700" {
		t.Fatalf("patch 解析不符: %+v", patch)
	}
	if patch.MarketLabel == nil || *patch.MarketLabel != "hk" || patch.FlagEnable == nil || *patch.FlagEnable != "1" {
		t.Fatalf("patch 解析不符: %+v", patch)
	}
	// 未出现的字段必须保持 nil（不更新）
	if patch.StockID != nil || patch.MarketCode != nil || patch.ReqSection != nil {
		t.Fatalf("未提交的字段不应被置值: %+v", patch)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			USC     string `json:"usc"`
			Updated bool   `json:"updated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON：%v", err)
	}
	if resp.Code != 0 || resp.Data.USC != "00700" || !resp.Data.Updated {
		t.Fatalf("响应不符: %+v", resp)
	}
}

func TestSecuFtmmCollectUpdateRejectsUnknownField(t *testing.T) {
	handler := NewSecuFtmmCollect(&fakeSecuFtmmCollectStore{})
	c, recorder := newCollectCtx(http.MethodPost, "/x", `{"usc":"00700","fields":{"nope":1}}`)
	handler.Update(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("未定义字段应 400，实际 %d", recorder.Code)
	}
}

func TestSecuFtmmCollectUpdateErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{cdeu.ErrSecuFtmmCollectInvalid, http.StatusBadRequest},
		{cdeu.ErrSecuFtmmCollectNotFound, http.StatusNotFound},
		{cdeu.ErrSecuFtmmCollectConflict, http.StatusConflict},
		{errors.New("连接池耗尽"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		handler := NewSecuFtmmCollect(&fakeSecuFtmmCollectStore{err: tc.err})
		c, recorder := newCollectCtx(http.MethodPost, "/x", `{"usc":"00700","fields":{"remark":"x"}}`)
		handler.Update(c)
		if recorder.Code != tc.want {
			t.Fatalf("%v 应映射 %d，实际 %d", tc.err, tc.want, recorder.Code)
		}
	}
}

func TestIsTruthyQuery(t *testing.T) {
	for _, raw := range []string{"1", "true", "TRUE", " yes ", "on"} {
		if !isTruthyQuery(raw) {
			t.Fatalf("%q 应为真", raw)
		}
	}
	for _, raw := range []string{"", "0", "false", "no", "2"} {
		if isTruthyQuery(raw) {
			t.Fatalf("%q 应为假", raw)
		}
	}
}
