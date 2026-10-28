package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tableSep 匹配 Markdown 表格分隔行（如 `|---|:---:|---|`）。
var tableSep = regexp.MustCompile(`^\s*\|[\s:|-]+\|\s*$`)

// TestMarkdownTablesHaveHeader 断言 Docs 下所有 Markdown 表格均带表头与分隔行。
//
// 背景：KLine 系列文档（含迁移后的 KLineUpsert.md）的错误码表格只有数据行、
// 缺少表头与 `|---|` 分隔行，Markdown 渲染成普通文本（需求 10778）。
// 代码块（``` 围栏）内的 `|` 行不参与检查。
func TestMarkdownTablesHaveHeader(t *testing.T) {
	root := filepath.Join("..", "..", "Docs")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")
		inCode := false
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "```") {
				inCode = !inCode
				continue
			}
			if inCode || !strings.HasPrefix(trimmed, "|") {
				continue
			}

			start := i
			block := make([]string, 0, 4)
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
				block = append(block, lines[i])
				i++
			}
			i-- // 抵消外层循环的 i++，让非表格行仍被检查

			// 合法表格：第一个数据行之前必须紧跟分隔行
			if len(block) >= 2 && tableSep.MatchString(block[1]) {
				continue
			}
			t.Errorf("%s:%d 表格缺少表头或分隔行（首行：%s）", path, start+1, strings.TrimSpace(block[0]))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 Docs 目录失败: %v", err)
	}
}
