package cdeu

// moomoo 网页接口的代理通道（需求 11337）。
//
// 背景（2026-11-19 实测）：直连 `https://www.moomoo.com/quote-api/...` 会被按 IP 阻断——
// 连未签名的 `/api/v1.0/server-time` 与站点首页都 `Recv failure: Connection reset by peer`，
// 服务端采集报 `Get "…get-stock-list?…": EOF`。同一时刻经 Cloudflare Worker 中转：
// 15/15 次 `200` + JSON、翻 6 页 6/6 成功（出口 IP 从本地宽带变为 AS13335 Cloudflare）。
//
// 通道形态：Worker 用 `POST {base_url}` + JSON `{method,target,headers,...}` 转发目标请求，
// 并把目标的状态码/头/体原样返回。因此这里实现 http.RoundTripper：
// 上层（MetaExt quoteapiv2）照常构造签名请求，由本层打包转发，解析与错误处理完全不用改。
//
// 配置只有**一个**系统配置项（用户要求不拆两项）：
//
//	sys.sys_config 20018  moomoo 网页接口代理配置
//	值格式：{enable}|{base_url}
//	  enable   ：1/true/on 启用；0/false/off/空 直连（默认）
//	  base_url ：代理端点根地址（由用户自行配置，代码与迁移都不预置具体端点）
//	环境变量兜底（同格式）：FINV_MOOMOO_QUOTE_PROXY
//
// 只作用于网页接口（quote-api，全部是 GET），**不作用于** moomoo OpenAPI（webapi）——
// AppKey 与 Ed25519 签名不应经过第三方中转。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
	"github.com/jackc/pgx/v5"
)

// SysConfigMoomooQuoteProxy moomoo 网页接口代理配置编号（V300231 发放）。
const SysConfigMoomooQuoteProxy = 20018

// EnvMoomooQuoteProxy 环境变量兜底（与 20018 同格式 `{enable}|{base_url}`）。
const EnvMoomooQuoteProxy = "FINV_MOOMOO_QUOTE_PROXY"

// moomooProxyDefaultTimeout 单次转发请求的兜底超时（上层 Context 更短时以更短的为准）。
const moomooProxyDefaultTimeout = 40 * time.Second

// MoomooQuoteProxyConfig 代理通道配置。
type MoomooQuoteProxyConfig struct {
	// Enabled 是否启用代理（配置项 enable 段为 1/true/on）。
	Enabled bool
	// BaseURL 代理端点根地址（已去掉末尾斜杠）。
	BaseURL string
}

// Active 配置是否足以启用代理（启用标志 + 端点地址都齐）。
func (c MoomooQuoteProxyConfig) Active() bool {
	return c.Enabled && strings.TrimSpace(c.BaseURL) != ""
}

// String 日志用描述（不含任何凭据，代理地址本身不是敏感信息）。
func (c MoomooQuoteProxyConfig) String() string {
	if !c.Active() {
		return "未启用（直连）"
	}
	return "已启用 → " + c.BaseURL
}

// parseMoomooQuoteProxy 解析 `{enable}|{base_url}`；没有竖线时整串按 base_url 处理（兼容只填地址）。
func parseMoomooQuoteProxy(raw string) MoomooQuoteProxyConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MoomooQuoteProxyConfig{}
	}
	head, tail, ok := strings.Cut(raw, "|")
	if !ok {
		// 只填了地址：视为启用（`1|` 的省略写法）
		return MoomooQuoteProxyConfig{Enabled: true, BaseURL: strings.TrimRight(strings.TrimSpace(raw), "/")}
	}
	return MoomooQuoteProxyConfig{
		Enabled: moomooProxyEnabledValue(head),
		BaseURL: strings.TrimRight(strings.TrimSpace(tail), "/"),
	}
}

// moomooProxyEnabledValue 启用标志取值：1/true/on/yes/y = 启用，其它（含空）= 关闭。
func moomooProxyEnabledValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "on", "yes", "y":
		return true
	}
	return false
}

// LoadMoomooQuoteProxyConfig 读取代理配置：sys.sys_config 20018 优先，环境变量兜底。
//
// 缺行或读库出错时不阻断采集，按「未启用」处理（代理是可选通道，坏配置不应连直连也受影响）。
func (s *Store) LoadMoomooQuoteProxyConfig(ctx context.Context) (MoomooQuoteProxyConfig, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(str_value, '')
  FROM sys.sys_config
 WHERE config_no = $1`, SysConfigMoomooQuoteProxy).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return MoomooQuoteProxyConfig{}, fmt.Errorf("读取 moomoo 代理配置失败: %w", err)
	}
	cfg := parseMoomooQuoteProxy(raw)
	if cfg.Active() {
		return cfg, nil
	}
	if env := strings.TrimSpace(os.Getenv(EnvMoomooQuoteProxy)); env != "" {
		if envCfg := parseMoomooQuoteProxy(env); envCfg.Active() {
			return envCfg, nil
		}
	}
	return cfg, nil
}

// moomooProxyTransport 把 GET 请求打包成 Worker 的 POST JSON 转发，再把目标响应原样返回。
//
// 关键约束：
//   - target 必须是**原始完整 URL**（含 query）：quote-token 签名只覆盖查询参数、不含 host，
//     所以换域名转发不会破坏签名；但只要改动/新增任何 query 参数就会失效。
//   - 请求头（user-agent / referer / accept-language / quote-token / cookie 等）全部透传；
//     Worker 的说明是「默认启用浏览器伪装头部」，实测调用方显式传的头优先。
//   - 目标状态码、响应头、响应体原样透传，上层解析/错误逻辑无需感知代理。
//   - 只支持 GET：本仓库用到的 quote-api 接口（列表/ETF/期货/汇率/K 线）全是 GET；
//     带请求体的 POST（期权报价，本仓库未使用）会明确报错，不做猜测性降级。
type moomooProxyTransport struct {
	// baseURL Worker 端点根地址（不含末尾斜杠）。
	baseURL string
	// client 访问 Worker 用的客户端（不经本 Transport，避免递归）。
	client *http.Client
}

// newMoomooProxyTransport 创建代理 Transport。
func newMoomooProxyTransport(baseURL string) *moomooProxyTransport {
	return &moomooProxyTransport{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: moomooProxyDefaultTimeout},
	}
}

// moomooProxyRequest Worker POST 模式的请求体（字段名以 Worker 说明为准）。
type moomooProxyRequest struct {
	Method       string            `json:"method"`
	Target       string            `json:"target"`
	Headers      map[string]string `json:"headers,omitempty"`
	Timeout      int               `json:"timeout"`
	RetryCount   int               `json:"retryCount"`
	ResponseType string            `json:"responseType"`
	CFOptions    map[string]any    `json:"cfOptions"`
}

// RoundTrip 实现 http.RoundTripper。
func (t *moomooProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.baseURL == "" {
		return nil, errors.New("moomoo 代理：配置已启用但端点地址为空")
	}
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("moomoo 代理：暂只支持 GET 请求，收到 %s（%s）——请关闭代理后重试",
			req.Method, req.URL.Path)
	}

	headers := make(map[string]string, len(req.Header))
	for name, values := range req.Header {
		if len(values) == 0 {
			continue
		}
		headers[strings.ToLower(name)] = values[0]
	}

	payload := moomooProxyRequest{
		Method:       req.Method,
		Target:       req.URL.String(),
		Headers:      headers,
		Timeout:      int(moomooProxyDefaultTimeout / time.Millisecond),
		RetryCount:   0,
		ResponseType: "text",
		// 行情按页取，必须禁用 Worker 缓存（实测 cf-cache-status: MISS）。
		CFOptions: map[string]any{"cacheEverything": false},
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("moomoo 代理：序列化转发请求失败: %w", err)
	}

	proxyReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, t.baseURL,
		bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("moomoo 代理：构造转发请求失败: %w", err)
	}
	proxyReq.Header.Set("content-type", "application/json")

	resp, err := t.client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("moomoo 代理：转发失败（endpoint=%s）: %w", t.baseURL, err)
	}

	// 透传目标响应，但去掉编码/长度相关头：Worker 已按明文返回，
	// 保留 content-encoding 会让上层误以为还要解压（上层不会解压自定义 Transport 的响应）。
	header := make(http.Header, len(resp.Header))
	for name, values := range resp.Header {
		switch strings.ToLower(name) {
		case "content-encoding", "content-length", "transfer-encoding", "connection":
			continue
		}
		header[name] = values
	}
	resp.Header = header
	// 响应体已由 Worker 解压并按明文返回：清掉长度与分块标记，
	// 否则 http.Client 会按 chunked/gzip 再解一次，读取出错（症状是「HTTP 错误：200」）。
	resp.ContentLength = -1
	resp.TransferEncoding = nil
	resp.Uncompressed = true
	resp.Request = req
	return resp, nil
}

// newMoomooQuoteClientWithProxy 按代理配置创建客户端；未启用时等价于直连。
//
// 注意：注入的是 `HTTPClient.Transport`，不是 `BaseURL`——Worker 的自定义头只能通过
// `POST {base_url}` 的 JSON 模式传递，路径式替换带不上 `quote-token`（实测简单模式失败）。
func newMoomooQuoteClientWithProxy(proxy MoomooQuoteProxyConfig) *quoteapiv2.Client {
	cfg := quoteapiv2.Config{
		Header: map[string]string{
			"accept-language": moomooWebAcceptLanguage,
		},
	}
	if proxy.Active() {
		cfg.HTTPClient = &http.Client{Transport: newMoomooProxyTransport(proxy.BaseURL)}
	}
	return quoteapiv2.New(cfg)
}
