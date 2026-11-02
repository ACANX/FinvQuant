// Package quoteapiv2 封装 moomoo（富途）网页行情接口 /quote-api/quote-v2/*。
//
// 本包只做「HTTP 协议 → 统一结构」的转换，不涉及存储与业务编排，结果交给调用方
// 自行处理。
//
// 本包镜像自 MetaExt 仓库 component/quote/moomoo/quoteapiv2（上游按「每个接口一个
// 源文件」组织：client.go 只放配置、错误与请求发送等公共部分，kline.go 放该接口的
// 请求参数、返回结构与解析）。同步上游时保留同名同签名，便于直接对照与移植。
//
// 相对上游的裁剪点（同步上游时不要当成漏搬）：
//
//   - 只保留 kline 接口，不提供其它接口的路径常量与 Config 字段；
//   - 不提供 post/do 与 DefaultOrigin（本包只有 GET 接口）；
//   - 不提供 IsParamsError / IsParamsMissing 与 500/404 业务码（仅其它接口用到）；
//   - 不提供 defaultIfEmpty / intParam（仅其它接口的参数校验用）。
//
// 对外方法名与接口路径保持映射关系：取路径末段按驼峰命名，例如 /quote-api/quote-v2/
// get-kline 对应 GetKLine。请求参数与返回类型同样带上接口前缀，避免不同接口之间用
// 泛泛的通用名。
//
// 请求流程上有两点必须遵守：
//
//   - 查询串的键序就是签名输入的键序，不能用 map 或 url.Values 排序后发送；
//   - 所有参数值都要先字符串化再参与签名，数值类型的差异会导致 token 不一致。
package quoteapiv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 协议默认值。
const (
	// DefaultBaseURL 接口站点。
	DefaultBaseURL = "https://www.moomoo.com"
	// DefaultKLinePath K 线接口路径。
	DefaultKLinePath = "/quote-api/quote-v2/get-kline"
	// DefaultLocale Cookie 中的语言标记。
	DefaultLocale = "zh-cn"
	// DefaultReferer 请求来源页。
	DefaultReferer = "https://www.moomoo.com/"
	// DefaultUserAgent 浏览器 UA；站点对非浏览器 UA 可能直接拒绝。
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	// DefaultTimeout 单次请求超时。
	DefaultTimeout = 20 * time.Second
	// KLineTypeDay 日 K 线类型编号。
	KLineTypeDay = "2"
)

// 业务错误码。
const (
	// CodeOK 业务成功。
	CodeOK = 0
	// CodeTokenInvalid quote-token 被服务端判定无效。
	CodeTokenInvalid = -403
)

// APIError 接口层错误：HTTP 失败或业务码非成功。
type APIError struct {
	// Path 本次调用的接口路径，用于区分同一客户端下的不同接口。
	Path string
	// URL 完整请求地址（含查询串），便于排查签名与参数问题。
	URL string
	// HTTPStatus HTTP 状态码；请求未到达服务端时为 0。
	HTTPStatus int
	// Code 业务错误码；HTTP 层失败时为 0。
	Code int
	// Message 服务端返回的错误描述或本地错误描述。
	Message string
	// Body 服务端原始响应体（已截断），便于定位解析失败原因。
	Body string
}

func (e *APIError) Error() string {
	switch {
	case e.Code != 0:
		return fmt.Sprintf("moomoo %s 调用失败：code=%d message=%s（url=%s）", e.Path, e.Code, e.Message, e.URL)
	case e.HTTPStatus != 0:
		return fmt.Sprintf("moomoo %s HTTP 错误：%d（url=%s）", e.Path, e.HTTPStatus, e.URL)
	case e.Message != "":
		// 解析失败、网络错误等情况下没有状态码与业务码，必须带上原因。
		return fmt.Sprintf("moomoo %s 请求失败：%s（url=%s）", e.Path, e.Message, e.URL)
	default:
		return fmt.Sprintf("moomoo %s 请求失败（url=%s）", e.Path, e.URL)
	}
}

// IsTokenRejected 判断错误是否为 quote-token 被服务端拒绝。
//
// 出现该错误通常意味着签名输入构造有误（键序、值字符串化或参数遗漏），
// 而不是站点风控。
func IsTokenRejected(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == CodeTokenInvalid
}

// Config 客户端配置；零值代表使用协议默认值。
type Config struct {
	// BaseURL 接口站点，默认 https://www.moomoo.com，结尾斜杠会被去掉。
	BaseURL string
	// KLinePath K 线接口路径，默认 DefaultKLinePath。
	KLinePath string
	// CSRF 对应请求头 futu-x-csrf-token 与 Cookie 中的 csrfToken。
	// 实测该接口不校验该值，但请求中必须存在，故默认使用占位值。
	CSRF string
	// Locale Cookie 语言标记，默认 zh-cn。
	Locale string
	// Timeout 单次请求超时，默认 20s。
	Timeout time.Duration
	// HTTPClient 可注入自定义客户端，主要用于测试与代理场景。
	HTTPClient *http.Client
	// Now 可注入时钟，用于测试时间戳与签名。
	Now func() time.Time
}

// withDefaults 补齐零值字段并规整路径格式。
func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = DefaultBaseURL
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	c.KLinePath = normalizePath(c.KLinePath, DefaultKLinePath)
	if strings.TrimSpace(c.CSRF) == "" {
		// 服务端不校验该值，但请求中必须存在，用占位值避免误以为需要登录态。
		c.CSRF = "0000000000"
	}
	if strings.TrimSpace(c.Locale) == "" {
		c.Locale = DefaultLocale
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// normalizePath 为空时取默认值，并补上前导斜杠。
func normalizePath(path, fallback string) string {
	if strings.TrimSpace(path) == "" {
		path = fallback
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// Client moomoo 行情接口客户端。
//
// Client 并发安全：创建后仅读取配置，内部不保存可变状态。
type Client struct {
	cfg  Config
	http *http.Client
}

// New 创建客户端，零值配置按协议默认值补齐。
func New(cfg Config) *Client {
	cfg = cfg.withDefaults()
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{cfg: cfg, http: client}
}

// Config 返回生效配置（已补齐默认值）。
func (c *Client) Config() Config { return c.cfg }

// get 按有序参数向指定路径发一次 GET 请求；签名输入为查询参数。
func (c *Client) get(ctx context.Context, path string, params []Param) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(path, params), nil)
	if err != nil {
		return nil, fmt.Errorf("构造 moomoo 请求失败: %w", err)
	}
	req.Header.Set("quote-token", SignParams(params))
	req.Header.Set("futu-x-csrf-token", c.cfg.CSRF)
	req.Header.Set("cookie", fmt.Sprintf("csrfToken=%s; locale=%s", c.cfg.CSRF, c.cfg.Locale))
	req.Header.Set("accept", "application/json")
	req.Header.Set("referer", DefaultReferer)
	req.Header.Set("user-agent", DefaultUserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &APIError{Path: path, URL: req.URL.String(), Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, &APIError{Path: path, URL: req.URL.String(), HTTPStatus: resp.StatusCode, Message: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			Path: path, URL: req.URL.String(), HTTPStatus: resp.StatusCode,
			Body: truncate(string(respBody), 500),
		}
	}
	return respBody, nil
}

// url 拼接完整请求地址。
func (c *Client) url(path string, params []Param) string {
	return c.cfg.BaseURL + path + "?" + SignedQuery(params)
}

// decode 检查业务码；非成功时按服务端返回的 message 构造错误，成功时返回 nil。
//
// path 与 url 只用于构造错误信息，便于定位是哪个接口、哪组参数出的问题。
func decode(body []byte, path string, url string, code int, message string) error {
	if code != CodeOK {
		return &APIError{
			Path: path, URL: url, Code: code,
			Message: message, Body: truncate(string(body), 500),
		}
	}
	return nil
}

// FlexNum 兼容「字符串数字」与「数字」两种 JSON 表示。
type FlexNum string

// UnmarshalJSON 接受字符串、数字与 null 三种形式。
func (n *FlexNum) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*n = ""
		return nil
	}
	if text[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*n = FlexNum(s)
		return nil
	}
	*n = FlexNum(text)
	return nil
}

// String 返回原始字符串形式。
func (n FlexNum) String() string { return string(n) }

// Float 转为 float64；空串或非法值返回 0（接口对缺失量给 0 而非 null）。
func (n FlexNum) Float() float64 {
	if n == "" {
		return 0
	}
	value, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		return 0
	}
	return value
}

// truncate 截断过长的响应体，避免日志膨胀。
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
