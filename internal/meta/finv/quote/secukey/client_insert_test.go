package secukey

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

func newInsertTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient(syscfg.SupabaseConn{ProjectID: "test", APIKey: "key"})
	client.baseURL = server.URL
	return client
}

// TestInsertRowRLSPermissionError Supabase RLS 拒绝写入 → PermissionError（需求 11345 实测响应）。
//
// 实测响应：HTTP 401 + {"code":"42501","message":"new row violates row-level security policy
//
//	for table \"finv_quote_secu_ft\""}
func TestInsertRowRLSPermissionError(t *testing.T) {
	client := newInsertTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"42501","details":null,"hint":null,` +
			`"message":"new row violates row-level security policy for table \"finv_quote_secu_ft\""}`))
	})
	_, err := client.InsertRow(context.Background(), "finv_quote_secu_ft", map[string]any{"usc": "00700"})
	if err == nil {
		t.Fatal("RLS 拒绝应返回错误")
	}
	var permErr *PermissionError
	if !asPermissionError(err, &permErr) {
		t.Fatalf("应返回 PermissionError，实际 %T: %v", err, err)
	}
}

// TestInsertRowConflict 唯一冲突（409）保留原样，不当成权限问题。
func TestInsertRowConflict(t *testing.T) {
	client := newInsertTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"23505","message":"duplicate key value violates unique constraint"}`))
	})
	_, err := client.InsertRow(context.Background(), "t", map[string]any{"usc": "00700"})
	if err == nil {
		t.Fatal("409 应返回错误")
	}
	var permErr *PermissionError
	if asPermissionError(err, &permErr) {
		t.Error("409 不应被当作权限错误")
	}
}

// TestInsertRowReturnsRepresentation 成功时返回插入后的行，且请求头带 return=representation。
func TestInsertRowReturnsRepresentation(t *testing.T) {
	var prefer string
	client := newInsertTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		prefer = r.Header.Get("Prefer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`[{"usc":"00700","flag_enable":0}]`))
	})
	raw, err := client.InsertRow(context.Background(), "t", map[string]any{"usc": "00700"})
	if err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	if prefer != "return=representation" {
		t.Errorf("Prefer 头应为 return=representation，实际 %q", prefer)
	}
	if string(raw) != `{"usc":"00700","flag_enable":0}` {
		t.Errorf("应返回第一行，实际 %s", raw)
	}
}

// asPermissionError 沿 Unwrap 链找 PermissionError。
func asPermissionError(err error, target **PermissionError) bool {
	for err != nil {
		if typed, ok := err.(*PermissionError); ok {
			*target = typed
			return true
		}
		type unwrapper interface{ Unwrap() error }
		next, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = next.Unwrap()
	}
	return false
}
