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
	// 身份列（仅 usc）必须前置且不重复；ftmm_symbol 不再强制
	if selected[0] != "usc" {
		t.Fatalf("身份列应前置，实际首列 = %v", selected[0])
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
	want := []string{"usc", "pe_ttm", "price"}
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

// TestIndicatorWhereTypeSecuSub 证券属性子类筛选（需求 11305）。
func TestIndicatorWhereTypeSecuSub(t *testing.T) {
	where, args := indicatorWhere(IndicatorPageFilter{TypeSecuSub: "Stock.US"})
	if !strings.Contains(where, "s.type_secu_sub = $1") {
		t.Fatalf("子类筛选应精确匹配 s.type_secu_sub，实际 %s", where)
	}
	if len(args) != 1 || args[0] != "Stock.US" {
		t.Fatalf("子类参数不符: %v", args)
	}

	// __empty__ = 未维护 / 未关联（不带参数）
	where, args = indicatorWhere(IndicatorPageFilter{TypeSecuSub: IndicatorTypeSecuSubEmpty})
	if !strings.Contains(where, "COALESCE(s.type_secu_sub, '') = ''") {
		t.Fatalf("__empty__ 应匹配空串，实际 %s", where)
	}
	if len(args) != 0 {
		t.Fatalf("__empty__ 不应带参数，实际 %v", args)
	}

	// all 视同不限
	where, args = indicatorWhere(IndicatorPageFilter{TypeSecuSub: "all"})
	if where != "" || len(args) != 0 {
		t.Fatalf("typeSecuSub=all 应视同不限，实际 where=%q args=%v", where, args)
	}
}

// TestIndicatorWhereTypeSecu 证券分类筛选（需求 11308）。
func TestIndicatorWhereTypeSecu(t *testing.T) {
	where, args := indicatorWhere(IndicatorPageFilter{TypeSecu: "Stock"})
	if !strings.Contains(where, "s.type_secu = $1") {
		t.Fatalf("分类筛选应精确匹配 s.type_secu，实际 %s", where)
	}
	if len(args) != 1 || args[0] != "Stock" {
		t.Fatalf("分类参数不符: %v", args)
	}

	where, args = indicatorWhere(IndicatorPageFilter{TypeSecu: IndicatorTypeSecuEmpty})
	if !strings.Contains(where, "COALESCE(s.type_secu, '') = ''") || len(args) != 0 {
		t.Fatalf("__empty__ 分类筛选不符: where=%s args=%v", where, args)
	}

	// 分类与子类同时给 → 两个条件 AND
	where, args = indicatorWhere(IndicatorPageFilter{TypeSecu: "Stock", TypeSecuSub: "Stock.US"})
	if len(args) != 2 || !strings.Contains(where, "s.type_secu = $1") ||
		!strings.Contains(where, "s.type_secu_sub = $2") {
		t.Fatalf("分类+子类组合不符: where=%s args=%v", where, args)
	}
}

// TestIndicatorVirtualColumns 虚拟列 type_secu / type_secu_sub 可被请求列与排序使用。
func TestIndicatorVirtualColumns(t *testing.T) {
	known := knownColumns("usc", "ftmm_symbol", "name", "dt_collect")
	known[IndicatorVirtualUSN] = true
	known[IndicatorVirtualTypeSecu] = true
	known[IndicatorVirtualTypeSecuSub] = true

	selected, err := resolveIndicatorColumns(
		[]string{IndicatorVirtualUSN, IndicatorVirtualTypeSecu, IndicatorVirtualTypeSecuSub}, known)
	if err != nil {
		t.Fatalf("虚拟列应可请求: %v", err)
	}
	if len(selected) != 4 || selected[1] != IndicatorVirtualUSN ||
		selected[2] != IndicatorVirtualTypeSecu || selected[3] != IndicatorVirtualTypeSecuSub {
		t.Fatalf("列集不符: %v", selected)
	}

	// 真实列目录里没有它时（未关联主数据表）必须报错，避免拼出无效 SQL
	if _, err := resolveIndicatorColumns([]string{IndicatorVirtualTypeSecu},
		knownColumns("usc", "ftmm_symbol", "name")); err == nil {
		t.Fatal("白名单缺少虚拟列时应报错")
	}
}
