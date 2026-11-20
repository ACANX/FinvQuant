package mvsv

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEscapeCellRoundTrip 校验单字段转义/反转义可逆（含 `|`、换行、回车、反斜杠组合）。
func TestEscapeCellRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"plain",
		"a|b",
		"a|b|c",
		"line1\nline2",
		"line1\r\nline2",
		`back\slash`,
		`mix\|end`,
		`trailing\`,
		`\\|\n|\r`,
		"中文|备注\n第二行",
	}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			escaped := escapeCell(value)
			// 转义后的值拼上真实分隔符与下一字段，切分结果必须是 [value, tail]：
			// 这比"字符里没有 |"更准确——转义后的 `\|` 仍含 `|` 字符，但不再起分隔作用。
			parts, err := splitEscaped(escaped+"|tail", 2)
			if err != nil {
				t.Fatalf("切分失败: %q → %q: %v", value, escaped, err)
			}
			if parts[0] != value || parts[1] != "tail" {
				t.Fatalf("转义后切分异常: %q → %q → %v", value, escaped, parts)
			}
			if got := unescapeCell(escaped); got != value {
				t.Fatalf("往返不一致: %q → %q → %q", value, escaped, got)
			}
		})
	}
}

// TestUnescapeKeepsUnknownEscape 校验未知转义序列原样保留（兼容旧数据里的裸反斜杠）。
func TestUnescapeKeepsUnknownEscape(t *testing.T) {
	if got := unescapeCell(`a\xb`); got != `a\xb` {
		t.Fatalf("未知转义应原样保留，实际 %q", got)
	}
	if got := unescapeCell(`end\`); got != `end\` {
		t.Fatalf("末尾孤立反斜杠应保留，实际 %q", got)
	}
}

// TestWriteV2EscapesPipeAndNewline 校验导出的 MVSV-2 文件：
// 头部声明 Escape、数据行不含裸 `|`/换行，且往返后文本字段完全一致。
func TestWriteV2EscapesPipeAndNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "escape.mvsv.gz")
	want := sampleV2Row()
	want.ExtField = strPtr("source|vendor")
	want.Remark = strPtr("第一行\n第二行")
	want.Provider = strPtr(`FT\MM`)
	if _, err := WriteV2(path, V2Options{Title: "escape"}, CompressionGzip, 1,
		func(yield func(Row) error) error { return yield(want) }); err != nil {
		t.Fatalf("WriteV2 失败: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("gzip 读取失败: %v", err)
	}
	defer gz.Close()

	var got Row
	header, err := ReadAll(gz, "escape.mvsv.gz", func(h *Header, row Row) error {
		got = row
		return nil
	})
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if !usesEscape(header.Values) {
		t.Fatal("头部应声明 # Escape : \"backslash\"")
	}
	if got.ExtField == nil || *got.ExtField != "source|vendor" {
		t.Fatalf("ext_field 往返不一致: %v", got.ExtField)
	}
	if got.Remark == nil || *got.Remark != "第一行\n第二行" {
		t.Fatalf("remark 往返不一致: %v", got.Remark)
	}
	if got.Provider == nil || *got.Provider != `FT\MM` {
		t.Fatalf("provider 往返不一致: %v", got.Provider)
	}
}

// TestReadV2LegacyWithoutEscapeHeader 校验历史文件（无 Escape 头）仍按旧规则解析。
func TestReadV2LegacyWithoutEscapeHeader(t *testing.T) {
	legacy := "# Title : \"legacy\"\n" +
		"# Format : \"MVSV-2\"\n" +
		"# Field : \"usc|ts|ext_field\"\n" +
		"# Count : 1\n" +
		"\n" +
		"A|1700000000|plain\n"
	var got Row
	header, err := ReadAll(strings.NewReader(legacy), "legacy.mvsv", func(h *Header, row Row) error {
		got = row
		return nil
	})
	if err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if usesEscape(header.Values) {
		t.Fatal("历史文件不应被识别为转义格式")
	}
	if got.ExtField == nil || *got.ExtField != "plain" {
		t.Fatalf("ext_field 解析异常: %v", got.ExtField)
	}
}

// TestReadV2UnescapedPipeReportsHint 校验未声明 Escape 且含裸 `|` 时报错并给出修复提示。
func TestReadV2UnescapedPipeReportsHint(t *testing.T) {
	bad := "# Title : \"bad\"\n" +
		"# Format : \"MVSV-2\"\n" +
		"# Field : \"usc|ts|ext_field\"\n" +
		"# Count : 1\n" +
		"\n" +
		"A|1700000000|a|b\n"
	_, err := ReadAll(strings.NewReader(bad), "bad.mvsv", func(h *Header, row Row) error { return nil })
	if err == nil {
		t.Fatal("含裸 `|` 且未声明 Escape 时应报错")
	}
	if !strings.Contains(err.Error(), "Escape") {
		t.Fatalf("错误信息应提示声明 Escape，实际: %v", err)
	}
}
