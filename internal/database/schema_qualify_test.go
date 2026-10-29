package database

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// domainTablePrefixes 业务域表名前缀 → 必须使用的 schema 限定前缀。
//
// 为什么要强制限定：融合后不同 schema 可能出现同名表，而 search_path
// （public, quote, quant, futu, export, sche, dedup, cdeu, archive, sys）中 public 位于首位——任何未限定的 CRUD
// 语句都会优先命中 public 的同名表，导致读写错表。
var domainTablePrefixes = map[string]string{
	// CDEU 工作流域（V100078 建表、V100079 统一前缀、V100080 迁入数据）：
	// finv_quote_cdeu_* 必须写 cdeu. 前缀，不得落到 quote.finv_quote_cdeu_*（不存在）或无限定。
	"finv_quote_cdeu_": "cdeu.",
	"finv_quote_":      "quote.",
	"finv_quant_":      "quant.",
	"finv_futu_":       "futu.",
}

// cdeuTables CDEU 工作流域的表清单（V100078 建表、V100079 统一前缀，共 26 张）。
//
// 全部 25 张业务表统一为 finv_quote_cdeu_ 前缀：
//
//	· 原 cdeu_*（12 张）：V100079 把 cdeu_ 替换为 finv_quote_cdeu_
//	· 原 finv_dedup_*（13 张）：V100078 建表时即替换
//
// 另有 cdeu.finv_quote_secu_kline_min 副本，由 exactTableSchema 单独管控。
//
// 过渡期说明：dedup 下同名同构的表仍在使用（代码尚未切到 cdeu），
// 因此这里不强制 cdeu. 前缀；工作流改造切换读写后，再把对应表名从 dedup. 白名单
// 切到 {dedup., cdeu.} 或 {cdeu.}。
var cdeuTables = map[string]bool{
	// 原 cdeu_*（V100079 改名）
	"finv_quote_cdeu_workflow":         true,
	"finv_quote_cdeu_phase":            true,
	"finv_quote_cdeu_process":          true,
	"finv_quote_cdeu_job":              true,
	"finv_quote_cdeu_job_step":         true,
	"finv_quote_cdeu_job_dynamic":      true,
	"finv_quote_cdeu_job_step_dynamic": true,
	"finv_quote_cdeu_workflow_run":     true,
	"finv_quote_cdeu_phase_run":        true,
	"finv_quote_cdeu_process_run":      true,
	"finv_quote_cdeu_job_run":          true,
	"finv_quote_cdeu_job_step_run":     true,

	// 原 finv_dedup_*（V100078 建表时改名）
	"finv_quote_cdeu_daily_budget":            true,
	"finv_quote_cdeu_export_batch":            true,
	"finv_quote_cdeu_export_file":             true,
	"finv_quote_cdeu_export_file_patch":       true,
	"finv_quote_cdeu_export_file_upload_log":  true,
	"finv_quote_cdeu_export_job":              true,
	"finv_quote_cdeu_ingest_stats":            true,
	"finv_quote_cdeu_job_run_log_export_file": true,
	"finv_quote_cdeu_job_run_log_line":        true,
	"finv_quote_cdeu_quote_maturity_mon":      true,
	"finv_quote_cdeu_scan_watermark":          true,
	"finv_quote_cdeu_trade_calendar":          true,
	"finv_quote_cdeu_usc":                     true,

	// 表名不变
	"finv_quote_secu_kline_min": true,
}

// exactTableSchema 指定个别表的固定 schema（优先于前缀规则）。
//
// finv_quote_secu_kline_min 存在**多 schema 隔离副本**（ACANX 2026-10-27 纠偏 + 2026-10-28 迁入 cdeu）：
//   - quote.finv_quote_secu_kline_min：量化回测专用（回测 SQL 必须写 quote. 前缀）；
//   - cdeu.finv_quote_secu_kline_min：去重 / 导出 / 上传链路专用（V100080 起代码读写此处，
//     数据自 dedup 一次性迁入）；
//   - export.finv_quote_secu_kline_min：导出中转暂存（快照表，允许清空）；
//   - dedup.finv_quote_secu_kline_min：历史副本，代码已不读写，待运维清理淘汰。
//
// 各副本同名同构但职责不同，靠 schema 前缀强制隔离；漏写前缀会命中 search_path
// 里的另一张表，所以这里要求必须显式限定为其中之一。
var exactTableSchema = map[string][]string{
	"finv_quote_secu_kline_min": {"dedup.", "quote.", "export.", "cdeu."},
}

// sqlCRUDKeywords 用于识别「该行是 SQL 语句」的上下文，避免把注释误判。
var sqlCRUDKeywords = []string{
	"FROM ",
	"INTO ",
	"UPDATE ",
	"JOIN ",
	"DELETE FROM ",
	"TRUNCATE ",
}

// TestSQLSchemaQualified 扫描 internal/ 与 cmd/ 下的 Go 源码，断言所有
// 涉及域表的 CRUD 语句都写了 schema 全限定名（如 quote.finv_quote_xxx）。
// 该测试是改造项「表按业务域分 schema」的防回归闸门：新增代码若漏写前缀，
// `go test ./...` 会直接失败。
func TestSQLSchemaQualified(t *testing.T) {
	roots := []string{"..", filepath.Join("..", "..", "cmd")}
	var violations []string

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "webui", "dist", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "schema_qualify_test.go") {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for lineNo, line := range strings.Split(string(content), "\n") {
				if !isSQLCRUDLine(line) {
					continue
				}
				for _, table := range unqualifiedDomainTables(withoutExactTables(line)) {
					violations = append(violations,
						fmt.Sprintf("%s:%d: %s 必须写成 %s%s", path, lineNo+1, table,
							domainTablePrefixes[table], table))
				}
				for _, message := range wrongExactSchema(line) {
					violations = append(violations, fmt.Sprintf("%s:%d: %s", path, lineNo+1, message))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("遍历 %s 失败: %v", root, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("发现 %d 处未使用 schema 全限定名的 SQL 表引用（融合后可能与其它 schema 同名表冲突）：\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// withoutExactTables 移除由 exactTableSchema 单独校验的表名，避免与前缀规则重复。
func withoutExactTables(line string) string {
	for table := range exactTableSchema {
		line = strings.ReplaceAll(line, table, "")
	}
	return line
}

// wrongExactSchema 校验 exactTableSchema 中的表是否使用了规定 schema。
func wrongExactSchema(line string) []string {
	var problems []string
	for table, allowed := range exactTableSchema {
		want := allowed[0]
		for offset := 0; ; {
			index := strings.Index(line[offset:], table)
			if index < 0 {
				break
			}
			index += offset
			offset = index + len(table)
			// 向前解析可选 schema 前缀。
			start := index
			for start > 0 && isIdentifierChar(line[start-1]) {
				start--
			}
			if start == 0 || line[start-1] != '.' {
				problems = append(problems, fmt.Sprintf("%s 必须写成 %s%s", table, want, table))
				continue
			}
			schemaEnd := start - 1
			schemaStart := schemaEnd
			for schemaStart > 0 && isIdentifierChar(line[schemaStart-1]) {
				schemaStart--
			}
			schema := line[schemaStart:schemaEnd] + "."
			matched := false
			for _, candidate := range allowed {
				if schema == candidate {
					matched = true
					break
				}
			}
			if !matched {
				problems = append(problems, fmt.Sprintf("%s 必须写成 %s%s（当前 %s%s）", table, want, table, schema, table))
			}
		}
	}
	return problems
}

func isSQLCRUDLine(line string) bool {
	for _, keyword := range sqlCRUDKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

// unqualifiedDomainTables 返回该行中「未带 schema 前缀」的域表名前缀列表。
// 判定规则：表名前一个字符既不是 '.' 也不是标识符字符（字母/数字/下划线）。
//
// 注意：本函数只判断「有没有前缀」，不判断「前缀是否正确」。当同一表名存在
// 于多个 schema（如 cdeu.finv_quote_secu_kline_min 与
// export.finv_quote_secu_kline_min 同名）时，写错前缀不会被拦下，需靠
// Docs/DevSpec/DbSchemaSpec.md §2/§3.1 的约定保证。
func unqualifiedDomainTables(line string) []string {
	var found []string
	for table := range domainTablePrefixes {
		// 若本前缀是另一个更长前缀的头部（如 finv_quote_ 之于 finv_quote_cdeu_），
		// 跳过它，由最长前缀负责判定，避免同一处报出两条互相矛盾的建议。
		shadowed := false
		for other := range domainTablePrefixes {
			if other != table && strings.HasPrefix(other, table) {
				shadowed = true
				break
			}
		}
		if shadowed {
			continue
		}
		offset := 0
		for {
			index := strings.Index(line[offset:], table)
			if index < 0 {
				break
			}
			index += offset
			if index == 0 || !isIdentifierChar(line[index-1]) && line[index-1] != '.' {
				found = append(found, table)
			}
			offset = index + len(table)
		}
	}
	return found
}

func isIdentifierChar(char byte) bool {
	return char == '_' ||
		(char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9')
}
