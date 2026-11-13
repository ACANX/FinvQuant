package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// legacyQueryNames 允许保留的 snake_case 查询参数。
//
// 规范：URL 查询参数统一小驼峰（Docs/DevSpec/ApiSpec.md §3）。本表**只**用于
// 「已迁移为小驼峰、旧名保留做兼容兜底」的历史参数，新增接口不得往这里添加。
// 表内每一项都必须在同文件里同时存在对应的小驼峰主名（测试会校验）。
var legacyQueryNames = map[string]string{
	"page_size":   "兼容旧参数：pageSize 为主名",
	"flag_enable": "兼容旧参数：flagEnable 为主名",
	"secu_name":   "兼容旧参数：secuName 为主名",
	"start_ts":    "兼容旧参数：startTs 为主名",
	"end_ts":      "兼容旧参数：endTs 为主名",
}

// queryReadRe 匹配 URL 查询参数读取：c.Query("x") / c.DefaultQuery("x", ...) / c.GetQuery("x")。
var queryReadRe = regexp.MustCompile(`(?:Query|DefaultQuery|GetQuery)\("([A-Za-z0-9_-]+)"`)

// TestQueryParamsUseLowerCamelCase 静态检查 handler 层的查询参数命名：
// 出现 snake_case（含下划线）且不在 legacyQueryNames 表内的参数名即失败。
func TestQueryParamsUseLowerCamelCase(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("枚举 handler 源文件失败: %v", err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", file, err)
		}
		text := string(src)
		for _, m := range queryReadRe.FindAllStringSubmatch(text, -1) {
			name := m[1]
			if !strings.Contains(name, "_") {
				continue
			}
			checked++
			if _, ok := legacyQueryNames[name]; !ok {
				t.Errorf("%s：查询参数 %q 使用 snake_case；URL 查询参数必须小驼峰（ApiSpec §3）", file, name)
				continue
			}
			// 兼容期参数必须同时提供小驼峰主名，否则等于仍在用旧名。
			camel := snakeToLowerCamel(name)
			if !strings.Contains(text, `"`+camel+`"`) {
				t.Errorf("%s：兼容参数 %q 缺少小驼峰主名 %q", file, name, camel)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("未扫描到任何 snake_case 查询参数，检查正则是否失效")
	}
}

// snakeToLowerCamel 把 snake_case 转小驼峰：page_size → pageSize。
func snakeToLowerCamel(s string) string {
	parts := strings.Split(s, "_")
	out := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}
