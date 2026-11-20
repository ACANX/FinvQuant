package cdeu

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLiveSecuParamPrefillBySnapshot 预填/补全以快照行为准（需求 11303，默认跳过）。
//
// 回归场景：`cdeu.finv_quote_secu_realtime` 里有一部分证券（FTMM 的部分港股、
// BDFS、沪深京指数）在键表 `futu.finv_futu_secu_key` 中没有对应行（或 stock_id 为空）。
// 列表取自快照表，点「预填」却按键表查 stock_id，会误报
// `实时行情快照入参非法: stock_id=... 不存在`。
//
// 需要可写 PG：
//
//	FINV_PREFILL_LIVE=1 \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveSecuParamPrefillBySnapshot -v ./internal/meta/finv/quote/cdeu/
func TestLiveSecuParamPrefillBySnapshot(t *testing.T) {
	if os.Getenv("FINV_PREFILL_LIVE") != "1" {
		t.Skip("未设置 FINV_PREFILL_LIVE=1，跳过预填真实库测试")
	}
	pool := testPool(t)
	store := NewStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 取「键表既无同 stock_id、也无同 ftmm_symbol」的样本：这类行此前预填必然 400。
	rows, err := pool.Query(ctx, `
SELECT r.stock_id
  FROM cdeu.finv_quote_secu_realtime r
  LEFT JOIN futu.finv_futu_secu_key k1 ON k1.stock_id::text = r.stock_id
  LEFT JOIN futu.finv_futu_secu_key k2
         ON k2.ftmm_symbol = upper(btrim(r.market_label) || '.' || btrim(r.stock_code))
 WHERE r.stock_id ~ '^[0-9]+$'
   AND k1.stock_id IS NULL
   AND k2.ftmm_symbol IS NULL
 ORDER BY r.stock_id DESC
 LIMIT 20`)
	if err != nil {
		t.Fatalf("取样本失败: %v", err)
	}
	var ids []int64
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("扫描 stock_id 失败: %v", err)
		}
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		t.Skip("库里没有被键表遗漏的快照行，跳过")
	}

	for _, id := range ids {
		prefill, err := store.PendingSecuPrefill(ctx, id)
		if err != nil {
			t.Fatalf("stock_id=%d 预填失败: %v", id, err)
		}
		if strings.TrimSpace(prefill.USC) == "" || prefill.USC == "." {
			t.Fatalf("stock_id=%d 预填未得到 usc: %+v", id, prefill)
		}
		if strings.TrimSpace(prefill.Name) == "" {
			t.Fatalf("stock_id=%d 预填未得到名称: %+v", id, prefill)
		}
		if _, err := store.SecuParamPrefill(ctx, SecuParamTargetSecurity, id); err != nil {
			t.Fatalf("stock_id=%d 单表预填（security）失败: %v", id, err)
		}
		if _, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, id); err != nil {
			t.Fatalf("stock_id=%d 单表预填（collect）失败: %v", id, err)
		}
		// 「入队」前的只读派生：键表没有该行时也必须能从快照行拿到 market / code。
		draft, err := store.derivePendingSecuKeyDraft(ctx, id)
		if err != nil {
			t.Fatalf("stock_id=%d 入队派生失败: %v", id, err)
		}
		if strings.TrimSpace(draft.Market) == "" || strings.TrimSpace(draft.Code) == "" {
			t.Fatalf("stock_id=%d 入队派生缺市场/代码: %+v", id, draft)
		}
	}

	// 具体样本：02778 冠君产业信托（键表无此键，此前点预填必报「stock_id 不存在」）。
	var hkID int64
	if err := pool.QueryRow(ctx, `
SELECT stock_id::bigint FROM cdeu.finv_quote_secu_realtime
 WHERE usc = '02778' AND stock_id ~ '^[0-9]+$'`).Scan(&hkID); err == nil {
		prefill, err := store.PendingSecuPrefill(ctx, hkID)
		if err != nil {
			t.Fatalf("02778 预填失败: %v", err)
		}
		if prefill.USC != "02778" {
			t.Fatalf("02778 预填 usc 应为 02778，实际 %q", prefill.USC)
		}
	}
	t.Logf("验证通过：%d 条「键表遗漏」快照行均可预填", len(ids))
}
