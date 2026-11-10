package uscbackfill

import (
	"context"
	"fmt"
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
	probes := []string{
		"US.AAPL", "SH.000300", "AU..XFL", "US.XRP", "HK.800700", "CA.HSI",
		// PostgREST 过滤器里有特殊字符的键：逗号、星号、双点，以及**带 & 的超长键**
		// （`&` 未百分号编码会被 URL 层当参数分隔符，导致上游报 PGRST100 过滤器截断）
		"US.SBT,OLD", "US.SESN,CVR", "US.ASAI*", "AU..XFL", "AU.quote-etf-au-index-etfs-s&amp",
		// 冲突排查：加密 LTC 与美股 LTC 争同一 usc
		"CC.LTC", "US.LTC",
	}
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

// TestLiveFullDiff 全量对比「本地 futu.finv_futu_secu_key.usc」与「上游 public.finv_quote_secu_key.usc」，
// 用来确认回推是否真的落到了上游（需求 11084/11095 的验收手段）。
//
// 默认跳过（需线上凭证与网络，且要跑几分钟）：
//
//	FINV_USC_WRITEBACK_LIVE=1 FINV_USC_WRITEBACK_LIVE_FULL=1 \
//	  FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveFullDiff -v -timeout 60m ./internal/meta/finv/quote/uscbackfill/
//
// **只读**：只 GET 上游，不 PATCH，可在正式库上跑。
// 输出：keep（上游已与本地一致）/ fill（上游为空需填）/ overwrite（上游有值但不同）/ missing（上游无该行），
// 并打印前若干条差异明细。
func TestLiveFullDiff(t *testing.T) {
	if os.Getenv("FINV_USC_WRITEBACK_LIVE") != "1" || os.Getenv("FINV_USC_WRITEBACK_LIVE_FULL") != "1" {
		t.Skip("未设置 FINV_USC_WRITEBACK_LIVE=1 与 FINV_USC_WRITEBACK_LIVE_FULL=1，跳过全量只读对比")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过全量只读对比")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	service := NewService(pool)
	values, err := syscfg.NewStore(pool).Load(ctx)
	if err != nil {
		t.Fatalf("读取系统配置 20013 失败: %v", err)
	}
	if values.SupabaseConnErr != nil {
		t.Fatalf("Supabase 连接不可用: %v", values.SupabaseConnErr)
	}
	client := service.newClient(values.SupabaseConn)

	local, err := service.store.ReadLocalUSC(ctx)
	if err != nil {
		t.Fatalf("读取本地键表失败: %v", err)
	}
	t.Logf("本地 %s 共 %d 行（usc 非空）", LocalKeyTable, len(local))

	const batch = 200
	var counts = map[string]int{}
	var samples []string
	for start := 0; start < len(local); start += batch {
		end := start + batch
		if end > len(local) {
			end = len(local)
		}
		chunk := local[start:end]
		keys := make([]string, 0, len(chunk))
		for _, row := range chunk {
			keys = append(keys, row.FtmmSymbol)
		}
		raw, err := client.FetchByTextKeys(ctx, KeyTable, keyUpstreamSelect, KeyKeyColumn, keys)
		if err != nil {
			t.Fatalf("第 %d 批读取上游失败: %v", start/batch+1, err)
		}
		upstream, err := indexKeyUpstream(raw)
		if err != nil {
			t.Fatalf("第 %d 批解析上游行失败: %v", start/batch+1, err)
		}
		for _, row := range chunk {
			up, ok := upstream[row.FtmmSymbol]
			class := classify(up, ok, row.USC)
			counts[class]++
			if class != classKeep && len(samples) < 20 {
				samples = append(samples, fmt.Sprintf("%-46s 本地=%-24s 上游=%-24s → %s",
					row.FtmmSymbol, row.USC, up.USC, class))
			}
		}
	}
	t.Logf("全量对比：keep=%d fill=%d overwrite=%d missing=%d（合计 %d）",
		counts[classKeep], counts[classFill], counts[classOverwrite], counts[classMissing], len(local))
	for _, line := range samples {
		t.Logf("差异样例：%s", line)
	}
	if counts[classFill]+counts[classOverwrite]+counts[classMissing] > 0 {
		t.Logf("结论：上游尚未与本地完全一致（上面三类合计 %d 行）",
			counts[classFill]+counts[classOverwrite]+counts[classMissing])
	} else {
		t.Logf("结论：上游 public.%s.usc 已与本地 %s 完全一致（%d 行全部 keep）",
			KeyTable, LocalKeyTable, len(local))
	}
}
