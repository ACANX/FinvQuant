package secuft

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLiveFiveDayFtPage 真实访问 Supabase（默认跳过）。
//
// 需要 FINV_TEST_PG_DSN（读 20013 的库）：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveFiveDayFt -v ./internal/meta/finv/quote/secuft/
//
// 断言口径（2026-11-19 实测）：`public.finv_quote_secu_ft` 可读但当前 **0 行**
// （PostgREST `Content-Range: *\/0`），因此这里只断言「查询链路通、列元数据齐、
// 越界页按空页处理」，不假设有数据。
func TestLiveFiveDayFtPage(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store := NewStore(pool)

	if got := len(Columns()); got != 21 {
		t.Fatalf("列元数据应为 21 列，实际 %d", got)
	}

	result, err := store.Page(ctx, Query{}) // 全默认：page=1 / pageSize=15 / usc 升序
	if err != nil {
		t.Fatalf("分页查询失败: %v", err)
	}
	total := result.Total
	t.Logf("实测 total=%d rows=%d elapsed_ms=%d", total, len(result.Rows), result.ElapsedMs)
	if total < 0 {
		t.Fatalf("total 不应为负: %d", total)
	}
	if int64(len(result.Rows)) > int64(DefaultPageSize) {
		t.Errorf("每页最多 %d 条，实际 %d", DefaultPageSize, len(result.Rows))
	}

	// 越界页：offset 超过 total 时上游返回 416，应被当作空页而不是错误。
	beyond, err := store.Page(ctx, Query{Page: 1000, PageSize: DefaultPageSize})
	if err != nil {
		t.Fatalf("越界页应返回空页，实际报错: %v", err)
	}
	if len(beyond.Rows) != 0 {
		t.Errorf("越界页应为空，实际 %d 行", len(beyond.Rows))
	}
	if beyond.Total != total {
		t.Errorf("越界页总数应与首页一致（%d），实际 %d", total, beyond.Total)
	}

	// 筛选与关键字下推不应报语法错（表为空时恒返回 0 行）。
	if _, err := store.Page(ctx, Query{
		Keyword:     "00700",
		TextFilters: map[string]string{"marketLabel": "HK", "instrument": "stock", "typeSecu": "Stock", "provider": "FTMM"},
		FlagEnable:  "1",
		SortBy:      "dt_update",
		SortDesc:    true,
	}); err != nil {
		t.Fatalf("带筛选与排序的查询失败: %v", err)
	}

	options, err := store.FilterOptions(ctx)
	if err != nil {
		t.Fatalf("下拉候选查询失败: %v", err)
	}
	for _, param := range []string{"marketLabel", "instrument", "typeSecu", "provider"} {
		if _, ok := options[param]; !ok {
			t.Errorf("下拉候选缺少 %s", param)
		}
	}
	t.Logf("下拉候选：marketLabel=%d instrument=%d typeSecu=%d provider=%d",
		len(options["marketLabel"]), len(options["instrument"]), len(options["typeSecu"]), len(options["provider"]))
}
