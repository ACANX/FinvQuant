package secukey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 本文件实现 Supabase PostgREST 的**按主键取值 + 按主键回写**通用动作
// （需求 11054/11056「上游主键参数回填」用；Tab 1/2 都是只读上游，不会用到这些方法）：
//
//	GET   /rest/v1/<表>?select=<列>&<键列>=in.(k1,k2,...)   批量取上游若干主键行
//	PATCH /rest/v1/<表>?<键列>=eq.<值>                     把一行的若干列改成指定值
//
// 与只读客户端一致的两点：
//   - schema 只能走 `Accept-Profile`（读）/ `Content-Profile`（写）头，路径里不放 schema；
//   - 网络错误 / 429 / 5xx 重试 3 次，4xx（除 429）直接失败。
//
// 写操作额外用 `Prefer: return=minimal, count=exact` 校验「受影响行数」：
// RLS 拒绝时 PostgREST 会返回 204 但影响 0 行，若不校验会被误判成写入成功。

// maxIDsPerRequest 单次 in.(...) 的主键个数上限。
//
// URL 长度限制来自上游网关（实测 500 个 bigint 约 8KB 仍可通过，取 200 更稳）。
const maxIDsPerRequest = 200

// maxFilterLen 单次 in.(...) 过滤器（编码后）的字符预算。
//
// 证券键里存在超长形态（实测最长的 quote-etf-… 系约 60 字符），200 个这样的键会把 URL 撑到
// 10KB 以上；上游网关按 URL 长度截断后 PostgREST 只会报「failed to parse filter」这种
// 看不出根因的错误。这里按字符预算再分一片，与个数上限同时生效。
const maxFilterLen = 4000

// FetchByIntIDs 按整型主键批量取上游行（原始 JSON），返回的列由 columns 指定。
//
// ids 超过 maxIDsPerRequest 时自动拆分多次请求并合并结果。
func (c *Client) FetchByIntIDs(ctx context.Context, table, columns, keyColumn string, ids []int64) ([]json.RawMessage, error) {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	return c.fetchByKeys(ctx, table, columns, keyColumn, keys)
}

// FetchByTextKeys 按文本主键（如 ftmm_symbol）批量取上游行（原始 JSON）。
func (c *Client) FetchByTextKeys(ctx context.Context, table, columns, keyColumn string, keys []string) ([]json.RawMessage, error) {
	return c.fetchByKeys(ctx, table, columns, keyColumn, keys)
}

// fetchByKeys 按主键列表分批取上游行；排序固定按键列升序，保证同一批请求顺序稳定。
func (c *Client) fetchByKeys(ctx context.Context, table, columns, keyColumn string, keys []string) ([]json.RawMessage, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	schema, name := splitTable(table)
	if strings.TrimSpace(columns) == "" {
		columns = "*"
	}
	tokens := make([]string, 0, len(keys))
	for _, key := range keys {
		tokens = append(tokens, encodeFilterValue(key))
	}

	rows := make([]json.RawMessage, 0, len(keys))
	for start := 0; start < len(tokens); {
		end, size := start, 0
		for end < len(tokens) && end-start < maxIDsPerRequest {
			add := len(tokens[end]) + 1 // +1 为分隔逗号
			if end > start && size+add > maxFilterLen {
				break
			}
			size += add
			end++
		}
		parts := tokens[start:end]
		requestURL := c.baseURL + "/rest/v1/" + name + "?" + strings.Join([]string{
			"select=" + columns,
			"order=" + keyColumn + ".asc",
			keyColumn + "=in.(" + strings.Join(parts, ",") + ")",
		}, "&")

		var lastErr error
		var chunkRows []json.RawMessage
		for attempt := 1; attempt <= c.maxAttempts; attempt++ {
			body, _, retryable, err := c.doGet(ctx, requestURL, schema)
			if err == nil {
				if err := json.Unmarshal(body, &chunkRows); err != nil {
					return nil, fmt.Errorf("解析 Supabase 响应失败: %w", err)
				}
				break
			}
			lastErr = err
			if !retryable || attempt == c.maxAttempts {
				return nil, fmt.Errorf("按 %s 取上游行失败（%d 个键）: %w", keyColumn, len(parts), lastErr)
			}
			backoff := c.retryBackoff * time.Duration(attempt)
			log.Printf("[cdeu][uscbackfill] fetch retry %d/%d after %s: %v", attempt, c.maxAttempts, backoff, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		rows = append(rows, chunkRows...)
		start = end
	}
	return rows, nil
}

// UpsertRows 批量写入若干行：按 conflictColumns 冲突则合并更新，否则插入（需求 11157）。
//
// 用于「本地回推上游」时补写上游缺失的记录：
//   - `Prefer: resolution=merge-duplicates` → 冲突时按主键合并（幂等，重跑不会重复插入）；
//   - `Prefer: return=minimal` → 不要回显整行，只靠 `count=exact` 取受影响行数；
//   - `on_conflict=<列>` → 明确冲突列（上游主键 ftmm_symbol）。
//
// 返回写入（插入 + 合并）的行数；行数不符由调用方判断。
func (c *Client) UpsertRows(ctx context.Context, table, conflictColumns string, rows []map[string]any) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	schema, name := splitTable(table)
	payload, err := json.Marshal(rows)
	if err != nil {
		return 0, fmt.Errorf("序列化写入内容失败: %w", err)
	}
	endpoint := c.baseURL + "/rest/v1/" + name
	if columns := strings.TrimSpace(conflictColumns); columns != "" {
		endpoint += "?on_conflict=" + url.QueryEscape(columns)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		affected, retryable, err := c.upsertOnce(ctx, endpoint, schema, payload)
		if err == nil {
			return int(affected), nil
		}
		lastErr = err
		if !retryable || attempt == c.maxAttempts {
			return 0, fmt.Errorf("写入上游 %s 失败（%d 行）: %w", name, len(rows), lastErr)
		}
		backoff := c.retryBackoff * time.Duration(attempt)
		log.Printf("[cdeu][uscbackfill] upsert retry %d/%d after %s: %v", attempt, c.maxAttempts, backoff, err)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return 0, fmt.Errorf("写入上游 %s 失败（%d 行）: %w", name, len(rows), lastErr)
}

// upsertOnce 发一次 POST（upsert 语义），返回受影响行数；retryable 表示该错误值得重试。
func (c *Client) upsertOnce(ctx context.Context, endpoint, schema string, payload []byte) (affected int64, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, false, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal, resolution=merge-duplicates, count=exact")
	if schema != "" {
		req.Header.Set("Accept-Profile", schema)
		req.Header.Set("Content-Profile", schema)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("请求 Supabase 失败: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return 0, true, fmt.Errorf("读取 Supabase 响应失败: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 300 {
			message = message[:300] + "…"
		}
		retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return 0, false, fmt.Errorf("Supabase 鉴权失败（HTTP %d），请检查 20013 的 APIKEY: %s",
				resp.StatusCode, message)
		}
		return 0, retryable, fmt.Errorf("Supabase 返回 HTTP %d: %s", resp.StatusCode, message)
	}
	return parseContentRangeTotal(resp.Header.Get("Content-Range")), false, nil
}

// PatchColumns 把键列等于 keyValue 的那一行的若干列改成 values。
//
// 用 `Prefer: return=minimal, count=exact` 校验受影响行数：必须正好 1 行，
// 否则返回可读错误（0 行多为 RLS 拒绝或该行已不存在）。
func (c *Client) PatchColumns(ctx context.Context, table, keyColumn, keyValue string, values map[string]any) error {
	schema, name := splitTable(table)
	payload, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("序列化回写内容失败: %w", err)
	}
	endpoint := c.baseURL + "/rest/v1/" + name + "?" + keyColumn + "=eq." + url.QueryEscape(keyValue)

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		affected, retryable, err := c.patchOnce(ctx, endpoint, schema, payload)
		if err == nil {
			if affected != 1 {
				return fmt.Errorf("上游 %s 未按预期更新：%s=%s 受影响 %d 行（多为 RLS 拒绝或该行不存在）",
					name, keyColumn, keyValue, affected)
			}
			return nil
		}
		lastErr = err
		if !retryable || attempt == c.maxAttempts {
			return fmt.Errorf("回写上游 %s 失败（%s=%s）: %w", name, keyColumn, keyValue, lastErr)
		}
		backoff := c.retryBackoff * time.Duration(attempt)
		log.Printf("[cdeu][uscbackfill] patch retry %d/%d after %s: %v", attempt, c.maxAttempts, backoff, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil
}

// patchOnce 发一次 PATCH，返回受影响行数；retryable 表示该错误值得重试。
func (c *Client) patchOnce(ctx context.Context, endpoint, schema string, payload []byte) (affected int64, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, false, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal, count=exact")
	if schema != "" {
		req.Header.Set("Accept-Profile", schema)
		req.Header.Set("Content-Profile", schema)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, true, fmt.Errorf("请求 Supabase 失败: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return 0, true, fmt.Errorf("读取 Supabase 响应失败: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 300 {
			message = message[:300] + "…"
		}
		retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return 0, false, fmt.Errorf("Supabase 鉴权失败（HTTP %d），请检查 20013 的 APIKEY: %s",
				resp.StatusCode, message)
		}
		return 0, retryable, fmt.Errorf("Supabase 返回 HTTP %d: %s", resp.StatusCode, message)
	}
	// 写操作的 count 形如 `*/3676` 或 `0-0/1`（无返回体时）。
	return parseContentRangeTotal(resp.Header.Get("Content-Range")), false, nil
}

// quotePostgRESTValue 把主键值转成 PostgREST `in.(...)` 里的写法。
//
// 值里出现 PostgREST 语法字符（逗号、括号、引号、反斜杠、空白）或**会被 URL 层吃掉**的
// 字符（`&` `%` `+` `#` `?` `=`）时，加双引号并转义；纯数字/字母/点号的键保持原样，
// 避免多引号影响 bigint 的隐式转换。
func quotePostgRESTValue(value string) string {
	if !strings.ContainsAny(value, ",()\"\\\\ \t\n&%+?#=") {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// encodeFilterValue 把主键值转成 `in.(...)` 里可直接拼进 URL 的片段。
//
// 两步：① 按 PostgREST 语法加引号（处理逗号/括号/引号）；② 按 RFC 3986 做百分号编码
// （只保留字母数字与 `-._~`）。第 ② 步是必需的——`ftmm_symbol` 里真实存在 `&`
// （如 `AU.quote-etf-au-index-etfs-s&amp`），未编码时 `&` 会被 URL 解析成参数分隔符，
// 上游只能看到被截断的过滤器，报 `PGRST100 unexpected end of input expecting "," or ")"`。
func encodeFilterValue(value string) string {
	quoted := quotePostgRESTValue(value)
	var b strings.Builder
	b.Grow(len(quoted))
	for i := 0; i < len(quoted); i++ {
		ch := quoted[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9',
			ch == '-', ch == '.', ch == '_', ch == '~':
			b.WriteByte(ch)
		default:
			b.WriteString("%")
			b.WriteByte("0123456789ABCDEF"[ch>>4])
			b.WriteByte("0123456789ABCDEF"[ch&0x0F])
		}
	}
	return b.String()
}
