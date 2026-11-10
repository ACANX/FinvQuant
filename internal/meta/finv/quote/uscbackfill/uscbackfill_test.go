package uscbackfill

import (
	"encoding/json"
	"testing"
)

// TestIndexKeyUpstream 校验上游 finv_quote_secu_key 行解析（usc 为 NULL 时留空串，键为空跳过）。
func TestIndexKeyUpstream(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"ftmm_symbol":"US.AAPL","usc":"US.AAPL"}`),
		json.RawMessage(`{"ftmm_symbol":"AU..XFL","usc":null}`),
		json.RawMessage(`{"ftmm_symbol":null,"usc":"x"}`),
		json.RawMessage(`{"ftmm_symbol":"  ","usc":"y"}`),
	}
	rows, err := indexKeyUpstream(raw)
	if err != nil {
		t.Fatalf("indexKeyUpstream 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("解析出 %d 行，期望 2 行（ftmm_symbol 为空的行被跳过）", len(rows))
	}
	if rows["US.AAPL"].USC != "US.AAPL" {
		t.Fatalf("rows[US.AAPL] = %+v", rows["US.AAPL"])
	}
	if rows["AU..XFL"].USC != "" {
		t.Fatalf("rows[AU..XFL] = %+v", rows["AU..XFL"])
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

// TestClassify 校验「填入 / 覆盖 / 未变 / 无对应」四类判定（需求 11084 的回推口径）。
func TestClassify(t *testing.T) {
	cases := []struct {
		name     string
		upstream keyUpstreamRow
		exists   bool
		want     string
		local    string
	}{
		{name: "上游无该 ftmm_symbol → missing", exists: false, local: "US.AAPL", want: "missing"},
		{name: "上游为空 → fill", exists: true, upstream: keyUpstreamRow{FtmmSymbol: "AU..XFL"}, local: "XFL", want: "fill"},
		{name: "上游一致 → keep", exists: true, upstream: keyUpstreamRow{FtmmSymbol: "US.AAPL", USC: "ETFAAPL"}, local: "ETFAAPL", want: "keep"},
		{name: "上游不同 → overwrite", exists: true, upstream: keyUpstreamRow{FtmmSymbol: "SH.000300", USC: "000300"}, local: "SH0300", want: "overwrite"},
		{name: "大小写不同 → overwrite", exists: true, upstream: keyUpstreamRow{FtmmSymbol: "US.AAPL", USC: "us.aapl"}, local: "US.AAPL", want: "overwrite"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := classify(item.upstream, item.exists, item.local); got != item.want {
				t.Fatalf("classify = %q，期望 %q", got, item.want)
			}
		})
	}
}

// TestBuildTarget 校验本地行归并：按 ftmm_symbol 去重升序、跳过 usc 为空的行。
func TestBuildTarget(t *testing.T) {
	rows := []LocalKeyRow{
		{FtmmSymbol: "US.ZZZ", USC: "ZZZ"},
		{FtmmSymbol: "US.AAA", USC: " "},
		{FtmmSymbol: "AU..XFL", USC: "XFL"},
		{FtmmSymbol: "US.ZZZ", USC: "ZZZ"},
	}
	order, target := buildTarget(rows)
	if len(order) != 2 || order[0] != "AU..XFL" || order[1] != "US.ZZZ" {
		t.Fatalf("order = %v，期望 [AU..XFL US.ZZZ]", order)
	}
	if target["US.ZZZ"] != "ZZZ" || target["AU..XFL"] != "XFL" {
		t.Fatalf("target = %v", target)
	}
	if _, exists := target["US.AAA"]; exists {
		t.Fatalf("usc 为空的行不应进入目标集：%v", target)
	}
}
