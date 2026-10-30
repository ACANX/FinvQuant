package dedup

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

const (
	// githubDefaultAPIBase GitHub 公共 API 端点（GitHub Enterprise 可覆盖）。
	githubDefaultAPIBase = "https://api.github.com"
	// githubAPIVersion 固定 REST API 版本，避免上游默认行为漂移。
	githubAPIVersion = "2022-11-28"
	// githubMaxFileBytes Contents API 单文件上限（GitHub 官方限制 100MB）。
	githubMaxFileBytes = 100 << 20
	// githubMaxAttempts 单次请求最大尝试次数（仅对 429 / 5xx / 网络错误重试）。
	githubMaxAttempts = 3
	// githubErrorSnippetLimit 错误响应片段上限（便于定位，避免刷日志）。
	githubErrorSnippetLimit = 4096
	// githubContentsReadLimit Contents API 响应体读取上限。
	//
	// Contents API 查询已有文件时会把内容以 base64 一并返回（800KB 的 mvsv ≈ 1.1MB
	// 响应），旧实现按 1MB 截断读取 → JSON 不完整 → 误报 "unexpected end of JSON
	// input"。这里提到 32MB 覆盖常见月文件，并保留超限时的明确报错。
	githubContentsReadLimit = 32 << 20
	// githubTreeReadLimit Git Trees API 响应上限（只含目录条目元数据，远小于此）。
	githubTreeReadLimit = 8 << 20
)

// githubTargetSource 提供实时上传目标配置（生产实现为 *syscfg.Store）。
type githubTargetSource interface {
	Load(ctx context.Context) (syscfg.Values, error)
}

// githubContent 对应 Contents API 返回的文件元信息。
type githubContent struct {
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	Path string `json:"path"`
}

// gitTree / gitTreeEntry 对应 Git Trees API 的响应（只含条目元数据，不含文件内容）。
type gitTree struct {
	Tree      []gitTreeEntry `json:"tree"`
	Truncated bool           `json:"truncated"`
}

type gitTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size *int64 `json:"size"`
}

// githubUploader 通过 GitHub Contents API 上传归档文件。
//
// 幂等策略：上传前先 GET 目标路径，若远端 sha 与本地 git blob sha 一致则直接
// 返回成功、不产生多余提交；内容不同则带上远端 sha 覆盖更新（Contents API 要求）。
type githubUploader struct {
	apiBase  string
	client   *http.Client
	source   githubTargetSource
	maxBytes int64

	// mu 保护 blobSHAs：Put 时记录本地文件的 git blob sha1，供紧随其后的
	// Verify 做内容级校验（GitHub 不返回 SHA256，用 git 对象 sha 可免下载比对）。
	mu       sync.Mutex
	blobSHAs map[string]string
}

func newGitHubUploader(source githubTargetSource, apiBase string) *githubUploader {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		base = githubDefaultAPIBase
	}
	return &githubUploader{
		apiBase:  base,
		source:   source,
		maxBytes: githubMaxFileBytes,
		blobSHAs: make(map[string]string),
		client: &http.Client{
			// 归档文件可能较大，PUT 需要较长时间；仍设上限避免连接悬挂。
			Timeout: 15 * time.Minute,
		},
	}
}

func (u *githubUploader) Mode() string { return UploadModeGitHub }

// RemoteChecksumAlgorithm GitHub 返回的是 git blob sha1（Contents/Trees API 不提供 SHA256）。
func (u *githubUploader) RemoteChecksumAlgorithm() string { return checksumAlgorithmGitBlob }

// DescribeTarget github 模式：给出「网站 / 组织 / 仓库 / 分支 / 文件路径」五元组，
// 便于运维直接从日志判断传到哪了。PAT（sys_config 20003）敏感，不入日志。
func (u *githubUploader) DescribeTarget(ctx context.Context, targetPath string) (string, error) {
	target, err := u.resolve(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("mode=github 网站=%s 组织/用户=%s 仓库=%s 分支=%s 目标路径=%s（完整 URL=%s）",
		u.apiBase, target.Repo.Owner, target.Repo.Repo, target.Repo.Branch,
		target.Repo.Join(targetPath), u.contentsURL(target.Repo, target.Repo.Join(targetPath))), nil
}

// resolve 读取系统配置并校验上传目标（开关 / PAT / 仓库定位）。
func (u *githubUploader) resolve(ctx context.Context) (syscfg.Target, error) {
	values, err := u.source.Load(ctx)
	if err != nil {
		return syscfg.Target{}, fmt.Errorf("读取系统配置失败: %w", err)
	}
	return values.ArchiveTarget()
}

// Stat 查询远端文件：exists 表示是否存在；checksum 为远端 git blob sha
// （Contents API 只提供该 sha，不提供 SHA256）。
func (u *githubUploader) Stat(ctx context.Context, targetPath string) (bool, int64, string, error) {
	target, err := u.resolve(ctx)
	if err != nil {
		return false, 0, "", err
	}
	path := target.Repo.Join(targetPath)
	content, exists, err := u.getContent(ctx, target, path)
	if err != nil {
		return false, 0, "", err
	}
	if !exists {
		return false, 0, "", nil
	}
	return true, content.Size, strings.ToLower(content.SHA), nil
}

// Put 上传本地文件到远端 path（pre_path_key/targetPath）。
func (u *githubUploader) Put(ctx context.Context, localPath, targetPath string) (int64, error) {
	target, err := u.resolve(ctx)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return 0, fmt.Errorf("读取本地文件失败: %w", err)
	}
	size := info.Size()
	if size <= 0 {
		return 0, fmt.Errorf("本地文件为空: %s", localPath)
	}
	if size > u.maxBytes {
		return 0, fmt.Errorf("文件大小 %d 字节超出 GitHub Contents API 单文件上限 %d 字节: %s",
			size, u.maxBytes, localPath)
	}

	path := target.Repo.Join(targetPath)
	content, exists, err := u.getContent(ctx, target, path)
	if err != nil {
		return 0, err
	}

	localBlobSHA, err := gitBlobSHA1(localPath)
	if err != nil {
		return 0, fmt.Errorf("计算本地文件 git 对象 sha 失败: %w", err)
	}

	remoteSHA := ""
	if exists {
		if content.Type != "" && content.Type != "file" {
			return 0, fmt.Errorf("目标路径已存在且不是文件（type=%s）: %s", content.Type, path)
		}
		if content.Size == size && strings.EqualFold(content.SHA, localBlobSHA) {
			// 远端内容与本地一致：不重复提交，直接视为成功（幂等重传）。
			u.rememberBlobSHA(path, localBlobSHA)
			return size, nil
		}
		remoteSHA = content.SHA
	}

	message := fmt.Sprintf("archive: upload %s (%d bytes)", path, size)
	response, err := u.do(ctx, http.MethodPut, u.contentsURL(target.Repo, path),
		func() (io.ReadCloser, int64, error) {
			return buildContentsBody(message, target.Repo.Branch, remoteSHA, localPath, size)
		}, target.Token)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return 0, u.statusError("上传文件", response)
	}
	// 丢弃响应体，复用连接。
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, githubErrorSnippetLimit))

	u.rememberBlobSHA(path, localBlobSHA)
	return size, nil
}

// Verify 校验远端对象：大小 + 内容 sha（本地 git blob sha1 与远端比对）。
//
// 入参 checksum 为本地 SHA256，GitHub Contents API 不返回 SHA256，故此处改用
// git 对象 sha 做内容级校验；若进程内没有该文件的 sha 记录（例如重启后单独
// 校验），退化为仅校验大小并在日志中说明。
func (u *githubUploader) Verify(ctx context.Context, targetPath string, size int64, checksum string) error {
	target, err := u.resolve(ctx)
	if err != nil {
		return err
	}
	path := target.Repo.Join(targetPath)
	content, exists, err := u.getContent(ctx, target, path)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("远端文件不存在: %s", path)
	}
	if size > 0 && content.Size != size {
		return fmt.Errorf("远端大小不一致（远端 %d，期望 %d）: %s", content.Size, size, path)
	}
	expected := u.rememberedBlobSHA(path)
	if expected == "" {
		return nil
	}
	if !strings.EqualFold(content.SHA, expected) {
		return fmt.Errorf("远端内容 sha 不一致（远端 %s，期望 %s，本地SHA256 %s）: %s",
			content.SHA, expected, checksum, path)
	}
	return nil
}

// ---------------------------------------------------------------------
// Contents API 访问
// ---------------------------------------------------------------------

// contentsURL 拼接 /repos/{owner}/{repo}/contents/{path}，路径逐段转义并保留 '/'。
func (u *githubUploader) contentsURL(repo syscfg.RepoSpec, path string) string {
	var builder strings.Builder
	builder.WriteString(u.apiBase)
	builder.WriteString("/repos/")
	builder.WriteString(url.PathEscape(repo.Owner))
	builder.WriteString("/")
	builder.WriteString(url.PathEscape(repo.Repo))
	builder.WriteString("/contents/")
	for index, segment := range strings.Split(path, "/") {
		if index > 0 {
			builder.WriteString("/")
		}
		builder.WriteString(url.PathEscape(segment))
	}
	return builder.String()
}

// treesURL 拼接 /repos/{owner}/{repo}/git/trees/{ref}；ref 可带 ":路径" 下钻子目录。
// 只对 owner/repo 做路径转义，ref 保留 "：" 与 "/"（Git Trees API 的 ref:path 语法）。
func (u *githubUploader) treesURL(repo syscfg.RepoSpec, ref string) string {
	var builder strings.Builder
	builder.WriteString(u.apiBase)
	builder.WriteString("/repos/")
	builder.WriteString(url.PathEscape(repo.Owner))
	builder.WriteString("/")
	builder.WriteString(url.PathEscape(repo.Repo))
	builder.WriteString("/git/trees/")
	builder.WriteString(ref)
	return builder.String()
}

// getContent 查询远端文件元数据；exists=false 表示不存在。
//
// 优先走 Git Trees API（`/git/trees/{branch}:{dir}` 只返回目录条目元数据，不带文件
// 内容）；失败（企业版不支持、分支名含 '/'、树被截断等）时回退 Contents API。
//
// 为什么要换：Contents API 查询已有文件会把内容以 base64 一并返回（800KB 的 mvsv
// ≈ 1.1MB 响应），上传前查重时既要多下载一遍内容、又容易因读取上限被截断。
func (u *githubUploader) getContent(ctx context.Context, target syscfg.Target, path string) (githubContent, bool, error) {
	content, exists, err := u.getTreeEntry(ctx, target, path)
	if err == nil {
		return content, exists, nil
	}
	// Trees API 不可用时不能让上传直接失败，回退到兼容性更好的 Contents API。
	return u.getContentsAPI(ctx, target, path)
}

// getTreeEntry 用 Git Trees API 查目录条目，返回目标文件的 blob sha 与大小。
func (u *githubUploader) getTreeEntry(ctx context.Context, target syscfg.Target, path string) (githubContent, bool, error) {
	if strings.Contains(target.Repo.Branch, "/") {
		return githubContent{}, false, fmt.Errorf("分支名含 '/'，无法使用 ref:path 语法")
	}
	dir, name := splitRepoPath(path)
	ref := target.Repo.Branch
	if dir != "" {
		ref += ":" + dir
	}
	response, err := u.do(ctx, http.MethodGet, u.treesURL(target.Repo, ref), nil, target.Token)
	if err != nil {
		return githubContent{}, false, err
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusNotFound:
		// 目录或分支不存在：交由 Contents API 区分「文件不存在」与「配置错误」。
		return githubContent{}, false, fmt.Errorf("目录树不存在: %s", ref)
	case http.StatusOK:
	default:
		return githubContent{}, false, u.statusError("查询远端目录树", response)
	}

	var tree gitTree
	if err := json.NewDecoder(io.LimitReader(response.Body, githubTreeReadLimit)).Decode(&tree); err != nil {
		return githubContent{}, false, fmt.Errorf("解析远端目录树失败: %w", err)
	}
	if tree.Truncated {
		return githubContent{}, false, errors.New("远端目录树被截断，无法可靠判断条目")
	}
	for _, entry := range tree.Tree {
		if entry.Path != name {
			continue
		}
		switch entry.Type {
		case "blob":
			if entry.Size == nil {
				return githubContent{}, false, errors.New("远端目录树未返回文件大小")
			}
			return githubContent{Type: "file", Size: *entry.Size, SHA: entry.SHA, Path: path}, true, nil
		case "tree":
			return githubContent{Type: "dir", SHA: entry.SHA, Path: path}, true, nil
		}
	}
	return githubContent{}, false, nil
}

// splitRepoPath 把仓库内路径拆成（目录, 文件名）；根目录下文件返回空目录。
func splitRepoPath(path string) (dir, name string) {
	trimmed := strings.Trim(path, "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[:index], trimmed[index+1:]
	}
	return "", trimmed
}

// getContentsAPI 查询远端文件（回退路径；会连文件内容一起返回 base64）。
func (u *githubUploader) getContentsAPI(ctx context.Context, target syscfg.Target, path string) (githubContent, bool, error) {
	endpoint := u.contentsURL(target.Repo, path) + "?ref=" + url.QueryEscape(target.Repo.Branch)
	response, err := u.do(ctx, http.MethodGet, endpoint, nil, target.Token)
	if err != nil {
		return githubContent{}, false, err
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusNotFound:
		return githubContent{}, false, nil
	case http.StatusOK:
	default:
		return githubContent{}, false, u.statusError("查询远端文件", response)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, githubContentsReadLimit+1))
	if err != nil {
		return githubContent{}, false, fmt.Errorf("读取远端响应失败: %w（HTTP %d，GET %s）", err, response.StatusCode, endpoint)
	}
	if int64(len(payload)) > githubContentsReadLimit {
		return githubContent{}, false, fmt.Errorf(
			"查询远端文件响应超过读取上限 %d 字节（HTTP %d，GET %s，Content-Length=%d）：Contents API 会连 base64 内容一起返回，文件较大时请依赖 Git Trees API 或调大上限",
			githubContentsReadLimit, response.StatusCode, endpoint, response.ContentLength)
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return githubContent{}, false, fmt.Errorf("目标路径是目录而非文件: %s", path)
	}
	var content githubContent
	if err := json.Unmarshal(trimmed, &content); err != nil {
		return githubContent{}, false, responseDecodeError("查询远端文件", response, endpoint, trimmed, err)
	}
	return content, true, nil
}

// responseDecodeError 把「响应体无法解析为预期 JSON」补成可直接定位的诊断串：
// HTTP 状态、请求方法/URL、Content-Type/Content-Length、响应体片段与 GitHub 请求 ID。
//
// 典型场景：中间代理/网关返回 200 但响应体为空，旧实现只报
// “解析远端响应失败: unexpected end of JSON input”，既看不出是目标仓库配置
// 问题还是被网关拦截，也看不出请求的是哪个 URL。
func responseDecodeError(action string, response *http.Response, endpoint string, payload []byte, cause error) error {
	snippet := strings.TrimSpace(string(payload))
	if runes := []rune(snippet); len(runes) > 200 {
		snippet = string(runes[:200]) + "…"
	}
	detail := fmt.Sprintf("%s响应解析失败: HTTP %d，GET %s，Content-Type=%q，Content-Length=%d，响应体片段=%q，请求ID=%q",
		action, response.StatusCode, endpoint,
		response.Header.Get("Content-Type"), response.ContentLength, snippet,
		response.Header.Get("X-GitHub-Request-Id"))
	if len(payload) == 0 {
		detail += "；响应体为空：Contents API 正常会返回 JSON，请核对「网站」（FINV_QUOTE_DEDUB_UPLOAD_GITHUB_API_BASE 是否指向 GitHub API）、" +
			"代理/网关是否吞掉响应体，以及仓库定位配置（sys_config 20004 的 organization:repo:branch:pre_path_key）"
	}
	return fmt.Errorf("%s: %w", detail, cause)
}

// do 发送请求并对 429 / 5xx / 网络错误做有限重试；bodyFactory 用于重放请求体。
func (u *githubUploader) do(
	ctx context.Context,
	method, endpoint string,
	bodyFactory func() (io.ReadCloser, int64, error),
	token string,
) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= githubMaxAttempts; attempt++ {
		var (
			body   io.ReadCloser
			length int64
			err    error
		)
		if bodyFactory != nil {
			body, length, err = bodyFactory()
			if err != nil {
				return nil, err
			}
		}
		request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
		if err != nil {
			if body != nil {
				_ = body.Close()
			}
			return nil, err
		}
		request.ContentLength = length
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
		request.Header.Set("User-Agent", "finvquant-archive-uploader")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if bodyFactory != nil {
			request.Header.Set("Content-Type", "application/json; charset=utf-8")
		}

		response, err := u.client.Do(request)
		if err != nil {
			lastErr = err
			if attempt < githubMaxAttempts && waitBackoff(ctx, attempt) {
				continue
			}
			return nil, fmt.Errorf("请求 GitHub 失败: %w", err)
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastErr = u.statusError("请求 GitHub", response)
			_ = response.Body.Close()
			if attempt < githubMaxAttempts && waitBackoff(ctx, attempt) {
				continue
			}
			return nil, lastErr
		}
		return response, nil
	}
	return nil, lastErr
}

// statusError 把非 2xx 响应转换为带处置提示的错误（不包含凭据）。
func (u *githubUploader) statusError(action string, response *http.Response) error {
	snippet, _ := io.ReadAll(io.LimitReader(response.Body, githubErrorSnippetLimit))
	hint := ""
	switch response.StatusCode {
	case http.StatusUnauthorized:
		hint = "，PAT 无效或已过期"
	case http.StatusForbidden:
		hint = "，PAT 权限不足或触发速率限制"
	case http.StatusNotFound:
		hint = "，仓库/分支不存在或 PAT 无访问权限"
	case http.StatusConflict:
		hint = "，远端文件已被并发修改"
	case http.StatusUnprocessableEntity:
		hint = "，请求被拒绝（检查分支名、文件大小与路径）"
	}
	return fmt.Errorf("%s失败: HTTP %d%s %s", action, response.StatusCode, hint, strings.TrimSpace(string(snippet)))
}

// waitBackoff 等待退避时长；ctx 取消时返回 false。
func waitBackoff(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt*attempt) * 500 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// ---------------------------------------------------------------------
// 请求体构造（流式 base64，避免大文件在内存里翻倍）
// ---------------------------------------------------------------------

// buildContentsBody 构造 Contents API 的 PUT 请求体：
//
//	{"message":"...","content":"<base64>","branch":"...","sha":"<可选>"}
//
// 返回的 reader 以流式方式编码文件内容，长度精确可算，便于设置 Content-Length。
func buildContentsBody(message, branch, sha, localPath string, size int64) (io.ReadCloser, int64, error) {
	prefix := `{"message":` + jsonString(message) + `,"content":"`
	suffix := `","branch":` + jsonString(branch)
	if sha != "" {
		suffix += `,"sha":` + jsonString(sha)
	}
	suffix += "}"

	file, err := os.Open(localPath)
	if err != nil {
		return nil, 0, fmt.Errorf("打开本地文件失败: %w", err)
	}

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		encoder := base64.NewEncoder(base64.StdEncoding, pipeWriter)
		_, copyErr := io.Copy(encoder, file)
		closeErr := encoder.Close()
		_ = file.Close()
		if copyErr != nil {
			_ = pipeWriter.CloseWithError(copyErr)
			return
		}
		_ = pipeWriter.CloseWithError(closeErr)
	}()

	length := int64(len(prefix)) + base64Length(size) + int64(len(suffix))
	body := io.MultiReader(strings.NewReader(prefix), pipeReader, strings.NewReader(suffix))
	return struct {
		io.Reader
		io.Closer
	}{Reader: body, Closer: pipeReader}, length, nil
}

// base64Length 计算 n 字节标准 base64 编码后的字符数（含填充，无换行）。
func base64Length(n int64) int64 {
	return (n + 2) / 3 * 4
}

// jsonString 生成 JSON 字符串字面量（含引号），保证消息体安全。
func jsonString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

// ---------------------------------------------------------------------
// git 对象 sha
// ---------------------------------------------------------------------

// gitBlobSHA1 计算文件内容的 git blob 对象 id：sha1("blob <size>\0" + content)。
//
// 用途：Contents API 返回的 sha 就是该值，因此本地算出来即可与远端比对，
// 无需下载文件内容。注意这是 git 的对象标识格式，不作为安全哈希使用。
func gitBlobSHA1(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	// #nosec G401 —— 此处复用 git 的对象 id 算法，仅用于与远端 sha 比对。
	hash := sha1.New()
	if _, err := fmt.Fprintf(hash, "blob %d\x00", info.Size()); err != nil {
		return "", err
	}
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (u *githubUploader) rememberBlobSHA(path, sha string) {
	u.mu.Lock()
	u.blobSHAs[path] = sha
	u.mu.Unlock()
}

func (u *githubUploader) rememberedBlobSHA(path string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.blobSHAs[path]
}
