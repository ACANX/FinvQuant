package quote

import (
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/finv/quote/mvsv"
)

func sp(v string) *string { return &v }

func TestComputeChangeFields(t *testing.T) {
	rows := []mvsv.Row{
		// 1) 已提供 cp/cr：保持不变（有值就不补全）
		{Close: sp("11"), LastClose: sp("10"), Open: sp("9"),
			ChangePrice: sp("1.5"), ChangeRatio: sp("15")},
		// 2) 缺 cp/cr：last_close 优先
		{Close: sp("11"), LastClose: sp("10"), Open: sp("9")},
		// 3) last_close 缺失：回退 open
		{Close: sp("21"), LastClose: nil, Open: sp("20")},
		// 4) last_close 为 0：回退 open
		{Close: sp("21"), LastClose: sp("0"), Open: sp("20")},
		// 5) 两者都不可用：不写
		{Close: sp("21"), LastClose: nil, Open: nil},
	}
	computeChangeFields(rows)

	if rows[0].ChangePrice == nil || *rows[0].ChangePrice != "1.5" ||
		rows[0].ChangeRatio == nil || *rows[0].ChangeRatio != "15" {
		t.Fatalf("已有 cp/cr 不应被重算: %+v", rows[0])
	}
	if *rows[1].ChangePrice != "1.000000" || *rows[1].ChangeRatio != "10.000000" {
		t.Fatalf("last_close 优先计算错误: %v / %v", *rows[1].ChangePrice, *rows[1].ChangeRatio)
	}
	if *rows[2].ChangePrice != "1.000000" || *rows[2].ChangeRatio != "5.000000" {
		t.Fatalf("open 兜底计算错误: %v / %v", *rows[2].ChangePrice, *rows[2].ChangeRatio)
	}
	if *rows[3].ChangePrice != "1.000000" || *rows[3].ChangeRatio != "5.000000" {
		t.Fatalf("last_close=0 时应回退 open: %v / %v", *rows[3].ChangePrice, *rows[3].ChangeRatio)
	}
	if rows[4].ChangePrice != nil || rows[4].ChangeRatio != nil {
		t.Fatalf("无参考价时应保持 NULL: %+v", rows[4])
	}
}

func TestBuildUpsertSQLIncludesChangeColumns(t *testing.T) {
	for _, mode := range []UpsertMode{UpsertModeField, UpsertModeRow} {
		sql := buildUpsertSQL(mode)
		for _, column := range []string{"change_price", "change_ratio"} {
			if !strings.Contains(sql, column) {
				t.Fatalf("%s 模式 SQL 缺少列 %s: %s", mode, column, sql)
			}
		}
	}
}
