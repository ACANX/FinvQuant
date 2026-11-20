package cdeu

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveSecuIndicatorCountUpdate 验证「更新次数」随写入自增（需求 11338，默认跳过）。
//
// 需要：
//
//	FINV_TEST_PG_DSN  PostgreSQL 连接串
//
// 运行：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveSecuIndicatorCountUpdate -v ./internal/meta/finv/quote/cdeu/
//
// 只发最小请求（symbol + 空 data），因此除 count_update / dt_update 外不会改动任何业务列；
// 测试结束把 count_update 还原成测试前的值。
func TestLiveSecuIndicatorCountUpdate(t *testing.T) {
	if os.Getenv("FINV_TEST_PG_DSN") == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	pool := testPool(t)
	store := NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 取一只已有指标行（优先 A 股，便于人工核对）。
	var usc, ftmm string
	if err := pool.QueryRow(ctx, `
SELECT usc, ftmm_symbol
  FROM cdeu.finv_quote_secu_indicator
 WHERE ftmm_symbol LIKE 'SH.%'
 ORDER BY usc LIMIT 1`).Scan(&usc, &ftmm); err != nil {
		t.Fatalf("取样本指标行失败: %v", err)
	}

	var before int
	var beforeUpdate time.Time
	if err := pool.QueryRow(ctx, `
SELECT COALESCE(count_update, 0), dt_update
  FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, usc).Scan(&before, &beforeUpdate); err != nil {
		t.Fatalf("读取更新次数失败: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(),
			`UPDATE cdeu.finv_quote_secu_indicator SET count_update = $2 WHERE usc = $1`,
			usc, before); err != nil {
			t.Logf("还原 %s 的 count_update 失败: %v", usc, err)
		}
	})
	t.Logf("样本 %s（%s）测试前 count_update=%d dt_update=%s", usc, ftmm, before, beforeUpdate.Format(time.RFC3339))

	// 连续写两次（每次都是一次真实更新）。
	for round := 1; round <= 2; round++ {
		req := &SecuIndicatorUpsertRequest{Data: &SecuIndicatorPayload{Symbol: ftmm}}
		result, err := store.UpsertSecuIndicator(ctx, req)
		if err != nil {
			t.Fatalf("第 %d 次写入失败: %v", round, err)
		}
		if result.Indicator != "UPDATED" {
			t.Fatalf("第 %d 次写入应为 UPDATED，实际 %s", round, result.Indicator)
		}
		var now int
		if err := pool.QueryRow(ctx,
			`SELECT count_update FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`,
			usc).Scan(&now); err != nil {
			t.Fatalf("读取第 %d 次写入后的 count_update 失败: %v", round, err)
		}
		want := before + round
		if now != want {
			t.Fatalf("第 %d 次写入后 count_update=%d，期望 %d（每次更新必须 +1）", round, now, want)
		}
		t.Logf("第 %d 次写入后 count_update=%d", round, now)
	}

	// dt_update 必须比测试前更晚（与计数同步前进）。
	var afterUpdate time.Time
	if err := pool.QueryRow(ctx,
		`SELECT dt_update FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, usc).Scan(&afterUpdate); err != nil {
		t.Fatalf("读取 dt_update 失败: %v", err)
	}
	if !afterUpdate.After(beforeUpdate) {
		t.Errorf("dt_update 未前进：before=%s after=%s",
			beforeUpdate.Format(time.RFC3339), afterUpdate.Format(time.RFC3339))
	}
	if strings.TrimSpace(usc) == "" {
		t.Error("样本 usc 为空")
	}
}

// TestLiveSecuIndicatorCountUpdateInsert 新行的更新次数从 1 起步，随后自增（需求 11338）。
//
// 用不存在的测试符号 ZZTEST11338 插入一行，验证 INSERT 的 count_update 默认值；
// 结束时删除该行（collect / key 表不会产生新行：请求没给 stock_id，collect 走 SKIPPED，key 表只 UPDATE）。
func TestLiveSecuIndicatorCountUpdateInsert(t *testing.T) {
	if os.Getenv("FINV_TEST_PG_DSN") == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	pool := testPool(t)
	store := NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const testSymbol = "ZZTEST11338"
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(),
			`DELETE FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, testSymbol); err != nil {
			t.Logf("清理测试行失败: %v", err)
		}
	})
	// 先清掉可能的残留，保证第一次写入走 INSERT。
	if _, err := pool.Exec(ctx,
		`DELETE FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, testSymbol); err != nil {
		t.Fatalf("清理残留失败: %v", err)
	}

	insertResult, err := store.UpsertSecuIndicator(ctx, &SecuIndicatorUpsertRequest{
		Data: &SecuIndicatorPayload{Symbol: testSymbol},
	})
	if err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	if insertResult.Indicator != "INSERTED" {
		t.Fatalf("首次写入应为 INSERTED，实际 %s", insertResult.Indicator)
	}
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count_update FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, testSymbol).Scan(&count); err != nil {
		t.Fatalf("读取新行 count_update 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("新行 count_update=%d want 1", count)
	}

	updateResult, err := store.UpsertSecuIndicator(ctx, &SecuIndicatorUpsertRequest{
		Data: &SecuIndicatorPayload{Symbol: testSymbol},
	})
	if err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}
	if updateResult.Indicator != "UPDATED" {
		t.Fatalf("第二次写入应为 UPDATED，实际 %s", updateResult.Indicator)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count_update FROM cdeu.finv_quote_secu_indicator WHERE usc = $1`, testSymbol).Scan(&count); err != nil {
		t.Fatalf("读取 count_update 失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("更新一次后 count_update=%d want 2", count)
	}
	t.Logf("新行写入 → count_update=1，再更新一次 → %d", count)
}
