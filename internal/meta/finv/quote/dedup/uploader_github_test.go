package dedup

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// ---------------------------------------------------------------------
// 测试替身
// ---------------------------------------------------------------------

// fakeConfigStore 模拟 syscfg.Store 的读取行为（可在线改值，模拟运维改配置）。
type fakeConfigStore struct {
	mu     sync.Mutex
	values syscfg.Values
	err    error
	loads  int
}

func (f *fakeConfigStore) Load(context.Context) (syscfg.Values, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	return f.values, f.err
}

func (f *fakeConfigStore) set(values syscfg.Values) {
	f.mu.Lock()
	f.values = values
	f.mu.Unlock()
}

// githubEnabledValues 构造「开关打开 + PAT + 仓库配置」的配置快照。
func githubEnabledValues(t *testing.T, token, spec string) syscfg.Values {
	t.Helper()
	repo, err := syscfg.ParseRepoSpec(spec)
	if err != nil {
		t.Fatalf("准备仓库配置失败: %v", err)
	}
	return syscfg.Values{ArchiveUploadEnabled: true, ArchiveToken: token, ArchiveRepo: repo}
}

// recordedPut 记录一次 PUT 请求的内容，便于断言请求体与请求头。
type recordedPut struct {
	Path       string
	Token      string
	Branch     string
	SHA        string
	Message    string
	Content    []byte
	HasNewline bool
}

// fakeGitHubRepo 内存版 GitHub Contents API（owner=o、repo=r）。
type fakeGitHubRepo struct {
	mu           sync.Mutex
	files        map[string][]byte
	putLog       []recordedPut
	getCount     int
	failPutsLeft int // >0 时 PUT 返回 500（用于验证重试）
}

func newFakeGitHubRepo() *fakeGitHubRepo {
	return &fakeGitHubRepo{files: make(map[string][]byte)}
}

func (f *fakeGitHubRepo) seed(path string, content []byte) {
	f.mu.Lock()
	f.files[path] = append([]byte(nil), content...)
	f.mu.Unlock()
}

func (f *fakeGitHubRepo) file(path string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, exists := f.files[path]
	return content, exists
}

func (f *fakeGitHubRepo) puts() []recordedPut {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedPut(nil), f.putLog...)
}

func (f *fakeGitHubRepo) gets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCount
}

func (f *fakeGitHubRepo) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/repos/o/r/contents/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, prefix)

		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			f.getCount++
			content, exists := f.files[path]
			f.mu.Unlock()
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"message":"Not Found"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type": "file",
				"size": len(content),
				"sha":  testGitBlobSHA(content),
				"path": path,
			})

		case http.MethodPut:
			var payload struct {
				Message string `json:"message"`
				Content string `json:"content"`
				Branch  string `json:"branch"`
				SHA     string `json:"sha"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(w, `{"message":"bad json"}`)
				return
			}
			decoded, err := base64.StdEncoding.DecodeString(payload.Content)
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(w, `{"message":"bad content"}`)
				return
			}

			f.mu.Lock()
			f.putLog = append(f.putLog, recordedPut{
				Path:       path,
				Token:      r.Header.Get("Authorization"),
				Branch:     payload.Branch,
				SHA:        payload.SHA,
				Message:    payload.Message,
				Content:    decoded,
				HasNewline: strings.Contains(payload.Content, "\n"),
			})
			if f.failPutsLeft > 0 {
				f.failPutsLeft--
				f.mu.Unlock()
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, `{"message":"server error"}`)
				return
			}
			f.files[path] = decoded
			f.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"commit":{"sha":"deadbeef"}}`)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

// testGitBlobSHA 计算 git blob 对象 sha（与远端比对口径一致）。
func testGitBlobSHA(content []byte) string {
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d\x00", len(content))
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// writeTempFile 写入临时归档文件并返回路径与内容。
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	return path
}

// newTestUploader 组装「fake 配置 + fake GitHub API」的上传器。
func newTestUploader(t *testing.T, store *fakeConfigStore) (*githubUploader, *fakeGitHubRepo) {
	t.Helper()
	repo := newFakeGitHubRepo()
	server := httptest.NewServer(repo.handler())
	t.Cleanup(server.Close)
	return newGitHubUploader(store, server.URL), repo
}

// ---------------------------------------------------------------------
// 用例
// ---------------------------------------------------------------------

// TestGitHubPutCreatesFile 远端不存在时新建文件，且请求体符合 Contents API 约定。
func TestGitHubPutCreatesFile(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive/Monthly")}
	uploader, repo := newTestUploader(t, store)
	content := []byte("# MVsv\n1|2|3\n")
	localPath := writeTempFile(t, "202610.mvsv", content)

	size, err := uploader.Put(context.Background(), localPath, "kline/202610.mvsv")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("返回字节数不符: %d != %d", size, len(content))
	}

	puts := repo.puts()
	if len(puts) != 1 {
		t.Fatalf("应发送 1 次 PUT，实际 %d 次", len(puts))
	}
	put := puts[0]
	if put.Path != "Archive/Monthly/kline/202610.mvsv" {
		t.Fatalf("目标路径应为 pre_path_key + 相对路径，实际 %q", put.Path)
	}
	if put.Token != "Bearer ghp_token" {
		t.Fatalf("Authorization 头不符: %q", put.Token)
	}
	if put.Branch != "main" {
		t.Fatalf("分支不符: %q", put.Branch)
	}
	if put.SHA != "" {
		t.Fatalf("新建文件不应携带 sha，实际 %q", put.SHA)
	}
	if put.HasNewline {
		t.Fatal("base64 内容不应包含换行（否则 Content-Length 会算错）")
	}
	if string(put.Content) != string(content) {
		t.Fatalf("远端内容与本地不一致: %q", string(put.Content))
	}
	if !strings.Contains(put.Message, "archive: upload") {
		t.Fatalf("提交说明不符: %q", put.Message)
	}
	if stored, ok := repo.file("Archive/Monthly/kline/202610.mvsv"); !ok || string(stored) != string(content) {
		t.Fatal("远端文件未按预期写入")
	}
}

// TestGitHubPutSkipsIdenticalContent 远端已存在相同内容时不再提交（幂等重传）。
func TestGitHubPutSkipsIdenticalContent(t *testing.T) {
	content := []byte("hello-archive\n")
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	repo.seed("Archive/a.mvsv", content)
	localPath := writeTempFile(t, "a.mvsv", content)

	size, err := uploader.Put(context.Background(), localPath, "a.mvsv")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("返回字节数不符: %d", size)
	}
	if puts := repo.puts(); len(puts) != 0 {
		t.Fatalf("内容一致时不应重复提交，实际 PUT %d 次", len(puts))
	}
}

// TestGitHubPutUpdatesWithRemoteSHA 远端内容不同时带 sha 覆盖更新。
func TestGitHubPutUpdatesWithRemoteSHA(t *testing.T) {
	oldContent := []byte("old\n")
	newContent := []byte("new-content\n")
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	repo.seed("Archive/a.mvsv", oldContent)
	localPath := writeTempFile(t, "a.mvsv", newContent)

	if _, err := uploader.Put(context.Background(), localPath, "a.mvsv"); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	puts := repo.puts()
	if len(puts) != 1 {
		t.Fatalf("应发送 1 次 PUT，实际 %d 次", len(puts))
	}
	if puts[0].SHA != testGitBlobSHA(oldContent) {
		t.Fatalf("覆盖更新应携带远端 sha，实际 %q 期望 %q", puts[0].SHA, testGitBlobSHA(oldContent))
	}
	if stored, _ := repo.file("Archive/a.mvsv"); string(stored) != string(newContent) {
		t.Fatalf("远端内容未更新: %q", string(stored))
	}
}

// TestGitHubStat 远端不存在返回 exists=false；存在返回大小与 git sha。
func TestGitHubStat(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	ctx := context.Background()

	exists, size, checksum, err := uploader.Stat(ctx, "missing.mvsv")
	if err != nil {
		t.Fatalf("Stat 失败: %v", err)
	}
	if exists || size != 0 || checksum != "" {
		t.Fatalf("不存在时应返回零值，实际 exists=%v size=%d checksum=%q", exists, size, checksum)
	}

	content := []byte("abc\n")
	repo.seed("Archive/present.mvsv", content)
	exists, size, checksum, err = uploader.Stat(ctx, "present.mvsv")
	if err != nil {
		t.Fatalf("Stat 失败: %v", err)
	}
	if !exists || size != int64(len(content)) {
		t.Fatalf("存在时应返回大小，实际 exists=%v size=%d", exists, size)
	}
	if checksum != testGitBlobSHA(content) {
		t.Fatalf("checksum 应为 git blob sha，实际 %q", checksum)
	}
}

// TestGitHubVerify 校验大小与内容 sha。
func TestGitHubVerify(t *testing.T) {
	content := []byte("verify-me\n")
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	repo.seed("Archive/v.mvsv", content)
	localPath := writeTempFile(t, "v.mvsv", content)

	ctx := context.Background()
	if _, err := uploader.Put(ctx, localPath, "v.mvsv"); err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if err := uploader.Verify(ctx, "v.mvsv", int64(len(content)), strings.Repeat("0", 64)); err != nil {
		t.Fatalf("Verify 应通过（内容 sha 一致），实际 %v", err)
	}
	if err := uploader.Verify(ctx, "v.mvsv", int64(len(content))+1, ""); err == nil {
		t.Fatal("大小不一致时 Verify 应失败")
	}
	if err := uploader.Verify(ctx, "absent.mvsv", int64(len(content)), ""); err == nil {
		t.Fatal("远端不存在时 Verify 应失败")
	}

	// 远端内容被替换后，即使大小相同也应校验失败。
	repo.seed("Archive/v.mvsv", []byte("verify-XX\n"))
	if err := uploader.Verify(ctx, "v.mvsv", int64(len(content)), ""); err == nil {
		t.Fatal("远端内容 sha 不一致时 Verify 应失败")
	}
}

// TestGitHubPutRejectsOversizeFile 超出 Contents API 单文件上限时明确报错。
func TestGitHubPutRejectsOversizeFile(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	uploader.maxBytes = 4
	localPath := writeTempFile(t, "big.mvsv", []byte("12345"))

	_, err := uploader.Put(context.Background(), localPath, "big.mvsv")
	if err == nil || !strings.Contains(err.Error(), "上限") {
		t.Fatalf("应返回超出上限错误，实际 %v", err)
	}
	if puts := repo.puts(); len(puts) != 0 {
		t.Fatalf("超限文件不应发起请求，实际 PUT %d 次", len(puts))
	}
}

// TestGitHubPutRequiresCredential 配置缺失时给出可读错误（而不是静默跳过）。
func TestGitHubPutRequiresCredential(t *testing.T) {
	localPath := writeTempFile(t, "a.mvsv", []byte("x\n"))

	noToken := &fakeConfigStore{values: syscfg.Values{
		ArchiveUploadEnabled: true,
		ArchiveRepo:          syscfg.RepoSpec{Owner: "o", Repo: "r", Branch: "main"},
	}}
	uploader, _ := newTestUploader(t, noToken)
	if _, err := uploader.Put(context.Background(), localPath, "a.mvsv"); err == nil ||
		!strings.Contains(err.Error(), "20003") {
		t.Fatalf("缺 PAT 应返回 20003 相关错误，实际 %v", err)
	}

	off := &fakeConfigStore{values: syscfg.Values{}}
	uploader, _ = newTestUploader(t, off)
	if _, err := uploader.Put(context.Background(), localPath, "a.mvsv"); err == nil ||
		!strings.Contains(err.Error(), "20002") {
		t.Fatalf("开关关闭应返回 20002 相关错误，实际 %v", err)
	}
}

// TestGitHubPutRetriesServerError 5xx 时按次数重试，最终成功。
func TestGitHubPutRetriesServerError(t *testing.T) {
	content := []byte("retry\n")
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	uploader, repo := newTestUploader(t, store)
	repo.failPutsLeft = 1
	localPath := writeTempFile(t, "r.mvsv", content)

	if _, err := uploader.Put(context.Background(), localPath, "r.mvsv"); err != nil {
		t.Fatalf("重试后应成功，实际 %v", err)
	}
	if puts := repo.puts(); len(puts) != 2 {
		t.Fatalf("应重试 1 次（共 2 次 PUT），实际 %d 次", len(puts))
	}
}

// TestGitHubTargetIsDirectory 目标路径是目录时报错而不是覆盖。
func TestGitHubTargetIsDirectory(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive")}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"type":"file","name":"a.mvsv"}]`)
	}))
	t.Cleanup(server.Close)
	uploader := newGitHubUploader(store, server.URL)

	if _, _, _, err := uploader.Stat(context.Background(), "dir"); err == nil ||
		!strings.Contains(err.Error(), "目录") {
		t.Fatalf("应提示目标路径是目录，实际 %v", err)
	}
}

// TestBuildContentsBody 请求体长度精确、内容可还原、base64 无换行。
func TestBuildContentsBody(t *testing.T) {
	content := []byte("0123456789abcdef") // 16 字节，base64 后 24 字符（含填充）
	localPath := writeTempFile(t, "b.mvsv", content)

	reader, length, err := buildContentsBody("msg", "main", "", localPath, int64(len(content)))
	if err != nil {
		t.Fatalf("构造请求体失败: %v", err)
	}
	defer func() { _ = reader.Close() }()

	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("读取请求体失败: %v", err)
	}
	if int64(len(payload)) != length {
		t.Fatalf("声明长度与实际不一致：%d != %d", length, len(payload))
	}

	var decoded struct {
		Message string `json:"message"`
		Content string `json:"content"`
		Branch  string `json:"branch"`
		SHA     string `json:"sha"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v", err)
	}
	if decoded.Message != "msg" || decoded.Branch != "main" || decoded.SHA != "" {
		t.Fatalf("请求体字段不符: %+v", decoded)
	}
	if strings.Contains(decoded.Content, "\n") {
		t.Fatal("base64 内容不应包含换行")
	}
	restored, err := base64.StdEncoding.DecodeString(decoded.Content)
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if string(restored) != string(content) {
		t.Fatalf("内容还原不一致: %q", string(restored))
	}
}

// TestBase64Length 编码长度估算与实际一致（Content-Length 精确性）。
func TestBase64Length(t *testing.T) {
	for _, size := range []int64{0, 1, 2, 3, 4, 5, 16, 1023, 1024} {
		content := make([]byte, size)
		actual := int64(len(base64.StdEncoding.EncodeToString(content)))
		if got := base64Length(size); got != actual {
			t.Fatalf("size=%d 时长度估算不符: %d != %d", size, got, actual)
		}
	}
}

// TestGitBlobSHA1 与 git 对象 id 口径一致（空文件与已知向量）。
func TestGitBlobSHA1(t *testing.T) {
	path := writeTempFile(t, "empty", nil)
	got, err := gitBlobSHA1(path)
	if err != nil {
		t.Fatalf("计算 sha 失败: %v", err)
	}
	// git hash-object -t blob /dev/null
	const want = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	if got != want {
		t.Fatalf("空文件 git blob sha 不符: %q != %q", got, want)
	}
}

// TestGitHubStatUsesTreeAPI 校验查询远端文件优先走 Git Trees API（只取元数据、不下载
// base64 内容）；Trees API 不可用时回退 Contents API。
//
// 背景（需求 10825）：Contents API 查询已有文件会把内容一并返回（800KB 的 mvsv ≈ 1.1MB
// 响应），旧实现按 1MB 截断读取，被误报成「解析远端响应失败: unexpected end of JSON input」。
func TestGitHubStatUsesTreeAPI(t *testing.T) {
	store := &fakeConfigStore{values: githubEnabledValues(t, "ghp_token", "o:r:main:Archive/Monthly")}

	var treeHits, contentsHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/git/trees/"):
			treeHits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"sha":"tree1","truncated":false,"tree":[`+
				`{"path":"big.mvsv","type":"blob","sha":"ABCDEF0123","size":800787}]}`)
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/contents/"):
			contentsHits++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"should not be called"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	uploader := newGitHubUploader(store, server.URL)

	exists, size, sha, err := uploader.Stat(context.Background(), "Dir/Sub/big.mvsv")
	if err != nil {
		t.Fatalf("Stat 失败: %v", err)
	}
	if !exists || size != 800787 || sha != "abcdef0123" {
		t.Fatalf("Stat 结果不符：exists=%v size=%d sha=%s", exists, size, sha)
	}
	if treeHits != 1 || contentsHits != 0 {
		t.Fatalf("应只走 Trees API：tree=%d contents=%d", treeHits, contentsHits)
	}

	// Trees API 返回 404（目录不存在）时回退 Contents API 判断文件是否存在。
	tree404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/o/r/git/trees/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Not Found"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"file","size":12,"sha":"x","path":"Dir/Sub/big.mvsv"}`)
	}))
	defer tree404.Close()
	fallback := newGitHubUploader(store, tree404.URL)
	exists, size, _, err = fallback.Stat(context.Background(), "Dir/Sub/big.mvsv")
	if err != nil || !exists || size != 12 {
		t.Fatalf("回退 Contents API 失败：exists=%v size=%d err=%v", exists, size, err)
	}
}

// TestSplitRepoPath 校验仓库内路径拆分（目录下文件与根目录文件）。
func TestSplitRepoPath(t *testing.T) {
	cases := []struct{ path, dir, name string }{
		{"Archive/Monthly/a.mvsv", "Archive/Monthly", "a.mvsv"},
		{"a.mvsv", "", "a.mvsv"},
		{"/leading/a.mvsv", "leading", "a.mvsv"},
	}
	for _, c := range cases {
		dir, name := splitRepoPath(c.path)
		if dir != c.dir || name != c.name {
			t.Fatalf("splitRepoPath(%q)=(%q,%q)，期望 (%q,%q)", c.path, dir, name, c.dir, c.name)
		}
	}
}
