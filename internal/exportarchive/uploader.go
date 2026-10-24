package exportarchive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrUploadDisabled 未配置上传目标（FINV_QUOTE_DEDUB_UPLOAD_MODE=none）。
var ErrUploadDisabled = errors.New("未启用独享存储上传（FINV_QUOTE_DEDUB_UPLOAD_MODE=none）")

// Uploader 独享存储库上传适配器。
//
// 约定：targetPath 为相对路径（可含 `/` 分层，由 Options.TargetPath 生成），
// 远端对象名与本地文件名一致（方案稿 C-04 / G-10：不生成、不支持 .csv 改名）。
type Uploader interface {
	// Mode 返回实现标识（none / dir / http）。
	Mode() string
	// Stat 查询远端对象：exists 表示是否已存在；size 为字节数；
	// checksum 为远端 SHA256（小写十六进制，服务端不支持时返回空串）。
	Stat(ctx context.Context, targetPath string) (exists bool, size int64, checksum string, err error)
	// Put 上传本地文件到 targetPath，返回上传字节数。
	Put(ctx context.Context, localPath, targetPath string) (int64, error)
	// Verify 校验远端对象大小与 SHA256（G-11 / M-03）。
	Verify(ctx context.Context, targetPath string, size int64, checksum string) error
}

// NewUploader 按配置创建上传适配器。
func NewUploader(options Options) (Uploader, error) {
	switch options.UploadMode {
	case UploadModeDir:
		return newDirUploader(options.UploadDir)
	case UploadModeHTTP:
		return newHTTPUploader(options.UploadHTTPBase, options.UploadHTTPToken)
	default:
		return noneUploader{}, nil
	}
}

// ---------------------------------------------------------------------
// none：未启用上传（导出与登记照常进行，上传 Job 空转并在日志中说明）
// ---------------------------------------------------------------------

type noneUploader struct{}

func (noneUploader) Mode() string { return UploadModeNone }

func (noneUploader) Stat(context.Context, string) (bool, int64, string, error) {
	return false, 0, "", nil
}

func (noneUploader) Put(context.Context, string, string) (int64, error) {
	return 0, ErrUploadDisabled
}

func (noneUploader) Verify(context.Context, string, int64, string) error { return nil }

// ---------------------------------------------------------------------
// dir：独享存储以目录 / NAS 挂载方式暴露（默认实现）
// ---------------------------------------------------------------------

type dirUploader struct {
	root string
}

func newDirUploader(root string) (Uploader, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("FINV_QUOTE_DEDUB_UPLOAD_DIR 未配置")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析上传目录失败: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	return &dirUploader{root: absolute}, nil
}

func (u *dirUploader) Mode() string { return UploadModeDir }

// resolve 把相对 targetPath 解析为 root 内的绝对路径，越出 root 时拒绝（防目录穿越）。
func (u *dirUploader) resolve(targetPath string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(targetPath))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("非法上传目标路径: %q", targetPath)
	}
	full := filepath.Join(u.root, cleaned)
	relative, err := filepath.Rel(u.root, full)
	if err != nil || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("上传目标路径越界: %q", targetPath)
	}
	return full, nil
}

func (u *dirUploader) Stat(_ context.Context, targetPath string) (bool, int64, string, error) {
	full, err := u.resolve(targetPath)
	if err != nil {
		return false, 0, "", err
	}
	if _, err := os.Stat(full); err != nil {
		if os.IsNotExist(err) {
			return false, 0, "", nil
		}
		return false, 0, "", err
	}
	checksum, size, err := FileSHA256(full)
	if err != nil {
		return false, 0, "", err
	}
	return true, size, checksum, nil
}

func (u *dirUploader) Put(ctx context.Context, localPath, targetPath string) (int64, error) {
	source, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer func() { _ = source.Close() }()

	full, err := u.resolve(targetPath)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, fmt.Errorf("创建目标目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), filepath.Base(full)+".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("创建上传临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmp, &contextReader{ctx: ctx, reader: source})
	if err != nil {
		return 0, fmt.Errorf("复制文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("同步上传文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("关闭上传文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, full); err != nil {
		return 0, fmt.Errorf("上传落盘失败: %w", err)
	}
	committed = true
	return written, nil
}

func (u *dirUploader) Verify(ctx context.Context, targetPath string, size int64, checksum string) error {
	exists, remoteSize, remoteChecksum, err := u.Stat(ctx, targetPath)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("远端文件不存在")
	}
	if size > 0 && remoteSize != size {
		return fmt.Errorf("远端大小不一致（远端 %d，期望 %d）", remoteSize, size)
	}
	if checksum != "" && !strings.EqualFold(remoteChecksum, checksum) {
		return fmt.Errorf("远端 checksum 不一致（远端 %s，期望 %s）", remoteChecksum, checksum)
	}
	return nil
}

// ---------------------------------------------------------------------
// http：独享存储以 HTTP PUT 网关方式暴露（可选实现）
// ---------------------------------------------------------------------

const (
	httpChecksumHeader = "X-Checksum-Sha256"
	httpMaxVerifyBytes = 512 << 20
)

type httpUploader struct {
	base   string
	token  string
	client *http.Client
}

func newHTTPUploader(base, token string) (Uploader, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil, errors.New("FINV_QUOTE_DEDUB_UPLOAD_HTTP_BASE 未配置")
	}
	if _, err := url.Parse(base); err != nil {
		return nil, fmt.Errorf("上传基地址非法: %w", err)
	}
	return &httpUploader{
		base:   base,
		token:  token,
		client: &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

func (u *httpUploader) Mode() string { return UploadModeHTTP }

func (u *httpUploader) urlFor(targetPath string) string {
	escaped := (&url.URL{Path: targetPath}).EscapedPath()
	return u.base + "/" + strings.TrimPrefix(escaped, "/")
}

func (u *httpUploader) newRequest(ctx context.Context, method, targetPath string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, u.urlFor(targetPath), body)
	if err != nil {
		return nil, err
	}
	if u.token != "" {
		request.Header.Set("Authorization", "Bearer "+u.token)
	}
	return request, nil
}

func (u *httpUploader) Stat(ctx context.Context, targetPath string) (bool, int64, string, error) {
	request, err := u.newRequest(ctx, http.MethodHead, targetPath, nil)
	if err != nil {
		return false, 0, "", err
	}
	response, err := u.client.Do(request)
	if err != nil {
		return false, 0, "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return false, 0, "", nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, 0, "", fmt.Errorf("远端 HEAD 失败: HTTP %d", response.StatusCode)
	}
	return true, response.ContentLength, strings.ToLower(response.Header.Get(httpChecksumHeader)), nil
}

func (u *httpUploader) Put(ctx context.Context, localPath, targetPath string) (int64, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	request, err := u.newRequest(ctx, http.MethodPut, targetPath, file)
	if err != nil {
		return 0, err
	}
	request.ContentLength = info.Size()
	request.Header.Set("Content-Type", "application/octet-stream")

	response, err := u.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return 0, fmt.Errorf("远端 PUT 失败: HTTP %d %s", response.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return info.Size(), nil
}

func (u *httpUploader) Verify(ctx context.Context, targetPath string, size int64, checksum string) error {
	exists, remoteSize, remoteChecksum, err := u.Stat(ctx, targetPath)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("远端文件不存在")
	}
	if size > 0 && remoteSize != size {
		return fmt.Errorf("远端大小不一致（远端 %d，期望 %d）", remoteSize, size)
	}
	if checksum == "" {
		return nil
	}
	if remoteChecksum != "" {
		if !strings.EqualFold(remoteChecksum, checksum) {
			return fmt.Errorf("远端 checksum 不一致（远端 %s，期望 %s）", remoteChecksum, checksum)
		}
		return nil
	}
	// 服务端不提供 checksum 时回源校验：GET 下载并计算 SHA256。
	request, err := u.newRequest(ctx, http.MethodGet, targetPath, nil)
	if err != nil {
		return err
	}
	response, err := u.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("远端 GET 失败: HTTP %d", response.StatusCode)
	}
	actual, written, err := sha256Of(io.LimitReader(response.Body, httpMaxVerifyBytes))
	if err != nil {
		return err
	}
	if size > 0 && written != size {
		return fmt.Errorf("远端下载大小不一致（%d != %d）", written, size)
	}
	if !strings.EqualFold(actual, checksum) {
		return fmt.Errorf("远端下载 checksum 不一致（%s != %s）", actual, checksum)
	}
	return nil
}

// contextReader 支持取消的读取包装（大文件复制时响应 ctx 取消）。
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
