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
