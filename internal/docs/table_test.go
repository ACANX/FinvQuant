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
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "Dist", "pkg", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
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
			if len(block) < 2 || !tableSep.MatchString(block[1]) {
				t.Errorf("%s:%d 表格缺少表头或分隔行（首行：%s）", path, start+1, strings.TrimSpace(block[0]))
				continue
			}
			want := tableColumns(block[0])
			if got := tableColumns(block[1]); got != want {
				t.Errorf("%s:%d 分隔行列数 %d != 表头 %d", path, start+1, got, want)
				continue
			}
			for k, row := range block[2:] {
				if got := tableColumns(row); got != want {
					t.Errorf("%s:%d 数据行列数 %d != 表头 %d（%s）",
						path, start+1+k+2, got, want, strings.TrimSpace(row))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 Docs 目录失败: %v", err)
	}
}

// tableColumns 统计表格行的列数：忽略行首/行尾分隔符，
// 转义管道 \| 与 HTML 实体 &#124; 不算列分隔。
func tableColumns(line string) int {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	s = strings.ReplaceAll(s, `\|`, "\x00")
	s = strings.ReplaceAll(s, "&#124;", "\x00")
	return len(strings.Split(s, "|"))
}

// mergedCell 匹配「多个反引号标识符用斜杠或顿号挤在同一个单元格」的写法，
// 例如 `serial` / `imei` / `mac`。
var mergedCell = regexp.MustCompile("^`[^`]+`(\\s*[/、]\\s*`[^`]+`)+$")

// fieldHeaderWords 判定一张表是否为字段/参数定义表（只看这类表的前两列）。
var fieldHeaderWords = []string{"字段", "列", "参数", "名称", "键"}

// fieldRowExemptDirs 只增不改或受限改动的文档，历史写法不追改。
// - RequirementList.md / CHANGELOG.md：需求与变更日志只增不改；
// - DevSpec/：规范正文改动需单独授权；
// - Refactor/：改造记录是当时的事实快照；
// - Archive/：历史归档。
var fieldRowExemptDirs = []string{"/DevSpec/", "/Refactor/", "/Archive/"}

// TestMarkdownFieldRowsNotMerged 断言字段定义表里不出现「多字段挤一行」。
//
// 背景：数据字典 / 参数表里把 `serial` / `imei` / `mac` / `android_id` 写成一行，
// 会导致类型、必填、约束无法逐字段表达，违反「一个字段一行、不合并展示」（FE17）。
func TestMarkdownFieldRowsNotMerged(t *testing.T) {
	root := filepath.Join("..", "..", "Docs")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "Dist", "pkg", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}

		slash := filepath.ToSlash(path)
		if strings.HasSuffix(slash, "/RequirementList.md") || strings.HasSuffix(slash, "/CHANGELOG.md") {
			return nil
		}
		for _, dir := range fieldRowExemptDirs {
			if strings.Contains(slash, dir) {
				return nil
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")
		inCode := false
		header := ""
		for i := 0; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "```") {
				inCode = !inCode
				header = ""
				continue
			}
			if inCode {
				continue
			}
			if !strings.HasPrefix(trimmed, "|") {
				header = ""
				continue
			}
			if header == "" {
				// 表头行：下一行必须是分隔行，才认作表格开始
				if i+1 < len(lines) && tableSep.MatchString(lines[i+1]) {
					header = trimmed
				}
				continue
			}
			if tableSep.MatchString(trimmed) {
				continue
			}

			isFieldTable := false
			for _, word := range fieldHeaderWords {
				if strings.Contains(header, word) {
					isFieldTable = true
					break
				}
			}
			if !isFieldTable {
				continue
			}

			cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(trimmed, "|"), "|"), "|")
			if len(cells) == 0 {
				continue
			}
			if mergedCell.MatchString(strings.TrimSpace(cells[0])) {
				t.Errorf("%s:%d 字段定义表把多个字段挤在一行（%s）——应拆成每个字段一行",
					path, i+1, strings.TrimSpace(cells[0]))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 Docs 目录失败: %v", err)
	}
}
