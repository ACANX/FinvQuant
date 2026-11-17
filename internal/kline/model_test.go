package kline

import (
	"encoding/json"
	"testing"
)

func testRow() Row {
	date, clock := 20261019, 93000
	return Row{
		Usc:         "000001",
		Ts:          1792373400,
		Date:        &date,
		Time:        &clock,
		Open:        nil,
		High:        nil,
		Low:         nil,
		Close:       strPtr("11.480000"),
		Volume:      strPtr("392600.000000"),
		Turnover:    strPtr("4507048.00000000"),
		ChangePrice: strPtr("-0.020000"),
		ChangeRatio: strPtr("-0.170000"),
		Paocd:       nil,
		ExtField:    nil,
		Provider:    strPtr("FTMM"),
	}
}

func TestRowJSON(t *testing.T) {
	raw, err := json.Marshal(testRow())
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	if decoded["ts"].(float64) != 1792373400 {
		t.Fatalf("ts 类型/值错误: %v", decoded["ts"])
	}
	if decoded["close"].(float64) != 11.48 {
		t.Fatalf("close 应为 JSON 数字 11.48，实际 %v", decoded["close"])
	}
	if value, ok := decoded["open"]; !ok || value != nil {
		t.Fatalf("open 应为 null，实际 %v", decoded["open"])
	}
	if _, ok := decoded["gmt_create"]; ok {
		t.Fatal("不应输出 gmt_create")
	}
	if decoded["provider"].(string) != "FTMM" {
		t.Fatalf("provider 输出错误: %v", decoded["provider"])
	}
}

func TestRowListItem(t *testing.T) {
	item := testRow().ListItem()
	if item.Cr == nil || *item.Cr != "-0.17" {
		t.Fatalf("cr 期望 -0.17，实际 %v", item.Cr)
	}
	if string(item.C) != "11.48" {
		t.Fatalf("c 期望 11.48，实际 %s", item.C)
	}
	if string(item.T) != "4507048" {
		t.Fatalf("t 期望 4507048，实际 %s", item.T)
	}
	raw, _ := json.Marshal(item)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if _, ok := decoded["remark"]; ok {
		t.Fatal("列表项不应输出 remark")
	}
	if decoded["pv"].(string) != "FTMM" {
		t.Fatalf("pv 输出错误: %v", decoded["pv"])
	}
}

func TestRowUniRow(t *testing.T) {
	columns := CompactFields()
	row := testRow().UniRow()
	if len(row) != len(columns) {
		t.Fatalf("row 长度 %d != column 长度 %d", len(row), len(columns))
	}
	want := map[int]string{
		0: "000001", 1: "1792373400", 2: "20261019", 3: "93000",
		7: "11.48", 8: "392600", 9: "4507048", 10: "-0.17",
		11: "-0.02", 13: "FTMM",
	}
	for index, expected := range want {
		if row[index] == nil || *row[index] != expected {
			t.Fatalf("row[%d]=%v，期望 %q", index, row[index], expected)
		}
	}
	for _, index := range []int{4, 5, 6, 12, 14} {
		if row[index] != nil {
			t.Fatalf("row[%d] 应为 null，实际 %q", index, *row[index])
		}
	}
}
