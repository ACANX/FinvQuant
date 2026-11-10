package uscbackfill

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// TestLiveReadUpstream 只读校验「上游 public.finv_quote_secu_key.usc」的读取与分类（需求 11084）。
//
// 默认跳过（需要 20013 里配置好的线上 Supabase 凭证与网络）：
//
//	FINV_USC_WRITEBACK_LIVE=1 FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:55432/finv?sslmode=disable" \
//	  go test -run TestLiveReadUpstream -v ./internal/meta/finv/quote/uscbackfill/
//
// **只读**：只 GET 上游，不 PATCH，因此可安全地在正式库上跑。
func TestLiveReadUpstream(t *testing.T) {
	if os.Getenv("FINV_USC_WRITEBACK_LIVE") != "1" {
		t.Skip("未设置 FINV_USC_WRITEBACK_LIVE=1，跳过真实上游只读校验")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过真实上游只读校验")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	service := NewService(pool)

	status, err := service.Status(ctx)
	if err != nil {
		t.Fatalf("读取状态失败: %v", err)
	}
	t.Logf("状态：configured=%v project=%s 本地 %s 共 %d 行（usc 有值 %d，为空 %d）",
		status.Configured, status.ProjectID, status.LocalTable, status.LocalRows,
		status.LocalWithUSC, status.LocalEmptyUSC)

	values, err := syscfg.NewStore(pool).Load(ctx)
	if err != nil {
		t.Fatalf("读取系统配置 20013 失败: %v", err)
	}
	if values.SupabaseConnErr != nil {
		t.Fatalf("Supabase 连接不可用: %v", values.SupabaseConnErr)
	}

	// 抽样几个已知形态（SH 指数统一 SH####、指数去前导点、撞名例外、港股改名）
	probes := []string{"US.AAPL", "SH.000300", "AU..XFL", "US.XRP", "HK.800700", "CA.HSI"}
	local := make(map[string]string, len(probes))
	rows, err := pool.Query(ctx, `
		SELECT ftmm_symbol, usc FROM `+LocalKeyTable+`
		 WHERE ftmm_symbol = ANY($1)`, probes)
	if err != nil {
		t.Fatalf("读取本地键表失败: %v", err)
	}
	for rows.Next() {
		var symbol, usc string
		if err := rows.Scan(&symbol, &usc); err != nil {
			t.Fatalf("解析本地行失败: %v", err)
		}
		local[symbol] = usc
	}
	rows.Close()

	client := service.newClient(values.SupabaseConn)
	raw, err := client.FetchByTextKeys(ctx, KeyTable, keyUpstreamSelect, KeyKeyColumn, probes)
	if err != nil {
		t.Fatalf("读取上游失败: %v", err)
	}
	upstream, err := indexKeyUpstream(raw)
	if err != nil {
		t.Fatalf("解析上游行失败: %v", err)
	}

	for _, symbol := range probes {
		up, ok := upstream[symbol]
		want := local[symbol]
		t.Logf("%-12s 本地=%-9s 上游=%-9s → %s", symbol, want, up.USC, classify(up, ok, want))
	}
	if len(upstream) == 0 {
		t.Fatalf("上游未返回任何行，检查 20013 配置或 RLS")
	}
}
