package cdeu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件实现「期货列表」的 HTTP 拉取与**宽松解析**（需求 11274 修正，2026-11-18）。
//
// 为什么不用 quoteapiv2.GetFuturesList：
//
//	线上响应里 `stockId` 是**字符串**（`"stockId": "70000294"`），而上游
//	`quoteapiv2.FuturesListItem.StockID` 声明为 `int64`，`json.Unmarshal` 直接报
//	`cannot unmarshal string into Go struct field ... of type int64`，整个分类采不到数。
//	上游当前最新版本（v0.1.2 / main tip）仍是 int64，本仓库无法通过配置绕过反序列化。
//
// 本实现的边界（刻意收窄，便于上游修复后整体删除）：
//   - **复用**上游导出的签名与协议常量（`quoteapiv2.SignedQuery` / `DefaultReferers` /
//     `DefaultUserAgent` / `CodeOK` 等），不复制签名算法，签名口径与其它接口完全一致；
//   - 只替换「发送请求 + 反序列化」两段，返回值仍是 `quoteapiv2.FuturesListPage`，
//     采集器（quote_list_futures_snapshot_sync.go）与落库层零改动；
//   - 类型不敏感：`stockId` / `fieldId` / `precision` / `contentType` / `pagination.*`
//     以及 `fields` 的值，**字符串与数字都能解析**，避免"修好一个字段又撞下一个"。
//
// 待上游把 `StockID` 改为兼容类型（或 `json.Number`）后：删除本文件，把
// `NewFutureListFetcher` 改回 `quoteapiv2.New(quoteapiv2.Config{})` 即可。

// FutureListHTTPFetcher 期货列表拉取实现（自建请求 + 宽松解析）。
type FutureListHTTPFetcher struct {
	cfg    quoteapiv2.Config
	client *http.Client
}

// NewFutureListFetcher 创建期货列表拉取实现（默认 moomoo 站点）。
//
// 默认值（BaseURL / 路径 / CSRF / Locale / Referer / Referers / Header / Timeout）
// 全部取自上游 `quoteapiv2.Config` 的规范化结果，本文件不硬编码任何协议常量。
func NewFutureListFetcher() FutureListFetcher {
	return newFutureListHTTPFetcher(quoteapiv2.Config{})
}

// newFutureListHTTPFetcher 按给定配置创建（单测注入 BaseURL/HTTPClient 用）。
func newFutureListHTTPFetcher(cfg quoteapiv2.Config) *FutureListHTTPFetcher {
	resolved := quoteapiv2.New(cfg).Config()
	timeout := resolved.Timeout
	if timeout <= 0 {
		timeout = quoteapiv2.DefaultTimeout
	}
	client := resolved.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &FutureListHTTPFetcher{cfg: resolved, client: client}
}

// GetFuturesList 拉取一页期货合约列表。
func (f *FutureListHTTPFetcher) GetFuturesList(ctx context.Context, req quoteapiv2.GetFuturesListRequest) (quoteapiv2.FuturesListPage, error) {
	marketType := strings.TrimSpace(req.MarketType)
	if marketType == "" {
		return quoteapiv2.FuturesListPage{}, errors.New("moomoo GetFuturesList：marketType 不能为空")
	}
	futuresType := strings.TrimSpace(req.FuturesType)
	if futuresType == "" {
		return quoteapiv2.FuturesListPage{}, errors.New("moomoo GetFuturesList：futuresType 不能为空")
	}
	page := strings.TrimSpace(req.Page)
	if page == "" {
		page = quoteapiv2.DefaultFuturesPage
	}
	if value, err := strconv.Atoi(page); err != nil || value < 0 {
		return quoteapiv2.FuturesListPage{}, fmt.Errorf("moomoo GetFuturesList：page 必须是 ≥0 的整数（收到 %q）", page)
	}
	pageSize := strings.TrimSpace(req.PageSize)
	if pageSize == "" {
		pageSize = quoteapiv2.DefaultFuturesPageSize
	}
	if value, err := strconv.Atoi(pageSize); err != nil || value < 1 {
		return quoteapiv2.FuturesListPage{}, fmt.Errorf("moomoo GetFuturesList：pageSize 必须是 ≥1 的整数（收到 %q）", pageSize)
	}

	// 参数顺序即签名键序（与上游一致：marketType / futuresType / page / pageSize / _）。
	params := []quoteapiv2.Param{
		{Name: "marketType", Value: marketType},
		{Name: "futuresType", Value: futuresType},
		{Name: "page", Value: page},
		{Name: "pageSize", Value: pageSize},
	}
	if timestamp := strings.TrimSpace(req.TimestampMillis); timestamp != "" {
		params = append(params, quoteapiv2.Param{Name: "_", Value: timestamp})
	}

	path := f.cfg.FuturesListPath
	target := f.cfg.BaseURL + path + "?" + quoteapiv2.SignedQuery(params)
	body, err := f.do(ctx, path, target, params, marketType)
	if err != nil {
		return quoteapiv2.FuturesListPage{}, err
	}

	var payload futuresRawPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return quoteapiv2.FuturesListPage{}, &quoteapiv2.APIError{
			Path: path, URL: target, Body: futureTruncate(string(body), 500),
			Message: "响应解析失败: " + err.Error(),
		}
	}
	if payload.Code != quoteapiv2.CodeOK {
		return quoteapiv2.FuturesListPage{}, &quoteapiv2.APIError{
			Path: path, URL: target, Code: payload.Code,
			Message: payload.Message, Body: futureTruncate(string(body), 500),
		}
	}

	items := make([]quoteapiv2.FuturesListItem, 0, len(payload.Data.List))
	for index, raw := range payload.Data.List {
		item, convErr := raw.toItem()
		if convErr != nil {
			return quoteapiv2.FuturesListPage{}, &quoteapiv2.APIError{
				Path: path, URL: target, Body: futureTruncate(string(body), 500),
				Message: fmt.Sprintf("响应解析失败: 第 %d 条 %v", index+1, convErr),
			}
		}
		items = append(items, item)
	}
	return quoteapiv2.FuturesListPage{
		Pagination: quoteapiv2.Pagination{
			Page:      futureAtoi(payload.Data.Pagination.Page),
			PageSize:  futureAtoi(payload.Data.Pagination.PageSize),
			PageCount: futureAtoi(payload.Data.Pagination.PageCount),
			Total:     futureAtoi(payload.Data.Pagination.Total),
		},
		Items: items,
	}, nil
}

// do 发送一次 GET 请求（请求头与上游 getList 保持一致）。
func (f *FutureListHTTPFetcher) do(ctx context.Context, path, target string, params []quoteapiv2.Param, marketType string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 moomoo 请求失败: %w", err)
	}
	req.Header.Set("quote-token", quoteapiv2.SignParams(params))
	req.Header.Set("futu-x-csrf-token", f.cfg.CSRF)
	req.Header.Set("cookie", fmt.Sprintf("csrfToken=%s; locale=%s", f.cfg.CSRF, f.cfg.Locale))
	req.Header.Set("accept", "application/json")
	req.Header.Set("referer", f.refererFor(marketType))
	req.Header.Set("user-agent", quoteapiv2.DefaultUserAgent)
	// 扩展头最后应用：同名覆盖内置值，调用方显式值优先（与上游 applyHeader 同序）。
	for key, value := range f.cfg.Header {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		req.Header.Set(name, value)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, &quoteapiv2.APIError{Path: path, URL: target, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, &quoteapiv2.APIError{
			Path: path, URL: target, HTTPStatus: resp.StatusCode, Message: err.Error(),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &quoteapiv2.APIError{
			Path: path, URL: target, HTTPStatus: resp.StatusCode,
			Body: futureTruncate(string(body), 500),
		}
	}
	return body, nil
}

// refererFor 按「市场覆盖 → 全局覆盖 → 市场默认 → 全局默认」选择来源页（与上游同序）。
func (f *FutureListHTTPFetcher) refererFor(marketType string) string {
	if marketType != "" {
		if value := strings.TrimSpace(f.cfg.Referers[marketType]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(f.cfg.Referer); value != "" {
		return value
	}
	if marketType != "" {
		if value := strings.TrimSpace(quoteapiv2.DefaultReferers[marketType]); value != "" {
			return value
		}
	}
	return quoteapiv2.DefaultReferer
}

// futuresRawPayload 响应外层（code/message/data）。
type futuresRawPayload struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    futuresRawData `json:"data"`
}

// futuresRawData 响应 data（pagination + list）。
type futuresRawData struct {
	Pagination futuresRawPagination `json:"pagination"`
	List       []futuresRawItem     `json:"list"`
}

// futuresRawPagination 分页元信息（数字/字符串都能解析）。
type futuresRawPagination struct {
	Page      futureFlexText `json:"page"`
	PageSize  futureFlexText `json:"pageSize"`
	PageCount futureFlexText `json:"pageCount"`
	Total     futureFlexText `json:"total"`
}

// futuresRawItem 一条合约（数字/字符串都能解析）。
type futuresRawItem struct {
	StockID     futureFlexText            `json:"stockId"`
	PriceDirect futureFlexText            `json:"priceDirect"`
	FieldList   []futuresRawField         `json:"fieldList"`
	Fields      map[string]futureFlexText `json:"fields"`
}

// futuresRawField 位置化字段定义（数字/字符串都能解析）。
type futuresRawField struct {
	FieldID     futureFlexText `json:"fieldId"`
	FieldName   futureFlexText `json:"fieldName"`
	Value       futureFlexText `json:"value"`
	Text        futureFlexText `json:"text"`
	Precision   futureFlexText `json:"precision"`
	ContentType futureFlexText `json:"contentType"`
}

// toItem 转成上游结构（stockId 在此处才转 int64，失败即报出是哪一条）。
func (raw futuresRawItem) toItem() (quoteapiv2.FuturesListItem, error) {
	item := quoteapiv2.FuturesListItem{
		PriceDirect: strings.TrimSpace(string(raw.PriceDirect)),
		Fields:      make(map[string]string, len(raw.Fields)),
	}
	if text := strings.TrimSpace(string(raw.StockID)); text != "" {
		stockID, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return item, fmt.Errorf("stockId=%q 不是整数: %w", text, err)
		}
		item.StockID = stockID
	}
	for fieldID, value := range raw.Fields {
		item.Fields[strings.TrimSpace(fieldID)] = string(value)
	}
	for _, field := range raw.FieldList {
		item.FieldList = append(item.FieldList, quoteapiv2.FuturesField{
			FieldID:     futureAtoi(field.FieldID),
			FieldName:   string(field.FieldName),
			Value:       string(field.Value),
			Text:        string(field.Text),
			Precision:   futureAtoi(field.Precision),
			ContentType: futureAtoi(field.ContentType),
		})
	}
	return item, nil
}

// futureFlexText 接受 JSON 字符串 / 数字 / 布尔 / null，统一成去掉首尾空白的字符串。
//
// 期货接口的数值字段类型不稳定（部分环境 `stockId` 是字符串、`pageSize` 是数字），
// 用同一个类型兜住，避免逐字段试错。
type futureFlexText string

func (v *futureFlexText) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	if text[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*v = futureFlexText(strings.TrimSpace(value))
		return nil
	}
	*v = futureFlexText(text)
	return nil
}

// futureAtoi 宽松转整数：非法或空串按 0 处理（这些字段只用于分页/字段编号展示）。
func futureAtoi(value futureFlexText) int {
	number, err := strconv.Atoi(strings.TrimSpace(string(value)))
	if err != nil {
		return 0
	}
	return number
}

// futureTruncate 截断过长的响应体，避免日志膨胀（与上游 truncate 同口径）。
func futureTruncate(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

// 编译期断言：本实现满足采集器的拉取接口。
var _ FutureListFetcher = (*FutureListHTTPFetcher)(nil)
