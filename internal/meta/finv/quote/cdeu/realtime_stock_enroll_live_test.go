package cdeu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLivePendingSecuPrefill 只读校验补录预填（需求 11147）。
//
// 默认跳过；需要真实库：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLivePendingSecuPrefill -v ./internal/meta/finv/quote/cdeu/
//
// **只读**：只跑 SELECT，可安全地在正式库上跑。
func TestLivePendingSecuPrefill(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过真实库预填校验")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	store := &Store{pool: pool}

	// 取一台有 HK 股票的 stock_id（不假设具体行，库空则跳过）。
	var stockID int64
	if err := pool.QueryRow(ctx, `
SELECT stock_id FROM cdeu.finv_cdeu_secu_realtime_stock
 WHERE market_label = 'HK' AND stock_code IS NOT NULL
 ORDER BY stock_id LIMIT 1`).Scan(&stockID); err != nil {
		t.Skipf("库里没有可用的 HK 快照行，跳过: %v", err)
	}

	prefill, err := store.PendingSecuPrefill(ctx, stockID)
	if err != nil {
		t.Fatalf("预填失败: %v", err)
	}
	if prefill.USC == "" || prefill.USC == "." {
		t.Fatalf("派生 usc 为空: %+v", prefill)
	}
	if len(prefill.Security) == 0 || len(prefill.Collect) == 0 {
		t.Fatalf("预填字段为空: security=%d collect=%d", len(prefill.Security), len(prefill.Collect))
	}
	// 关键派生规则断言：exchange_code 来自字典、usc 前缀与市场标签一致。
	for _, f := range prefill.Security {
		if f.Key == "exchange_code" && f.Value == "" {
			t.Fatalf("exchange_code 未预填（HK 应能由 finv_exchange 推出）: %+v", f)
		}
	}
	if want := "HK."; len(prefill.USC) <= len(want) || prefill.USC[:len(want)] != want {
		t.Fatalf("usc 应以 HK. 开头: %s", prefill.USC)
	}
	t.Logf("stock_id=%d usc=%s security=%d 项 collect=%d 项 warnings=%d 条",
		stockID, prefill.USC, len(prefill.Security), len(prefill.Collect), len(prefill.Warnings))
	for _, w := range prefill.Warnings {
		t.Logf("  warning: %s", w)
	}
}

// TestSubInstrumentType / TestSecurityTypeByInstrumentType 派生规则单测（不依赖数据库）。
func TestSubInstrumentType(t *testing.T) {
	cases := map[int]int{3: 3002, 4: 4002, 6: 6002, 0: 3002}
	for in, want := range cases {
		if got := subInstrumentType(in); got != want {
			t.Fatalf("subInstrumentType(%d)=%d，期望 %d", in, got, want)
		}
	}
}

func TestSecurityTypeByInstrumentType(t *testing.T) {
	cases := map[int]string{3: "Stock", 4: "ETF", 6: "StockIndex", 11: "Crypto", 12: "Stock"}
	for in, want := range cases {
		if got := securityTypeByInstrumentType(in); got != want {
			t.Fatalf("securityTypeByInstrumentType(%d)=%s，期望 %s", in, got, want)
		}
	}
}

func TestBuildAutoRemark(t *testing.T) {
	at := time.Date(2026, 1, 15, 10, 20, 30, 0, time.FixedZone("CST", 8*3600))
	if got, want := buildAutoRemark("", at), "[AUTO-FILL] 待录入证券补录 2026-01-15T10:20:30+08:00"; got != want {
		t.Fatalf("空备注 remark=%q，期望 %q", got, want)
	}
	if got, want := buildAutoRemark(" 核对无误 ", at),
		"[AUTO-FILL] 待录入证券补录 2026-01-15T10:20:30+08:00；备注：核对无误"; got != want {
		t.Fatalf("带备注 remark=%q，期望 %q", got, want)
	}
}

// TestKeyInstrumentByInstrumentType 键表 instrument 推导规则（需求 11153）。
//
// 词表与采集表不同：键表里板块是 `sector`（单数），采集表是 `sectors`；
// 这里断言推导结果一定落在键表词表内，避免写进去匹配不到的取值。
func TestKeyInstrumentByInstrumentType(t *testing.T) {
	cases := []struct {
		instrumentType, marketType int
		want                       string
	}{
		{3, 1, "stock"},   // HK 股票
		{3, 2, "stock"},   // US 股票
		{12, 2, "stock"},  // 富途 12 也是股票口径
		{4, 2, "etf"},     // ETF
		{6, 4, "index"},   // 指数
		{7, 2, "sector"},  // 板块（注意不是 sectors）
		{9, 2, "futures"}, // 期货
		{11, 17, "crypto"},
		// instrument_type 缺失时用 market_type 兜底
		{0, 11, "forex"},
		{0, 17, "crypto"},
		{0, 1, "stock"},
	}
	for _, c := range cases {
		got := keyInstrumentByInstrumentType(c.instrumentType, c.marketType)
		if got != c.want {
			t.Fatalf("instrument_type=%d market_type=%d → %s，期望 %s",
				c.instrumentType, c.marketType, got, c.want)
		}
		if !keyInstrumentAllowed(got) {
			t.Fatalf("推导值 %s 不在键表词表内（%v）", got, KeyInstrumentOptions)
		}
	}
	if keyInstrumentAllowed("sectors") {
		t.Fatalf("sectors 是采集表口径，键表不应接受")
	}
}

// TestLivePendingSecuEnqueuePreview 只读校验入队预检（需求 11153）。
//
// 需要真实库（**只读**，不插入）：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLivePendingSecuEnqueuePreview -v ./internal/meta/finv/quote/cdeu/
func TestLivePendingSecuEnqueuePreview(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过真实库入队预检")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := &Store{pool: pool}

	var stockID int64
	if err := pool.QueryRow(ctx, `
SELECT stock_id FROM cdeu.finv_cdeu_secu_realtime_stock
 WHERE market_label = 'HK' AND stock_code IS NOT NULL
 ORDER BY stock_id LIMIT 1`).Scan(&stockID); err != nil {
		t.Skipf("库里没有可用的 HK 快照行，跳过: %v", err)
	}

	preview, err := store.PreviewPendingSecuEnqueue(ctx, stockID)
	if err != nil {
		t.Fatalf("预检失败: %v", err)
	}
	if preview.FtmmSymbol == "" || preview.Market != "HK" || preview.Code == "" {
		t.Fatalf("派生字段不全: %+v", preview)
	}
	if !keyInstrumentAllowed(preview.Instrument) {
		t.Fatalf("instrument=%s 不在键表词表内", preview.Instrument)
	}
	if preview.Existed == preview.Writable {
		t.Fatalf("existed 与 writable 必须互斥: existed=%t writable=%t", preview.Existed, preview.Writable)
	}
	t.Logf("stock_id=%d ftmm=%s instrument=%s existed=%t status=%s usc=%q hint=%s",
		stockID, preview.FtmmSymbol, preview.Instrument, preview.Existed, preview.Status, preview.USC, preview.Hint)
}
