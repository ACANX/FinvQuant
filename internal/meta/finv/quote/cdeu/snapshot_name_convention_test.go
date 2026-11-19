package cdeu

import (
	"os"
	"strings"
	"testing"
)

// TestNoDirectSnapshotName 名称取值规范的防回归检查（需求 11339）。
//
// 规范见 internal/meta/component/quote/moomoo/webapi 包注释「名称取值规范」：
// 快照名称一律走 webapi.Snapshot.DisplayName()（简体中文 sc_name 优先、英文 name 兜底），
// 业务代码不得直接读 snapshot.Name（会把中文名丢掉）。
// 只有「中英文各存一列」的场景才允许分别取 SCName / Name。
func TestNoDirectSnapshotName(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取当前目录失败: %v", err)
	}
	forbidden := []string{"snapshot.Name", "snap.Name"}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		scanned++
		text := string(data)
		for _, expr := range forbidden {
			if strings.Contains(text, expr) {
				t.Errorf("%s 直接读取了 %s：请改用 DisplayName()（简体中文优先，见 webapi 包注释「名称取值规范」）",
					name, expr)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("未扫描到任何源文件，检查路径是否失效")
	}
}
