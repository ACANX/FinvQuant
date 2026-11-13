package backtest

import (
	"testing"
	"time"
)

// mockRow 模拟 pgx 行扫描：按目标类型回填（nil 指针保持 nil 以模拟 NULL 列）。
type mockRow struct {
	vals []any
}

func (m *mockRow) Scan(dest ...any) error {
	for i, d := range dest {
		if i >= len(m.vals) {
			break
		}
		switch v := d.(type) {
		case *string:
			if s, ok := m.vals[i].(string); ok {
				*v = s
			}
			// 非 string（含 nil）保持零值，模拟 NULL
		case **string:
			if s, ok := m.vals[i].(*string); ok {
				*v = s
			}
		case *int:
			if n, ok := m.vals[i].(int); ok {
				*v = n
			}
		case *int64:
			if n, ok := m.vals[i].(int64); ok {
				*v = n
			}
		case *float64:
			if n, ok := m.vals[i].(float64); ok {
				*v = n
			}
		case *bool:
			if b, ok := m.vals[i].(bool); ok {
				*v = b
			}
		case *time.Time:
			if t, ok := m.vals[i].(time.Time); ok {
				*v = t
			}
		case **time.Time:
			if t, ok := m.vals[i].(*time.Time); ok {
				*v = t
			}
		case *[]byte:
			if b, ok := m.vals[i].([]byte); ok {
				*v = b
			}
		}
	}
	return nil
}

// TestScanRunNullColumns 验证 error_message/env_id/created_by 为 NULL 时扫描不崩溃
// （回归：can't scan into dest[24] (col: error_message): cannot scan NULL into *string）。
func TestScanRunNullColumns(t *testing.T) {
	now := time.Now()
	row := &mockRow{vals: []any{
		"run-1", int64(1), "default", "strat-1", "STRAT-DUALMA-GC", "双均线", []byte(`{}`),
		"acc-1", "ACCT-GOLD-001", "黄金账户", []byte(`{}`),
		nil, []byte(`{}`), // env_id = NULL, env_snapshot = {}
		"GCMain", 33, "Min", "Day",
		int64(1514764800), int64(1753920000), 20180101, 20260731, []byte(`{}`),
		"PENDING", 0, nil, nil, // status, progress, error_message = NULL, report = NULL
		nil, nil, nil, now, // started_at/finished_at = NULL, created_by = NULL, gmt_update
	}}
	r, err := scanRun(row)
	if err != nil {
		t.Fatalf("scanRun 遇 NULL 列不应报错: %v", err)
	}
	if r.ErrorMessage != "" {
		t.Errorf("ErrorMessage 应为空串，实际=%q", r.ErrorMessage)
	}
	if r.EnvID != "" {
		t.Errorf("EnvID 应为空串，实际=%q", r.EnvID)
	}
	if r.CreatedBy != "" {
		t.Errorf("CreatedBy 应为空串，实际=%q", r.CreatedBy)
	}
	if r.Status != "PENDING" {
		t.Errorf("Status 应为 PENDING，实际=%q", r.Status)
	}
}

// TestScanRunNullFilled 验证非 NULL 值正常回填。
func TestScanRunNullFilled(t *testing.T) {
	now := time.Now()
	msg := "标的 GCMain 无任何行情数据"
	envID := "env-1"
	createdBy := "console"
	row := &mockRow{vals: []any{
		"run-2", int64(2), "default", "strat-1", "STRAT-DUALMA-GC", "双均线", []byte(`{}`),
		"acc-1", "ACCT-GOLD-001", "黄金账户", []byte(`{}`),
		&envID, []byte(`{}`),
		"GCMain", 33, "Min", "Day",
		int64(1514764800), int64(1753920000), 20180101, 20260731, []byte(`{}`),
		"FAILED", 100, &msg, []byte(`{}`),
		nil, &now, &createdBy, now,
	}}
	r, err := scanRun(row)
	if err != nil {
		t.Fatalf("scanRun 失败: %v", err)
	}
	if r.ErrorMessage != msg {
		t.Errorf("ErrorMessage 应为 %q，实际=%q", msg, r.ErrorMessage)
	}
	if r.EnvID != "env-1" {
		t.Errorf("EnvID 应为 env-1，实际=%q", r.EnvID)
	}
	if r.CreatedBy != "console" {
		t.Errorf("CreatedBy 应为 console，实际=%q", r.CreatedBy)
	}
	if r.Status != "FAILED" || r.Progress != 100 {
		t.Errorf("Status/Progress 回填错误: %q/%d", r.Status, r.Progress)
	}
}
