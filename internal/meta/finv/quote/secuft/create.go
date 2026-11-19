package secuft

// 「新增采集证券」的写入实现（需求 11345）。
//
// 写入目标是 Supabase `public.finv_quote_secu_ft`（PostgREST POST），只写 21 列里
// 由本系统负责的列；三列时间列显式写 now()（无法从外部确认 Supabase 侧有无默认值，
// 显式写可避免 NOT NULL 无默认导致插入失败）。
//
// 默认口径（用户确认）：
//   - flag_enable 默认 0 —— 新增后先停采，之后由人工改成 1 再开；
//   - 重复 usc 直接拒绝（409），不做 upsert，避免误改线上采集配置；
//   - 富途编码类补不出来的列允许留空（由用户在对话框里手填）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
)

// CreateInput 新增一行的请求（键为与源表一致的列名）。
type CreateInput struct {
	// Usc 统一编号（必填，且必须存在于本地证券主数据）。
	Usc string `json:"usc"`
	// Fields 其余列的取值；未提供的列用服务端默认值填充。
	Fields map[string]json.RawMessage `json:"fields"`
}

// CreateResult 写入结果。
type CreateResult struct {
	// Usc 写入的统一编号。
	Usc string `json:"usc"`
	// Row Supabase 返回的行（`Prefer: return=representation`），用于回显实际落库值。
	Row Row `json:"row"`
}

// writableColumns 允许由本接口写入的列（32 列里除 usc 与三列时间列之外）。
//
// usc 单独取（避免 Fields 覆盖主键）；时间列由服务端写 now()（避免前端传入错乱时间）。
var writableColumns = func() map[string]bool {
	out := make(map[string]bool, len(columns))
	for _, column := range columns {
		if column.Name == "usc" || column.Kind == KindTimestamp {
			continue
		}
		out[column.Name] = true
	}
	return out
}()

// isCreatableColumn 该列是否允许在「新增」里填写（排除时间列）。
func isCreatableColumn(name string) bool {
	return writableColumns[name]
}

// Create 向 Supabase 插入一行采集配置（需求 11345）。
//
// 行为：
//   - usc 去空白后必须非空，且必须在本地 `public.finv_security` 存在（防止配出孤儿行）；
//   - 未提供的 `flag_enable` 强制为 0（新增后先停采）；
//   - 未提供的 `provider` 用 DefaultProvider；
//   - 三列时间列写 now()；
//   - Fields 里出现未知列名直接报错（防拼写错误静默丢字段）；
//   - 上游 409（唯一冲突）由 InsertRow 翻成「该记录已存在」。
func (s *Store) Create(ctx context.Context, input CreateInput) (*CreateResult, error) {
	usc := strings.TrimSpace(input.Usc)
	if usc == "" {
		return nil, inputError("usc 不能为空")
	}

	// 1) 先做纯参数校验（不连库）：未知列名 / 非法 JSON 直接拒绝，
	//    防止拼写错误的字段被静默丢弃，也避免无效查库。
	payload := map[string]any{"usc": usc}
	for column, raw := range input.Fields {
		if !writableColumns[column] {
			return nil, inputError(fmt.Sprintf("字段 %s 不在可写列范围内（可写 %d 列，见列元数据）",
				column, len(writableColumns)))
		}
		if len(raw) == 0 || string(raw) == "null" {
			continue // 空值不写，让 Supabase 侧默认值生效
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, inputError(fmt.Sprintf("字段 %s 的值不是合法 JSON: %v", column, err))
		}
		payload[column] = value
	}

	// 2) usc 必须在本地证券主数据里（本次实测：23,742 行主数据，4 表联查 1:1）。
	exists, err := s.securityExists(ctx, usc)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, inputError(fmt.Sprintf(
			"usc %s 不在本地证券主数据（public.finv_security）里，请先登记证券主数据后再新增采集配置", usc))
	}
	if _, ok := payload["flag_enable"]; !ok {
		payload["flag_enable"] = DefaultFlagEnable
	}
	if value, ok := payload["provider"]; !ok || isEmptyValue(value) {
		payload["provider"] = DefaultProvider
	}
	if value, ok := payload["req_section"]; !ok || isEmptyValue(value) {
		payload["req_section"] = 0
	}
	if value, ok := payload["weight_priority"]; !ok || isEmptyValue(value) {
		payload["weight_priority"] = DefaultWeightPriority
	}
	if value, ok := payload["weight_frequency"]; !ok || isEmptyValue(value) {
		payload["weight_frequency"] = DefaultWeightFrequency
	}

	// 3) 时间列显式写 now()（同一时刻，保持一致）。
	//    采集统计时间（dt_last_check / dt_last_fetch）不写：新增的证券还没被采集过。
	now := time.Now().UTC().Format(time.RFC3339Nano)
	payload["gmt_create"] = now
	payload["gmt_update"] = now
	payload["dt_update"] = now

	// 4) 写入。
	conn, err := s.supabaseConn(ctx)
	if err != nil {
		return nil, err
	}
	client := secukey.NewClient(conn)
	raw, err := client.InsertRow(ctx, TableName, payload)
	if err != nil {
		return nil, err
	}

	result := &CreateResult{Usc: usc}
	if len(raw) > 0 {
		row := Row{}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("解析 Supabase 返回行失败: %w", err)
		}
		result.Row = row
	}
	return result, nil
}

// securityExists 检查 usc 是否在本地证券主数据里。
func (s *Store) securityExists(ctx context.Context, usc string) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM public.finv_security WHERE usc = $1)`, usc).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("查询证券主数据失败: %w", err)
	}
	return found, nil
}
