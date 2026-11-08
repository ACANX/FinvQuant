package secukey

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 本文件实现 Supabase PostgREST 只读客户端（需求 11043）：
//   GET https://<PROJECT_ID>.supabase.co/rest/v1/<源表>?select=...&order=ftmm_symbol.asc&limit=&offset=
//
// 鉴权：`apikey` 与 `Authorization: Bearer` 两个头都带 key（publishable / secret 均可）。
// 总数：`Prefer: count=exact` + 响应头 `Content-Range: 0-999/73212` 的第二段。
// 失败重试：网络错误 / 429 / 5xx 重试 3 次（退避 500ms、1s）；4xx（除 429）直接失败，
// 因为多为表名或 key 配错，重试没有意义。

// 客户端默认参数。
const (
	// defaultFetchTimeout 单批拉取的超时。
	defaultFetchTimeout = 120 * time.Second
	// defaultMaxAttempts 单批拉取的最大尝试次数（含首次）。
	defaultMaxAttempts = 3
	// defaultRetryBackoff 首次重试的退避时长（之后翻倍）。
	defaultRetryBackoff = 500 * time.Millisecond
	// sourceSelect 只取需要同步的列，避免把 note/count_collect 等大字段拉回来。
	sourceSelect = "ftmm_symbol,market,code,instrument,enable,url,priority,status"
)

// Client Supabase PostgREST 客户端（只读，线程安全，可复用）。
type Client struct {
	baseURL      string // https://<project>.supabase.co
	apiKey       string
	httpClient   *http.Client
	maxAttempts  int
	retryBackoff time.Duration
}

// NewClient 按 20013 的解析结果创建客户端。
func NewClient(conn syscfg.SupabaseConn) *Client {
	return &Client{
		baseURL:      conn.BaseURL(),
		apiKey:       conn.APIKey,
		httpClient:   &http.Client{Timeout: defaultFetchTimeout},
		maxAttempts:  defaultMaxAttempts,
		retryBackoff: defaultRetryBackoff,
	}
}

// splitTable 把 `schema.table` 拆成两段；不带 schema 时用 SupabaseDefaultSchema。
func splitTable(table string) (schema, name string) {
	if index := strings.Index(table, "."); index > 0 {
		return table[:index], table[index+1:]
	}
	return syscfg.SupabaseDefaultSchema, table
}

// SourceURL 拼接某批的请求地址（导出供日志与单测核对）。
//
// 注意 PostgREST 的路径只放表名，schema 由 `Accept-Profile` 头指定
// （写成 `/rest/v1/public.t` 会被当成 `public.public.t`，实测返回 PGRST205）。
// 表名必须已通过 syscfg.NormalizeSupabaseTableName 校验（调用方负责）。
func (c *Client) SourceURL(table string, offset, limit int) string {
	_, name := splitTable(table)
	query := strings.Join([]string{
		"select=" + sourceSelect,
		"order=ftmm_symbol.asc",
		"limit=" + strconv.Itoa(limit),
		"offset=" + strconv.Itoa(offset),
	}, "&")
	return c.baseURL + "/rest/v1/" + name + "?" + query
}

// FetchBatch 拉取一批源行（按 ftmm_symbol 升序）。
//
// 返回：本批行、源表总行数（取不到时为 0）。表不存在（404）等 4xx 直接返回错误。
func (c *Client) FetchBatch(ctx context.Context, table string, offset, limit int) ([]SourceRow, int64, error) {
	schema, _ := splitTable(table)
	url := c.SourceURL(table, offset, limit)
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		rows, total, retryable, err := c.fetchOnce(ctx, url, schema)
		if err == nil {
			return rows, total, nil
		}
		lastErr = err
		if !retryable || attempt == c.maxAttempts {
			break
		}
		backoff := c.retryBackoff * time.Duration(attempt)
		log.Printf("[cdeu][secukey] fetch retry %d/%d after %s: %v", attempt, c.maxAttempts, backoff, err)
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, 0, fmt.Errorf("拉取 Supabase 源表失败（offset=%d, limit=%d）: %w", offset, limit, lastErr)
}

// FetchRawBatch 拉取一批**原始 JSON 行**（供「证券指标数据同步」按列透传给数据库）。
//
// selectCols 为 PostgREST 的 select 表达式（如 "*" 或 "a,b,c"）；表名需已通过
// syscfg.NormalizeSupabaseTableName 校验；排序固定 ftmm_symbol.asc（上游唯一且 NOT NULL，
// 保证分页稳定）。返回本批原始行与源表总行数。
func (c *Client) FetchRawBatch(ctx context.Context, table, selectCols string, offset, limit int) ([]json.RawMessage, int64, error) {
	if strings.TrimSpace(selectCols) == "" {
		selectCols = "*"
	}
	schema, name := splitTable(table)
	url := c.baseURL + "/rest/v1/" + name + "?" + strings.Join([]string{
		"select=" + selectCols,
		"order=ftmm_symbol.asc",
		"limit=" + strconv.Itoa(limit),
		"offset=" + strconv.Itoa(offset),
	}, "&")

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		body, total, retryable, err := c.doGet(ctx, url, schema)
		if err == nil {
			var rows []json.RawMessage
			if err := json.Unmarshal(body, &rows); err != nil {
				return nil, 0, fmt.Errorf("解析 Supabase 响应失败: %w", err)
			}
			return rows, total, nil
		}
		lastErr = err
		if !retryable || attempt == c.maxAttempts {
			break
		}
		backoff := c.retryBackoff * time.Duration(attempt)
		log.Printf("[cdeu][secukey] raw fetch retry %d/%d after %s: %v", attempt, c.maxAttempts, backoff, err)
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, 0, fmt.Errorf("拉取 Supabase 源表失败（offset=%d, limit=%d）: %w", offset, limit, lastErr)
}

// fetchOnce 发起一次请求并解析成结构化行；retryable 表示该错误值得重试（网络/429/5xx）。
func (c *Client) fetchOnce(ctx context.Context, url, schema string) (rows []SourceRow, total int64, retryable bool, err error) {
	body, total, retryable, err := c.doGet(ctx, url, schema)
	if err != nil {
		return nil, 0, retryable, err
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, 0, false, fmt.Errorf("解析 Supabase 响应失败: %w", err)
	}
	return rows, total, false, nil
}

// doGet 执行一次 GET 并返回响应体与源表总行数；retryable 表示该错误值得重试。
func (c *Client) doGet(ctx context.Context, url, schema string) (body []byte, total int64, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, false, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Prefer", "count=exact")
	// schema 只能通过 profile 头指定（路径里带 schema 会被 PostgREST 当作两级表名）。
	if schema != "" {
		req.Header.Set("Accept-Profile", schema)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, true, fmt.Errorf("请求 Supabase 失败: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if readErr != nil {
		return nil, 0, true, fmt.Errorf("读取 Supabase 响应失败: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 300 {
			message = message[:300] + "…"
		}
		retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if resp.StatusCode == http.StatusNotFound {
			return nil, 0, false, fmt.Errorf(
				"Supabase 源表不存在或不可读（HTTP 404），请检查源表名与 API Key 权限: %s", message)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, 0, false, fmt.Errorf(
				"Supabase 鉴权失败（HTTP %d），请检查 20013 的 APIKEY: %s", resp.StatusCode, message)
		}
		return nil, 0, retryable, fmt.Errorf("Supabase 返回 HTTP %d: %s", resp.StatusCode, message)
	}

	return body, parseContentRangeTotal(resp.Header.Get("Content-Range")), false, nil
}

// parseContentRangeTotal 从 `Content-Range: 0-999/73212` 解析总行数；`*` 或缺失时返回 0。
func parseContentRangeTotal(contentRange string) int64 {
	slash := strings.LastIndex(contentRange, "/")
	if slash < 0 || slash == len(contentRange)-1 {
		return 0
	}
	total, err := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64)
	if err != nil {
		return 0
	}
	return total
}
