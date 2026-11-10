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

// TestClassify 校验「填入 / 覆盖 / 未变 / 无对应」四类判定（需求 11084 回推口径 + 需求 11142 起带 priority）。
func TestClassify(t *testing.T) {
	twenty := 20
	ten := 10
	cases := []struct {
		name     string
		upstream keyUpstreamRow
		exists   bool
		want     string
		local    keyTarget
	}{
		{name: "上游无该 ftmm_symbol → missing", exists: false, local: keyTarget{USC: "US.AAPL", Priority: twenty}, want: "missing"},
		{name: "上游两列都为空 → fill", exists: true, upstream: keyUpstreamRow{FtmmSymbol: "AU..XFL"}, local: keyTarget{USC: "XFL", Priority: twenty}, want: "fill"},
		{name: "上游 usc 为空、priority 有值 → fill", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "AU..XFL", Priority: &ten}, local: keyTarget{USC: "XFL", Priority: twenty}, want: "fill"},
		{name: "上游 priority 为空 → fill", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "AU..XFL", USC: "XFL"}, local: keyTarget{USC: "XFL", Priority: twenty}, want: "fill"},
		{name: "两列都一致 → keep", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "US.AAPL", USC: "ETFAAPL", Priority: &twenty}, local: keyTarget{USC: "ETFAAPL", Priority: twenty}, want: "keep"},
		{name: "usc 不同 → overwrite", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "SH.000300", USC: "000300", Priority: &twenty}, local: keyTarget{USC: "SH0300", Priority: twenty}, want: "overwrite"},
		{name: "只有 priority 不同 → overwrite", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "US.AAPL", USC: "ETFAAPL", Priority: &ten}, local: keyTarget{USC: "ETFAAPL", Priority: twenty}, want: "overwrite"},
		{name: "大小写不同 → overwrite", exists: true,
			upstream: keyUpstreamRow{FtmmSymbol: "US.AAPL", USC: "us.aapl", Priority: &twenty}, local: keyTarget{USC: "US.AAPL", Priority: twenty}, want: "overwrite"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := classify(item.upstream, item.exists, item.local); got != item.want {
				t.Fatalf("classify = %q，期望 %q", got, item.want)
			}
		})
	}
}

// TestIndexKeyUpstreamPriority 校验上游 priority 被解析进 keyUpstreamRow（需求 11142）。
//
// 回归点：曾只解析 usc，priority 漏赋值 → 所有行都被判成 fill（上游 priority 明明有值）。
func TestIndexKeyUpstreamPriority(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"ftmm_symbol":"US.AAPL","usc":"AAPL","priority":26}`),
		json.RawMessage(`{"ftmm_symbol":"AU..XFL","usc":"XFL","priority":null}`),
	}
	rows, err := indexKeyUpstream(raw)
	if err != nil {
		t.Fatalf("indexKeyUpstream 失败: %v", err)
	}
	if got := rows["US.AAPL"].Priority; got == nil || *got != 26 {
		t.Fatalf("US.AAPL priority = %v，期望 26", got)
	}
	if got := rows["AU..XFL"].Priority; got != nil {
		t.Fatalf("AU..XFL priority 应为 nil，实际 %d", *got)
	}
	// upstream 有 priority 时，与本地一致必须判 keep
	if got := classify(rows["US.AAPL"], true, keyTarget{USC: "AAPL", Priority: 26}); got != classKeep {
		t.Fatalf("classify = %q，期望 keep", got)
	}
}

// TestBuildTarget 校验本地行归并：按 ftmm_symbol 去重升序、跳过 usc 为空的行。
func TestBuildTarget(t *testing.T) {
	rows := []LocalKeyRow{
		{FtmmSymbol: "US.ZZZ", USC: "ZZZ", Priority: 26},
		{FtmmSymbol: "US.AAA", USC: " ", Priority: 10},
		{FtmmSymbol: "AU..XFL", USC: "XFL", Priority: 10},
		{FtmmSymbol: "US.ZZZ", USC: "ZZZ", Priority: 26},
	}
	order, target := buildTarget(rows)
	if len(order) != 2 || order[0] != "AU..XFL" || order[1] != "US.ZZZ" {
		t.Fatalf("order = %v，期望 [AU..XFL US.ZZZ]", order)
	}
	if got := target["US.ZZZ"]; got.USC != "ZZZ" || got.Priority != 26 {
		t.Fatalf("target[US.ZZZ] = %+v", got)
	}
	if got := target["AU..XFL"]; got.USC != "XFL" || got.Priority != 10 {
		t.Fatalf("target[AU..XFL] = %+v", got)
	}
	if _, exists := target["US.AAA"]; exists {
		t.Fatalf("usc 为空的行不应进入目标集：%v", target)
	}
}
