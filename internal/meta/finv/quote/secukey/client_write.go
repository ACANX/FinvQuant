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

// 本文件实现 Supabase PostgREST 的**按主键取值 + 单列回写**两个通用动作
// （需求 11054「上游 usc 回填」用；Tab 1/2 都是只读上游，不会用到这些方法）：
//
//	GET   /rest/v1/<表>?select=<列>&<键列>=in.(1,2,3)   批量取上游若干主键行
//	PATCH /rest/v1/<表>?<键列>=eq.<值>                 把一行的一个文本列改成指定值
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

// patchConcurrency 回写时的并发度（逐行走 PATCH，串行太慢）。
const patchConcurrency = 8

// FetchByIntIDs 按整型主键批量取上游行（原始 JSON），返回的列由 columns 指定。
//
// ids 超过 maxIDsPerRequest 时自动拆分多次请求并合并结果；排序固定按键列升序，
// 保证同一批请求的顺序稳定（便于日志核对）。
func (c *Client) FetchByIntIDs(ctx context.Context, table, columns, keyColumn string, ids []int64) ([]json.RawMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	schema, name := splitTable(table)
	if strings.TrimSpace(columns) == "" {
		columns = "*"
	}
	rows := make([]json.RawMessage, 0, len(ids))
	for start := 0; start < len(ids); start += maxIDsPerRequest {
		end := start + maxIDsPerRequest
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		parts := make([]string, 0, len(chunk))
		for _, id := range chunk {
			parts = append(parts, strconv.FormatInt(id, 10))
		}
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
				return nil, fmt.Errorf("按 %s 取上游行失败（%d 个键）: %w", keyColumn, len(chunk), lastErr)
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
	}
	return rows, nil
}

// PatchTextColumn 把键列等于 id 的那一行的 column 改成 value。
//
// 用 `Prefer: return=minimal, count=exact` 校验受影响行数：必须正好 1 行，
// 否则返回可读错误（0 行多为 RLS 拒绝或该行已不存在）。
func (c *Client) PatchTextColumn(ctx context.Context, table, keyColumn string, id int64, column, value string) error {
	schema, name := splitTable(table)
	payload, err := json.Marshal(map[string]string{column: value})
	if err != nil {
		return fmt.Errorf("序列化回写内容失败: %w", err)
	}
	endpoint := c.baseURL + "/rest/v1/" + name + "?" + keyColumn + "=eq." +
		url.QueryEscape(strconv.FormatInt(id, 10))

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		affected, retryable, err := c.patchOnce(ctx, endpoint, schema, payload)
		if err == nil {
			if affected != 1 {
				return fmt.Errorf("上游 %s.%s 未按预期更新：%s=%d 受影响 %d 行（多为 RLS 拒绝或该行不存在）",
					name, column, keyColumn, id, affected)
			}
			return nil
		}
		lastErr = err
		if !retryable || attempt == c.maxAttempts {
			return fmt.Errorf("回写上游 %s.%s 失败（%s=%d）: %w", name, column, keyColumn, id, lastErr)
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
	// PATCH 的 count 形如 `Content-Range: */3676`；缺失时按 1 行处理（200 + representation 场景）。
	return parseContentRangeTotal(resp.Header.Get("Content-Range")), false, nil
}
