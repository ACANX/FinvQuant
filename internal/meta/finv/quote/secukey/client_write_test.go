package secukey

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUpsertRowsRequestShape 校验 upsert 写上游的请求形态（需求 11157）。
//
// 不碰线上 Supabase：用 httptest 假服务端断言
// ① 方法 POST、路径只放表名（schema 走 Profile 头）；
// ② 查询串带 `on_conflict=ftmm_symbol`；
// ③ `Prefer` 同时带 merge-duplicates 与 count=exact；
// ④ 请求体是行数组；⑤ 从 Content-Range 解析受影响行数。
func TestUpsertRowsRequestShape(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotQuery   string
		gotPrefer  string
		gotProfile string
		gotBody    []map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotPrefer = r.Header.Get("Prefer")
		gotProfile = r.Header.Get("Content-Profile")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Range", "*/2")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &Client{
		baseURL:      server.URL,
		apiKey:       "test-key",
		httpClient:   server.Client(),
		maxAttempts:  1,
		retryBackoff: 0,
	}
	rows := []map[string]any{
		{"ftmm_symbol": "US.TXCP", "market": "US", "code": "TXCP", "instrument": "stock",
			"usc": "US.TXCP", "priority": 10},
		{"ftmm_symbol": "HK.09999", "market": "HK", "code": "09999", "instrument": "stock",
			"usc": "HK.09999", "priority": 10},
	}
	affected, err := client.UpsertRows(context.Background(), "public.finv_quote_secu_key", "ftmm_symbol", rows)
	if err != nil {
		t.Fatalf("UpsertRows 失败: %v", err)
	}
	if affected != 2 {
		t.Fatalf("受影响行数=%d，期望 2（从 Content-Range 解析）", affected)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("方法=%s，期望 POST", gotMethod)
	}
	if gotPath != "/rest/v1/finv_quote_secu_key" {
		t.Fatalf("路径=%s，期望 /rest/v1/finv_quote_secu_key（不能把 schema 拼进路径）", gotPath)
	}
	if gotProfile != "public" {
		t.Fatalf("Content-Profile=%s，期望 public", gotProfile)
	}
	if gotQuery != "on_conflict=ftmm_symbol" {
		t.Fatalf("查询串=%s，期望 on_conflict=ftmm_symbol", gotQuery)
	}
	for _, want := range []string{"resolution=merge-duplicates", "return=minimal", "count=exact"} {
		if !contains(gotPrefer, want) {
			t.Fatalf("Prefer=%q 缺少 %s", gotPrefer, want)
		}
	}
	if len(gotBody) != 2 || gotBody[0]["ftmm_symbol"] != "US.TXCP" {
		t.Fatalf("请求体不符: %+v", gotBody)
	}

	// 空切片不发请求
	if affected, err := client.UpsertRows(context.Background(), "public.finv_quote_secu_key", "ftmm_symbol", nil); err != nil || affected != 0 {
		t.Fatalf("空切片应直接返回 0/nil，实际 %d/%v", affected, err)
	}
}

// TestUpsertRowsErrorSurfaced 上游返回 4xx 时必须把消息透出（便于定位列名/权限问题）。
func TestUpsertRowsErrorSurfaced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"null value in column \"status\" violates not-null constraint"}`))
	}))
	defer server.Close()
	client := &Client{
		baseURL: server.URL, apiKey: "k", httpClient: server.Client(),
		maxAttempts: 1, retryBackoff: 0,
	}
	_, err := client.UpsertRows(context.Background(), "public.finv_quote_secu_key", "ftmm_symbol",
		[]map[string]any{{"ftmm_symbol": "US.X"}})
	if err == nil {
		t.Fatalf("4xx 应返回错误")
	}
	if !contains(err.Error(), "not-null constraint") {
		t.Fatalf("错误消息未透出上游原因: %v", err)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
