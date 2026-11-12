package cdeu

// 证券指标 Upsert 的纯函数单测（需求 11248）。
//
// 只测不依赖数据库的部分：请求解包、字段映射、stock_id 兼容、collect 参数派生、
// SQL 参数个数与列数一致性。写库路径由 live 测试覆盖（需真实 PG）。

import (
	"encoding/json"
	"strings"
	"testing"
)

// secuIndicatorSampleJSON 采集端的真实样本（截取关键字段，含 stock_id 为字符串的形态）。
const secuIndicatorSampleJSON = `{
  "symbol": "SH.600006",
  "market": "SH",
  "code": "600006",
  "page_type": "stock",
  "collected_at": "2026-11-12T07:53:32+08:00",
  "source_url": "https://www.moomoo.com/hans/stock/600006-SH",
  "run_id": "20261112-075328-9uxwal",
  "idempotency_key": "72bc85cd46cc",
  "mode": "upsert",
  "data": {
    "symbol": "SH.600006",
    "market": "SH",
    "stock_code": "600006",
    "stock_id": "46377058462214",
    "name": "东风股份",
    "spread_code": "100",
    "price_direct": "down",
    "market_status_text": "已收盘",
    "source_url": "https://www.moomoo.com/hans/stock/600006-SH",
    "source_channel": "http_requests",
    "collected_at": "2026-11-12T07:53:32+08:00",
    "unmapped_json": "{}",
    "market_type": 4,
    "market_code": 30,
    "instrument_type": 3,
    "sub_instrument_type": 3002,
    "is_option": 0,
    "is_futures": 0,
    "ex_type": 2,
    "price_accuracy": 2,
    "price": 5.06,
    "change": -0.05,
    "change_ratio": -0.98,
    "price_highest_52week": 8.23,
    "kcb_stock_static_volume": 0,
    "time": 1794380400000,
    "market_status": 29,
    "delay_time": 900,
    "exchange_data_time_ms": 1794380400000,
    "update_count": 1
  }
}`

// TestSecuIndicatorUnwrap 验证两种请求形态：裸内层 + CF Worker 外壳。
func TestSecuIndicatorUnwrap(t *testing.T) {
	var bare SecuIndicatorUpsertRequest
	if err := json.Unmarshal([]byte(secuIndicatorSampleJSON), &bare); err != nil {
		t.Fatalf("解析裸请求失败: %v", err)
	}
	if bare.unwrap() != &bare {
		t.Fatal("没有 body 节点时 unwrap 必须返回自身")
	}
	if got := bare.rtmmSymbol(); got != "SH.600006" {
		t.Fatalf("ftmm_symbol 期望 SH.600006，实际 %q", got)
	}

	// 外壳形态：内层 body 与顶层字段并存时以 body 为准。
	wrapped := `{"timestamp":"2026-11-11T23:53:33.211Z","body":` + secuIndicatorSampleJSON + `}`
	var outer SecuIndicatorUpsertRequest
	if err := json.Unmarshal([]byte(wrapped), &outer); err != nil {
		t.Fatalf("解析外壳请求失败: %v", err)
	}
	if outer.Body == nil || outer.Body.Data == nil {
		t.Fatal("外壳形态未解出内层 body.data")
	}
	if got := outer.unwrap().rtmmSymbol(); got != "SH.600006" {
		t.Fatalf("外壳形态 ftmm_symbol 期望 SH.600006，实际 %q", got)
	}
}

// TestSecuIndicatorArgsAlignment 断言参数个数与列数一致（写错顺序会直接写到错列）。
func TestSecuIndicatorArgsAlignment(t *testing.T) {
	var req SecuIndicatorUpsertRequest
	if err := json.Unmarshal([]byte(secuIndicatorSampleJSON), &req); err != nil {
		t.Fatalf("解析样本失败: %v", err)
	}
	args := req.Data.indicatorArgs("600006", "SH.600006", nil)
	if len(args) != len(secuIndicatorUpsertColumns) {
		t.Fatalf("参数个数 %d 与列数 %d 不一致", len(args), len(secuIndicatorUpsertColumns))
	}
	if args[0] != "600006" || args[1] != "SH.600006" {
		t.Fatalf("前两列必须固定为 usc / ftmm_symbol，实际 %v / %v", args[0], args[1])
	}
}

// TestSecuIndicatorFieldMapping 抽查改名映射：请求字段 → 目标列。
func TestSecuIndicatorFieldMapping(t *testing.T) {
	var req SecuIndicatorUpsertRequest
	if err := json.Unmarshal([]byte(secuIndicatorSampleJSON), &req); err != nil {
		t.Fatalf("解析样本失败: %v", err)
	}
	args := req.Data.indicatorArgs("600006", "SH.600006", nil)

	index := map[string]int{}
	for i, column := range secuIndicatorUpsertColumns {
		index[column] = i
	}
	cases := []struct {
		column string
		want   any
	}{
		{"change_price", "-0.05"},        // change → change_price
		{"price_highest_week52", "8.23"}, // price_highest_52week → price_highest_week52
		{"kcb_static_volume", "0"},       // kcb_stock_static_volume → kcb_static_volume
		{"ts", int64(1794380400000)},     // time → ts
		{"ts_exchange_dt", int64(1794380400000)},
		{"market_status", int64(29)},
		{"count_update", int64(1)}, // update_count → count_update
		{"stock_id", int64(46377058462214)},
		{"secu_url", "https://www.moomoo.com/hans/stock/600006-SH"},
		{"name", "东风股份"},
	}
	for _, tc := range cases {
		pos, ok := index[tc.column]
		if !ok {
			t.Fatalf("列 %s 未在 secuIndicatorUpsertColumns 中登记", tc.column)
		}
		if args[pos] != tc.want {
			t.Fatalf("列 %s 期望 %#v，实际 %#v", tc.column, tc.want, args[pos])
		}
	}

	// 需求口径：无对应列的字段不做处理（不存在 is_option 这种列）。
	for _, dropped := range []string{"is_option", "is_futures", "first_seen_at", "updated_at"} {
		if _, ok := index[dropped]; ok {
			t.Fatalf("字段 %s 不应出现在写入列表里（指标表无对应列）", dropped)
		}
	}
}

// TestSecuIndicatorStockIDForms stock_id 三种形态：字符串 / 数字 / null。
func TestSecuIndicatorStockIDForms(t *testing.T) {
	cases := []struct {
		raw   string
		want  any
		valid bool
	}{
		{`"46377058462214"`, int64(46377058462214), true},
		{`46377058462214`, int64(46377058462214), true},
		{`null`, nil, false},
	}
	for _, tc := range cases {
		var payload SecuIndicatorPayload
		body := `{"stock_id":` + tc.raw + `}`
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("%s 解析失败: %v", tc.raw, err)
		}
		if got := payload.stockIDArg(); got != tc.want {
			t.Fatalf("%s 期望 %#v，实际 %#v", tc.raw, tc.want, got)
		}
	}

	var bad SecuIndicatorPayload
	if err := json.Unmarshal([]byte(`{"stock_id":"abc"}`), &bad); err == nil {
		t.Fatal("非整数 stock_id 必须报错")
	}
}

// TestSecuIndicatorDeriveCollectArgs 采集参数派生：instrument / sub_instrument_type / market_type。
func TestSecuIndicatorDeriveCollectArgs(t *testing.T) {
	var req SecuIndicatorUpsertRequest
	if err := json.Unmarshal([]byte(secuIndicatorSampleJSON), &req); err != nil {
		t.Fatalf("解析样本失败: %v", err)
	}
	args := req.Data.deriveCollectArgs("SH")
	if args.FutuSymbol != "600006" {
		t.Fatalf("futu_symbol 期望 600006，实际 %q", args.FutuSymbol)
	}
	if args.Instrument != "stock" {
		t.Fatalf("instrument_type=3 应派生 stock，实际 %q", args.Instrument)
	}
	if args.SubInstrumentType != 3002 {
		t.Fatalf("sub_instrument_type 期望 3002，实际 %v", args.SubInstrumentType)
	}
	if args.MarketType != 4 {
		t.Fatalf("market_type 期望 4，实际 %v", args.MarketType)
	}
	if args.MarketCode != int64(30) {
		t.Fatalf("market_code 期望 30，实际 %v", args.MarketCode)
	}

	// 缺 stock_code 时从 ftmm_symbol 去掉市场前缀。
	payload := &SecuIndicatorPayload{Symbol: "cc.snteth", InstrumentType: jsonNumber("4")}
	args = payload.deriveCollectArgs("CC")
	if args.FutuSymbol != "SNTETH" {
		t.Fatalf("应从 CC.SNTETH 拆出 SNTETH，实际 %q", args.FutuSymbol)
	}
	if args.Instrument != "etf" || args.SubInstrumentType != 4002 {
		t.Fatalf("instrument_type=4 应派生 etf/4002，实际 %q/%v", args.Instrument, args.SubInstrumentType)
	}
}

// TestValidateSecuIndicatorUpsert 入参校验分支。
func TestValidateSecuIndicatorUpsert(t *testing.T) {
	empty := &SecuIndicatorUpsertRequest{}
	if err := ValidateSecuIndicatorUpsert(empty); err == nil ||
		!strings.Contains(err.Error(), "data") {
		t.Fatalf("缺 data 必须报错，实际 %v", err)
	}

	name := "x"
	noSymbol := &SecuIndicatorUpsertRequest{Data: &SecuIndicatorPayload{Name: &name}}
	if err := ValidateSecuIndicatorUpsert(noSymbol); err == nil ||
		!strings.Contains(err.Error(), "symbol") {
		t.Fatalf("缺 symbol 必须报错，实际 %v", err)
	}

	badMode := &SecuIndicatorUpsertRequest{
		Mode: "insert", Data: &SecuIndicatorPayload{Symbol: "SH.600006"}}
	if err := ValidateSecuIndicatorUpsert(badMode); err == nil ||
		!strings.Contains(err.Error(), "mode") {
		t.Fatalf("mode=insert 必须报错，实际 %v", err)
	}

	ok := &SecuIndicatorUpsertRequest{
		Mode: "upsert", Data: &SecuIndicatorPayload{Symbol: "sh.600006"}}
	if err := ValidateSecuIndicatorUpsert(ok); err != nil {
		t.Fatalf("合法请求不应报错，实际 %v", err)
	}
	if got := ok.rtmmSymbol(); got != "SH.600006" {
		t.Fatalf("ftmm_symbol 应归一成大写，实际 %q", got)
	}
}

// TestCollectedTime 采集时刻解析：RFC3339 带时区 / 不带时区 / 非法。
func TestCollectedTime(t *testing.T) {
	if got := collectedTime("2026-11-12T07:53:32+08:00"); got == nil {
		t.Fatal("带时区的 RFC3339 应能解析")
	}
	if got := collectedTime("2026-11-12T07:53:32"); got == nil {
		t.Fatal("不带时区的本地时间应能解析")
	}
	if got := collectedTime("not-a-time"); got != nil {
		t.Fatalf("非法时间应返回 nil，实际 %v", got)
	}
}

// jsonNumber 构造 *json.Number。
func jsonNumber(raw string) *json.Number {
	value := json.Number(raw)
	return &value
}
