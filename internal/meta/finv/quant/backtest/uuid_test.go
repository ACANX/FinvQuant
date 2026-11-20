package backtest

import (
	"regexp"
	"strings"
	"testing"
)

// TestNewUUIDUpperCase 验证 newUUID 生成的 UUID 全大写（禁止出现小写英文字母）。
func TestNewUUIDUpperCase(t *testing.T) {
	// 标准 UUID 格式：8-4-4-4-12 十六进制段
	uuidRe := regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := newUUID()
		if !uuidRe.MatchString(id) {
			t.Fatalf("UUID 格式或大小写不正确: %q", id)
		}
		if strings.Contains(id, "a-f") {
			// 上面的正则已保证全大写；此处额外防御小写字母出现
			if strings.ToLower(id) == id {
				t.Errorf("UUID 含小写字母: %q", id)
			}
		}
		// 唯一性
		if seen[id] {
			t.Errorf("UUID 重复: %q", id)
		}
		seen[id] = true
	}
}

// TestNewUUIDNoLowercase 额外验证：任何小写 a-f 都不应出现（正则已隐含，这里直接断言）。
func TestNewUUIDNoLowercase(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := newUUID()
		for _, c := range id {
			if c >= 'a' && c <= 'f' {
				t.Fatalf("UUID 含小写英文字母 %q（应全大写）: %q", string(c), id)
			}
		}
	}
}
