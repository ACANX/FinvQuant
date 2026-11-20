package cdeu

import (
	"errors"
	"strings"
	"testing"
)

// 采集参数查询 / 维护（secu_ftmm_collect_store.go）的纯函数与校验单测：不连库。

func TestSecuFtmmCollectWhereDefaults(t *testing.T) {
	where, args := secuFtmmCollectWhere(SecuFtmmCollectFilter{})
	if where != "" || len(args) != 0 {
		t.Fatalf("无条件时不应生成 WHERE，实际 where=%q args=%v", where, args)
	}
}

func TestSecuFtmmCollectWhereFilters(t *testing.T) {
	where, args := secuFtmmCollectWhere(SecuFtmmCollectFilter{
		Keyword:      "00700",
		MarketLabel:  "hk",
		Instrument:   "STOCK",
		SecurityType: "Stock",
		FlagEnable:   "1",
		USCMissing:   true,
	})
	if len(args) != 5 {
		t.Fatalf("应产生 5 个参数，实际 %v", args)
	}
	// 市场统一大写、品种统一小写（列里存的是 HK / stock）
	if args[1] != "HK" || args[2] != "stock" {
		t.Fatalf("筛选值大小写归一不符：market=%v instrument=%v", args[1], args[2])
	}
	for _, want := range []string{
		"c.usc ILIKE $1",
		"COALESCE(s.security_name_cn, '') ILIKE $1",
		"upper(c.market_label) = $2",
		"lower(c.instrument) = $3",
		"s.security_type = $4",
		"c.flag_enable = $5",
		"s.usc IS NULL",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("WHERE 缺少 %q，实际 %s", want, where)
		}
	}
}

func TestSecuFtmmCollectWhereIgnoresAll(t *testing.T) {
	for _, filter := range []SecuFtmmCollectFilter{
		{MarketLabel: "all"},
		{Instrument: "ALL"},
		{SecurityType: "all"},
		{FlagEnable: "all"},
	} {
		where, args := secuFtmmCollectWhere(filter)
		if where != "" || len(args) != 0 {
			t.Fatalf("all 应视同不限，实际 where=%q args=%v", where, args)
		}
	}
}

func TestUpdateSecuFtmmCollectValidation(t *testing.T) {
	store := NewStore(nil) // 校验在触库之前，nil 池足够
	intPtr := func(v int) *int { return &v }
	int64Ptr := func(v int64) *int64 { return &v }
	strPtr := func(v string) *string { return &v }

	cases := map[string]SecuFtmmCollectPatch{
		"缺 usc":             {MarketLabel: strPtr("HK")},
		"没有字段":              {USC: "00700"},
		"stock_id <= 0":     {USC: "00700", StockID: int64Ptr(0)},
		"market_type <= 0":  {USC: "00700", MarketType: intPtr(-1)},
		"market_code <= 0":  {USC: "00700", MarketCode: intPtr(0)},
		"instrument_type 0": {USC: "00700", InstrumentType: intPtr(0)},
		"sub_type 0":        {USC: "00700", SubInstrumentType: intPtr(0)},
		"空市场标签":             {USC: "00700", MarketLabel: strPtr("  ")},
		"空品种":               {USC: "00700", Instrument: strPtr(" ")},
		"空富途代码":             {USC: "00700", FutuSymbol: strPtr("")},
		"非法启用标志":            {USC: "00700", FlagEnable: strPtr("2")},
	}
	for name, patch := range cases {
		if _, err := store.UpdateSecuFtmmCollect(t.Context(), patch); !errors.Is(err, ErrSecuFtmmCollectInvalid) {
			t.Fatalf("%s：应返回 ErrSecuFtmmCollectInvalid，实际 %v", name, err)
		}
	}
}

func TestSecuFtmmCollectOrderColumns(t *testing.T) {
	for _, column := range []string{"usc", "ftmm_symbol", "market_label", "gmt_update"} {
		if !SecuFtmmCollectOrderColumns[column] {
			t.Fatalf("%s 应在排序白名单里", column)
		}
	}
	// 生成列 / 关联列不作为排序字段，避免误以为可按证券名称排序
	for _, column := range []string{"security_name", "security_type", "gmt_create"} {
		if SecuFtmmCollectOrderColumns[column] {
			t.Fatalf("%s 不应在排序白名单里", column)
		}
	}
}

func TestNormalizeFilterValue(t *testing.T) {
	if got := normalizeFilterValue("  "); got != "" {
		t.Fatalf("空白应为不限，实际 %q", got)
	}
	if got := normalizeFilterValue("All"); got != "" {
		t.Fatalf("all 应为不限，实际 %q", got)
	}
	if got := normalizeFilterValue(" HK "); got != "HK" {
		t.Fatalf("应去空白，实际 %q", got)
	}
}
