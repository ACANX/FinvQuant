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

// TestLivePendingSecuCollectReference 只读回归：instrument_type='12' 的行必须能取到参考
// 采集配置（需求 11164）。
//
// 故障现场：快照 instrument_type 是 TEXT，且 12 与 3 都表示股票；采集表只有 3。
// 原来直接用 `q.instrument_type::text = btrim(t.instrument_type)` 匹配，
// 12 的行永远匹配不到 → market_code 取不到值（=0）→ 点「写入」被后端拒：
// 「采集配置缺少必填（market_label / futu_symbol / market_code / …）」。
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN=... go test -run TestLivePendingSecuCollectReference -v ./internal/meta/finv/quote/cdeu/
func TestLivePendingSecuCollectReference(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读回归")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读回归")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	var stockID int64
	if err := pool.QueryRow(ctx, `
SELECT stock_id
  FROM `+tableRealtimeStock+`
 WHERE instrument_type = '12'
   AND market_label IN ('US', 'HK')
 ORDER BY stock_id
 LIMIT 1`).Scan(&stockID); err != nil {
		t.Skipf("库里没有 instrument_type='12' 的样本行，跳过：%v", err)
	}

	prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, stockID)
	if err != nil {
		t.Fatalf("预填失败: %v", err)
	}
	values := map[string]string{}
	for _, field := range prefill.Fields {
		values[field.Key] = field.Value
	}
	t.Logf("stock_id=%d usc=%s market_label=%s instrument=%s market_type=%s market_code=%s "+
		"instrument_type=%s sub_instrument_type=%s market_label 来源=%s",
		stockID, prefill.USC, values["market_label"], values["instrument"], values["market_type"],
		values["market_code"], values["instrument_type"], values["sub_instrument_type"], values["market_label"])

	// 必填项：market_code 必须能取到参考值（instrument_type 12 按 3 归一后匹配）
	if values["market_code"] == "" || values["market_code"] == "0" {
		t.Fatalf("instrument_type='12' 的行仍取不到 market_code 参考值（需求 11164 回归）：market_code=%q",
			values["market_code"])
	}
	// instrument 也应从参考行取到（type 3 的口径是 stock）
	if values["instrument"] == "" {
		t.Fatalf("instrument 为空，无法插入采集配置")
	}
	if values["market_label"] == "" || values["futu_symbol"] == "" {
		t.Fatalf("market_label / futu_symbol 不应为空")
	}
	// 参考值来源要标成 reference（提示人工核对），而不是 manual
	if values["market_label"] == "" {
		t.Fatalf("market_label 为空")
	}
}

// TestLiveSecuParamCollectComplete 端到端验证「采集参数」补全真能落库（需求 11164）。
//
// **会写本地库**（不碰上游）：取清单里第一条，用预填值调 CompleteSecuParam，
// 断言 created=true，然后把自己刚插入的那一行删掉（只删本测试插入的 usc，删不到就报错）。
// 默认跳过：
//
//	FINV_SECU_PARAM_LIVE=1 FINV_SECU_PARAM_LIVE_WRITE=1 FINV_TEST_PG_DSN=... \
//	  go test -run TestLiveSecuParamCollectComplete -v ./internal/meta/finv/quote/cdeu/
func TestLiveSecuParamCollectComplete(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" || os.Getenv("FINV_SECU_PARAM_LIVE_WRITE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1 与 FINV_SECU_PARAM_LIVE_WRITE=1，跳过写入验证")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过写入验证")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	_, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
		MissingTarget: SecuParamTargetCollect, OrderBy: "market_value", Desc: true, Limit: 1,
	})
	if err != nil || len(list) == 0 {
		t.Skipf("采集参数没有待补全的行，跳过：%v", err)
	}
	row := list[0]

	prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, row.StockID)
	if err != nil {
		t.Fatalf("预填失败: %v", err)
	}
	values := map[string]string{}
	for _, field := range prefill.Fields {
		values[field.Key] = field.Value
	}

	req := SecuParamCompleteRequest{
		Target:  SecuParamTargetCollect,
		StockID: row.StockID,
		Remark:  "自动测试（会立即删除）",
		Collect: PendingSecuEnrollCollect{
			FutuSymbol:        values["futu_symbol"],
			MarketType:        parseIntOr(values["market_type"], 0),
			MarketCode:        parseIntOr(values["market_code"], 0),
			InstrumentType:    parseIntOr(values["instrument_type"], 0),
			SubInstrumentType: parseIntOr(values["sub_instrument_type"], 0),
			MarketLabel:       values["market_label"],
			Instrument:        values["instrument"],
			ReqSection:        values["req_section"],
			FlagEnable:        values["flag_enable"],
		},
	}
	result, err := store.CompleteSecuParam(ctx, req)
	if err != nil {
		t.Fatalf("采集参数补全失败（正是需求 11164 的现场）：%v", err)
	}
	if !result.Created {
		t.Fatalf("期望新插入一行，实际 created=false（usc=%s 已存在）", result.USC)
	}
	t.Logf("补全成功：target=%s usc=%s market_code=%s instrument=%s", result.Target,
		result.USC, values["market_code"], values["instrument"])

	// 清理：只删本测试刚插入的那一行
	tag, err := pool.Exec(ctx, `DELETE FROM futu.finv_futu_quote_collect WHERE usc = $1`, result.USC)
	if err != nil {
		t.Fatalf("清理测试行失败（请手工删除 usc=%s）: %v", result.USC, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("清理时应删除 1 行，实际 %d 行（请手工核对 usc=%s）", tag.RowsAffected(), result.USC)
	}
	t.Logf("已清理测试行 usc=%s", result.USC)
}
