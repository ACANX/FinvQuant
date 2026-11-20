package cdeu

import (
	"context"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// fakeCryptoFetcher 返回固定快照（记录收到的 code 列表）。
type fakeCryptoFetcher struct {
	gotCodes []string
	rows     []webapi.Snapshot
	err      error
}

func (f *fakeCryptoFetcher) GetQuoteSnapshots(_ context.Context, codes []string) ([]webapi.Snapshot, error) {
	f.gotCodes = append([]string(nil), codes...)
	return f.rows, f.err
}

// fakeCryptoStore 记录写入行与收藏 usc。
type fakeCryptoStore struct {
	uscs     []string
	saved    [][]CryptoRealtimeRow
	inserted int64
	updated  int64
}

func (s *fakeCryptoStore) ListFavoriteCryptoUSCs(context.Context) ([]string, error) {
	return s.uscs, nil
}

func (s *fakeCryptoStore) SaveCryptoRealtimeRows(_ context.Context, rows []CryptoRealtimeRow) (int64, int64, error) {
	s.saved = append(s.saved, rows)
	return s.inserted, s.updated, nil
}

// TestCryptoRowFromSnapshot 快照 → 落库行：usc 去前缀、计价货币、涨跌与方向。
func TestCryptoRowFromSnapshot(t *testing.T) {
	row, ok := cryptoRowFromSnapshot(webapi.Snapshot{
		Code:           "CC.BTCUSD",
		Name:           "BTC/USD",
		SCName:         "比特币/美元",
		LastPrice:      100,
		OpenPrice:      95,
		HighPrice:      101,
		LowPrice:       94,
		PrevClosePrice: 90,
		Amplitude:      7.7,
		Volume:         1234,
		Turnover:       5678.9,
		BidAskRatio:    -12.5,
		SecStatus:      "NORMAL",
		DataDate:       "2026-11-17",
		UpdateTime:     1794971178277,
	})
	if !ok {
		t.Fatal("合法 CC.BTCUSD 应转换成功")
	}
	if row.USC != "BTCUSD" || row.Code != "CC.BTCUSD" || row.QuoteCurrency != "USD" {
		t.Fatalf("身份字段不符: %+v", row)
	}
	// 名称优先简体中文（上游 sc_name），取不到才用 name（需求 11339）
	if row.Name != "比特币/美元" {
		t.Fatalf("名称应取 sc_name，实际 %q", row.Name)
	}
	if fallback, _ := cryptoRowFromSnapshot(webapi.Snapshot{Code: "CC.BTCUSD", Name: "BTC/USD"}); fallback.Name != "BTC/USD" {
		t.Fatalf("sc_name 缺失时应回退 name，实际 %q", fallback.Name)
	}
	if row.ChangePrice == nil || row.ChangePrice.String() != "10" {
		t.Fatalf("涨跌额应为 10，实际 %v", row.ChangePrice)
	}
	if row.ChangeRatio == nil || !strings.HasPrefix(row.ChangeRatio.String(), "11.11") {
		t.Fatalf("涨跌幅应以 90 为基准，实际 %v", row.ChangeRatio)
	}
	if row.PriceDirect == nil || *row.PriceDirect != "+" {
		t.Fatalf("方向应为 +，实际 %v", row.PriceDirect)
	}
	if row.Provider != CryptoProvider || row.DtUpdate == nil || *row.DtUpdate != 1794971178277 {
		t.Fatalf("provider/dt_update 不符: %+v", row)
	}

	// 非 CC. 前缀（如美股）不转换
	if _, ok := cryptoRowFromSnapshot(webapi.Snapshot{Code: "US.AAPL"}); ok {
		t.Fatal("非加密货币代码不应转换")
	}
}

// TestCryptoFavSnapshotCollectorHook 采集过程事件（begin / page）与批次请求 code。
func TestCryptoFavSnapshotCollectorHook(t *testing.T) {
	fetcher := &fakeCryptoFetcher{rows: []webapi.Snapshot{
		{Code: "CC.BTCUSD", LastPrice: 1, PrevClosePrice: 1},
		{Code: "CC.ETHUSD", LastPrice: 2, PrevClosePrice: 1},
	}}
	store := &fakeCryptoStore{uscs: []string{"BTCUSD", "ETHUSD"}, inserted: 2}
	collector := NewCryptoFavSnapshotCollector(fetcher, store, nil)

	var stages []string
	collector.WithHook(func(event StockListCollectEvent) {
		stages = append(stages, event.Stage)
	})

	result, err := collector.Collect(context.Background(), CryptoFavSnapshotCollectOptions{})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(fetcher.gotCodes) != 2 || fetcher.gotCodes[0] != "CC.BTCUSD" || fetcher.gotCodes[1] != "CC.ETHUSD" {
		t.Fatalf("请求 code 不符: %v", fetcher.gotCodes)
	}
	if result.USCTotal != 2 || result.CodesRequested != 2 || result.RowsReceived != 2 || result.RowsInsert != 2 {
		t.Fatalf("统计不符: %+v", result)
	}
	if len(stages) < 2 || stages[0] != "begin" || stages[1] != "page" {
		t.Fatalf("hook 事件不符: %v", stages)
	}
	if len(store.saved) != 1 || len(store.saved[0]) != 2 {
		t.Fatalf("写入行数不符: %+v", store.saved)
	}
}

// TestCryptoResultToStockListResult 采集结果 → 运行面板结果结构。
func TestCryptoResultToStockListResult(t *testing.T) {
	got := cryptoResultToStockListResult(CryptoFavSnapshotResult{
		USCTotal: 3, CodesRequested: 3, RowsReceived: 3,
		RowsInsert: 2, RowsUpdate: 1, Batches: 1, ElapsedMs: 120,
	})
	if got.InstrumentSymbol != CryptoFavSymbol || got.Pages != 1 ||
		got.RowsReceived != 3 || got.RowsInsert != 2 || got.RowsUpdate != 1 ||
		got.CountExists != 3 || got.TotalAvailable != 3 || got.ElapsedMs != 120 {
		t.Fatalf("转换结果不符: %+v", got)
	}
}
