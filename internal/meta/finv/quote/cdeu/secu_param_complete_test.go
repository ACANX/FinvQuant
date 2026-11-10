package cdeu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSecuParamCompleteTable 目标表白名单（需求 11161）：只接受 security / collect，大小写不敏感。
func TestSecuParamCompleteTable(t *testing.T) {
	cases := []struct {
		target string
		table  string
		ok     bool
	}{
		{"security", "public.finv_security", true},
		{"collect", "futu.finv_futu_quote_collect", true},
		{"SECURITY", "public.finv_security", true},
		{"  Collect  ", "futu.finv_futu_quote_collect", true},
		{"", "", false},
		{"finv_security", "", false},
		{"public.finv_security", "", false},
		{"security; DROP TABLE x", "", false},
	}
	for _, c := range cases {
		table, err := secuParamCompleteTable(c.target)
		if c.ok {
			if err != nil {
				t.Fatalf("target=%q 应通过，实际报错 %v", c.target, err)
			}
			if table != c.table {
				t.Fatalf("target=%q 映射为 %q，期望 %q", c.target, table, c.table)
			}
			continue
		}
		if err == nil {
			t.Fatalf("target=%q 应被拒绝，实际通过并返回 %q", c.target, table)
		}
	}
}

// TestLiveSecuParamComplete 只读核对「证券参数补全」清单与预填（需求 11161）。
//
// 默认跳过：
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveSecuParamComplete -v ./internal/meta/finv/quote/cdeu/
//
// 断言：两张表的缺口清单都能查出来；第一条的预填 `exists` 必须为 false（它正是清单里「缺」的那条）。
func TestLiveSecuParamComplete(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读核对")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读核对")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	for _, target := range []string{SecuParamTargetSecurity, SecuParamTargetCollect} {
		total, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
			MissingTarget: target, OrderBy: "market_value", Desc: true, Limit: 5,
		})
		if err != nil {
			t.Fatalf("target=%s 查询清单失败: %v", target, err)
		}
		t.Logf("target=%s 待补全 %d 行，前 %d 行样例", target, total, len(list))
		if len(list) == 0 {
			t.Logf("target=%s 没有待补全的行（跳过预填核对）", target)
			continue
		}
		first := list[0]
		prefill, err := store.SecuParamPrefill(ctx, target, first.StockID)
		if err != nil {
			t.Fatalf("target=%s stock_id=%d 预填失败: %v", target, first.StockID, err)
		}
		if prefill.Exists {
			t.Fatalf("清单里的行不应在目标表已存在：target=%s stock_id=%d usc=%s",
				target, first.StockID, prefill.USC)
		}
		if prefill.Target != target {
			t.Fatalf("预填 target=%q，期望 %q", prefill.Target, target)
		}
		if len(prefill.Fields) == 0 {
			t.Fatalf("target=%s 预填字段为空", target)
		}
		t.Logf("target=%s 样例 stock_id=%d usc=%s 名称=%s 字段数=%d 警告数=%d",
			target, first.StockID, prefill.USC, prefill.Name, len(prefill.Fields), len(prefill.Warnings))
		for _, field := range prefill.Fields {
			t.Logf("  字段 %-22s = %-24q 来源=%s 必填=%t", field.Key, field.Value, field.Source, field.Required)
		}
	}

	// 非法 target 必须被拒绝
	if _, err := store.SecuParamPrefill(ctx, "bogus", 1); err == nil {
		t.Fatalf("非法 target 应报错")
	}
}
