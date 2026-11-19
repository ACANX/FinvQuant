package secuft

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLiveLookup 真实库联查补全（默认跳过）。
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveLookup -v ./internal/meta/finv/quote/secuft/
//
// 断言的是**实测过的事实**：
//   - 00700：symbol=HK.00700（quote_collect.ftmm_symbol）、futu_symbol=00700、name=腾讯控股、
//     provider 空→FTMM、flag_enable 默认 0；
//   - 03011：symbol 取 quote_collect 的 HK.03011（而非 secu_key 的 BMS.03011）；
//   - BKX：symbol=US..BKX（美股指数双点形态）、futu_symbol=.BKX；
//   - CSI300：无 quote_collect 记录 → 富途列全部进 missing；
//   - 不存在的 usc → InSecurity=false。
func TestLiveLookup(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store := NewStore(pool)

	cases := []struct {
		usc          string
		wantSymbol   string
		wantFutu     string
		wantName     string
		wantProvider string
		wantMissing  bool
		wantInSecu   bool
	}{
		{usc: "00700", wantSymbol: "HK.00700", wantFutu: "00700", wantName: "腾讯控股", wantProvider: "FTMM", wantInSecu: true},
		{usc: "03011", wantSymbol: "HK.03011", wantFutu: "03011", wantProvider: "FTMM", wantInSecu: true},
		{usc: "BKX", wantSymbol: "US..BKX", wantFutu: ".BKX", wantProvider: "FTMM", wantInSecu: true},
		{usc: "CSI300", wantProvider: "FTMM", wantMissing: true, wantInSecu: true},
		{usc: "__no_such_usc_11345__", wantInSecu: false},
	}

	for _, item := range cases {
		t.Run(item.usc, func(t *testing.T) {
			result, err := store.Lookup(ctx, item.usc)
			if err != nil {
				t.Fatalf("Lookup 失败: %v", err)
			}
			if result.InSecurity != item.wantInSecu {
				t.Fatalf("InSecurity=%t，期望 %t（notes=%v）", result.InSecurity, item.wantInSecu, result.Notes)
			}
			if !item.wantInSecu {
				if len(result.Missing) == 0 {
					t.Error("不在主数据时应把所有列标为缺失")
				}
				return
			}
			if item.wantSymbol != "" {
				if got := result.Fields["symbol"]; got != item.wantSymbol {
					t.Errorf("symbol = %v，期望 %s（来源 %v）", got, item.wantSymbol, result.Sources["symbol"])
				}
			}
			if item.wantFutu != "" {
				if got := result.Fields["futu_symbol"]; got != item.wantFutu {
					t.Errorf("futu_symbol = %v，期望 %s", got, item.wantFutu)
				}
			}
			if item.wantName != "" {
				if got := result.Fields["name"]; got != item.wantName {
					t.Errorf("name = %v，期望 %s", got, item.wantName)
				}
			}
			if item.wantProvider != "" {
				if got := result.Fields["provider"]; got != item.wantProvider {
					t.Errorf("provider = %v，期望 %s", got, item.wantProvider)
				}
			}
			if got := result.Fields["flag_enable"]; got != DefaultFlagEnable {
				t.Errorf("flag_enable 默认应为 0（先停采），实际 %v", got)
			}
			if item.wantMissing && len(result.Missing) == 0 {
				t.Error("无富途采集记录时应给出缺失列")
			}
			t.Logf("%s → symbol=%v futu_symbol=%v name=%v missing=%d notes=%d",
				item.usc, result.Fields["symbol"], result.Fields["futu_symbol"],
				result.Fields["name"], len(result.Missing), len(result.Notes))
		})
	}
}
