package dedup

import (
	"testing"
	"time"
)

func testMeta() FileMeta {
	return FileMeta{
		Usc:      "159934",
		BizMonth: 202610,
		Region:   "CN",
		Market:   "SZ",
		Provider: "FTMM",
		Name:     "黄金ETF易方达",
		Timezone: "Asia/Shanghai",
		Currency: "CNY",
	}
}

// TestFileNames 校验 BASE / PATCH / MANIFEST 命名契约（C-01~C-03）。
func TestFileNames(t *testing.T) {
	meta := testMeta()
	if got, want := meta.BaseFileName(), "CN_SZ_159934_Min_FTMM_Mon_202610.mvsv"; got != want {
		t.Fatalf("BASE 文件名不符：got=%q want=%q", got, want)
	}
	if got, want := meta.PatchFileName(2), "CN_SZ_159934_Min_FTMM_Mon_202610_Incr_2.mvsv"; got != want {
		t.Fatalf("PATCH 文件名不符：got=%q want=%q", got, want)
	}
	if got, want := meta.ManifestFileName(), "CN_SZ_159934_Min_FTMM_Mon_202610_Manifest.json"; got != want {
		t.Fatalf("MANIFEST 文件名不符：got=%q want=%q", got, want)
	}
}

// TestSanitizeFilePart 校验文件名片段清理（防目录穿越）。
func TestSanitizeFilePart(t *testing.T) {
	cases := map[string]string{
		"":         "UNKNOWN",
		"  ":       "UNKNOWN",
		"A/B":      "A_B",
		`A\B`:      "A_B",
		"CN:SH":    "CN_SH",
		"a<b>c|d":  "a_b_c_d",
		"159934":   "159934",
		"黄金ETF易方达": "黄金ETF易方达",
	}
	for input, want := range cases {
		if got := sanitizeFilePart(input); got != want {
			t.Fatalf("sanitizeFilePart(%q)=%q，期望 %q", input, got, want)
		}
	}
}

// TestTrimNumeric 校验数值文本裁剪：保留值、去掉无意义尾零、空值输出空串。
func TestTrimNumeric(t *testing.T) {
	text := func(value string) *string { return &value }
	cases := []struct {
		input *string
		want  string
	}{
		{nil, ""},
		{text(""), ""},
		{text("8.797000"), "8.797"},
		{text("0.000000"), "0"},
		{text("-0.000000"), "0"},
		{text("100.000000"), "100"},
		{text("100"), "100"},
		{text("-0.420000"), "-0.42"},
		{text("3458980.40000000"), "3458980.4"},
	}
	for _, item := range cases {
		if got := trimNumeric(item.input); got != item.want {
			t.Fatalf("trimNumeric(%v)=%q，期望 %q", item.input, got, item.want)
		}
	}
}

// TestMonthRanges 校验月份区间与收口 / 冻结日期。
func TestMonthRanges(t *testing.T) {
	from, to := MonthRange(202610)
	if from != 20261001 || to != 20261031 {
		t.Fatalf("MonthRange(202610)=(%d,%d)", from, to)
	}

	seal := SealDateOf(202610)
	if got, want := seal.Format("2006-01-02"), "2026-11-25"; got != want {
		t.Fatalf("SealDateOf(202610)=%s，期望 %s", got, want)
	}

	freeze := FreezeDateOf(202610, 5)
	if got, want := freeze.Format("2006-01-02"), "2026-11-30"; got != want {
		t.Fatalf("FreezeDateOf(202610,5)=%s，期望 %s", got, want)
	}

	first, last := MonthFirstLastDay(202602)
	if first.Format("2006-01-02") != "2026-02-01" || last.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("MonthFirstLastDay(202602)=(%s,%s)", first, last)
	}
}

// TestBuildManifest 校验 Manifest 必填字段与行数合计。
func TestBuildManifest(t *testing.T) {
	meta := testMeta()
	generated := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	base := ManifestFile{FileName: meta.BaseFileName(), RowCount: 241, FileSize: 1024, Checksum: "abc", Version: 1}
	patches := []ManifestFile{
		{FileName: meta.PatchFileName(1), RowCount: 3, FileSize: 128, Checksum: "def", Version: 1, Reason: PatchReasonLateArrival},
		{FileName: meta.PatchFileName(2), RowCount: 2, FileSize: 96, Checksum: "ghi", Version: 2, Reason: PatchReasonLateArrival},
	}

	manifest := BuildManifest(meta, SealDateOf(202610), CompressionNone, base, patches, generated)

	if manifest.Format != "mvsv" || manifest.Delimiter != "|" || manifest.Encoding != "utf-8" {
		t.Fatalf("Manifest 格式契约不符：%+v", manifest)
	}
	if manifest.HeaderLines != 16 || !manifest.BlankLineAfterHeader {
		t.Fatalf("Manifest 文件头契约不符：header_lines=%d blank=%v", manifest.HeaderLines, manifest.BlankLineAfterHeader)
	}
	if manifest.Field != MvsvFieldLine || len(manifest.Columns) != 12 {
		t.Fatalf("Manifest 列契约不符：field=%q columns=%v", manifest.Field, manifest.Columns)
	}
	if manifest.RowCount != 246 {
		t.Fatalf("Manifest 行数应为 246，实际 %d", manifest.RowCount)
	}
	if manifest.SealDate != "2026-11-25" {
		t.Fatalf("Manifest seal_date 不符：%s", manifest.SealDate)
	}
	if manifest.Checksum != "abc" {
		t.Fatalf("Manifest checksum 应取 base checksum，实际 %s", manifest.Checksum)
	}
}
