package uscbackfill

import (
	"encoding/json"
	"testing"
)

// TestPickUSC 覆盖「本地同一 stock_id 多条 usc 时如何定值」（需求 11055）。
func TestPickUSC(t *testing.T) {
	cases := []struct {
		name       string
		rows       []LocalRow
		ftmmSymbol string
		want       string
		wantOK     bool
	}{
		{
			name:       "单条直接取该条",
			rows:       []LocalRow{{USC: "US.AAPL", StockID: 1}},
			ftmmSymbol: "US.AAPL",
			want:       "US.AAPL",
			wantOK:     true,
		},
		{
			name:       "单条但 usc 为空白 → 不写",
			rows:       []LocalRow{{USC: "  ", StockID: 1}},
			ftmmSymbol: "US.AAPL",
			wantOK:     false,
		},
		{
			name:       "多条取等于 ftmm_symbol 的那条",
			rows:       []LocalRow{{USC: "DASHTRY"}, {USC: "CC.DASHTRY.FTUS"}},
			ftmmSymbol: "CC.DASHTRY.FTUS",
			want:       "CC.DASHTRY.FTUS",
			wantOK:     true,
		},
		{
			name:       "多条都不等于 ftmm_symbol → 歧义（不猜）",
			rows:       []LocalRow{{USC: "A"}, {USC: "B"}},
			ftmmSymbol: "C",
			wantOK:     false,
		},
		{
			name:       "多条且 ftmm_symbol 为空 → 歧义",
			rows:       []LocalRow{{USC: "A"}, {USC: "B"}},
			ftmmSymbol: "",
			wantOK:     false,
		},
		{
			name:   "空输入 → 不写",
			rows:   nil,
			wantOK: false,
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			got, ok := pickUSC(item.rows, item.ftmmSymbol)
			if ok != item.wantOK || got != item.want {
				t.Fatalf("pickUSC = (%q, %v)，期望 (%q, %v)", got, ok, item.want, item.wantOK)
			}
		})
	}
}

// TestGroupByStockID 校验分组与 stock_id 升序（分批顺序依赖它）。
func TestGroupByStockID(t *testing.T) {
	rows := []LocalRow{
		{USC: "B", StockID: 20},
		{USC: "A", StockID: 10},
		{USC: "C", StockID: 10},
	}
	groups, order := groupByStockID(rows)
	if len(order) != 2 || order[0] != 10 || order[1] != 20 {
		t.Fatalf("order = %v，期望 [10 20]", order)
	}
	if len(groups[10]) != 2 || groups[10][0].USC != "A" || groups[10][1].USC != "C" {
		t.Fatalf("groups[10] = %v", groups[10])
	}
	if len(groups[20]) != 1 {
		t.Fatalf("groups[20] = %v", groups[20])
	}
}

// TestIndexMetaUpstream 校验上游表 ① 行解析（usc 为 NULL 时留空串，不报错）。
func TestIndexUpstream(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"stock_id":203520,"usc":"BRK-B","ftmm_symbol":"US.BRK.B"}`),
		json.RawMessage(`{"stock_id":12001500,"usc":null,"ftmm_symbol":"CC.AAVE"}`),
		json.RawMessage(`{"stock_id":null,"usc":"x","ftmm_symbol":"y"}`),
	}
	rows, err := indexMetaUpstream(raw)
	if err != nil {
		t.Fatalf("indexMetaUpstream 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("解析出 %d 行，期望 2 行（stock_id 为 NULL 的行被跳过）", len(rows))
	}
	if rows[203520].USC != "BRK-B" || rows[203520].FtmmSymbol != "US.BRK.B" {
		t.Fatalf("rows[203520] = %+v", rows[203520])
	}
	if rows[12001500].USC != "" || rows[12001500].FtmmSymbol != "CC.AAVE" {
		t.Fatalf("rows[12001500] = %+v", rows[12001500])
	}
}

// TestNormalizeBatchSize 批量大小收敛。
func TestNormalizeBatchSize(t *testing.T) {
	cases := map[int]int{-1: DefaultBatchSize, 0: DefaultBatchSize, 50: MinBatchSize, 800: 800, 99999: MaxBatchSize}
	for input, want := range cases {
		if got := NormalizeBatchSize(input); got != want {
			t.Fatalf("NormalizeBatchSize(%d) = %d，期望 %d", input, got, want)
		}
	}
}

// TestIndexKeyUpstream 校验上游表 ② 行解析（stock_id 为 NULL 时保持 nil，与 0 区分）。
func TestIndexKeyUpstream(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"ftmm_symbol":"US.AAPL","usc":"US.AAPL","stock_id":2049}`),
		json.RawMessage(`{"ftmm_symbol":"AU..XFL","usc":null,"stock_id":null}`),
		json.RawMessage(`{"ftmm_symbol":null,"usc":"x","stock_id":1}`),
	}
	rows, err := indexKeyUpstream(raw)
	if err != nil {
		t.Fatalf("indexKeyUpstream 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("解析出 %d 行，期望 2 行（ftmm_symbol 为空的行被跳过）", len(rows))
	}
	if rows["US.AAPL"].USC != "US.AAPL" || rows["US.AAPL"].StockID == nil || *rows["US.AAPL"].StockID != 2049 {
		t.Fatalf("rows[US.AAPL] = %+v", rows["US.AAPL"])
	}
	if rows["AU..XFL"].USC != "" || rows["AU..XFL"].StockID != nil {
		t.Fatalf("rows[AU..XFL] = %+v", rows["AU..XFL"])
	}
}

// TestGroupByFtmmSymbol 校验按 ftmm_symbol 归并与升序（阶段 ② 的分批依据）。
func TestGroupByFtmmSymbol(t *testing.T) {
	rows := []LocalKeyRow{
		{FtmmSymbol: "US.ZZZ", USC: "ZZZ", StockID: 3},
		{FtmmSymbol: "US.AAA", USC: "AAA", StockID: 1},
		{FtmmSymbol: "US.AAA", USC: "US.AAA", StockID: 1},
	}
	groups, order := groupByFtmmSymbol(rows)
	if len(order) != 2 || order[0] != "US.AAA" || order[1] != "US.ZZZ" {
		t.Fatalf("order = %v，期望 [US.AAA US.ZZZ]", order)
	}
	if len(groups["US.AAA"]) != 2 {
		t.Fatalf("groups[US.AAA] 应有 2 条，实际 %v", groups["US.AAA"])
	}
	// 多条时取等于 ftmm_symbol 的那条
	picked, ok := pickUSC(groups["US.AAA"], "US.AAA")
	if !ok || picked != "US.AAA" {
		t.Fatalf("pickUSC = (%q, %v)，期望 (US.AAA, true)", picked, ok)
	}
}
