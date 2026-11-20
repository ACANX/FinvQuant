// MVSV 数据区的字段转义（MVSV-2 可选扩展，头部 `# Escape : "backslash"`）。
//
// 背景：MVSV 数据行以 `|` 分隔字段、以换行分隔记录，而 `ext_field` / `remark` 等
// 文本列本身可能包含 `|`、换行或反斜杠（历史数据中存在）。若不转义：
//
//	· 含 `|` 的行会被切出多余列 → 解析端列数校验失败，整个文件不可用；
//	· 含换行的值会把一条记录拆成两行 → 行数错乱，定位困难。
//
// 规则（可逆、最小侵入）：
//
//	`\`  → `\\`
//	`|`  → `\|`
//	换行 → `\n`
//	回车 → `\r`
//
// 兼容性：
//   - 写出时无条件转义，并在头部标记 `# Escape : "backslash"`；
//   - 读取时**只有**头部声明了该标记才按转义解析；未声明则沿用旧的 `strings.Split`，
//     历史文件（不含转义）行为完全不变；
//   - 未知转义序列（如 `\x`）按原样保留（`\` + 字符），避免把旧数据里的反斜杠吃掉。
package mvsv

import (
	"fmt"
	"strings"
)

// EscapeHeaderKey 头部转义标记键。
const EscapeHeaderKey = "Escape"

// EscapeBackslash 当前唯一的转义方案名。
const EscapeBackslash = "backslash"

// escapeCell 对单个字段值做转义（写侧）。
//
// 顺序要求：先转 `\`，再转 `|` 与换行，避免二次转义。
func escapeCell(value string) string {
	if !strings.ContainsAny(value, "\\|\n\r") {
		return value
	}
	var b strings.Builder
	b.Grow(len(value) + 8)
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '|':
			b.WriteString(`\|`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unescapeCell 还原单个字段值（读侧）。
func unescapeCell(value string) string {
	if !strings.Contains(value, `\`) {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	escaped := false
	for _, r := range value {
		if escaped {
			switch r {
			case '\\':
				b.WriteRune('\\')
			case '|':
				b.WriteRune('|')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			default:
				// 未知转义：保留反斜杠与字符本身（兼容旧数据里的裸反斜杠）。
				b.WriteRune('\\')
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		// 末尾孤立反斜杠：原样保留。
		b.WriteRune('\\')
	}
	return b.String()
}

// splitEscaped 按未转义的 `|` 切分一行，并还原各字段。
//
// expected > 0 时校验列数；expected <= 0 不校验（由调用方自行处理）。
func splitEscaped(line string, expected int) ([]string, error) {
	fields := make([]string, 0, expected)
	var b strings.Builder
	escaped := false
	for _, r := range line {
		if escaped {
			b.WriteRune('\\')
			b.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '|':
			fields = append(fields, unescapeCell(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	fields = append(fields, unescapeCell(b.String()))

	if expected > 0 && len(fields) != expected {
		return nil, fmt.Errorf("列数=%d，与 Field 列数 %d 不一致（已按 Escape=%s 解析）",
			len(fields), expected, EscapeBackslash)
	}
	return fields, nil
}

// usesEscape 判断头部是否声明了反斜杠转义。
func usesEscape(values map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(values[EscapeHeaderKey]), EscapeBackslash)
}
