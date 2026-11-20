package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

// TestPutObjectRequest 用本地 httptest 模拟 S3，校验 path-style 与对象 key。
func TestPutObjectRequest(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("ETag", `"etag-1"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := New(oss.Config{
		Provider: "s3", Endpoint: server.URL, Bucket: "bucket", Region: "auto",
		AccessKey: "ak", SecretKey: "sk", Prefix: "brs",
	})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	if err := store.Put(context.Background(), "20261029/a.dump", strings.NewReader("data"), 4, "application/octet-stream"); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("方法期望 PUT，实际 %s", gotMethod)
	}
	if gotPath != "/bucket/brs/20261029/a.dump" {
		t.Fatalf("path 期望 /bucket/brs/20261029/a.dump，实际 %s", gotPath)
	}
}

// TestNewRequiresBucket 校验 bucket 必填。
func TestNewRequiresBucket(t *testing.T) {
	if _, err := New(oss.Config{Provider: "s3"}); err == nil {
		t.Fatal("缺少 bucket 应报错")
	}
}
