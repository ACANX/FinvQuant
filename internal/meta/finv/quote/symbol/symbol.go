// Package symbol 统一 ftmm_symbol 的入库形态（需求 11143）。
//
// ftmm_symbol 是证券键（`{市场标签}.{富途代码}`），既是本地主键也是上游关联键。
// 统一规则：**英文字母一律大写**，非字母字符（`.`、`,`、`&`、`*`、`-` 等）原样保留，
// 前后空白去掉。上游偶尔给出小写形态（如 `GL.quote-forex-all-forex`），
// 若原样入库会与已有大写键并存，同一只证券被当成两行。
package symbol

import "strings"

// Normalize 把 ftmm_symbol 归一成入库形态：去空白 + 英文字母转大写。
//
// 入库的每一条路径（单条 insert、批量 upsert、增量更新、外部接口回写）都必须先过这里，
// 或在 SQL 侧用 `upper(btrim(...))` 做同样的事。
func Normalize(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

// NormalizeAll 批量归一（就地修改并返回入参切片，便于链式调用）。
func NormalizeAll(values []string) []string {
	for i, value := range values {
		values[i] = Normalize(value)
	}
	return values
}
