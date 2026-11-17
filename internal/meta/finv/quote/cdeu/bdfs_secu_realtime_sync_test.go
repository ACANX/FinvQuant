package cdeu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
)

// 本文件覆盖「BDFS 板块成分股 → cdeu.finv_quote_secu_realtime」的字段转换与采集流程
// （需求 11262）：映射口径、无行情行、usc 解析缺失、接口失败。

func bdfsTestLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// fakeBdfsFetcher 假拉取器。
type fakeBdfsFetcher struct {
	result BdfsFetchResult
	err    error
	gotReq BdfsFetchRequest
}

func (f *fakeBdfsFetcher) FetchConstituents(_ context.Context, req BdfsFetchRequest) (BdfsFetchResult, error) {
	f.gotReq = req
	return f.result, f.err
}

// fakeUscResolver 假 usc 解析器。
type fakeUscResolver struct {
	mapping map[SecuUscKey]string
	err     error
	keys    []SecuUscKey
}

func (f *fakeUscResolver) ResolveSecuUsc(_ context.Context, keys []SecuUscKey) (map[SecuUscKey]string, error) {
	f.keys = keys
	if f.err != nil {
		return nil, f.err
	}
	return f.mapping, nil
}

// fakeRealtimeSink 假落库实现：记录收到的行。
type fakeRealtimeSink struct {
	rows     []QuoteSecuRealtimeRow
	inserted int64
	updated  int64
	err      error
}

func (f *fakeRealtimeSink) SaveQuoteSecuRealtimeRows(_ context.Context, rows []QuoteSecuRealtimeRow) (int64, int64, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	f.rows = append(f.rows, rows...)
	return f.inserted, f.updated, nil
}

func TestBdfsRealtimeRowMapping(t *testing.T) {
	item := BdfsConstituent{
		Code:         "430047",
		Name:         "诺思兰德",
		Exchange:     "BJ",
		HasQuote:     1,
		HasRaw:       true,
		PriceNominal: 12.11,
		ChangePrice:  -0.25,
		ChangeRatio:  2.5,
		Volume:       1276.5,
		Amount:       1542319.5,
		MarketValue:  3167980619.5,
	}
	row := BdfsRealtimeRow(item, "430047.BJ", 1763000000000, 3, "stock")

	if row.USC != "430047.BJ" {
		t.Fatalf("usc 应为主键值，实际 %q", row.USC)
	}
	if row.Provider != "BDFS" || BdfsProvider != "BDFS" {
		t.Fatalf("provider 应为 BDFS，实际 %q", row.Provider)
	}
	if row.StockID != nil {
		t.Fatalf("BDFS 不提供 stock_id，应为 NULL，实际 %v", *row.StockID)
	}
	if got := bdfsDerefString(row.StockCode); got != "430047" {
		t.Fatalf("stock_code 不符：%q", got)
	}
	if got := bdfsDerefString(row.MarketType); got != "4" {
		t.Fatalf("market_type 应为 4（CN A 股），实际 %q", got)
	}
	if got := bdfsDerefString(row.MarketLabel); got != "BJ" {
		t.Fatalf("market_label 应为 BJ，实际 %q", got)
	}
	if got := bdfsDerefString(row.Currency); got != "CNY" {
		t.Fatalf("currency 应为 CNY，实际 %q", got)
	}
	if got := bdfsDerefString(row.InstrumentType); got != "stock" {
		t.Fatalf("instrument_type 应为 stock，实际 %q", got)
	}
	if got := bdfsDerefNumber(row.PriceNominal); got != "12.11" {
		t.Fatalf("price_nominal 不符：%s", got)
	}
	if got := bdfsDerefNumber(row.ChangePrice); got != "-0.25" {
		t.Fatalf("change_price 不符：%s", got)
	}
	// 百分数 2.5 → 小数比例 0.025
	if got := bdfsDerefNumber(row.ChangeRatio); got != "0.025" {
		t.Fatalf("change_ratio 应为小数比例 0.025，实际 %s", got)
	}
	// 手 → 股：1276.5 手 = 127650 股
	if got := bdfsDerefNumber(row.TradeVolumn); got != "127650" {
		t.Fatalf("trade_volumn 应为 127650（股），实际 %s", got)
	}
	if got := bdfsDerefNumber(row.TradeTrunover); got != "1542319.5" {
		t.Fatalf("trade_trunover 不符：%s", got)
	}
	if got := bdfsDerefNumber(row.MarketValue); got != "3167980619.5" {
		t.Fatalf("market_value 不符：%s", got)
	}
	if got := bdfsDerefString(row.PriceDirect); got != "-" {
		t.Fatalf("price_direct 应为 -，实际 %q", got)
	}
	if row.Page == nil || *row.Page != 3 {
		t.Fatalf("page 应为 3，实际 %v", row.Page)
	}
	if row.DtUpdate == nil || *row.DtUpdate != 1763000000000 {
		t.Fatalf("dt_update 不符：%v", row.DtUpdate)
	}
	if got := bdfsDerefString(row.Remark); !strings.Contains(got, "BDFS") || !strings.Contains(got, "BJ") {
		t.Fatalf("remark 应带来源与交易所，实际 %q", got)
	}
	// 无来源列保持 NULL
	for name, ptr := range map[string]*json.Number{
		"total_shares":     row.TotalShares,
		"volumn_ratio":     row.VolumnRatio,
		"trust_aum":        row.TrustAum,
		"rmb_market_value": row.RmbMarketValue,
	} {
		if ptr != nil {
			t.Fatalf("%s 无来源应为 NULL，实际 %s", name, string(*ptr))
		}
	}
}

func TestBdfsRealtimeRowNoQuote(t *testing.T) {
	item := BdfsConstituent{Code: "920001", Name: "停牌股", Exchange: "BJ", HasQuote: 0, HasRaw: false}
	row := BdfsRealtimeRow(item, "920001.BJ", 1, 1, "stock")

	if row.PriceNominal != nil || row.ChangePrice != nil || row.MarketValue != nil {
		t.Fatalf("无行情行不应写任何行情数值列")
	}
	if row.PriceDirect != nil {
		t.Fatalf("无行情行不应写 price_direct")
	}
	if got := bdfsDerefString(row.StockCode); got != "920001" {
		t.Fatalf("身份列仍应写入，实际 %q", got)
	}
}

func TestBdfsRealtimeRowPriceDirect(t *testing.T) {
	cases := []struct {
		change float64
		want   string
	}{{1.2, "+"}, {-1.2, "-"}, {0, "="}}
	for _, c := range cases {
		row := BdfsRealtimeRow(BdfsConstituent{Code: "1", Exchange: "SH", HasQuote: 1, HasRaw: true, ChangePrice: c.change}, "1.SH", 1, 1, "stock")
		if got := bdfsDerefString(row.PriceDirect); got != c.want {
			t.Fatalf("change=%v 时 price_direct 应为 %q，实际 %q", c.change, c.want, got)
		}
	}
}

func TestBdfsCollectorSkipsUnresolvedUsc(t *testing.T) {
	fetcher := &fakeBdfsFetcher{result: BdfsFetchResult{
		Pages: 1,
		Items: []BdfsConstituent{
			{Code: "430047", Name: "诺思兰德", Exchange: "BJ", HasQuote: 1, HasRaw: true, PriceNominal: 10},
			{Code: "920001", Name: "未入库股", Exchange: "BJ", HasQuote: 1, HasRaw: true, PriceNominal: 5},
			{Code: "430047", Name: "诺思兰德重复", Exchange: "BJ", HasQuote: 1, HasRaw: true, PriceNominal: 11},
		},
	}}
	resolver := &fakeUscResolver{mapping: map[SecuUscKey]string{
		{Exchange: "BJ", Code: "430047"}: "430047.BJ",
	}}
	sink := &fakeRealtimeSink{inserted: 1, updated: 1}

	var events []StockListCollectEvent
	collector := NewBdfsSecuRealtimeCollector(fetcher, resolver, sink, bdfsTestLogger()).
		WithHook(func(e StockListCollectEvent) { events = append(events, e) })

	result, err := collector.Collect(context.Background(), BdfsCollectOptions{})
	if err != nil {
		t.Fatalf("采集应成功，实际 %v", err)
	}
	if fetcher.gotReq.Code != BdfsBseBlockCode {
		t.Fatalf("默认来源代码应为 %s，实际 %q", BdfsBseBlockCode, fetcher.gotReq.Code)
	}
	if fetcher.gotReq.FinanceType != "block" {
		t.Fatalf("默认成分类型应为 block，实际 %q", fetcher.gotReq.FinanceType)
	}
	if fetcher.gotReq.PageSize != BdfsDefaultPageSize {
		t.Fatalf("默认页大小应为 %d，实际 %d", BdfsDefaultPageSize, fetcher.gotReq.PageSize)
	}
	if result.InstrumentSymbol != BdfsInstrumentSymbol || result.MarketType != "BJ" {
		t.Fatalf("结果品种/市场不符：%+v", result)
	}
	if result.RowsReceived != 3 || result.CountExists != 3 {
		t.Fatalf("收到条数应为 3，实际 receive=%d count_exists=%d", result.RowsReceived, result.CountExists)
	}
	if len(sink.rows) != 2 {
		t.Fatalf("未解析到 usc 的行应跳过，期望写入 2 行，实际 %d", len(sink.rows))
	}
	if result.RowsInsert != 1 || result.RowsUpdate != 1 {
		t.Fatalf("落库统计不符：insert=%d update=%d", result.RowsInsert, result.RowsUpdate)
	}
	if len(resolver.keys) != 2 {
		t.Fatalf("解析键应去重为 2 个，实际 %d", len(resolver.keys))
	}
	// 去重键解析只做一次，两行都应带上同一 usc
	if sink.rows[0].USC != "430047.BJ" || sink.rows[1].USC != "430047.BJ" {
		t.Fatalf("usc 应一致，实际 %q / %q", sink.rows[0].USC, sink.rows[1].USC)
	}
	hasWarn := false
	var detailLines []string
	for _, e := range events {
		if e.Stage != "retry" {
			continue
		}
		if strings.Contains(e.Message, "未能在 public.finv_security 解析到 usc") {
			hasWarn = true
		}
		if strings.HasPrefix(e.Message, "未解析 usc：") {
			detailLines = append(detailLines, e.Message)
		}
	}
	if !hasWarn {
		t.Fatalf("应记录「跳过未解析 usc」的 WARN 日志，实际事件 %+v", events)
	}
	if len(detailLines) != 1 {
		t.Fatalf("未解析行应逐条打印（期望 1 行），实际 %+v", detailLines)
	}
	if !strings.Contains(detailLines[0], "exchange=BJ") || !strings.Contains(detailLines[0], "code=920001") || !strings.Contains(detailLines[0], "name=未入库股") {
		t.Fatalf("未解析明细应含 exchange/code/name，实际 %q", detailLines[0])
	}
	if sink.rows[0].Provider != "BDFS" {
		t.Fatalf("写入行 provider 应为 BDFS，实际 %q", sink.rows[0].Provider)
	}
}

func TestBdfsCollectorFetchError(t *testing.T) {
	fetcher := &fakeBdfsFetcher{err: errors.New("hit risk")}
	collector := NewBdfsSecuRealtimeCollector(fetcher, &fakeUscResolver{}, &fakeRealtimeSink{}, bdfsTestLogger())

	result, err := collector.Collect(context.Background(), BdfsCollectOptions{})
	if err == nil {
		t.Fatal("接口失败时 Collect 应返回错误")
	}
	if !strings.Contains(result.Error, "hit risk") {
		t.Fatalf("结果应带失败原因，实际 %q", result.Error)
	}
	if result.InstrumentSymbol != BdfsInstrumentSymbol {
		t.Fatalf("失败结果仍应标识品种，实际 %q", result.InstrumentSymbol)
	}
}

func TestBdfsCollectorEmptyItems(t *testing.T) {
	fetcher := &fakeBdfsFetcher{result: BdfsFetchResult{}}
	collector := NewBdfsSecuRealtimeCollector(fetcher, &fakeUscResolver{}, &fakeRealtimeSink{}, bdfsTestLogger())

	if _, err := collector.Collect(context.Background(), BdfsCollectOptions{}); err == nil {
		t.Fatal("空成分股应返回错误")
	}
}

func TestBdfsCollectorAllUscUnresolved(t *testing.T) {
	fetcher := &fakeBdfsFetcher{result: BdfsFetchResult{Pages: 1, Items: []BdfsConstituent{
		{Code: "430047", Exchange: "BJ", HasQuote: 1, HasRaw: true},
	}}}
	collector := NewBdfsSecuRealtimeCollector(fetcher, &fakeUscResolver{mapping: map[SecuUscKey]string{}}, &fakeRealtimeSink{}, bdfsTestLogger())

	if _, err := collector.Collect(context.Background(), BdfsCollectOptions{}); err == nil {
		t.Fatal("全部未解析到 usc 时应返回错误，而不是静默成功")
	}
}

func TestBdfsCollectOptionsNormalize(t *testing.T) {
	opts := BdfsCollectOptions{}.normalize()
	if opts.Source.Code != BdfsBseBlockCode || opts.PageSize != BdfsDefaultPageSize || opts.Timeout != BdfsDefaultTimeout {
		t.Fatalf("零值应补齐默认值，实际 %+v", opts)
	}
	src, ok := BdfsSourceBySymbol(BdfsEtfSymbol)
	if !ok {
		t.Fatalf("应能按 %s 取到来源", BdfsEtfSymbol)
	}
	custom := BdfsCollectOptions{Source: src, PageSize: 100}.normalize()
	if custom.Source.Code != BdfsEtfIndexCode || custom.PageSize != 100 {
		t.Fatalf("显式值不应被覆盖，实际 %+v", custom)
	}
}

func TestBdfsEtfSourceUsesIndexConstituents(t *testing.T) {
	fetcher := &fakeBdfsFetcher{result: BdfsFetchResult{Pages: 1, Items: []BdfsConstituent{
		{Code: "510010", Name: "治理ETF", Exchange: "SH", HasQuote: 1, HasRaw: true, PriceNominal: 1.742},
	}}}
	sink := &fakeRealtimeSink{inserted: 1}
	collector := NewBdfsSecuRealtimeCollector(fetcher, &fakeUscResolver{mapping: map[SecuUscKey]string{
		{Exchange: "SH", Code: "510010"}: "510010",
	}}, sink, bdfsTestLogger())

	src, _ := BdfsSourceBySymbol(BdfsEtfSymbol)
	result, err := collector.Collect(context.Background(), BdfsCollectOptions{Source: src})
	if err != nil {
		t.Fatalf("ETF 采集应成功，实际 %v", err)
	}
	if fetcher.gotReq.FinanceType != "index" || fetcher.gotReq.Code != BdfsEtfIndexCode {
		t.Fatalf("ETF 来源应走 index/399380，实际 %+v", fetcher.gotReq)
	}
	if result.InstrumentSymbol != BdfsEtfSymbol {
		t.Fatalf("结果品种应为 %s，实际 %q", BdfsEtfSymbol, result.InstrumentSymbol)
	}
	if len(sink.rows) != 1 {
		t.Fatalf("应写入 1 行，实际 %d", len(sink.rows))
	}
	row := sink.rows[0]
	if got := bdfsDerefString(row.InstrumentType); got != "etf" {
		t.Fatalf("instrument_type 应为 etf，实际 %q", got)
	}
	if got := bdfsDerefString(row.MarketType); got != "4" {
		t.Fatalf("market_type 应为 4，实际 %q", got)
	}
	if got := bdfsDerefString(row.MarketLabel); got != "SH" {
		t.Fatalf("market_label 应按交易所为 SH，实际 %q", got)
	}
}

func bdfsDerefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func bdfsDerefNumber(p *json.Number) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
