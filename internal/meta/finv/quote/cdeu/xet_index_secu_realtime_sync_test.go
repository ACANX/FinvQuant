package cdeu

import (
	"context"
	"errors"
	"testing"

	"github.com/acnxo/MetaExt/component/quote/citics/xetapp"
)

// 本文件覆盖「沪深京指数 → cdeu.finv_quote_secu_realtime」的映射与采集流程（需求 11294）：
// 内部码 → 交易所代码、编码域 → usc、成分市场 → market_label、字段转换、未知代码与失败路径。

// fakeXetIndexFetcher 假拉取器。
type fakeXetIndexFetcher struct {
	result *xetapp.IndexSnapshotResult
	err    error
	gotReq xetapp.IndexSnapshotRequest
}

func (f *fakeXetIndexFetcher) GetQuoteSnapshotCNAIndex(_ context.Context, req xetapp.IndexSnapshotRequest) (*xetapp.IndexSnapshotResult, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// TestXetIndexCodeMapping 嵌入映射表应覆盖 1239 只，且关键样本口径正确。
func TestXetIndexCodeMapping(t *testing.T) {
	mapping, err := XetIndexCodeMapping()
	if err != nil {
		t.Fatalf("加载映射失败: %v", err)
	}
	if len(mapping) != 1239 {
		t.Fatalf("映射行数应为 1239，实际 %d", len(mapping))
	}
	cases := []struct {
		code   string
		secu   string
		domain string
	}{
		{"1A0001", "000001", "SH"},
		{"1B0001", "000004", "SH"}, // §3.1 例外：不能按数字直译成 000001
		{"1B0688", "000688", "SH"},
		{"2A01", "399001", "SZ"},
		{"2C01", "399106", "SZ"}, // §3.2 例外：不是 399001
		{"399006", "399006", "SZ"},
		{"899050", "899050", "BJ"},
		{"899001", "899001", "NEEQ"},
		{"931743", "931743", "CSI"},
		{"H50001", "H50001", "CSI"},
		{"980028", "980028", "CNI"},
		{"CESCPD", "CESCPD", "CES"},
		{"716567", "716567", "MSCI"},
		{"980018", "980018", "CNI"}, // §A 附录
	}
	for _, tc := range cases {
		entry, ok := mapping[tc.code]
		if !ok {
			t.Fatalf("映射缺少 %s", tc.code)
		}
		if entry.SecuCode != tc.secu || entry.Domain != tc.domain {
			t.Fatalf("%s：期望 %s/%s，实际 %s/%s", tc.code, tc.secu, tc.domain, entry.SecuCode, entry.Domain)
		}
	}
}

// TestXetIndexUSC 各编码域的 usc 构造口径。
func TestXetIndexUSC(t *testing.T) {
	cases := []struct {
		entry XetIndexCodeEntry
		want  string
	}{
		{XetIndexCodeEntry{SecuCode: "000001", Domain: "SH"}, "SH0001"},
		{XetIndexCodeEntry{SecuCode: "000004", Domain: "SH"}, "SH0004"},
		{XetIndexCodeEntry{SecuCode: "399001", Domain: "SZ"}, "399001"},
		{XetIndexCodeEntry{SecuCode: "899050", Domain: "BJ"}, "BSE50"},
		{XetIndexCodeEntry{SecuCode: "899601", Domain: "BJ"}, "899601"},
		{XetIndexCodeEntry{SecuCode: "899001", Domain: "NEEQ"}, "NEEQ899001"},
		{XetIndexCodeEntry{SecuCode: "931743", Domain: "CSI"}, "CSI931743"},
		{XetIndexCodeEntry{SecuCode: "980028", Domain: "CNI"}, "CNI980028"},
		{XetIndexCodeEntry{SecuCode: "CES300", Domain: "CES"}, "CES300"},
		{XetIndexCodeEntry{SecuCode: "704843", Domain: "MSCI"}, "704843"},
	}
	for _, tc := range cases {
		if got := XetIndexUSC(tc.entry); got != tc.want {
			t.Fatalf("%s/%s：期望 %s，实际 %s", tc.entry.Domain, tc.entry.SecuCode, tc.want, got)
		}
	}
}

// TestXetIndexSecuKey 沪市指数用 6 位 usc 形态查库，非交易所域不查库。
func TestXetIndexSecuKey(t *testing.T) {
	cases := []struct {
		entry XetIndexCodeEntry
		want  SecuUscKey
		ok    bool
	}{
		{XetIndexCodeEntry{SecuCode: "000001", Domain: "SH"}, SecuUscKey{Exchange: "SH", Code: "SH0001"}, true},
		{XetIndexCodeEntry{SecuCode: "399001", Domain: "SZ"}, SecuUscKey{Exchange: "SZ", Code: "399001"}, true},
		{XetIndexCodeEntry{SecuCode: "899050", Domain: "BJ"}, SecuUscKey{Exchange: "BJ", Code: "899050"}, true},
		{XetIndexCodeEntry{SecuCode: "931743", Domain: "CSI"}, SecuUscKey{}, false},
		{XetIndexCodeEntry{SecuCode: "980028", Domain: "CNI"}, SecuUscKey{}, false},
		{XetIndexCodeEntry{SecuCode: "899001", Domain: "NEEQ"}, SecuUscKey{}, false},
	}
	for _, tc := range cases {
		got, ok := XetIndexSecuKey(tc.entry)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%s/%s：期望 %+v ok=%v，实际 %+v ok=%v", tc.entry.Domain, tc.entry.SecuCode, tc.want, tc.ok, got, ok)
		}
	}
}

// TestXetIndexMarketLabel 站点市场号 → market_label（用户确认：以市场号为准，不看成分市场）。
func TestXetIndexMarketLabel(t *testing.T) {
	cases := map[string]string{
		"1100": "SH", // 沪市（含中证 93xxxx/H* 与跨市场的沪深300、中证2000）
		"1200": "SZ", // 深市（含国证 98xxxx、新三板 899xxx）
		"1400": "BJ", // 北交所
		"7100": "CN", // 国证非标，无对应交易所
		"":     "CN",
		"9999": "CN",
	}
	for in, want := range cases {
		if got := XetIndexMarketLabel(in); got != want {
			t.Fatalf("市场号 %q：期望 %s，实际 %s", in, want, got)
		}
	}
}

// TestXetIndexCurrency 币种按名称后缀。
func TestXetIndexCurrency(t *testing.T) {
	cases := map[string]string{
		"上证指数":   "CNY",
		"CNTHKD": "HKD",
		"CNTUSD": "USD",
		"港股通消费CNY": "CNY",
	}
	for name, want := range cases {
		if got := xetIndexCurrency(name); got != want {
			t.Fatalf("%s：期望 %s，实际 %s", name, want, got)
		}
	}
}

// TestXetIndexRealtimeRow 字段转换口径：比例除 100、成交量乘 100、price_direct、page 为空。
func TestXetIndexRealtimeRow(t *testing.T) {
	entry := XetIndexCodeEntry{Code: "1A0001", Name: "上证指数", SecuCode: "000001", Domain: "SH", ComponentMarket: "沪", MarketNo: "1100"}
	item := xetapp.Row{
		Code: "1A0001", Name: "上证指数",
		Last: 3634.35, ChangePrice: -21.48, ChangeRatio: -0.59,
		Volume: 472000000, Amount: 770614000000,
		Amplitude: 2.54, BidAskRatio: 10.25, VolumeRatio: 1.02,
		MarketCap: 77620000000000, TotalShares: 20583000000,
		FloatShares: 18445000000, FloatMarketCap: 7000000000000,
	}
	row := XetIndexRealtimeRow(entry, item, "SH0001", 1763000000000)

	if row.USC != "SH0001" || row.Provider != "ZXET" {
		t.Fatalf("usc/provider 不符: %s/%s", row.USC, row.Provider)
	}
	if row.MarketType == nil || *row.MarketType != "4" || row.InstrumentType == nil || *row.InstrumentType != "4" {
		t.Fatalf("market_type/instrument_type 应为 4")
	}
	if row.MarketLabel == nil || *row.MarketLabel != "SH" {
		t.Fatalf("market_label 应为 SH，实际 %v", row.MarketLabel)
	}
	if row.Currency == nil || *row.Currency != "CNY" {
		t.Fatalf("currency 应为 CNY")
	}
	if row.ChangeRatio == nil || row.ChangeRatio.String() != "-0.0059" {
		t.Fatalf("change_ratio 应为 -0.0059，实际 %v", row.ChangeRatio)
	}
	if row.TradeVolumn == nil || row.TradeVolumn.String() != "47200000000" {
		t.Fatalf("trade_volumn 应为 47200000000（手→股），实际 %v", row.TradeVolumn)
	}
	if row.PriceDirect == nil || *row.PriceDirect != "-" {
		t.Fatalf("price_direct 应为 -，实际 %v", row.PriceDirect)
	}
	if row.Page != nil {
		t.Fatalf("page 应为 NULL（指数无分页）")
	}
	if row.StockID != nil {
		t.Fatalf("stock_id 应为 NULL")
	}
	if row.Remark == nil || *row.Remark == "" {
		t.Fatalf("remark 不应为空")
	}
}

// TestXetIndexRealtimeRowNoQuote 无行情（Last=0）时行情列写 NULL，身份列与市值列仍写。
func TestXetIndexRealtimeRowNoQuote(t *testing.T) {
	entry := XetIndexCodeEntry{Code: "980018", Name: "卫星通信", SecuCode: "980018", Domain: "CNI", ComponentMarket: "CNI", MarketNo: "1200"}
	row := XetIndexRealtimeRow(entry, xetapp.Row{Code: "980018", Name: "卫星通信"}, "CNI980018", 1763000000000)
	if row.PriceNominal != nil || row.ChangeRatio != nil || row.PriceDirect != nil {
		t.Fatalf("无行情时行情列应为 NULL")
	}
	if row.MarketLabel == nil || *row.MarketLabel != "SZ" {
		t.Fatalf("市场号 1200 的 market_label 应为 SZ，实际 %v", row.MarketLabel)
	}
}

// TestXetIndexCollector 采集流程：命中库内 usc、未登记按域构造、未知代码跳过。
func TestXetIndexCollector(t *testing.T) {
	fetcher := &fakeXetIndexFetcher{result: &xetapp.IndexSnapshotResult{
		Rows: []xetapp.Row{
			{Code: "1A0001", Name: "上证指数", Last: 3634.35, ChangePrice: 12.5, ChangeRatio: 0.35, Volume: 100, Amount: 2000},
			{Code: "932000", Name: "中证2000", Last: 2000, ChangeRatio: 1.0, Volume: 10, Amount: 20},
			{Code: "XXXXXX", Name: "未知指数"},
		},
		Pools: []xetapp.IndexPool{{Name: "中证指数", Fetched: 3}},
	}}
	resolver := &fakeUscResolver{mapping: map[SecuUscKey]string{
		{Exchange: "SH", Code: "SH0001"}: "SH0001",
	}}
	sink := &fakeRealtimeSink{inserted: 2}
	collector := NewXetIndexCollector(fetcher, resolver, sink, bdfsTestLogger())

	result, err := collector.Collect(context.Background(), XetIndexCollectOptions{})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(fetcher.gotReq.Boards) != 6 {
		t.Fatalf("默认应为 6 个池（4 池 + 深市 1200 + 北交所 1400），实际 %d", len(fetcher.gotReq.Boards))
	}
	if len(sink.rows) != 2 {
		t.Fatalf("应写入 2 行（未知代码跳过），实际 %d", len(sink.rows))
	}
	if result.RowsReceived != 3 || result.RowsInsert != 2 || result.Pages != 1 {
		t.Fatalf("统计不符: %+v", result)
	}
	if sink.rows[0].USC != "SH0001" {
		t.Fatalf("第一行应命中库内 usc SH0001，实际 %s", sink.rows[0].USC)
	}
	if sink.rows[1].USC != "CSI932000" {
		t.Fatalf("第二行未登记应按域构造 CSI932000，实际 %s", sink.rows[1].USC)
	}
	if len(resolver.keys) != 1 || resolver.keys[0] != (SecuUscKey{Exchange: "SH", Code: "SH0001"}) {
		t.Fatalf("查库键应为 (SH, SH0001)，实际 %+v", resolver.keys)
	}
	if sink.rows[1].MarketLabel == nil || *sink.rows[1].MarketLabel != "SH" {
		t.Fatalf("中证2000（市场号 1100）的 market_label 应为 SH，实际 %v", sink.rows[1].MarketLabel)
	}
}

// TestXetIndexCollectorFetchError 上游失败应整体失败并带错误信息。
func TestXetIndexCollectorFetchError(t *testing.T) {
	collector := NewXetIndexCollector(
		&fakeXetIndexFetcher{err: errors.New("上游 502")},
		&fakeUscResolver{}, &fakeRealtimeSink{}, bdfsTestLogger())
	result, err := collector.Collect(context.Background(), XetIndexCollectOptions{})
	if err == nil {
		t.Fatalf("应返回错误")
	}
	if result.Error == "" {
		t.Fatalf("结果应带错误信息")
	}
}

// TestXetIndexCollectorEmptyResult 空结果应报错，不写库。
func TestXetIndexCollectorEmptyResult(t *testing.T) {
	sink := &fakeRealtimeSink{}
	collector := NewXetIndexCollector(
		&fakeXetIndexFetcher{result: &xetapp.IndexSnapshotResult{}},
		&fakeUscResolver{}, sink, bdfsTestLogger())
	if _, err := collector.Collect(context.Background(), XetIndexCollectOptions{}); err == nil {
		t.Fatalf("空结果应返回错误")
	}
	if len(sink.rows) != 0 {
		t.Fatalf("空结果不应写库")
	}
}
