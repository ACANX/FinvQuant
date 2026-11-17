package cdeu

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveXetIndexSnapshot 真实拉取沪深京指数并写入 cdeu.finv_quote_secu_realtime（默认跳过）。
//
// 需要可访问 https://xetapp.citics.com 的网络与可写的 PG：
//
//	FINV_XET_LIVE=1 \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveXetIndexSnapshot -v ./internal/meta/finv/quote/cdeu/
//
// 该测试会真实写入/provider=ZXET 的全量指数快照（按 usc upsert，不清理）——
// 它本身就是「采集一次真实数据」的最小可用入口。
func TestLiveXetIndexSnapshot(t *testing.T) {
	if os.Getenv("FINV_XET_LIVE") != "1" {
		t.Skip("未设置 FINV_XET_LIVE=1，跳过真实指数接口测试")
	}
	pool := testPool(t)
	store := NewStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	collector := NewXetIndexCollector(NewXetIndexFetcher(), store, store, nil)
	result, err := collector.Collect(ctx, XetIndexCollectOptions{})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if result.RowsReceived < 1000 {
		t.Fatalf("去重后行数异常（期望 ≥1000，实际 %d）", result.RowsReceived)
	}
	if result.RowsInsert+result.RowsUpdate == 0 {
		t.Fatalf("未写入任何行: %+v", result)
	}
	t.Logf("采集完成：池 %d / 收到 %d 条 / 新增 %d / 覆盖 %d / 耗时 %d ms",
		result.Pages, result.RowsReceived, result.RowsInsert, result.RowsUpdate, result.ElapsedMs)

	cases := []struct {
		usc   string
		label string
		code  string
	}{
		{"SH0001", "SH", "000001"},   // 1A0001 上证指数（市场号 1100）
		{"CSI932000", "SH", "932000"}, // 932000 中证2000（编码域 CSI，市场号 1100 → SH）
		{"BSE50", "BJ", "899050"},     // 899050 北证50（V300219 改名后的 usc）
		{"CNI980028", "SZ", "980028"}, // 980028 龙头家电（国证，市场号 1200 → SZ）
	}
	for _, tc := range cases {
		var label, code string
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(market_label, ''), COALESCE(stock_code, '')
			   FROM cdeu.finv_quote_secu_realtime
			  WHERE usc = $1 AND provider = 'ZXET'`, tc.usc).Scan(&label, &code); err != nil {
			t.Fatalf("查询 %s 失败: %v", tc.usc, err)
		}
		if label != tc.label || code != tc.code {
			t.Fatalf("%s：期望 market_label=%s stock_code=%s，实际 %s/%s", tc.usc, tc.label, tc.code, label, code)
		}
	}
}
