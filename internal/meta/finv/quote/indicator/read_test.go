package indicator

import (
	"errors"
	"strings"
	"testing"
)

// 只读查询（read.go）的纯函数单测：不连库，覆盖列归一、白名单校验与 WHERE 拼装。

func knownColumns(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func TestResolverIndicatorColumnsDefaults(t *testing.T) {
	known := knownColumns("usc", "ftmm_symbol", "name", "price", "dt_update")
	for _, name := range indicatorDefaultColumns {
		known[name] = true
	}

	selected, err := resolveIndicatorColumns(nil, known)
	if err != nil {
		t.Fatalf("默认列集不应报错: %v", err)
	}
	if len(selected) != len(indicatorDefaultColumns) {
		t.Fatalf("默认列集长度 = %d，期望 %d", len(selected), len(indicatorDefaultColumns))
	}
	// 身份列必须前置且不重复
	if selected[0] != "usc" || selected[1] != "ftmm_symbol" {
		t.Fatalf("身份列应前置，实际前两列 = %v", selected[:2])
	}
	seen := map[string]bool{}
	for _, name := range selected {
		if seen[name] {
			t.Fatalf("列 %s 重复出现", name)
		}
		seen[name] = true
	}
}

func TestResolverIndicatorColumnsKeepsRequestOrder(t *testing.T) {
	known := knownColumns("usc", "ftmm_symbol", "price", "pe_ttm", "json_unmapped")

	selected, err := resolveIndicatorColumns([]string{"pe_ttm", "", "price ", "pe_ttm"}, known)
	if err != nil {
		t.Fatalf("应通过: %v", err)
	}
	want := []string{"usc", "ftmm_symbol", "pe_ttm", "price"}
	if strings.Join(selected, ",") != strings.Join(want, ",") {
		t.Fatalf("列集 = %v，期望 %v", selected, want)
	}
}

func TestResolverIndicatorColumnsRejectsUnknown(t *testing.T) {
	known := knownColumns("usc", "ftmm_symbol", "price")

	_, err := resolveIndicatorColumns([]string{"price; DROP TABLE x"}, known)
	if !errors.Is(err, ErrQueryInvalid) {
		t.Fatalf("未登记的列应返回 ErrQueryInvalid，实际 %v", err)
	}
}

func TestResolverIndicatorColumnsRequiresKeyColumns(t *testing.T) {
	_, err := resolveIndicatorColumns([]string{"price"}, knownColumns("price"))
	if !errors.Is(err, ErrQueryInvalid) {
		t.Fatalf("缺少身份列应返回 ErrQueryInvalid，实际 %v", err)
	}
}

func TestIndicatorDataType(t *testing.T) {
	cases := map[string]string{
		"numeric(30,10)":           "numeric",
		"integer":                  "integer",
		"bigint":                   "integer",
		"smallint":                 "integer",
		"text":                     "text",
		"character varying(32)":    "text",
		"timestamp with time zone": "timestamp",
		"boolean":                  "boolean",
		"jsonb":                    "other",
	}
	for typeName, want := range cases {
		if got := indicatorDataType(typeName); got != want {
			t.Errorf("indicatorDataType(%q) = %q，期望 %q", typeName, got, want)
		}
	}
}

func TestNewIndicatorColumnFallsBackToName(t *testing.T) {
	column := newIndicatorColumn("pe_ttm", "  ", "numeric(30,10)")
	if column.Title != "pe_ttm" {
		t.Fatalf("注释为空时标题应回退列名，实际 %q", column.Title)
	}
	if !column.Sortable || column.DataType != "numeric" {
		t.Fatalf("列元数据不符: %+v", column)
	}
}

func TestIndicatorWhere(t *testing.T) {
	where, args := indicatorWhere(IndicatorPageFilter{})
	if where != "" || len(args) != 0 {
		t.Fatalf("无条件时不应生成 WHERE，实际 where=%q args=%v", where, args)
	}

	where, args = indicatorWhere(IndicatorPageFilter{Keyword: "00700", Market: "hk"})
	if len(args) != 2 {
		t.Fatalf("两个筛选项应产生 2 个参数，实际 %v", args)
	}
	if !strings.Contains(where, "ILIKE $1") || !strings.Contains(where, "split_part(t.ftmm_symbol, '.', 1) = $2") {
		t.Fatalf("WHERE 子句不符: %s", where)
	}
	if args[1] != "HK" {
		t.Fatalf("市场参数应大写，实际 %v", args[1])
	}

	// all 视同不限
	where, args = indicatorWhere(IndicatorPageFilter{Market: "all"})
	if where != "" || len(args) != 0 {
		t.Fatalf("market=all 应视同不限，实际 where=%q args=%v", where, args)
	}
}
