package webapi

import (
	"encoding/json"
	"testing"
)

// TestSnapshotDisplayName 名称优先取简体中文 sc_name，取不到才回退 name（需求 11339）。
//
// 实测样本（2026-11-19）：US..SPX / HK.800000 / CC.BTC 三个名称字段都有值。
func TestSnapshotDisplayName(t *testing.T) {
	const raw = `{
		"code": "US..SPX",
		"name": "S&P 500 Index",
		"sc_name": "标普500指数",
		"tc_name": "標普500指數",
		"last_price": 7314.3
	}`
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if snapshot.Name != "S&P 500 Index" {
		t.Errorf("Name 应为上游英文名，实际 %q", snapshot.Name)
	}
	if snapshot.SCName != "标普500指数" {
		t.Errorf("SCName 应为简体中文名，实际 %q", snapshot.SCName)
	}
	if snapshot.TCName != "標普500指數" {
		t.Errorf("TCName 应为繁体中文名，实际 %q", snapshot.TCName)
	}
	if got := snapshot.DisplayName(); got != "标普500指数" {
		t.Errorf("DisplayName 应优先简体中文，实际 %q", got)
	}

	// sc_name 缺失 / 空串 → 回退 name
	cases := []struct {
		name     string
		snapshot Snapshot
		want     string
	}{
		{"sc_name 缺失", Snapshot{Name: "Bitcoin"}, "Bitcoin"},
		{"sc_name 空白", Snapshot{Name: "Bitcoin", SCName: "   "}, "Bitcoin"},
		{"sc_name 有值", Snapshot{Name: "Bitcoin", SCName: "比特币"}, "比特币"},
		{"全部为空", Snapshot{}, ""},
	}
	for _, tc := range cases {
		if got := tc.snapshot.DisplayName(); got != tc.want {
			t.Errorf("%s：期望 %q，实际 %q", tc.name, tc.want, got)
		}
	}
}
