package secuft

import (
	"encoding/json"
	"net/url"
	"testing"
)

// TestNormalizeQuery 缺省与越界参数收敛。
func TestNormalizeQuery(t *testing.T) {
	query := Query{}
	query.Normalize()
	if query.Page != 1 {
		t.Errorf("page 默认应为 1，实际 %d", query.Page)
	}
	if query.PageSize != DefaultPageSize || DefaultPageSize != 15 {
		t.Errorf("pageSize 默认应为 15，实际 %d", query.PageSize)
	}
	if query.SortBy != DefaultSortBy {
		t.Errorf("默认排序应为 %s，实际 %s", DefaultSortBy, query.SortBy)
	}

	query = Query{Page: 3, PageSize: 1000, SortBy: "no_such_column", FlagEnable: "all"}
	query.Normalize()
	if query.PageSize != MaxPageSize {
		t.Errorf("pageSize 应被截到 %d，实际 %d", MaxPageSize, query.PageSize)
	}
	if query.SortBy != DefaultSortBy {
		t.Errorf("非法排序列应回退默认，实际 %s", query.SortBy)
	}
	if query.FlagEnable != "" {
		t.Errorf("flagEnable=all 应视为不限，实际 %q", query.FlagEnable)
	}
	if query.Offset() != 2*MaxPageSize {
		t.Errorf("offset 应为 %d，实际 %d", 2*MaxPageSize, query.Offset())
	}
}

// TestBuildQueryValues 查询参数组装：列清单、排序兜底、筛选与分页。
func TestBuildQueryValues(t *testing.T) {
	query := Query{
		Page:        2,
		PageSize:    15,
		Keyword:     "00700",
		TextFilters: map[string]string{"marketLabel": "HK", "instrument": "all", "typeSecu": "", "provider": "FTMM"},
		FlagEnable:  "1",
		SortBy:      "dt_update",
		SortDesc:    true,
	}
	query.Normalize()
	values := buildQueryValues(query)

	if got := values.Get("select"); got == "" {
		t.Fatal("select 不能为空")
	} else {
		var cols []string
		for _, name := range columnNames() {
			cols = append(cols, name)
		}
		if len(cols) != 32 {
			t.Errorf("select 应含 32 列，实际 %d", len(cols))
		}
	}
	if got := values.Get("order"); got != "dt_update.desc,usc.asc" {
		t.Errorf("order 应为 dt_update.desc,usc.asc，实际 %q", got)
	}
	if values.Get("limit") != "15" || values.Get("offset") != "15" {
		t.Errorf("分页参数错误: limit=%s offset=%s", values.Get("limit"), values.Get("offset"))
	}
	if got := values.Get("market_label"); got != "eq.HK" {
		t.Errorf("market_label 应为 eq.HK，实际 %q", got)
	}
	if got := values.Get("provider"); got != "eq.FTMM" {
		t.Errorf("provider 应为 eq.FTMM，实际 %q", got)
	}
	if values.Has("instrument") {
		t.Error("instrument=all 不应下推为筛选条件")
	}
	if values.Has("type_secu") {
		t.Error("typeSecu 为空不应下推为筛选条件")
	}
	if got := values.Get("flag_enable"); got != "eq.1" {
		t.Errorf("flag_enable 应为 eq.1，实际 %q", got)
	}
	if got := values.Get("or"); got != "(usc.ilike.*00700*,symbol.ilike.*00700*,futu_symbol.ilike.*00700*,name.ilike.*00700*)" {
		t.Errorf("关键字表达式不符: %q", got)
	}

	// 排序本身是 usc 时不再追加兜底键
	plain := Query{Page: 1, PageSize: 15, SortBy: "usc"}
	plain.Normalize()
	if got := buildQueryValues(plain).Get("order"); got != "usc.asc" {
		t.Errorf("usc 排序不应重复追加，实际 %q", got)
	}
}

// TestKeywordExpressionEscapes 关键字里的通配/分隔字符必须转义，避免越权重写查询语义。
func TestKeywordExpressionEscapes(t *testing.T) {
	expr := keywordExpression("a*b,c(d)")
	for _, column := range KeywordColumns {
		want := column + `.ilike.*a\*b\,c\(d\)*`
		if idx := indexOf(expr, want); idx < 0 {
			t.Errorf("缺少转义后的表达式 %q，实际 %q", want, expr)
		}
	}
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestJSONText 原始 JSON 值转展示文本。
func TestJSONText(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`"00700"`, "00700"},
		{`83571475946861`, "83571475946861"},
		{`null`, ""},
		{``, ""},
		{`"深圳"`, "深圳"},
	}
	for _, item := range cases {
		if got := jsonText(json.RawMessage(item.raw)); got != item.want {
			t.Errorf("jsonText(%s) = %q，期望 %q", item.raw, got, item.want)
		}
	}
}

// TestBuildQueryURLEncodes 编码后仍能被 PostgREST 正确解析（括号/逗号不得丢失语义）。
func TestBuildQueryURLEncodes(t *testing.T) {
	query := Query{Page: 1, PageSize: 15, Keyword: "腾讯", TextFilters: map[string]string{"marketLabel": "HK"}}
	query.Normalize()
	encoded := buildQueryValues(query).Encode()
	parsed, err := url.ParseQuery(encoded)
	if err != nil {
		t.Fatalf("参数编码后无法解析: %v", err)
	}
	if parsed.Get("or") != "(usc.ilike.*腾讯*,symbol.ilike.*腾讯*,futu_symbol.ilike.*腾讯*,name.ilike.*腾讯*)" {
		t.Errorf("or 参数往返丢失语义: %q", parsed.Get("or"))
	}
}
