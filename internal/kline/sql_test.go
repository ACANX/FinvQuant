package kline

import (
	"strings"
	"testing"
)

func TestTableConstantQualified(t *testing.T) {
	// K 线 API（写入/查询）自 V100080 起读写 cdeu.finv_quote_secu_kline_min
	// （原 dedup 副本的数据已迁入 cdeu，dedup 侧待淘汰）。
	if !strings.HasPrefix(Table, "cdeu.") {
		t.Fatalf("Table 常量必须使用 cdeu. 前缀，实际 %s", Table)
	}
}

func TestInsertSQLUsesSchemaQualifiedTable(t *testing.T) {
	sql := insertSQL(2, "")
	if !strings.Contains(sql, "INSERT INTO cdeu.finv_quote_secu_kline_min") {
		t.Fatalf("缺少 schema 全限定表名: %s", sql)
	}
	if strings.Contains(sql, "quote.finv_quote_secu_kline_min") &&
		!strings.Contains(sql, "cdeu.finv_quote_secu_kline_min") {
		t.Fatal("不得使用 quote. 前缀")
	}
	// 两行共 32 个参数占位符：$1..$32。
	for _, placeholder := range []string{"$1,", "$16)", "($17,", "$32)"} {
		if !strings.Contains(sql, placeholder) {
			t.Fatalf("缺少占位符 %s: %s", placeholder, sql)
		}
	}
	if strings.Contains(sql, "?") {
		t.Fatalf("不得使用 ? 占位符: %s", sql)
	}
}

func TestUpsertSQLModes(t *testing.T) {
	skip := upsertSQL("skip")
	if !strings.Contains(skip, "ON CONFLICT (usc, ts) DO NOTHING") {
		t.Fatalf("skip 模式错误: %s", skip)
	}
	update := upsertSQL("update")
	for _, column := range UpsertUpdateColumns {
		if !strings.Contains(update, column+` = EXCLUDED.`+column) {
			// 列名带引号，改用引号形式断言
			if !strings.Contains(update, `"`+column+`" = EXCLUDED."`+column+`"`) {
				t.Fatalf("update 模式缺少列 %s: %s", column, update)
			}
		}
	}
	if !strings.Contains(update, `"gmt_update" = now()`) {
		t.Fatalf("update 模式应刷新 gmt_update: %s", update)
	}
	if strings.Contains(update, `"usc" = EXCLUDED.`) || strings.Contains(update, `"ts" = EXCLUDED.`) {
		t.Fatalf("update 模式不得覆盖 usc/ts: %s", update)
	}
}

func TestPatchSQLCasts(t *testing.T) {
	sql := patchSQL([]string{ColVolume, ColRemark}, 2)
	if !strings.Contains(sql, "UPDATE cdeu.finv_quote_secu_kline_min AS t") {
		t.Fatalf("缺少表名: %s", sql)
	}
	for _, cast := range []string{"$1::varchar", "$2::bigint", "$3::numeric(20,6)", "$4::text"} {
		if !strings.Contains(sql, cast) {
			t.Fatalf("缺少类型转换 %s: %s", cast, sql)
		}
	}
	if !strings.Contains(sql, `"volume" = v."volume"`) {
		t.Fatalf("缺少赋值: %s", sql)
	}
	if !strings.Contains(sql, "AS v(\"usc\", \"ts\", \"volume\", \"remark\")") {
		t.Fatalf("别名列缺失: %s", sql)
	}
}

func TestListWhere(t *testing.T) {
	usc, provider := "A", "FTMM"
	dateFrom, dateTo := 20260101, 20261231
	tsFrom, tsTo := int64(1), int64(100)
	where, args := listWhere(ListFilter{
		Usc: &usc, Provider: &provider, DateFrom: &dateFrom, DateTo: &dateTo,
		TsFrom: &tsFrom, TsTo: &tsTo,
	})
	if len(args) != 6 {
		t.Fatalf("参数数量错误: %d", len(args))
	}
	for i := 1; i <= 6; i++ {
		if !strings.Contains(where, "$"+itoa(i)) {
			t.Fatalf("缺少占位符 $%d: %s", i, where)
		}
	}
	if !strings.Contains(where, `"date" >= $3`) || !strings.Contains(where, `"ts" <= $6`) {
		t.Fatalf("条件构造错误: %s", where)
	}
	// 无条件时为空串。
	if w, a := listWhere(ListFilter{}); w != "" || len(a) != 0 {
		t.Fatalf("空条件应返回空串，实际 %q %v", w, a)
	}
}

func TestQueryWhere(t *testing.T) {
	provider := "FTMM"
	where, args := queryWhere(QuerySpec{
		Keys:     []Key{{Usc: "A", Ts: 1}, {Usc: "B", Ts: 2}},
		Uscs:     []string{"C"},
		Provider: &provider,
	})
	if len(args) != 6 { // 4(keys) + 1(uscs) + 1(provider)
		t.Fatalf("参数数量错误: %d", len(args))
	}
	if !strings.Contains(where, `("usc", "ts") IN (($1, $2), ($3, $4))`) {
		t.Fatalf("keys 条件错误: %s", where)
	}
	if !strings.Contains(where, `"usc" = ANY($5)`) {
		t.Fatalf("uscs 条件错误: %s", where)
	}
	if !strings.Contains(where, `"provider" = $6`) {
		t.Fatalf("provider 条件错误: %s", where)
	}
}

func TestDeleteWhere(t *testing.T) {
	where, args := deleteWhere([]Key{{Usc: "A", Ts: 1}}, []string{"B"})
	if len(args) != 3 {
		t.Fatalf("参数数量错误: %d", len(args))
	}
	if !strings.Contains(where, " OR ") {
		t.Fatalf("keys/uscs 应为 OR 语义: %s", where)
	}
	if w, _ := deleteWhere(nil, nil); w != "" {
		t.Fatalf("空条件应返回空串，实际 %q", w)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
