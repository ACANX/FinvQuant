package secuft

// 「编辑采集配置」的写入实现（需求 11346）。
//
// 与新增的区别：
//   - 新增会填一堆默认值（provider / weight_* / flag_enable=0），编辑**只写调用方明确提交的列**，
//     不补默认值，避免把现网已有值（如 flag_enable=1）覆盖成默认值；
//   - 定位键是 usc（主键不可改）；`usc` / 三列时间列 / 采集统计列都在可编辑范围之外；
//   - 条件命不中任何行（usc 不存在）返回 ErrNotFound，由 handler 映射 404。
//
// 采集统计列（ts_latest_data / count_* / dt_last_check / dt_last_fetch）不在可编辑范围：
// 它们由采集链路自己维护，人工改会让统计口径失真。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
)

// ErrNotFound 目标行不存在（handler 映射 404）。
var ErrNotFound = errors.New("目标记录不存在")

// UpdateInput 编辑一行的请求。
type UpdateInput struct {
	// Usc 定位键（主键，不可修改）。
	Usc string `json:"usc"`
	// Fields 要修改的列；只写请求里出现的列，其它列保持原值。
	Fields map[string]json.RawMessage `json:"fields"`
}

// UpdateResult 编辑结果。
type UpdateResult struct {
	// Usc 定位的 usc。
	Usc string `json:"usc"`
	// Updated 实际写入的列数。
	Updated int `json:"updated"`
	// Row Supabase 返回的更新后行。
	Row Row `json:"row"`
}

// editableColumns 允许人工编辑的列（32 列里去掉 usc、三列时间列、采集统计列）。
//
// `gmt_create` / `gmt_update` / `dt_update` 由本服务写 now()；
// `dt_last_check` / `dt_last_fetch` / `ts_latest_data` / `count_*` 由采集链路维护。
var editableColumns = func() map[string]bool {
	out := make(map[string]bool, len(columns))
	for _, column := range columns {
		switch {
		case column.Name == "usc", column.Kind == KindTimestamp, collectStatColumns[column.Name]:
			continue
		}
		out[column.Name] = true
	}
	return out
}()

// EditableColumns 返回可编辑列名（前端据此决定哪些列在编辑对话框里可改）。
func EditableColumns() []string {
	out := make([]string, 0, len(editableColumns))
	for _, column := range columns {
		if editableColumns[column.Name] {
			out = append(out, column.Name)
		}
	}
	return out
}

// Update 按 usc 更新一行（需求 11346）。
func (s *Store) Update(ctx context.Context, input UpdateInput) (*UpdateResult, error) {
	usc := strings.TrimSpace(input.Usc)
	if usc == "" {
		return nil, inputError("usc 不能为空")
	}
	if len(input.Fields) == 0 {
		return nil, inputError("没有要修改的字段")
	}

	payload := make(map[string]any, len(input.Fields))
	for column, raw := range input.Fields {
		if !editableColumns[column] {
			return nil, inputError(fmt.Sprintf("字段 %s 不允许人工修改（可编辑 %d 列：不含 usc、时间列与采集统计列）",
				column, len(editableColumns)))
		}
		if len(raw) == 0 {
			continue
		}
		var value any
		if string(raw) == "null" {
			value = nil
		} else if err := json.Unmarshal(raw, &value); err != nil {
			return nil, inputError(fmt.Sprintf("字段 %s 的值不是合法 JSON: %v", column, err))
		}
		payload[column] = value
	}
	if len(payload) == 0 {
		return nil, inputError("没有要修改的字段（传了 null 的列会被忽略）")
	}

	// gmt_update 由服务端维护（不属于人工可编辑列，但每次更新都要刷新）。
	payload["gmt_update"] = time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := s.supabaseConn(ctx)
	if err != nil {
		return nil, err
	}
	filter := buildUscFilter(usc)
	client := secukey.NewClient(conn)
	rows, err := client.UpdateRow(ctx, TableName, filter, payload)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// PostgREST 的 UPDATE 在 RLS 策略不满足时是**静默过滤**：HTTP 200 + 0 行受影响，
		// 不报 42501（与 INSERT 不同）。所以这里要先确认记录到底存不存在，
		// 才能区分「usc 不存在（404）」与「更新被 RLS 拦下（403）」。
		exists, existsErr := s.Exists(ctx, usc)
		if existsErr == nil && exists {
			return nil, secukey.NewPermissionError(fmt.Sprintf(
				"usc %s 在采集配置表里存在，但更新影响了 0 行：被 Supabase 的行级安全策略（RLS）静默过滤。"+
					"请在 Supabase 侧执行 create policy ... for update to anon, authenticated "+
					"using (true) with check (true) 后重试", usc))
		}
		return nil, fmt.Errorf("%w: usc %s 在采集配置表里不存在", ErrNotFound, usc)
	}

	result := &UpdateResult{Usc: usc, Updated: len(payload) - 1}
	row := Row{}
	if err := json.Unmarshal(rows[0], &row); err != nil {
		return nil, fmt.Errorf("解析 Supabase 返回行失败: %w", err)
	}
	result.Row = row
	return result, nil
}

// buildUscFilter 组装按 usc 定位的 PostgREST 过滤条件。
func buildUscFilter(usc string) url.Values {
	values := url.Values{}
	values.Set("usc", "eq."+usc)
	return values
}
