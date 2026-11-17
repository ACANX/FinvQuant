package cdeu

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件覆盖「ETF 列表采集」（需求 11279）：配置表 instrument_symbol=etf 时走
// moomoo get-etf-list，按市场映射 plateId（HK=900075 / US=10002799），
// 条目与股票同构，落库复用 SaveStockListPage（cdeu.finv_quote_secu_realtime）。

// fakeEtfConfigStore 假配置表：固定返回 ETF 配置行，记录进度回写。
type fakeEtfConfigStore struct {
	configs  []StockListCollectConfig
	progress []StockListProgress
}

func (f *fakeEtfConfigStore) LoadStockListConfigs(context.Context) ([]StockListCollectConfig, error) {
	return f.configs, nil
}

func (f *fakeEtfConfigStore) SaveStockListProgress(_ context.Context, _ StockListCollectConfig, progress StockListProgress) error {
	f.progress = append(f.progress, progress)
	return nil
}

// fakeEtfFetcher 假 ETF 接口：记录每次请求，按 plateId 返回预设条目。
type fakeEtfFetcher struct {
	pages    map[string]quoteapiv2.EtfListPage // key = plateId
	requests []quoteapiv2.GetEtfListRequest
}

func (f *fakeEtfFetcher) GetEtfList(_ context.Context, req quoteapiv2.GetEtfListRequest) (quoteapiv2.EtfListPage, error) {
	f.requests = append(f.requests, req)
	page, ok := f.pages[req.PlateID]
	if !ok {
		return quoteapiv2.EtfListPage{}, fmt.Errorf("未预置的 plateId: %s", req.PlateID)
	}
	return page, nil
}

// fakeEtfSink 假落库：记录每页条目，返回全部为新增。
type fakeEtfSink struct {
	saved   []quoteapiv2.StockListItem
	pages   []int
	configs []StockListCollectConfig
}

func (f *fakeEtfSink) SaveStockListPage(_ context.Context, cfg StockListCollectConfig, page int, items []quoteapiv2.StockListItem) ([]StockListSavedRow, error) {
	f.pages = append(f.pages, page)
	f.configs = append(f.configs, cfg)
	rows := make([]StockListSavedRow, 0, len(items))
	for _, item := range items {
		f.saved = append(f.saved, item)
		rows = append(rows, StockListSavedRow{
			StockID:   item.StockID,
			StockCode: item.StockCode,
			Name:      item.Name,
			Inserted:  true,
		})
	}
	return rows, nil
}

// etfTestItem 构造一条 ETF 条目（与股票同构，InstrumentType=4）。
func etfTestItem(stockID int64, code, market string) quoteapiv2.StockListItem {
	return quoteapiv2.StockListItem{
		StockID:        stockID,
		StockCode:      code,
		Name:           code + " ETF",
		MarketLabel:    market,
		InstrumentType: 4,
		PriceNominal:   "12.34",
		ChangeRatio:    "1.23%",
	}
}

// etfTestPage 构造一页 ETF（total/pageCount 由参数给出）。
func etfTestPage(total, pageCount int, items ...quoteapiv2.StockListItem) quoteapiv2.EtfListPage {
	return quoteapiv2.EtfListPage{
		Pagination: quoteapiv2.Pagination{PageSize: len(items), PageCount: pageCount, Total: total},
		Items:      items,
	}
}

// TestStockListCollectEtfPlateIDByMarket 两个市场各自命中正确 plateId，页码 0-based、leverage 空串
// （空串必须参与签名，不能省略键），条目交给 sink 落库。
func TestStockListCollectEtfPlateIDByMarket(t *testing.T) {
	store := &fakeEtfConfigStore{configs: []StockListCollectConfig{
		{InstrumentSymbol: InstrumentSymbolEtf, MarketType: "1"},
		{InstrumentSymbol: InstrumentSymbolEtf, MarketType: "2"},
	}}
	fetcher := &fakeEtfFetcher{pages: map[string]quoteapiv2.EtfListPage{
		"900075":   etfTestPage(2, 1, etfTestItem(900001, "02800", "HK"), etfTestItem(900002, "03033", "HK")),
		"10002799": etfTestPage(1, 1, etfTestItem(100001, "SPY", "US")),
	}}
	sink := &fakeEtfSink{}
	collector := NewStockListCollector(store, nil, sink, nil).WithEtfFetcher(fetcher)

	results, err := collector.Collect(context.Background(), StockListCollectOptions{
		PageSize:     50,
		Interval:     time.Millisecond,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("结果数 = %d，期望 2", len(results))
	}
	if results[0].InstrumentSymbol != InstrumentSymbolEtf || results[0].MarketType != "1" {
		t.Fatalf("结果品种/市场不符: %+v", results[0])
	}
	if results[0].RowsReceived != 2 || results[1].RowsReceived != 1 {
		t.Fatalf("收到行数不符: %+v", results)
	}

	if len(fetcher.requests) != 2 {
		t.Fatalf("接口调用次数 = %d，期望 2", len(fetcher.requests))
	}
	hk, us := fetcher.requests[0], fetcher.requests[1]
	if hk.PlateID != etfPlateIDByMarket["1"] || hk.PlateID != "900075" {
		t.Fatalf("港股 ETF plateId 应为 900075，实际 %q", hk.PlateID)
	}
	if us.PlateID != etfPlateIDByMarket["2"] || us.PlateID != "10002799" {
		t.Fatalf("美股 ETF plateId 应为 10002799，实际 %q", us.PlateID)
	}
	for _, req := range fetcher.requests {
		if req.Page != "0" {
			t.Fatalf("页码应为 0-based 从 0 起，实际 %q", req.Page)
		}
		if req.PageSize != "50" {
			t.Fatalf("每页条数应为 50，实际 %q", req.PageSize)
		}
		if req.RouteType != quoteapiv2.DefaultEtfRouteType || req.RankType != quoteapiv2.DefaultEtfRankType {
			t.Fatalf("routeType/rankType 应取上游默认值: %+v", req)
		}
		if req.TimestampMillis == "" {
			t.Fatal("必须携带 `_` 毫秒时间戳（签名输入）")
		}
	}

	if len(sink.saved) != 3 {
		t.Fatalf("落库条目数 = %d，期望 3", len(sink.saved))
	}
	for _, cfg := range sink.configs {
		if cfg.InstrumentSymbol != InstrumentSymbolEtf {
			t.Fatalf("落库时的品种符号应为 etf: %+v", cfg)
		}
	}
	if len(store.progress) == 0 {
		t.Fatal("应逐页回写进度")
	}
}

// TestStockListCollectEtfUnknownMarket 未登记市场直接报错，且不会发出请求（避免采错板块）。
func TestStockListCollectEtfUnknownMarket(t *testing.T) {
	store := &fakeEtfConfigStore{configs: []StockListCollectConfig{
		{InstrumentSymbol: InstrumentSymbolEtf, MarketType: "4"}, // CN 未登记 plateId
	}}
	fetcher := &fakeEtfFetcher{pages: map[string]quoteapiv2.EtfListPage{}}
	sink := &fakeEtfSink{}
	collector := NewStockListCollector(store, nil, sink, nil).WithEtfFetcher(fetcher)

	results, err := collector.Collect(context.Background(), StockListCollectOptions{
		PageSize:        50,
		MaxRetries:      -1, // 归一为 0，不重试
		ContinueOnError: true,
	})
	if err == nil {
		t.Fatal("未登记市场应返回错误")
	}
	if len(results) != 1 || !strings.Contains(results[0].Error, "未登记的 ETF 板块") {
		t.Fatalf("错误信息应指出未登记板块: %+v", results)
	}
	if len(fetcher.requests) != 0 {
		t.Fatalf("未登记市场不应发出请求，实际 %d 次", len(fetcher.requests))
	}
	if len(sink.saved) != 0 {
		t.Fatalf("未登记市场不应落库，实际 %d 条", len(sink.saved))
	}
}

// TestStockListCollectEtfPlateIDOverride 运行时 plate_id 覆盖优先于市场映射。
func TestStockListCollectEtfPlateIDOverride(t *testing.T) {
	store := &fakeEtfConfigStore{configs: []StockListCollectConfig{
		{InstrumentSymbol: InstrumentSymbolEtf, MarketType: "2"},
	}}
	fetcher := &fakeEtfFetcher{pages: map[string]quoteapiv2.EtfListPage{
		"123456": etfTestPage(1, 1, etfTestItem(1, "QQQ", "US")),
	}}
	sink := &fakeEtfSink{}
	collector := NewStockListCollector(store, nil, sink, nil).WithEtfFetcher(fetcher)

	if _, err := collector.Collect(context.Background(), StockListCollectOptions{
		PageSize: 50,
		PlateID:  "123456",
	}); err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(fetcher.requests) != 1 || fetcher.requests[0].PlateID != "123456" {
		t.Fatalf("plate_id 覆盖未生效: %+v", fetcher.requests)
	}
}

// TestStockListCollectEtfWithoutFetcher 未装配 ETF 拉取实现时给出明确错误（而不是空指针）。
func TestStockListCollectEtfWithoutFetcher(t *testing.T) {
	store := &fakeEtfConfigStore{configs: []StockListCollectConfig{
		{InstrumentSymbol: InstrumentSymbolEtf, MarketType: "1"},
	}}
	collector := NewStockListCollector(store, nil, &fakeEtfSink{}, nil)

	results, err := collector.Collect(context.Background(), StockListCollectOptions{
		PageSize:        50,
		MaxRetries:      -1,
		ContinueOnError: true,
	})
	if err == nil {
		t.Fatal("未装配 ETF 拉取实现时应返回错误")
	}
	if !strings.Contains(results[0].Error, "ETF 拉取实现未装配") {
		t.Fatalf("错误信息不符: %+v", results[0])
	}
}

// TestEtfPlateIDFor 映射与覆盖的单元校验。
func TestEtfPlateIDFor(t *testing.T) {
	cases := []struct{ market, override, want string }{
		{"1", "", "900075"},
		{"2", "", "10002799"},
		{" 2 ", "", "10002799"},
		{"4", "888888", "888888"},
	}
	for _, c := range cases {
		got, err := etfPlateIDFor(c.market, c.override)
		if err != nil || got != c.want {
			t.Fatalf("etfPlateIDFor(%q,%q) = (%q,%v)，期望 %q", c.market, c.override, got, err, c.want)
		}
	}
	if _, err := etfPlateIDFor("4", ""); err == nil {
		t.Fatal("未登记市场应返回错误")
	}
}
