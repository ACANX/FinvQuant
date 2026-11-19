package secuft

import (
	"encoding/json"
	"testing"
)

func strPtr(v string) *string { return &v }
func i64Ptr(v int64) *int64   { return &v }
func i32Ptr(v int32) *int32   { return &v }

// TestColumnsMatchProbedTable 列元数据必须与实测的 32 列一致（需求 11344/11345）。
//
// 实测方式：抓 `select=*` 的 CSV 表头（32 列），并用 `?<col>=eq.@@@` 反推类型。
func TestColumnsMatchProbedTable(t *testing.T) {
	want := []string{
		"usc", "name", "region", "market", "market_label", "symbol", "currency", "timezone",
		"provider", "type_secu", "type_secu_sub", "stock_id", "market_type", "market_code",
		"instrument_type", "sub_instrument_type", "req_section", "weight_priority",
		"weight_frequency", "ts_latest_data", "count_last_fetch", "count_fail", "count_stale",
		"dt_update", "dt_last_check", "dt_last_fetch", "gmt_update", "gmt_create",
		"futu_symbol", "flag_enable", "instrument", "remark",
	}
	cols := Columns()
	if len(cols) != len(want) {
		t.Fatalf("列数 %d，期望 %d", len(cols), len(want))
	}
	for i, name := range want {
		if cols[i].Name != name {
			t.Errorf("第 %d 列应为 %s，实际 %s", i+1, name, cols[i].Name)
		}
		if cols[i].Title == "" {
			t.Errorf("列 %s 缺少中文表头", name)
		}
	}
}

// TestBuildLookupResultFull 四表齐全（真实样本 00700）。
func TestBuildLookupResultFull(t *testing.T) {
	row := lookupRow{
		usc: "00700", securityName: "TENCENT", securityNameCn: "腾讯控股",
		typeSecu: "Stock", typeSecuSub: "", provider: "FTMM",
		currencyType: "HKD", tz: "Asia/Hong_Kong", exchangeCode: 21,
		collectFtmm: strPtr("HK.00700"), futuSymbol: strPtr("00700"), stockID: i64Ptr(54047868453564),
		marketLabel: strPtr("HK"), marketType: i32Ptr(1), marketCode: i32Ptr(1),
		instrument: strPtr("stock"), instrumentType: i32Ptr(3), subInstrumentTyp: i32Ptr(3002),
		reqSection: strPtr(""), keyMarket: strPtr("HK"), keyFtmm: strPtr("HK.00700"),
	}
	result := buildLookupResult(row)

	if !result.InSecurity {
		t.Error("InSecurity 应为 true")
	}
	checks := map[string]any{
		"symbol":           "HK.00700",
		"futu_symbol":      "00700",
		"name":             "腾讯控股",
		"region":           "HK",
		"market":           "HK",
		"market_label":     "HK",
		"currency":         "HKD",
		"timezone":         "Asia/Hong_Kong",
		"provider":         "FTMM",
		"type_secu":        "Stock",
		"req_section":      0,
		"weight_priority":  50,
		"weight_frequency": 10,
		"flag_enable":      0,
		"instrument":       "stock",
	}
	for column, want := range checks {
		if got := result.Fields[column]; got != want {
			t.Errorf("%s = %v，期望 %v", column, got, want)
		}
	}
	// 空的 type_secu_sub 应进 missing
	found := false
	for _, column := range result.Missing {
		if column == "type_secu_sub" {
			found = true
		}
	}
	if !found {
		t.Errorf("空的 type_secu_sub 应进 missing，实际 %v", result.Missing)
	}
	// 时间列不应进 missing（由服务端写 now()）
	for _, column := range result.Missing {
		if column == "gmt_create" || column == "dt_update" {
			t.Errorf("时间列不应进 missing: %s", column)
		}
	}
}

// TestBuildLookupResultNoCollect 无富途采集记录（字母码标的）时的兜底与提示。
func TestBuildLookupResultNoCollect(t *testing.T) {
	row := lookupRow{
		usc: "BXD", securityName: "BXD Index", securityNameCn: "BXD指数",
		typeSecu: "StockIndex", typeSecuSub: "StockIndex.US", provider: "",
		currencyType: "USD", tz: "America/New_York", exchangeCode: 30,
		marketLabel: strPtr("US"),
		keyFtmm:     strPtr("US..BXD"), keyMarket: strPtr("US"),
		indFtmm: strPtr("US..BXD"),
	}
	result := buildLookupResult(row)

	if got := result.Fields["symbol"]; got != "US..BXD" {
		t.Errorf("symbol 应回退 secu_key.ftmm_symbol，实际 %v", got)
	}
	if result.Sources["symbol"] != sourceKey {
		t.Errorf("symbol 来源标注不符: %s", result.Sources["symbol"])
	}
	if got := result.Fields["region"]; got != "US" {
		t.Errorf("region 应为 US，实际 %v", got)
	}
	if got := result.Fields["provider"]; got != DefaultProvider {
		t.Errorf("provider 应固定 %s，实际 %v", DefaultProvider, got)
	}
	for _, column := range []string{"futu_symbol", "stock_id", "market_type", "market_code",
		"instrument_type", "sub_instrument_type"} {
		found := false
		for _, item := range result.Missing {
			if item == column {
				found = true
			}
		}
		if !found {
			t.Errorf("%s 应为缺失列，实际 missing=%v", column, result.Missing)
		}
	}
	if len(result.Notes) == 0 {
		t.Error("应给出提示")
	}
	if !result.HasIndicator {
		t.Error("有 indicator 记录时 HasIndicator 应为 true")
	}
}

// TestRegionOf region 映射（含板块按交易所兜底）。
func TestRegionOf(t *testing.T) {
	cases := []struct {
		label    string
		exchange int32
		want     any
	}{
		{"SH", 11, "CN"}, {"SZ", 13, "CN"}, {"BJ", 13, "CN"},
		{"HK", 21, "HK"}, {"US", 30, "US"}, {"FX", 100, "FX"}, {"CC", 10001, "CC"},
		{"BK", 11, "CN"}, {"BK", 13, "CN"}, {"BK", 12, "HK"}, {"BK", 21, "HK"}, {"BK", 30, "US"},
		{"bk", 12, "HK"},
		{"", 0, nil}, {"XX", 0, nil},
	}
	for _, item := range cases {
		if got := regionOf(item.label, item.exchange); got != item.want {
			t.Errorf("regionOf(%q,%d) = %v，期望 %v", item.label, item.exchange, got, item.want)
		}
	}
}

// TestNormalizeInstrument 品种归一：只归一 etfs/sectors/currency，futures 保留原样。
func TestNormalizeInstrument(t *testing.T) {
	cases := map[string]any{
		"stock":     "stock",
		"etfs":      "etf",
		"sectors":   "index",
		"currency":  "forex",
		"futures":   "futures", // 实测该表里保留 futures，不能改成 future
		"crypto":    "crypto",
		"future":    "future",
		"":          nil,
		"  index  ": "index",
	}
	for input, want := range cases {
		if got := normalizeInstrument(input); got != want {
			t.Errorf("normalizeInstrument(%q) = %v，期望 %v", input, got, want)
		}
	}
}

// TestReqSectionInt varchar → integer，空/非数字回 0。
func TestReqSectionInt(t *testing.T) {
	cases := []struct {
		raw       any
		want      int
		wantIsSrc bool
	}{
		{nil, 0, false}, {"", 0, false}, {"  ", 0, false},
		{"0", 0, true}, {"3002", 3002, true}, {"12a", 0, false},
	}
	for _, item := range cases {
		got, source := reqSectionInt(item.raw)
		if got != item.want {
			t.Errorf("reqSectionInt(%v) = %d，期望 %d", item.raw, got, item.want)
		}
		if (source == sourceCollect) != item.wantIsSrc {
			t.Errorf("reqSectionInt(%v) 来源判定不符: %s", item.raw, source)
		}
	}
}

// TestCreateRejectsUnknownColumn 未知列名（含时间列）必须报错。
func TestCreateRejectsUnknownColumn(t *testing.T) {
	store := NewStore(nil)
	for _, column := range []string{"no_such_column", "gmt_create", "usc"} {
		_, err := store.Create(t.Context(), CreateInput{
			Usc:    "00700",
			Fields: map[string]json.RawMessage{column: json.RawMessage(`1`)},
		})
		if err == nil {
			t.Fatalf("列 %s 应被拒绝", column)
		}
		var inputErr *InputError
		if !asInputError(err, &inputErr) {
			t.Errorf("应返回 InputError，实际 %T", err)
		}
	}
}

// TestCreateRejectsEmptyUsc 空 usc 直接拒绝。
func TestCreateRejectsEmptyUsc(t *testing.T) {
	store := NewStore(nil)
	if _, err := store.Create(t.Context(), CreateInput{Usc: "   "}); err == nil {
		t.Fatal("空 usc 应报错")
	}
}

// asInputError 沿 Unwrap 链找 InputError。
func asInputError(err error, target **InputError) bool {
	for err != nil {
		if typed, ok := err.(*InputError); ok {
			*target = typed
			return true
		}
		type unwrapper interface{ Unwrap() error }
		next, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = next.Unwrap()
	}
	return false
}

// TestMissingExcludesCollectStatColumns 采集统计列与时间列不进 missing（需求 11345）。
func TestMissingExcludesCollectStatColumns(t *testing.T) {
	row := lookupRow{
		usc: "000001", securityNameCn: "平安银行", typeSecu: "Stock", typeSecuSub: "Stock.SZ",
		provider: "FTMM", currencyType: "CNY", tz: "Asia/Shanghai", exchangeCode: 13,
		collectFtmm: strPtr("SZ.000001"), futuSymbol: strPtr("000001"), stockID: i64Ptr(33333243184257),
		marketLabel: strPtr("SZ"), marketType: i32Ptr(4), marketCode: i32Ptr(31),
		instrument: strPtr("stock"), instrumentType: i32Ptr(3), subInstrumentTyp: i32Ptr(3002),
		reqSection: strPtr(""), keyMarket: strPtr("SZ"), keyFtmm: strPtr("SZ.000001"),
	}
	result := buildLookupResult(row)
	for _, column := range result.Missing {
		if collectStatColumns[column] {
			t.Errorf("采集统计列 %s 不应进 missing", column)
		}
		if column == "gmt_create" || column == "dt_update" || column == "gmt_update" {
			t.Errorf("时间列 %s 不应进 missing", column)
		}
	}
	if len(result.Missing) != 0 {
		t.Errorf("四表齐全且 type_secu_sub 有值时不应有缺失列，实际 %v", result.Missing)
	}
}
