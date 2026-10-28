package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestOpenAPISpecsValid 校验 Docs 下的 OpenAPI 契约文件（.yml / .yaml）：
//   - 是合法 YAML；
//   - 含 `openapi` 与 `paths` 顶层字段。
//
// 约定见 Docs/DevSpec/ApiSpec.md §7.7：接口文档由「同名 .md（人读）+ .yml（机读）」
// 成对组成，.md 正文不再内嵌大段 YAML。
func TestOpenAPISpecsValid(t *testing.T) {
	root := filepath.Join("..", "..", "Docs")
	count := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := yaml.Unmarshal(content, &doc); err != nil {
			t.Errorf("%s 不是合法 YAML: %v", path, err)
			return nil
		}
		if _, ok := doc["openapi"]; !ok {
			t.Errorf("%s 缺少顶层字段 openapi", path)
		}
		if _, ok := doc["paths"]; !ok {
			t.Errorf("%s 缺少顶层字段 paths", path)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 Docs 目录失败: %v", err)
	}
	if count == 0 {
		t.Fatal("未找到 OpenAPI 契约文件（.yml/.yaml）")
	}
}
