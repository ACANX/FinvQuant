package kline

import (
	"encoding/json"
	"testing"
)

// TestToRowVwapPreferred 校验 vwap 与旧字段 pacd 同时出现时以 vwap 为准。
func TestToRowVwapPreferred(t *testing.T) {
	row, err := decodeItem(t, `{"usc":"A","ts":1792373400,"vwap":3.1,"pacd":2.2}`).ToRow("s")
	if err != nil {
		t.Fatalf("ToRow 失败: %v", err)
	}
	if row.Vwap == nil || *row.Vwap != "3.1" {
		t.Fatalf("vwap 应为 3.1，实际 %v", row.Vwap)
	}
}

// TestToRowPacdCompat 校验仅传旧字段 pacd 时仍能落库（兼容期，见待办需求 10903）。
func TestToRowPacdCompat(t *testing.T) {
	row, err := decodeItem(t, `{"usc":"A","ts":1792373400,"pacd":2.2}`).ToRow("s")
	if err != nil {
		t.Fatalf("ToRow 失败: %v", err)
	}
	if row.Vwap == nil || *row.Vwap != "2.2" {
		t.Fatalf("pacd 兼容应落为 vwap=2.2，实际 %v", row.Vwap)
	}
}

// TestToRowVwapNullFallsBackToPacd 校验显式 vwap=null 时回退到 pacd。
func TestToRowVwapNullFallsBackToPacd(t *testing.T) {
	row, err := decodeItem(t, `{"usc":"A","ts":1792373400,"vwap":null,"pacd":2.2}`).ToRow("s")
	if err != nil {
		t.Fatalf("ToRow 失败: %v", err)
	}
	if row.Vwap == nil || *row.Vwap != "2.2" {
		t.Fatalf("vwap=null 应回退 pacd=2.2，实际 %v", row.Vwap)
	}
}

// TestToRowVwapNeverSet 校验两列都没传时落 nil（不写库）。
func TestToRowVwapNeverSet(t *testing.T) {
	row, err := decodeItem(t, `{"usc":"A","ts":1792373400}`).ToRow("s")
	if err != nil {
		t.Fatalf("ToRow 失败: %v", err)
	}
	if row.Vwap != nil {
		t.Fatalf("未传时应为 nil，实际 %v", row.Vwap)
	}
}

// TestToPatchItemVwapColumn 校验批量增量更新落到 vwap 列，且兼容旧字段。
func TestToPatchItemVwapColumn(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"vwap 优先", `{"usc":"A","ts":1792373400,"vwap":3.1,"pacd":2.2}`, "3.1"},
		{"pacd 兼容", `{"usc":"A","ts":1792373400,"pacd":2.2}`, "2.2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			patch, apiErr := decodeItem(t, tc.body).ToPatchItem("s")
			if apiErr != nil {
				t.Fatalf("ToPatchItem 失败: %v", apiErr)
			}
			idx := -1
			for i, column := range patch.Columns {
				if column == ColVwap {
					idx = i
				}
			}
			if idx < 0 {
				t.Fatalf("补丁应包含列 %s，实际 %v", ColVwap, patch.Columns)
			}
			if patch.Values[idx] == nil || *patch.Values[idx] != tc.want {
				t.Fatalf("列 %s 期望 %q，实际 %v", ColVwap, tc.want, patch.Values[idx])
			}
		})
	}
}

// TestCompactFieldShortNameVwa 校验第 13 项短名已由 pa 改为 vwa（全名 vwap）。
func TestCompactFieldShortNameVwa(t *testing.T) {
	fields := CompactFields()
	if len(fields) != 15 {
		t.Fatalf("column 应固定 15 项，实际 %d", len(fields))
	}
	if fields[12] != "vwa" {
		t.Fatalf("第 13 项短名应为 vwa，实际 %q", fields[12])
	}
	if CompactFieldMap[12].Column != ColVwap {
		t.Fatalf("第 13 项应对应列 %s，实际 %s", ColVwap, CompactFieldMap[12].Column)
	}
}

// TestListItemShortNameVwa 校验 /Page 输出用 vwa 而非旧的 pa。
func TestListItemShortNameVwa(t *testing.T) {
	item := testRow().ListItem()
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if _, ok := decoded["vwa"]; !ok {
		t.Fatal("/Page 输出应包含 vwa")
	}
	if _, ok := decoded["pa"]; ok {
		t.Fatal("/Page 不应再输出旧短名 pa")
	}
}
