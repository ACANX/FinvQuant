package cdeu

// moomoo OpenAPI（https://webapi.moomoo.com）的最小客户端（需求 11314）。
//
// 本文件只实现「加密货币行情快照」用到的一个接口：
//
//	POST /api/v1.0/quote/snapshot   body {"code_list":["CC.BTCUSD", ...]}（一次最多 400 个）
//
// 认证走「传统 API Key」：AppKey + Ed25519 私钥对请求内容签名。
// 签名原文（字段间用 \n 连接，空字段也要保留位置）：
//
//	{timestamp_ms}\n{HTTP_METHOD}\n{request_path}\n{query_string}\n{sha256(body)_hex}
//
// 请求头：X-Api-Key / X-Timestamp / X-Nonce / Authorization（Base64 签名，不带 Bearer 前缀）。
//
// 凭据只从环境变量读取，**永不落盘**：
//
//	FINV_MOOMOO_APP_KEY       AppKey ID
//	FINV_MOOMOO_PRIVATE_KEY   Ed25519 PKCS#8 私钥 PEM（可含真实换行或字面 \n）
//	FINV_MOOMOO_BASE_URL      可选，默认 https://webapi.moomoo.com
//
// 未配置凭据时 NewMoomooOpenAPIClientFromEnv 返回 nil，调用方按「未启用」处理。

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
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

// 协议常量。
const (
	// MoomooDefaultBaseURL moomoo OpenAPI 主机。
	MoomooDefaultBaseURL = "https://webapi.moomoo.com"
	// MoomooSnapshotPath 行情快照接口路径。
	MoomooSnapshotPath = "/api/v1.0/quote/snapshot"
	// MoomooSnapshotMaxCodes 单次请求的 code 上限（文档：400）。
	MoomooSnapshotMaxCodes = 400
	// MoomooCryptoCodePrefix 加密货币代码前缀（CC.BTCUSD）。
	MoomooCryptoCodePrefix = "CC."
	// CryptoProvider 本表 provider 固定值。
	CryptoProvider = "MMAPI"
)

// MoomooSnapshotFetcher 快照拉取能力（默认走 moomoo OpenAPI；测试可注入）。
type MoomooSnapshotFetcher interface {
	GetQuoteSnapshots(ctx context.Context, codes []string) ([]MoomooSnapshot, error)
}

// MoomooSnapshot 快照里本表用到的字段（其余字段不解析）。
type MoomooSnapshot struct {
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	LastPrice           float64 `json:"last_price"`
	OpenPrice           float64 `json:"open_price"`
	HighPrice           float64 `json:"high_price"`
	LowPrice            float64 `json:"low_price"`
	PrevClosePrice      float64 `json:"prev_close_price"`
	AvgPrice            float64 `json:"avg_price"`
	Amplitude           float64 `json:"amplitude"`
	Volume              float64 `json:"volume"`
	Turnover            float64 `json:"turnover"`
	VolumeRatio         float64 `json:"volume_ratio"`
	BidPrice            float64 `json:"bid_price"`
	AskPrice            float64 `json:"ask_price"`
	BidVol              float64 `json:"bid_vol"`
	AskVol              float64 `json:"ask_vol"`
	BidAskRatio         float64 `json:"bid_ask_ratio"`
	Highest52WeeksPrice float64 `json:"highest52weeks_price"`
	Lowest52WeeksPrice  float64 `json:"lowest52weeks_price"`
	HighestHistoryPrice float64 `json:"highest_history_price"`
	LowestHistoryPrice  float64 `json:"lowest_history_price"`
	SecStatus           string  `json:"sec_status"`
	Suspension          bool    `json:"suspension"`
	DataDate            string  `json:"data_date"`
	UpdateTime          int64   `json:"update_time"`
}

// MoomooOpenAPIClient moomoo OpenAPI 客户端（传统 API Key 认证）。
type MoomooOpenAPIClient struct {
	baseURL string
	appKey  string
	priv    ed25519.PrivateKey
	http    *http.Client
}

// NewMoomooOpenAPIClientFromEnv 从环境变量构造客户端；缺少 AppKey 或私钥时返回 nil（未启用）。
func NewMoomooOpenAPIClientFromEnv() (*MoomooOpenAPIClient, error) {
	appKey := strings.TrimSpace(os.Getenv("FINV_MOOMOO_APP_KEY"))
	keyPEM := os.Getenv("FINV_MOOMOO_PRIVATE_KEY")
	if appKey == "" || strings.TrimSpace(keyPEM) == "" {
		return nil, nil
	}
	baseURL := strings.TrimSpace(os.Getenv("FINV_MOOMOO_BASE_URL"))
	if baseURL == "" {
		baseURL = MoomooDefaultBaseURL
	}
	return NewMoomooOpenAPIClient(MoomooOpenAPIConfig{
		BaseURL:       baseURL,
		AppKey:        appKey,
		PrivateKeyPEM: keyPEM,
	})
}

// MoomooOpenAPIConfig 客户端配置。
type MoomooOpenAPIConfig struct {
	// BaseURL 默认 https://webapi.moomoo.com。
	BaseURL string
	// AppKey 控制台创建的 AppKey ID。
	AppKey string
	// PrivateKeyPEM Ed25519 PKCS#8 私钥 PEM（支持字面 \n）。
	PrivateKeyPEM string
	// Timeout 单次请求超时，默认 20s。
	Timeout time.Duration
}

// NewMoomooOpenAPIClient 创建客户端并解析私钥。
func NewMoomooOpenAPIClient(cfg MoomooOpenAPIConfig) (*MoomooOpenAPIClient, error) {
	appKey := strings.TrimSpace(cfg.AppKey)
	if appKey == "" {
		return nil, errors.New("moomoo OpenAPI：缺少 AppKey")
	}
	priv, err := parseEd25519PrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = MoomooDefaultBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &MoomooOpenAPIClient{
		baseURL: baseURL,
		appKey:  appKey,
		priv:    priv,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// parseEd25519PrivateKey 解析 Ed25519 PKCS#8 PEM（兼容字面 \n 与首尾空白）。
func parseEd25519PrivateKey(raw string) (ed25519.PrivateKey, error) {
	pemText := strings.ReplaceAll(strings.TrimSpace(raw), `\n`, "\n")
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("moomoo OpenAPI：私钥不是合法的 PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：解析 PKCS#8 私钥失败: %w", err)
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("moomoo OpenAPI：私钥不是 Ed25519")
	}
	return priv, nil
}

// GetQuoteSnapshots 批量拉取行情快照（一次最多 400 个 code）。
func (c *MoomooOpenAPIClient) GetQuoteSnapshots(ctx context.Context, codes []string) ([]MoomooSnapshot, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	if len(codes) > MoomooSnapshotMaxCodes {
		return nil, fmt.Errorf("moomoo OpenAPI：单次最多 %d 个 code，实际 %d", MoomooSnapshotMaxCodes, len(codes))
	}
	payload, err := json.Marshal(map[string]any{"code_list": codes})
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：序列化请求体失败: %w", err)
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := randomNonce()
	bodyHash := sha256Hex(payload)
	signPlain := strings.Join([]string{ts, http.MethodPost, MoomooSnapshotPath, "", bodyHash}, "\n")
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(c.priv, []byte(signPlain)))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+MoomooSnapshotPath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.appKey)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("Authorization", signature)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：读取响应失败: %w", err)
	}

	var envelope struct {
		RetCode int                     `json:"ret_code"`
		RetMsg  string                  `json:"ret_msg"`
		Data    *MoomooSnapshotEnvelope `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：解析响应失败（HTTP %d）: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || envelope.RetCode != 0 {
		return nil, fmt.Errorf("moomoo OpenAPI：HTTP %d ret_code=%d ret_msg=%s",
			resp.StatusCode, envelope.RetCode, envelope.RetMsg)
	}
	if envelope.Data == nil {
		return nil, errors.New("moomoo OpenAPI：响应缺少 data")
	}
	return envelope.Data.SnapshotList, nil
}

// MoomooSnapshotEnvelope data 节点。
type MoomooSnapshotEnvelope struct {
	SnapshotList []MoomooSnapshot `json:"snapshot_list"`
}

// sha256Hex 请求体 SHA256 小写十六进制（签名用）。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// randomNonce 生成 X-Nonce（字母数字，1–64 位）。
func randomNonce() string {
	return "n" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

// 编译期断言：默认客户端满足采集器依赖的接口。
var _ MoomooSnapshotFetcher = (*MoomooOpenAPIClient)(nil)
