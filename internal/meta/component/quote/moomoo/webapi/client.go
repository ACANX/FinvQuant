// Package webapi 封装 moomoo OpenAPI（https://webapi.moomoo.com）的传统 API Key 认证客户端。
//
// 认证：AppKey + Ed25519（或其它算法）私钥对请求内容签名，请求头携带
//
//	X-Api-Key     AppKey ID
//	X-Timestamp   毫秒时间戳（与签名原文一致）
//	X-Nonce       随机串（字母/数字/下划线/连字符，1–64 位，防重放）
//	Authorization Base64 签名结果（AppKey 场景不加 Bearer 前缀）
//
// 签名原文（字段间用 \n 连接，空字段也保留位置）：
//
//	{timestamp_ms}\n{HTTP_METHOD}\n{request_path}\n{query_string}\n{sha256(body)_hex}
//
// 凭据只从环境变量读取，**永不落盘**：
//
//	FINV_MOOMOO_APP_KEY       AppKey ID
//	FINV_MOOMOO_PRIVATE_KEY   Ed25519 PKCS#8 私钥 PEM（可含真实换行或字面 \n）
//	FINV_MOOMOO_BASE_URL      可选，默认 https://webapi.moomoo.com
//
// 未配置凭据时 NewClientFromEnv 返回 (nil, nil)，调用方按「未启用」处理。
// 时间戳与服务端偏差超过 5 秒会返回 -12006；可先调 GET /api/v1.0/server-time 校准。
package webapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// 协议默认值。
const (
	// DefaultBaseURL moomoo OpenAPI 主机。
	DefaultBaseURL = "https://webapi.moomoo.com"
	// SnapshotPath 行情快照接口路径。
	SnapshotPath = "/api/v1.0/quote/snapshot"
	// ServerTimePath 服务端时间接口（可用于校准时间戳）。
	ServerTimePath = "/api/v1.0/server-time"
	// MaxCodes 行情快照单次请求的 code 上限。
	MaxCodes = 400
	// DefaultTimeout 单次请求超时。
	DefaultTimeout = 20 * time.Second
)

// Config 客户端配置。
type Config struct {
	// BaseURL 默认 https://webapi.moomoo.com。
	BaseURL string
	// AppKey 控制台创建的 AppKey ID。
	AppKey string
	// PrivateKeyPEM Ed25519 PKCS#8 私钥 PEM（支持字面 \n）。
	PrivateKeyPEM string
	// Timeout 单次请求超时，默认 20s。
	Timeout time.Duration
}

// Client moomoo OpenAPI 客户端。
type Client struct {
	baseURL string
	appKey  string
	priv    ed25519.PrivateKey
	http    *http.Client
}

// NewClientFromEnv 从环境变量构造客户端；缺少 AppKey 或私钥时返回 (nil, nil)。
func NewClientFromEnv() (*Client, error) {
	appKey := strings.TrimSpace(os.Getenv("FINV_MOOMOO_APP_KEY"))
	keyPEM := os.Getenv("FINV_MOOMOO_PRIVATE_KEY")
	if appKey == "" || strings.TrimSpace(keyPEM) == "" {
		return nil, nil
	}
	baseURL := strings.TrimSpace(os.Getenv("FINV_MOOMOO_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return NewClient(Config{BaseURL: baseURL, AppKey: appKey, PrivateKeyPEM: keyPEM})
}

// NewClient 创建客户端并解析私钥。
func NewClient(cfg Config) (*Client, error) {
	appKey := strings.TrimSpace(cfg.AppKey)
	if appKey == "" {
		return nil, errors.New("moomoo OpenAPI：缺少 AppKey")
	}
	priv, err := ParseEd25519PrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		baseURL: baseURL,
		appKey:  appKey,
		priv:    priv,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// BaseURL 返回站点地址（排障/日志用，不含凭据）。
func (c *Client) BaseURL() string { return c.baseURL }

// ServerTime 读取服务端毫秒时间戳（需要有效凭据；用于校准本机时钟偏差）。
func (c *Client) ServerTime(ctx context.Context) (int64, error) {
	raw, status, err := c.do(ctx, http.MethodGet, ServerTimePath, "", nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		ServerTimeMs json.Number `json:"server_time_ms"`
		RetCode      int         `json:"ret_code"`
		RetMsg       string      `json:"ret_msg"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("moomoo OpenAPI：解析 server-time 失败（HTTP %d）: %w", status, err)
	}
	if status != http.StatusOK || resp.RetCode != 0 {
		return 0, fmt.Errorf("moomoo OpenAPI：server-time HTTP %d ret_code=%d ret_msg=%s",
			status, resp.RetCode, resp.RetMsg)
	}
	ms, err := strconv.ParseInt(resp.ServerTimeMs.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("moomoo OpenAPI：server_time_ms 非法: %w", err)
	}
	return ms, nil
}

// do 发送一次签名请求，返回响应体与 HTTP 状态码。
func (c *Client) do(ctx context.Context, method, path, query string, body []byte) ([]byte, int, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := randomNonce()
	signature := base64.StdEncoding.EncodeToString(
		ed25519.Sign(c.priv, []byte(SigningText(ts, method, path, query, body))))

	url := c.baseURL + path
	if query != "" {
		url += "?" + query
	}
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("moomoo OpenAPI：构造请求失败: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Api-Key", c.appKey)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("Authorization", signature)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("moomoo OpenAPI：请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("moomoo OpenAPI：读取响应失败: %w", err)
	}
	return raw, resp.StatusCode, nil
}

// ParseEd25519PrivateKey 解析 Ed25519 PKCS#8 私钥。
//
// 容错范围（都是人工粘贴时真实会遇到的形态）：
//   - 标准三行 PEM（`-----BEGIN PRIVATE KEY-----` / base64 / `-----END PRIVATE KEY-----`）；
//   - 字面 `\n` 分隔的一行文本；
//   - Windows 换行（`\r\n`）、行首尾空白、空行；
//   - 不可见字符（BOM `\ufeff`、零宽空格 `\u200b`、不换行空格 `\u00a0`）；
//   - **只粘贴 base64 主体**（缺少 BEGIN/END 头尾）——自动按 PKCS#8 DER 解析。
//
// 错误信息区分「不是 PEM / 只贴了主体且解不开」与「PEM 类型不对（贴成公钥）」。
func ParseEd25519PrivateKey(raw string) (ed25519.PrivateKey, error) {
	text := normalizePEMText(raw)
	if text == "" {
		return nil, errors.New("moomoo OpenAPI：私钥为空")
	}
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		// 容错：只贴了 base64 主体（没有 BEGIN/END 头尾）
		if der, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(text, "\n", "")); err == nil {
			if priv, parseErr := parsePKCS8Ed25519(der); parseErr == nil {
				return priv, nil
			}
		}
		return nil, errors.New("moomoo OpenAPI：私钥不是合法的 PEM（需要完整的 " +
			"-----BEGIN PRIVATE KEY----- / base64 主体 / -----END PRIVATE KEY----- 三行，或只粘贴 base64 主体）")
	}
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("moomoo OpenAPI：私钥类型应为 PRIVATE KEY（实际 %s）；请勿粘贴公钥", block.Type)
	}
	return parsePKCS8Ed25519(block.Bytes)
}

// normalizePEMText 归一化人工粘贴的 PEM 文本（换行/空白/不可见字符）。
func normalizePEMText(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.ReplaceAll(text, `\n`, "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.Map(func(r rune) rune {
		switch r {
		case '\ufeff', '\u200b', '\u00a0':
			return -1
		}
		return r
	}, text)

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// parsePKCS8Ed25519 解析 PKCS#8 DER 并断言为 Ed25519。
func parsePKCS8Ed25519(der []byte) (ed25519.PrivateKey, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：解析 PKCS#8 私钥失败: %w", err)
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("moomoo OpenAPI：私钥不是 Ed25519")
	}
	return priv, nil
}

// randomNonce 生成 X-Nonce（字母数字，1–64 位）。
func randomNonce() string {
	return "n" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
