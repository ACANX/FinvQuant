package cdeu

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// fakeIndexFetcher 返回固定快照（记录收到的 code 列表）。
type fakeIndexFetcher struct {
	gotCodes [][]string
	rows     []webapi.Snapshot
	err      error
}

func (f *fakeIndexFetcher) GetQuoteSnapshots(_ context.Context, codes []string) ([]webapi.Snapshot, error) {
	f.gotCodes = append(f.gotCodes, append([]string(nil), codes...))
	return f.rows, f.err
}

// fakeIndexStore 记录写入行与收藏目标。
type fakeIndexStore struct {
	targets  []IndexFavTarget
	saved    [][]QuoteSecuRealtimeRow
	inserted int64
	updated  int64
	err      error
}

func (s *fakeIndexStore) ListFavoriteExchangeIndexTargets(context.Context) ([]IndexFavTarget, error) {
	return s.targets, s.err
}

func (s *fakeIndexStore) SaveQuoteSecuRealtimeRows(_ context.Context, rows []QuoteSecuRealtimeRow) (int64, int64, error) {
	s.saved = append(s.saved, rows)
	return s.inserted, s.updated, nil
}

// TestIndexFavTargetMoomooCode 交易所 → moomoo code（港股单点、美股双点）。
func TestIndexFavTargetMoomooCode(t *testing.T) {
	cases := []struct {
		name string
		in   IndexFavTarget
		want string
	}{
		{"港股指数", IndexFavTarget{SecurityCode: "800000", ExchangeFlag: "HK"}, "HK.800000"},
		{"港股小写", IndexFavTarget{SecurityCode: "800700", ExchangeFlag: "hk"}, "HK.800700"},
		{"美股指数双点", IndexFavTarget{SecurityCode: "DJI", ExchangeFlag: "US"}, "US..DJI"},
		{"美股小写代码", IndexFavTarget{SecurityCode: "ndx", ExchangeFlag: "us"}, "US..NDX"},
		{"空代码", IndexFavTarget{SecurityCode: "  ", ExchangeFlag: "HK"}, ""},
		{"不支持的交易所", IndexFavTarget{SecurityCode: "000001", ExchangeFlag: "SH"}, ""},
		// futu 映射表优先：HSIII 的 security_code 是字母别名，真实柜台代码是 HK.800806
		{"映射表优先（港股）", IndexFavTarget{SecurityCode: "HSIII", ExchangeFlag: "HK", FtmmSymbol: "hk.800806"}, "HK.800806"},
		{"映射表优先（美股）", IndexFavTarget{SecurityCode: "DJI", ExchangeFlag: "US", FtmmSymbol: " US..DJI "}, "US..DJI"},
		{"映射表为空回退拼装", IndexFavTarget{SecurityCode: "800000", ExchangeFlag: "HK", FtmmSymbol: "  "}, "HK.800000"},
	}
	for _, c := range cases {
		if got := c.in.MoomooCode(); got != c.want {
			t.Errorf("%s: MoomooCode()=%q want %q", c.name, got, c.want)
		}
	}
}

// TestIndexFavRealtimeRow 快照 → 落库行：市场/类型/货币/涨跌口径。
func TestIndexFavRealtimeRow(t *testing.T) {
	target := IndexFavTarget{USC: "HSI", SecurityCode: "800000", ExchangeFlag: "HK", Currency: "HKD"}
	row, ok := indexFavRealtimeRow(target, webapi.Snapshot{
		Code:           "HK.800000",
		Name:           "Hang Seng Index",
		LastPrice:      23294.75,
		PrevClosePrice: 23508.17,
		Amplitude:      1.38,
		Volume:         1234,
		Turnover:       5678.9,
		VolumeRatio:    1.41,
		BidAskRatio:    12.5,
		UpdateTime:     1795011480000,
	}, 1795011480123)
	if !ok {
		t.Fatal("合法港股指数应转换成功")
	}
	if row.USC != "HSI" {
		t.Errorf("usc=%q want HSI", row.USC)
	}
	if row.MarketType == nil || *row.MarketType != "1" {
		t.Errorf("market_type=%v want 1（港股）", row.MarketType)
	}
	if row.MarketLabel == nil || *row.MarketLabel != "HK" {
		t.Errorf("market_label=%v want HK", row.MarketLabel)
	}
	if row.InstrumentType == nil || *row.InstrumentType != "6" {
		t.Errorf("instrument_type=%v want 6（指数）", row.InstrumentType)
	}
	if row.StockCode == nil || *row.StockCode != "800000" {
		t.Errorf("stock_code=%v want 800000", row.StockCode)
	}
	if row.Currency == nil || *row.Currency != "HKD" {
		t.Errorf("currency=%v want HKD", row.Currency)
	}
	if row.Provider != IndexFavProvider {
		t.Errorf("provider=%q want %q", row.Provider, IndexFavProvider)
	}
	if row.DtUpdate == nil || *row.DtUpdate != 1795011480123 {
		t.Errorf("dt_update=%v want 1795011480123", row.DtUpdate)
	}
	// 涨跌额 = last - prev_close；涨跌幅是小数比例（-0.009079...），不是百分数
	if row.ChangePrice == nil || row.ChangePrice.String() != "-213.42" {
		t.Errorf("change_price=%v want -213.42", row.ChangePrice)
	}
	if row.ChangeRatio == nil || !strings.HasPrefix(row.ChangeRatio.String(), "-0.0090") {
		t.Errorf("change_ratio=%v want 约 -0.00907（小数比例）", row.ChangeRatio)
	}
	if row.PriceDirect == nil || *row.PriceDirect != "-" {
		t.Errorf("price_direct=%v want -", row.PriceDirect)
	}
	// amplitude / bid_ask_ratio 上游是百分数，落库除以 100
	if row.PriceAmplitude == nil || row.PriceAmplitude.String() != "0.0138" {
		t.Errorf("price_amplitude=%v want 0.0138", row.PriceAmplitude)
	}
	if row.BuysellRatio == nil || row.BuysellRatio.String() != "0.125" {
		t.Errorf("buysell_ratio=%v want 0.125", row.BuysellRatio)
	}
	// volume_ratio 是倍数，不除 100（与 ZXET 同口径）
	if row.VolumnRatio == nil || row.VolumnRatio.String() != "1.41" {
		t.Errorf("volumn_ratio=%v want 1.41", row.VolumnRatio)
	}
}

// TestIndexFavRealtimeRowUSFallbackCurrency 美股指数缺 currency_type 时兜底 USD；无行情只写身份列。
func TestIndexFavRealtimeRowUSFallbackCurrency(t *testing.T) {
	target := IndexFavTarget{USC: "DJI", SecurityCode: "DJI", ExchangeFlag: "US"}
	row, ok := indexFavRealtimeRow(target, webapi.Snapshot{Code: "US..DJI", Name: "Dow Jones", SCName: "道琼斯指数"}, 1700000000000)
	if !ok {
		t.Fatal("美股指数应转换成功")
	}
	if row.Currency == nil || *row.Currency != "USD" {
		t.Errorf("currency=%v want USD", row.Currency)
	}
	if row.MarketType == nil || *row.MarketType != "2" {
		t.Errorf("market_type=%v want 2（美股）", row.MarketType)
	}
	// 名称优先简体中文（上游 sc_name），取不到才用 name（需求 11339）
	if row.Name == nil || *row.Name != "道琼斯指数" {
		t.Errorf("name=%v want 道琼斯指数（应取 sc_name）", row.Name)
	}
	if fallback, ok := indexFavRealtimeRow(target, webapi.Snapshot{Code: "US..DJI", Name: "Dow Jones"}, 1700000000000); !ok || fallback.Name == nil || *fallback.Name != "Dow Jones" {
		t.Errorf("sc_name 缺失时应回退 name，实际 %v", fallback.Name)
	}
	if row.PriceNominal != nil || row.ChangePrice != nil {
		t.Errorf("last=0 时不应写行情列，实际 price=%v change=%v", row.PriceNominal, row.ChangePrice)
	}
}

// TestIndexFavSnapshotCollectorCollect 端到端：code 拼装、Missing 统计、落库行数。
func TestIndexFavSnapshotCollectorCollect(t *testing.T) {
	fetcher := &fakeIndexFetcher{rows: []webapi.Snapshot{
		{Code: "HK.800000", Name: "Hang Seng Index", LastPrice: 23294.75, PrevClosePrice: 23508.17},
		{Code: "US..DJI", Name: "Dow Jones", LastPrice: 50759.42, PrevClosePrice: 50990},
	}}
	store := &fakeIndexStore{
		targets: []IndexFavTarget{
			{USC: "HSI", SecurityCode: "800000", ExchangeFlag: "HK", Currency: "HKD"},
			{USC: "DJI", SecurityCode: "DJI", ExchangeFlag: "US"},
			{USC: "SPX", SecurityCode: "SPX", ExchangeFlag: "US"},
		},
		inserted: 2,
	}
	collector := NewIndexFavSnapshotCollector(fetcher, store, nil)
	result, err := collector.Collect(context.Background(), IndexFavSnapshotCollectOptions{})
	if err != nil {
		t.Fatalf("Collect 失败: %v", err)
	}
	if len(fetcher.gotCodes) != 1 {
		t.Fatalf("批次数=%d want 1", len(fetcher.gotCodes))
	}
	got := strings.Join(fetcher.gotCodes[0], ",")
	if got != "HK.800000,US..DJI,US..SPX" {
		t.Errorf("请求 codes=%q want HK.800000,US..DJI,US..SPX", got)
	}
	if result.USCTotal != 3 || result.CodesRequested != 3 {
		t.Errorf("usc_total=%d codes=%d want 3/3", result.USCTotal, result.CodesRequested)
	}
	if result.RowsReceived != 2 || result.RowsInsert != 2 {
		t.Errorf("rows=%d insert=%d want 2/2", result.RowsReceived, result.RowsInsert)
	}
	if len(result.Missing) != 1 || result.Missing[0] != "US..SPX" {
		t.Errorf("missing=%v want [US..SPX]", result.Missing)
	}
	// 落库两行且 usc 按请求 code 反查（不是从 code 反推）
	if len(store.saved) != 1 || len(store.saved[0]) != 2 {
		t.Fatalf("落库批次数=%d 行数=%v", len(store.saved), store.saved)
	}
	if store.saved[0][0].USC != "HSI" || store.saved[0][1].USC != "DJI" {
		t.Errorf("落库 usc=%q/%q want HSI/DJI", store.saved[0][0].USC, store.saved[0][1].USC)
	}
}

// TestIndexFavSnapshotCollectorEmpty 收藏表没有美股/港股指数时不报错、不发请求。
func TestIndexFavSnapshotCollectorEmpty(t *testing.T) {
	fetcher := &fakeIndexFetcher{}
	store := &fakeIndexStore{}
	collector := NewIndexFavSnapshotCollector(fetcher, store, nil)
	result, err := collector.Collect(context.Background(), IndexFavSnapshotCollectOptions{})
	if err != nil {
		t.Fatalf("空收藏不应报错: %v", err)
	}
	if result.USCTotal != 0 || len(fetcher.gotCodes) != 0 || len(store.saved) != 0 {
		t.Errorf("空收藏应空跑，实际 usc=%d codes=%d saved=%d",
			result.USCTotal, len(fetcher.gotCodes), len(store.saved))
	}
}

// TestIndexFavSnapshotCollectorFetcherError 拉取失败直接返回错误（不写半批）。
func TestIndexFavSnapshotCollectorFetcherError(t *testing.T) {
	fetcher := &fakeIndexFetcher{err: errors.New("boom")}
	store := &fakeIndexStore{targets: []IndexFavTarget{
		{USC: "HSI", SecurityCode: "800000", ExchangeFlag: "HK"},
	}}
	collector := NewIndexFavSnapshotCollector(fetcher, store, nil)
	if _, err := collector.Collect(context.Background(), IndexFavSnapshotCollectOptions{}); err == nil {
		t.Fatal("拉取失败应返回错误")
	}
	if len(store.saved) != 0 {
		t.Errorf("失败时不应落库，实际写入 %d 批", len(store.saved))
	}
}

// TestIndexFavStoreSatisfies 编译期断言：*Store 满足采集器依赖。
func TestIndexFavStoreSatisfies(t *testing.T) {
	var _ IndexFavSnapshotStore = (*Store)(nil)
}
