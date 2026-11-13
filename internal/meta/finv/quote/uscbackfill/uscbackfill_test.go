package uscbackfill

import (
	"encoding/json"
	"testing"
	"time"
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
//
// 默认两列都勾选（需求 11163 的默认值），字段选择的差异见 TestClassifyByFieldSelection。
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
			if got := classify(item.upstream, item.exists, item.local, AllFields()); got != item.want {
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
	if got := classify(rows["US.AAPL"], true, keyTarget{USC: "AAPL", Priority: 26}, AllFields()); got != classKeep {
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

// TestBuildInsertRow 上游缺记录时的插入行组装（需求 11157）。
func TestBuildInsertRow(t *testing.T) {
	at := time.Date(2026, 1, 15, 10, 20, 30, 0, time.FixedZone("CST", 8*3600))
	stockID := int64(89184996156542)
	want := keyTarget{
		USC: "US.TXCP", Priority: 26, Instrument: "stock",
		Market: "US", Code: "TXCP", StockID: &stockID, Status: "PENDING", FlagEnable: 1,
	}
	row, ok := buildInsertRow("US.TXCP", want, at)
	if !ok {
		t.Fatalf("字段齐全时应返回 ok=true")
	}
	expect := map[string]any{
		"ftmm_symbol": "US.TXCP",
		"market":      "US",
		"code":        "TXCP",
		"instrument":  "stock",
		"usc":         "US.TXCP",
		"priority":    26,
		"status":      "PENDING",
		"enable":      1,
		"stock_id":    stockID,
		"note":        "[AUTO-FILL] 本地回推上游插入 2026-01-15T10:20:30+08:00",
	}
	for key, value := range expect {
		if row[key] != value {
			t.Fatalf("列 %s = %v，期望 %v", key, row[key], value)
		}
	}
	if _, exists := row["flag_enable"]; exists {
		t.Fatalf("上游列名是 enable，不应写 flag_enable")
	}

	// 本地缺身份列时不能插入
	for _, broken := range []keyTarget{
		{USC: "X", Instrument: "", Market: "US", Code: "TXCP"},
		{USC: "X", Instrument: "stock", Market: "", Code: "TXCP"},
		{USC: "X", Instrument: "stock", Market: "US", Code: ""},
	} {
		if _, ok := buildInsertRow("US.X", broken, at); ok {
			t.Fatalf("缺 instrument/market/code 时应 ok=false: %+v", broken)
		}
	}

	// stock_id 为空 / status 为空时不写这两列（上游有默认值）
	sparse, ok := buildInsertRow("HK.09999", keyTarget{
		USC: "HK.09999", Priority: 10, Instrument: "stock", Market: "HK", Code: "09999",
	}, at)
	if !ok {
		t.Fatalf("stock_id / status 缺失不应影响插入")
	}
	if _, exists := sparse["stock_id"]; exists {
		t.Fatalf("stock_id 为空时不应写入该列")
	}
	if _, exists := sparse["status"]; exists {
		t.Fatalf("status 为空时不应写入该列")
	}
}

// TestNormalizeFields 字段选择归一（需求 11163）：没传 → 两列都勾；显式全不勾 → 拒绝。
func TestNormalizeFields(t *testing.T) {
	fields, err := NormalizeFields(nil)
	if err != nil {
		t.Fatalf("未传 fields 不应报错: %v", err)
	}
	if !fields.USC || !fields.Priority {
		t.Fatalf("未传 fields 应默认两列都勾，实际 %+v", fields)
	}

	onlyUSC, err := NormalizeFields(&FieldSelection{USC: true})
	if err != nil {
		t.Fatalf("只勾 usc 应通过: %v", err)
	}
	if !onlyUSC.USC || onlyUSC.Priority {
		t.Fatalf("只勾 usc 归一错误: %+v", onlyUSC)
	}

	if _, err := NormalizeFields(&FieldSelection{}); err == nil {
		t.Fatalf("两个都不勾应报错")
	}

	if got := AllFields().Text(); got != "usc+priority" {
		t.Fatalf("AllFields().Text() = %q", got)
	}
	if got := (FieldSelection{Priority: true}).Text(); got != "priority" {
		t.Fatalf("只勾 priority 的 Text() = %q", got)
	}
	if got := (FieldSelection{}).Columns(); len(got) != 0 {
		t.Fatalf("空选择 Columns() 应为空，实际 %v", got)
	}
}

// TestClassifyByFieldSelection 只勾一列时另一列完全不参与判定（需求 11163）。
//
// 场景来源：上游 BJ.899050 的 priority=29 是人工设定的，本地 priority=26；
// 只勾 usc 时这条不能算 overwrite，更不能把 29 改成 26。
func TestClassifyByFieldSelection(t *testing.T) {
	twentySix := 26
	twentyNine := 29
	upstream := keyUpstreamRow{FtmmSymbol: "BJ.899050", USC: "899050", Priority: &twentyNine}
	local := keyTarget{USC: "899050", Priority: twentySix}

	if got := classify(upstream, true, local, AllFields()); got != classOverwrite {
		t.Fatalf("两列都勾时应判 overwrite，实际 %q", got)
	}
	if got := classify(upstream, true, local, FieldSelection{USC: true}); got != classKeep {
		t.Fatalf("只勾 usc、usc 一致时应判 keep（priority 差异不算差异），实际 %q", got)
	}
	if got := classify(upstream, true, local, FieldSelection{Priority: true}); got != classOverwrite {
		t.Fatalf("只勾 priority、priority 不同时应判 overwrite，实际 %q", got)
	}

	// priority 为 NULL 时：只勾 usc 不应判成 fill（那一列根本不写）
	onlyNullPriority := keyUpstreamRow{FtmmSymbol: "BJ.899050", USC: "899050"}
	if got := classify(onlyNullPriority, true, local, FieldSelection{USC: true}); got != classKeep {
		t.Fatalf("只勾 usc 且 usc 一致时应判 keep，实际 %q", got)
	}
	if got := classify(onlyNullPriority, true, local, FieldSelection{Priority: true}); got != classFill {
		t.Fatalf("只勾 priority 且 upstream 为空时应判 fill，实际 %q", got)
	}
}
