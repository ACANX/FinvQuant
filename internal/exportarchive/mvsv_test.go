package exportarchive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleRows MVsv 示例数据行（含空值列，验证 `||` 形态）。
func sampleRows() [][12]string {
	return [][12]string{
		{"1792546200", "20261021", "093000", "", "", "", "8.797", "393200", "3458980.4", "-0.037", "-0.42", ""},
		{"1792546260", "20261021", "093100", "8.797", "", "", "8.802", "882631", "7766642.638", "-0.032", "-0.36", "8.797"},
	}
}

func writeSample(t *testing.T, compression string) (string, WriteResult) {
	t.Helper()
	meta := testMeta()
	dest := filepath.Join(t.TempDir(), meta.BaseFileName())
	rows := sampleRows()
	write, err := WriteMVsv(dest, meta, compression, int64(len(rows)), func(yield func(cols [12]string) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WriteMVsv 失败: %v", err)
	}
	return dest, write
}

// TestWriteMVsvStructure 校验「16 行文件头 + 1 行空行 + 数据行」与 Count（B-01/B-05/B-09）。
func TestWriteMVsvStructure(t *testing.T) {
	dest, write := writeSample(t, CompressionNone)
	if write.RowCount != 2 {
		t.Fatalf("数据行数应为 2，实际 %d", write.RowCount)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("文件不应包含 CR（换行必须为 \\n）")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != MvsvHeaderLines+1+2 {
		t.Fatalf("总行数应为 %d，实际 %d", MvsvHeaderLines+1+2, len(lines))
	}
	if got, want := lines[0], "# 标题 : 159934 分钟级行情数据"; got != want {
		t.Fatalf("文件头第 1 行不符：%q", got)
	}
	if got, want := lines[3], `# Field : "ts|da|ti|o|h|l|c|v|t|cp|cr|lc"`; got != want {
		t.Fatalf("Field 行不符：%q", got)
	}
	if got, want := lines[7], "# Count : 2"; got != want {
		t.Fatalf("Count 行不符：%q", got)
	}
	if got, want := lines[15], "# Currency : CNY"; got != want {
		t.Fatalf("文件头末行不符：%q", got)
	}
	if lines[16] != "" {
		t.Fatalf("第 17 行必须为空行，实际 %q", lines[16])
	}
	if got, want := lines[17], "1792546200|20261021|093000||||8.797|393200|3458980.4|-0.037|-0.42|"; got != want {
		t.Fatalf("数据行不符：%q", got)
	}

	if err := VerifyMVsv(dest, CompressionNone, 2); err != nil {
		t.Fatalf("VerifyMVsv 失败: %v", err)
	}

	checksum, size, err := FileSHA256(dest)
	if err != nil {
		t.Fatalf("计算 SHA256 失败: %v", err)
	}
	if checksum != write.Checksum || size != write.Size {
		t.Fatalf("返回的 checksum/size 与文件不一致：%s/%d vs %s/%d", checksum, size, write.Checksum, write.Size)
	}
}

// TestWriteMVsvGzip 校验 gzip 压缩后扩展名不变、仍可通过结构校验（F-09）。
func TestWriteMVsvGzip(t *testing.T) {
	dest, _ := writeSample(t, CompressionGzip)
	if !strings.HasSuffix(dest, ".mvsv") {
		t.Fatalf("压缩后扩展名必须保持 .mvsv，实际 %s", dest)
	}
	if err := VerifyMVsv(dest, CompressionGzip, 2); err != nil {
		t.Fatalf("gzip 文件 VerifyMVsv 失败: %v", err)
	}
	if err := VerifyMVsv(dest, CompressionNone, 2); err == nil {
		t.Fatalf("按未压缩方式校验 gzip 文件应当失败")
	}
}

// TestWriteMVsvRowCountMismatch 校验「Count 与实际行数不一致」时失败且不留半成品（F-06/M-02）。
func TestWriteMVsvRowCountMismatch(t *testing.T) {
	meta := testMeta()
	dest := filepath.Join(t.TempDir(), meta.BaseFileName())
	rows := sampleRows()
	_, err := WriteMVsv(dest, meta, CompressionNone, 3, func(yield func(cols [12]string) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		t.Fatalf("行数与 Count 不一致时应返回错误")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("失败时不应留下目标文件")
	}
	entries, readErr := os.ReadDir(filepath.Dir(dest))
	if readErr != nil {
		t.Fatalf("读取目录失败: %v", readErr)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("失败时应清理临时文件，实际残留 %s", entry.Name())
		}
	}
}

// TestVerifyMVsvRejectsBrokenFiles 校验结构异常能被识别（5.1「文件结构」）。
func TestVerifyMVsvRejectsBrokenFiles(t *testing.T) {
	meta := testMeta()
	dir := t.TempDir()

	cases := map[string]string{
		"missing-blank-line": strings.Join(meta.headerLines(1), "\n") + "\n" +
			"1792546200|20261021|093000||||8.797|393200|3458980.4|-0.037|-0.42|\n",
		"wrong-count": strings.Join(meta.headerLines(5), "\n") + "\n\n" +
			"1792546200|20261021|093000||||8.797|393200|3458980.4|-0.037|-0.42|\n",
	}
	for name, content := range cases {
		path := filepath.Join(dir, name+".mvsv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("写入测试文件失败: %v", err)
		}
		if err := VerifyMVsv(path, CompressionNone, -1); err == nil {
			t.Fatalf("%s 应校验失败", name)
		}
	}
}
