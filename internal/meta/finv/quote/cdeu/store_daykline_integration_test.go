package cdeu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/database"
	"github.com/acanx/finvquant/internal/meta/finv/quote/moomoo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// stubFetcher 固定返回若干根 K 线（避免测试依赖外网）。
type stubFetcher struct {
	bars []moomoo.Bar
	err  error
	got  moomoo.KlineRequest
}

func (f *stubFetcher) Kline(_ context.Context, req moomoo.KlineRequest) ([]moomoo.Bar, error) {
	f.got = req
	if f.err != nil {
		return nil, f.err
	}
	return f.bars, nil
}

// TestDayKlineDate 时间戳 → 市场本地日期（跨时区不能按 UTC 取日）。
func TestDayKlineDate(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}
	// 美东 2026-10-01 16:00 = UTC 2026-10-01 20:00
	ts := time.Date(2026, 10, 1, 16, 0, 0, 0, ny).Unix()
	if got := dayKlineDate(ts, ny); got != 20261001 {
		t.Fatalf("纽约时区日期期望 20261001，实际 %d", got)
	}
	if got := dayKlineDate(ts, time.UTC); got != 20261001 {
		t.Fatalf("UTC 日期期望 20261001，实际 %d", got)
	}
	// 美东 2026-10-01 20:00 = UTC 2026-10-02 00:00：按市场时区仍是 10-01，按 UTC 是 10-02。
	late := time.Date(2026, 10, 1, 20, 0, 0, 0, ny).Unix()
	if got := dayKlineDate(late, ny); got != 20261001 {
		t.Fatalf("跨日时刻按市场时区应为 20261001，实际 %d", got)
	}
	if got := dayKlineDate(late, time.UTC); got != 20261002 {
		t.Fatalf("同一时刻按 UTC 应为 20261002，实际 %d", got)
	}
	if got := dayKlineDate(0, ny); got != 0 {
		t.Fatalf("0 时间戳应返回 0，实际 %d", got)
	}
}

// TestDayKlineChangeRatio 涨跌幅按「涨跌额 / 前收盘 × 100」计算（与分钟线口径一致）。
func TestDayKlineChangeRatio(t *testing.T) {
	cases := []struct {
		name string
		bar  moomoo.Bar
		want string
	}{
		{"正常", moomoo.Bar{LastClose: 100, ChangePrice: "2.5"}, "2.500000"},
		{"下跌", moomoo.Bar{LastClose: 200, ChangePrice: "-3"}, "-1.500000"},
		{"前收缺失", moomoo.Bar{LastClose: 0, ChangePrice: "1"}, ""},
		{"涨跌额缺失", moomoo.Bar{LastClose: 100}, ""},
		{"涨跌额非法", moomoo.Bar{LastClose: 100, ChangePrice: "abc"}, ""},
	}
	for _, tc := range cases {
		got := dayKlineChangeRatio(tc.bar)
		if tc.want == "" {
			if got != nil {
				t.Fatalf("%s：应返回 nil，实际 %s", tc.name, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Fatalf("%s：期望 %s，实际 %v", tc.name, tc.want, got)
		}
	}
}

// TestOptionalPositive 前收盘 <=0 视为缺失。
func TestOptionalPositive(t *testing.T) {
	if optionalPositive(0) != nil || optionalPositive(-1) != nil {
		t.Fatal("<=0 应返回 nil")
	}
	if got := optionalPositive(187.5); got == nil || *got != "187.500000" {
		t.Fatalf("正数应格式化写入，实际 %v", got)
	}
}

// TestUpsertDayKlinesIntegration 验证日K upsert 的插入/覆盖/多来源字段保留语义（需 DSN）。
func TestUpsertDayKlinesIntegration(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool)
	const usc = "ITSTK_DAY"
	cleanup := func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM `+tableDay+` WHERE usc = $1`, usc)
	}
	cleanup()
	defer cleanup()

	open, closePrice := "10.5", "11"
	lastClose, volume := "10.2", "1000"
	stat, err := store.UpsertDayKlines(context.Background(), usc, []DayKlineRow{
		{Date: 20261001, Open: &open, Close: &closePrice, LastClose: &lastClose, Volume: &volume, Provider: DayKlineProviderMoomoo},
		{Date: 20261002, Close: &closePrice, Provider: DayKlineProviderMoomoo},
	})
	if err != nil {
		t.Fatalf("首次 upsert 失败: %v", err)
	}
	if stat.Inserted != 2 || stat.Updated != 0 {
		t.Fatalf("首次应插入 2 行，实际 %+v", stat)
	}

	// 其它来源回填了持仓量/PE，本次同步只给 close，不应把这两列抹成 NULL。
	if _, err := pool.Exec(context.Background(), `
UPDATE `+tableDay+` SET open_interest = 500, pe_ratio = 12.5
 WHERE usc = $1 AND date = 20261001`, usc); err != nil {
		t.Fatalf("预置其它来源字段失败: %v", err)
	}
	newClose := "11.8"
	stat, err = store.UpsertDayKlines(context.Background(), usc, []DayKlineRow{
		{Date: 20261001, Close: &newClose, Provider: DayKlineProviderMoomoo},
	})
	if err != nil {
		t.Fatalf("覆盖 upsert 失败: %v", err)
	}
	if stat.Inserted != 0 || stat.Updated != 1 {
		t.Fatalf("重复执行应只更新 1 行，实际 %+v", stat)
	}

	var gotClose, gotOpen string
	var gotOpenInterest, gotPe *string
	if err := pool.QueryRow(context.Background(), `
SELECT close::text, COALESCE(open::text, ''), open_interest::text, pe_ratio::text
  FROM `+tableDay+` WHERE usc = $1 AND date = 20261001`, usc).
		Scan(&gotClose, &gotOpen, &gotOpenInterest, &gotPe); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if gotClose != "11.800000" {
		t.Fatalf("close 应被覆盖为 11.800000，实际 %s", gotClose)
	}
	if gotOpen != "10.500000" {
		t.Fatalf("open 应保留原值 10.500000（本次未提供），实际 %s", gotOpen)
	}
	if gotOpenInterest == nil || *gotOpenInterest != "500.000000" {
		t.Fatalf("open_interest 不应被抹掉，实际 %v", gotOpenInterest)
	}
	if gotPe == nil || *gotPe != "12.500000" {
		t.Fatalf("pe_ratio 不应被抹掉，实际 %v", gotPe)
	}

	stats, err := store.DayKlineStats(context.Background(), usc)
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if stats.Rows != 2 || stats.DateFrom != 20261001 || stats.DateTo != 20261002 {
		t.Fatalf("统计不符：%+v", stats)
	}
}

// TestSyncDayKlineFromMoomooIntegration 端到端：futu 参数 → 拉取（桩）→ 时区换算 → upsert（需 DSN）。
func TestSyncDayKlineFromMoomooIntegration(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool)
	const usc = "ITSTK_DAY_SYNC"
	cleanup := func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM `+tableDay+` WHERE usc = $1`, usc)
		_, _ = pool.Exec(ctx, `DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, usc)
		_, _ = pool.Exec(ctx, `DELETE FROM futu.finv_futu_quote_collect WHERE usc = $1`, usc)
	}
	cleanup()
	defer cleanup()

	if _, err := pool.Exec(context.Background(), `
INSERT INTO futu.finv_futu_quote_collect
  (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
   sub_instrument_type, req_section, quote_market, type_symbol, flag_enable)
VALUES ($1, 70000294, 'ITSTKDAY', 2, 9001, 9, 9008, '', 'US', 'futures', '1')`, usc); err != nil {
		t.Fatalf("插入 futu 参数失败: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO cdeu.finv_quote_cdeu_usc
  (usc, category, region, market, provider, name, timezone, currency)
VALUES ($1, 'Futures', 'US', 'COMEX', 'FTMM', '日K同步测试', 'America/New_York', 'USD')`, usc); err != nil {
		t.Fatalf("插入 usc 主数据失败: %v", err)
	}

	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}
	fetcher := &stubFetcher{bars: []moomoo.Bar{
		{Time: time.Date(2026, 10, 1, 16, 0, 0, 0, ny).Unix(), Open: "10", High: "11", Low: "9", Close: "10.5", LastClose: 10.2, ChangePrice: "0.3", Volume: 100, Turnover: 1050},
		{Time: time.Date(2026, 10, 2, 16, 0, 0, 0, ny).Unix(), Open: "10.5", High: "12", Low: "10", Close: "11.5", LastClose: 10.5, ChangePrice: "1", Volume: 200, Turnover: 2200},
		{Time: time.Date(2026, 10, 5, 16, 0, 0, 0, ny).Unix(), Open: "11.5", High: "12", Low: "11", Close: "11.8", LastClose: 11.5, ChangePrice: "0.3", Volume: 150, Turnover: 1750},
	}}
	service := &Service{store: store, klineFetcher: fetcher}

	result, err := service.SyncDayKlineFromMoomoo(context.Background(), DayKlineSyncRequest{USC: usc})
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if fetcher.got.StockID != "70000294" || fetcher.got.MarketCode != "9001" ||
		fetcher.got.InstrumentType != "9" || fetcher.got.SubInstrumentType != "9008" ||
		fetcher.got.KlineType != moomoo.KlineTypeDay {
		t.Fatalf("请求参数应原样透传 futu 表取值，实际 %+v", fetcher.got)
	}
	if result.Total != 3 || result.InRange != 3 || result.Inserted != 3 || result.Updated != 0 {
		t.Fatalf("首次同步统计不符：%+v", result)
	}
	if result.DateFrom != 20261001 || result.DateTo != 20261005 {
		t.Fatalf("日期范围不符：%+v", result)
	}
	if result.Timezone != "America/New_York" || result.Provider != DayKlineProviderMoomoo {
		t.Fatalf("时区/来源不符：%+v", result)
	}

	// 幂等：同数据再同步一次应全部走 UPDATE。
	result2, err := service.SyncDayKlineFromMoomoo(context.Background(), DayKlineSyncRequest{USC: usc})
	if err != nil {
		t.Fatalf("二次同步失败: %v", err)
	}
	if result2.Inserted != 0 || result2.Updated != 3 {
		t.Fatalf("二次同步应全部更新，实际 %+v", result2)
	}

	// 区间过滤：只同步 2026-10-02。
	result3, err := service.SyncDayKlineFromMoomoo(context.Background(), DayKlineSyncRequest{
		USC: usc, From: 20261002, To: 20261002,
	})
	if err != nil {
		t.Fatalf("区间同步失败: %v", err)
	}
	if result3.Total != 3 || result3.InRange != 1 || result3.Updated != 1 {
		t.Fatalf("区间过滤统计不符：%+v", result3)
	}

	// 未登记的 usc 应返回明确错误。
	if _, err := service.SyncDayKlineFromMoomoo(context.Background(), DayKlineSyncRequest{USC: "ITSTK_NOT_EXIST"}); !errors.Is(err, ErrCollectParamsNotFound) {
		t.Fatalf("未登记 usc 应返回 ErrCollectParamsNotFound，实际 %v", err)
	}

	// 校验落库值：涨跌幅按百分数、前收盘、成交量写入。
	var ratio string
	if err := pool.QueryRow(context.Background(), `
SELECT COALESCE(change_ratio::text, '') FROM `+tableDay+` WHERE usc = $1 AND date = 20261001`, usc).Scan(&ratio); err != nil {
		t.Fatalf("查询涨跌幅失败: %v", err)
	}
	if ratio != "2.941176" {
		t.Fatalf("涨跌幅期望 2.941176（0.3/10.2*100），实际 %s", ratio)
	}
}

// testPool 打开测试库并执行全量迁移；未设置 FINV_TEST_PG_DSN 时跳过。
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("解析测试 DSN 失败: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,cdeu,archive,sys"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "..", "..", "..", "Deploy", "Migrations")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	return pool
}
