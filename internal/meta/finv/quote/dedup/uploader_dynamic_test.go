package dedup

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// TestDynamicUploaderModeFollowsConfig 断言上传模式完全由 sys_config 20002 决定。
func TestDynamicUploaderModeFollowsConfig(t *testing.T) {
	store := &fakeConfigStore{values: syscfg.Values{}}
	router := newDynamicUploader(store, noneUploader{}, "")

	if got := router.Mode(); got != UploadModeNone {
		t.Fatalf("20002 关闭时应回退兜底模式 none，实际 %q", got)
	}

	// 运维在线把 20002 改为 '1'：无需重启，下一次判定即生效。
	store.set(githubEnabledValues(t, "ghp_token", "o:r:main:Archive"))
	if got := router.Mode(); got != UploadModeGitHub {
		t.Fatalf("20002 启用后应切到 github，实际 %q", got)
	}

	// 再关回 '0'。
	store.set(syscfg.Values{})
	if got := router.Mode(); got != UploadModeNone {
		t.Fatalf("20002 关闭后应回退 none，实际 %q", got)
	}
}

// TestDynamicUploaderModeFallbackOnConfigError 读配置失败时回退兜底模式并在日志说明。
func TestDynamicUploaderModeFallbackOnConfigError(t *testing.T) {
	store := &fakeConfigStore{err: errors.New("db down")}
	router := newDynamicUploader(store, noneUploader{}, "")
	if got := router.Mode(); got != UploadModeNone {
		t.Fatalf("配置读取失败时应回退兜底模式，实际 %q", got)
	}
}

// TestDynamicUploaderPutUsesFallbackWhenDisabled 开关关闭时沿用既有实现（none 报未启用）。
func TestDynamicUploaderPutUsesFallbackWhenDisabled(t *testing.T) {
	store := &fakeConfigStore{values: syscfg.Values{}}
	router := newDynamicUploader(store, noneUploader{}, "")

	if _, err := router.Put(context.Background(), "/tmp/not-used", "a.mvsv"); !errors.Is(err, ErrUploadDisabled) {
		t.Fatalf("开关关闭时应走兜底实现并返回 ErrUploadDisabled，实际 %v", err)
	}
}

// TestDynamicUploaderPutRoutesToGitHub 开关启用时把上传路由到 GitHub Contents API。
func TestDynamicUploaderPutRoutesToGitHub(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive/Monthly")}
	repo := newFakeGitHubRepo()
	server := httptest.NewServer(repo.handler())
	t.Cleanup(server.Close)

	router := newDynamicUploader(store, noneUploader{}, server.URL)
	content := []byte("routed-content\n")
	localPath := writeTempFile(t, "routed.mvsv", content)

	size, err := router.Put(context.Background(), localPath, "kline/routed.mvsv")
	if err != nil {
		t.Fatalf("路由到 GitHub 上传失败: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("返回字节数不符: %d", size)
	}
	puts := repo.puts()
	if len(puts) != 1 {
		t.Fatalf("应向 GitHub 发送 1 次 PUT，实际 %d 次", len(puts))
	}
	if puts[0].Path != "Archive/Monthly/kline/routed.mvsv" {
		t.Fatalf("远端路径应为 pre_path_key + 相对路径，实际 %q", puts[0].Path)
	}
	if err := router.Verify(context.Background(), "kline/routed.mvsv", size, ""); err != nil {
		t.Fatalf("Verify 应通过: %v", err)
	}
}

// TestDynamicUploaderStatRoutesToGitHub 查询也走同一路由（避免消息与实现不一致）。
func TestDynamicUploaderStatRoutesToGitHub(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	repo := newFakeGitHubRepo()
	repo.seed("Archive/known.mvsv", []byte("abc\n"))
	server := httptest.NewServer(repo.handler())
	t.Cleanup(server.Close)

	router := newDynamicUploader(store, noneUploader{}, server.URL)
	exists, size, checksum, err := router.Stat(context.Background(), "known.mvsv")
	if err != nil {
		t.Fatalf("Stat 失败: %v", err)
	}
	if !exists || size != 4 || checksum == "" {
		t.Fatalf("Stat 结果不符: exists=%v size=%d checksum=%q", exists, size, checksum)
	}
}
