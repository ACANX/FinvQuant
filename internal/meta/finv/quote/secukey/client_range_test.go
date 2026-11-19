package secukey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// TestFetchQueryOffsetBeyondTotal 上游 offset 越界（HTTP 416 / PGRST103）按空页处理（需求 11344）。
//
// PostgREST 实测响应：`416` + `Content-Range: *\/0` +
// `{"code":"PGRST103","details":"An offset of 15 was requested, but there are only 0 rows."}`。
// 若不特判，翻到越界页会整页报 502。
func TestFetchQueryOffsetBeyondTotal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "*/0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		_, _ = w.Write([]byte(`{"code":"PGRST103","details":"An offset of 15 was requested, but there are only 0 rows.","message":"Requested range not satisfiable"}`))
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "test", APIKey: "key"})
	client.baseURL = server.URL
	client.maxAttempts = 1

	rows, total, err := client.FetchQuery(context.Background(), "finv_quote_secu_ft", url.Values{"limit": {"15"}, "offset": {"15"}})
	if err != nil {
		t.Fatalf("416 应视为空页而不是错误: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("应返回空页，实际 %d 行", len(rows))
	}
	if total != 0 {
		t.Errorf("总数应取 Content-Range 尾段 0，实际 %d", total)
	}
}

// TestFetchQueryOffsetBeyondTotalKeepsTotal 越界页仍保留真实总数（Content-Range 尾段）。
func TestFetchQueryOffsetBeyondTotalKeepsTotal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "*/7")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		_, _ = w.Write([]byte(`{"code":"PGRST103","message":"Requested range not satisfiable"}`))
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "test", APIKey: "key"})
	client.baseURL = server.URL

	_, total, err := client.FetchQuery(context.Background(), "t", url.Values{})
	if err != nil {
		t.Fatalf("416 应视为空页: %v", err)
	}
	if total != 7 {
		t.Errorf("总数应为 7，实际 %d", total)
	}
}
