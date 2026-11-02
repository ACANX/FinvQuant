package webquotev2

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

// 协议默认值（见 1108-Kline.md §1、§2.4）。
const (
	// DefaultBaseURL 接口站点。
	DefaultBaseURL = "https://www.moomoo.com"
	// DefaultKlinePath K 线接口路径。
	DefaultKlinePath = "/quote-api/quote-v2/get-kline"
	// DefaultLocale Cookie 语言标记。
	DefaultLocale = "zh-cn"
	// DefaultReferer 请求来源页。
	DefaultReferer = "https://www.moomoo.com/"
	// DefaultUserAgent 浏览器 UA（站点对非浏览器 UA 可能直接拒绝）。
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	// DefaultTimeout 单次请求超时。
	DefaultTimeout = 20 * time.Second
	// KlineTypeDay 日 K（Dictionaries.md §13）。
	KlineTypeDay = "2"
)

// 业务错误码（CommonObjects.md §6）。
const (
	CodeOK           = 0
	CodeTokenInvalid = -403
)

// APIError 接口层错误（HTTP 或业务码失败）。
type APIError struct {
	Path       string
	URL        string
	HTTPStatus int
	Code       int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	switch {
	case e.Code != 0:
		return fmt.Sprintf("moomoo %s 调用失败：code=%d message=%s（url=%s）", e.Path, e.Code, e.Message, e.URL)
	case e.HTTPStatus != 0:
		return fmt.Sprintf("moomoo %s HTTP 错误：%d（url=%s）", e.Path, e.HTTPStatus, e.URL)
	default:
		return fmt.Sprintf("moomoo %s 请求失败（url=%s）", e.Path, e.URL)
	}
}

// ErrTokenRejected 便捷判断：quote-token 被服务端拒绝（通常意味着签名输入构造有误）。
func ErrTokenRejected(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == CodeTokenInvalid
}

// Config 客户端配置；零值即协议默认值（站点、路径、超时）。
type Config struct {
	BaseURL string
	Path    string
	// CSRF 对应请求头 futu-x-csrf-token 与 Cookie 中的 csrfToken。
	// get-kline 实测不校验该值，但仍需存在（1108 §2.4），默认为占位值。
	CSRF string
	// Locale Cookie 语言标记，默认 zh-cn。
	Locale string
	// Timeout 单次请求超时，默认 20s。
	Timeout time.Duration
	// HTTPClient 可注入自定义 http.Client（测试用）。
	HTTPClient *http.Client
	// Now 可注入时钟（测试签名与时间戳用）。
	Now func() time.Time
}

func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = DefaultBaseURL
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if strings.TrimSpace(c.Path) == "" {
		c.Path = DefaultKlinePath
	}
	if !strings.HasPrefix(c.Path, "/") {
		c.Path = "/" + c.Path
	}
	if strings.TrimSpace(c.CSRF) == "" {
		// 服务端不校验，但请求头必须存在；占位值避免误以为需要登录态。
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

// Client moomoo 行情接口客户端（当前只实现 get-kline）。
type Client struct {
	cfg  Config
	http *http.Client
}

// New 创建客户端（零值配置按协议默认值补齐）。
func New(cfg Config) *Client {
	cfg = cfg.withDefaults()
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &Client{cfg: cfg, http: client}
}

// Config 返回生效配置（已补默认值）。
func (c *Client) Config() Config { return c.cfg }

// KlineRequest K 线请求参数。
//
// 五个定位参数一律来自 futu.finv_futu_quote_collect（stock_id / market_type /
// market_code / instrument_type / sub_instrument_type），不做任何语义换算：
// 同一编号在不同接口含义不同（Dictionaries.md §3），透传才是正确做法。
type KlineRequest struct {
	StockID           string
	MarketType        string
	KlineType         string // 2=日K；为空取 KlineTypeDay
	MarketCode        string
	InstrumentType    string
	SubInstrumentType string
	TimestampMillis   string // 毫秒；为空自动取当前时间
}

// Bar 一根 K 线（原始口径：时间为秒级 UNIX 时间戳，价格为字符串避免精度损失）。
type Bar struct {
	Time        int64  // 秒
	Open        string // 原始字符串，可能为空
	High        string
	Low         string
	Close       string
	Volume      float64
	Turnover    float64
	ChangeRatio *float64 // 响应 r，样本中均为 null
	LastClose   float64
	ChangePrice string // 含符号的涨跌额字符串
}

// Kline 拉取全量 K 线序列（接口无分页、无时间范围参数，一次返回全部历史）。
//
// 返回值按时间升序（最早在前），与接口一致；区间过滤由调用方完成。
func (c *Client) Kline(ctx context.Context, req KlineRequest) ([]Bar, error) {
	if strings.TrimSpace(req.StockID) == "" {
		return nil, errors.New("moomoo Kline：stockId 不能为空")
	}
	klineType := strings.TrimSpace(req.KlineType)
	if klineType == "" {
		klineType = KlineTypeDay
	}
	timestamp := strings.TrimSpace(req.TimestampMillis)
	if timestamp == "" {
		timestamp = strconv.FormatInt(c.cfg.Now().UnixMilli(), 10)
	}
	// 键序即签名键序（规则 R2 / MoomooQuoteV2WebSignature.md §5）。
	params := []Param{
		{Name: "stockId", Value: strings.TrimSpace(req.StockID)},
		{Name: "marketType", Value: strings.TrimSpace(req.MarketType)},
		{Name: "type", Value: klineType},
		{Name: "marketCode", Value: strings.TrimSpace(req.MarketCode)},
		{Name: "instrumentType", Value: strings.TrimSpace(req.InstrumentType)},
		{Name: "subInstrumentType", Value: strings.TrimSpace(req.SubInstrumentType)},
		{Name: "_", Value: timestamp},
	}

	body, err := c.get(ctx, params)
	if err != nil {
		return nil, err
	}
	var payload klinePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &APIError{
			Path: c.cfg.Path, URL: c.url(params), Body: truncate(string(body), 500),
			Message: "响应解析失败: " + err.Error(),
		}
	}
	if payload.Code != CodeOK {
		return nil, &APIError{
			Path: c.cfg.Path, URL: c.url(params), Code: payload.Code,
			Message: payload.Message, Body: truncate(string(body), 500),
		}
	}
	return payload.Data.Bars(), nil
}

// get 发一次 GET 请求并返回响应体（非 200 视为错误）。
func (c *Client) get(ctx context.Context, params []Param) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(params), nil)
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
		return nil, &APIError{Path: c.cfg.Path, URL: req.URL.String(), Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, &APIError{Path: c.cfg.Path, URL: req.URL.String(), HTTPStatus: resp.StatusCode, Message: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			Path: c.cfg.Path, URL: req.URL.String(), HTTPStatus: resp.StatusCode,
			Body: truncate(string(body), 500),
		}
	}
	return body, nil
}

func (c *Client) url(params []Param) string {
	return c.cfg.BaseURL + c.cfg.Path + "?" + SignedQuery(params)
}

// klinePayload 响应外层包裹（CommonObjects.md §2）。
type klinePayload struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    klineData `json:"data"`
}

// klineItem 接口原样字段（1108-Kline.md §3.2）：o/c/h/l/cp 为字符串，
// k/v/t/lc 为数值，类型混用是接口行为，这里用 flexNum 兼容两种写法。
type klineItem struct {
	K  int64    `json:"k"`
	O  flexNum  `json:"o"`
	H  flexNum  `json:"h"`
	L  flexNum  `json:"l"`
	C  flexNum  `json:"c"`
	V  flexNum  `json:"v"`
	T  flexNum  `json:"t"`
	R  *flexNum `json:"r"`
	LC flexNum  `json:"lc"`
	CP flexNum  `json:"cp"`
}

// klineData data 字段。
type klineData struct {
	List []klineItem `json:"list"`
}

func (p klineData) Bars() []Bar {
	bars := make([]Bar, 0, len(p.List))
	for _, item := range p.List {
		bar := Bar{
			Time:        item.K,
			Open:        item.O.String(),
			High:        item.H.String(),
			Low:         item.L.String(),
			Close:       item.C.String(),
			Volume:      item.V.Float(),
			Turnover:    item.T.Float(),
			LastClose:   item.LC.Float(),
			ChangePrice: item.CP.String(),
		}
		if item.R != nil {
			ratio := item.R.Float()
			bar.ChangeRatio = &ratio
		}
		bars = append(bars, bar)
	}
	return bars
}

// flexNum 兼容「字符串数字」与「数字」两种 JSON 表示。
type flexNum string

func (n *flexNum) UnmarshalJSON(data []byte) error {
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
		*n = flexNum(s)
		return nil
	}
	*n = flexNum(text)
	return nil
}

func (n flexNum) String() string { return string(n) }

// Float 数值化；空串/非法值返回 0（接口对缺失量给 0 而非 null）。
func (n flexNum) Float() float64 {
	if n == "" {
		return 0
	}
	value, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		return 0
	}
	return value
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
