package local

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := New(oss.Config{Provider: "local", Endpoint: t.TempDir(), Prefix: "brs"})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	content := []byte("hello-object-store")
	if err := store.Put(ctx, "20261029/a.dump", bytes.NewReader(content), int64(len(content)), "application/octet-stream"); err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	reader, err := store.Get(ctx, "20261029/a.dump")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	got, _ := io.ReadAll(reader)
	_ = reader.Close()
	if string(got) != string(content) {
		t.Fatalf("内容不一致: %q", got)
	}
	info, err := store.Stat(ctx, "20261029/a.dump")
	if err != nil {
		t.Fatalf("Stat 失败: %v", err)
	}
	if info.Size != int64(len(content)) || len(info.ETag) != 64 {
		t.Fatalf("元数据异常: %+v", info)
	}
	if err := store.Delete(ctx, "20261029/a.dump"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := store.Stat(ctx, "20261029/a.dump"); err == nil {
		t.Fatal("删除后 Stat 应报错")
	}
}

func TestRejectTraversal(t *testing.T) {
	store, err := New(oss.Config{Provider: "local", Endpoint: t.TempDir()})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	if err := store.Put(context.Background(), "../escape", strings.NewReader("x"), 1, ""); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}

func TestFactoryRegistered(t *testing.T) {
	found := false
	for _, provider := range oss.Providers() {
		if provider == "local" {
			found = true
		}
	}
	if !found {
		t.Fatalf("local 适配器未注册: %v", oss.Providers())
	}
}
