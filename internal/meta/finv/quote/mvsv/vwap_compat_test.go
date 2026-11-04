package mvsv

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestV2ImportColumnAliases 校验导入兼容三名额：paocd（旧导出文件）、pacd（旧 API 名）、vwap（新名）。
func TestV2ImportColumnAliases(t *testing.T) {
	cases := []struct {
		name   string
		column string
	}{
		{"旧导出文件 paocd", "paocd"},
		{"旧 API 字段 pacd", "pacd"},
		{"新名 vwap", "vwap"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header, err := buildHeaderV2(map[string]string{
				"Title":  "alias",
				"Format": FormatV2,
				"Field":  "usc|ts|" + tc.column,
				"Count":  "1",
			})
			if err != nil {
				t.Fatalf("构建头部失败: %v", err)
			}
			row, err := parseRowV2("A|1700000000|3.56", header, 1)
			if err != nil {
				t.Fatalf("解析数据行失败: %v", err)
			}
			if row.Vwap == nil || *row.Vwap != "3.56" {
				t.Fatalf("列 %s 应解析为 vwap=3.56，实际 %v", tc.column, row.Vwap)
			}
		})
	}
}

// TestV2ExportUsesVwap 校验导出的列名、FieldName 与中文名都改写为 vwap，不再出现 paocd。
func TestV2ExportUsesVwap(t *testing.T) {
	columns := V2Columns(false)
	joined := strings.Join(columns, "|")
	if !strings.Contains(joined, "|vwap|") {
		t.Fatalf("导出列名应含 vwap，实际 %s", joined)
	}
	if strings.Contains(joined, "paocd") {
		t.Fatalf("导出列名不应再含 paocd，实际 %s", joined)
	}
	if got := v2FieldNames["vwap"]; got != "Vwap" {
		t.Fatalf("FieldName 应为 Vwap，实际 %q", got)
	}
	if got := v2FieldNamesCN["vwap"]; got != "成交量加权平均价" {
		t.Fatalf("中文名应为「成交量加权平均价」，实际 %q", got)
	}
	if got := v2FieldTypes["vwap"]; got != "Decimal" {
		t.Fatalf("类型应为 Decimal，实际 %q", got)
	}
}

// TestWriteV2VwapRoundTrip 校验 vwap 值经导出再导入后保持一致。
func TestWriteV2VwapRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vwap.mvsv.gz")
	want := sampleV2Row()
	want.Vwap = strPtr("9.876543")
	if _, err := WriteV2(path, V2Options{Title: "vwap"}, CompressionGzip, 1,
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
	if _, err := ReadAll(gz, "vwap.mvsv.gz", func(_ *Header, row Row) error {
		got = row
		return nil
	}); err != nil {
		t.Fatalf("ReadAll 失败: %v", err)
	}
	if got.Vwap == nil || *got.Vwap != "9.876543" {
		t.Fatalf("vwap 往返不一致: %v", got.Vwap)
	}
}
