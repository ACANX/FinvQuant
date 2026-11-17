package mvsv

import (
	"os"
	"testing"
)

// 验证 Docs/DataFormat/ 下的示例文件能被解析器正确解析（合规性回归）
func TestSampleFilesParse(t *testing.T) {
	files := []string{
		"../../Docs/DataFormat/Example_US_NVDA_Min_V4_2026.mvsv",
		"../../Docs/DataFormat/Example_GCmain_Min_V3_2026.mvsv",
	}
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取示例文件失败 %s: %v", path, err)
		}
		result, err := Parse(content, path)
		if err != nil {
			t.Fatalf("示例文件解析失败 %s: %v", path, err)
		}
		if len(result.Rows) < 20 {
			t.Fatalf("%s 记录数=%d，期望 ≥20", path, len(result.Rows))
		}
		t.Logf("✅ %s: %d 条记录解析成功（Code=%s MarketCode=%s）",
			path, len(result.Rows), result.Header.Values["Code"], result.Header.Values["MarketCode"])
	}
}
