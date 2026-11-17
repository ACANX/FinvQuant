package static

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerProxiesAPIRequests(t *testing.T) {
	// 后端模拟 API 服务
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","path":"` + r.URL.Path + `"}`))
	}))
	defer backend.Close()

	handler := Handler(backend.URL)
	req := httptest.NewRequest(http.MethodPost, "/API/V1/Meta/Finv/Quant/Quote/Import/Upload", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码=%d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/API/V1/Meta/Finv/Quant/Quote/Import/Upload") {
		t.Fatalf("代理未保留原始路径: %s", rec.Body.String())
	}
}

func TestHandlerServesIndexForSPA(t *testing.T) {
	handler := Handler("http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// SPA fallback 返回 index.html（200，HTML 内容）
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码=%d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") && !strings.Contains(rec.Body.String(), "<div id=\"app\">") {
		t.Fatalf("SPA fallback 未返回 index.html: %s", rec.Body.String()[:200])
	}
}
