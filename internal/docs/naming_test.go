package docs

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"testing"
)

// processDocName 过程文档命名：yyMM-NNN-大驼峰名称.md（见 Docs/DevSpec/FileNamingSpec.md 第 4 节）。
var processDocName = regexp.MustCompile(`^(\d{4}-\d{3})-`)

// TestProcessDocNumberUnique 断言 Docs 下过程文档的 yyMM-NNN 编号全局唯一。
func TestProcessDocNumberUnique(t *testing.T) {
	root := filepath.Join("..", "..", "Docs")
	seen := map[string]string{}
	duplicates := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		match := processDocName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil
		}
		prefix := match[1]
		if previous, exists := seen[prefix]; exists {
			duplicates = append(duplicates, prefix+": "+previous+" <-> "+path)
			return nil
		}
		seen[prefix] = path
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 Docs 目录失败: %v", err)
	}
	for _, duplicate := range duplicates {
		t.Errorf("文档编号重复: %s", duplicate)
	}
}
