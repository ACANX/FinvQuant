package cdeu

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveIndexFavSnapshot 用真实 moomoo OpenAPI 拉取收藏的美股/港股指数快照并落库（默认跳过）。
//
// 需要三样东西：
//
//	FINV_MOOMOO_APP_KEY       控制台创建的 AppKey ID（或已在 sys.sys_config 20016 配置）
//	FINV_MOOMOO_PRIVATE_KEY   Ed25519 PKCS#8 私钥 PEM（可含字面 \n；**不要提交到仓库**）
//	FINV_TEST_PG_DSN          PostgreSQL 连接串
//
// 运行：
//
//	FINV_MOOMOO_LIVE=1 FINV_MOOMOO_APP_KEY=... FINV_MOOMOO_PRIVATE_KEY="$(cat key.pem)" \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveIndexFavSnapshot -v ./internal/meta/finv/quote/cdeu/
//
// 测试会临时收藏 DJI / SPX（原本未收藏时），结束后删除自己加的收藏；
// 快照行留在 cdeu.finv_quote_secu_realtime 里（幂等，可重复执行）。
func TestLiveIndexFavSnapshot(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过 moomoo OpenAPI 真实采集")
	}

	pool := testPool(t)
	store := NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := store.MoomooClientFromConfig(ctx)
	if err != nil {
		t.Fatalf("构造 moomoo 客户端失败: %v", err)
	}
	if client == nil {
		t.Skip("未配置 moomoo 凭据（sys_config 20016 / FINV_MOOMOO_APP_KEY+PRIVATE_KEY）")
	}

	// 临时收藏两只美股指数（港股 HSI/HTI 依赖上线环境已有收藏）。
	targets := []string{"DJI", "SPX"}
	added := make([]string, 0, len(targets))
	for _, usc := range targets {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM quote.finv_quote_secu_fav WHERE usc = $1)`, usc).Scan(&exists); err != nil {
			t.Fatalf("检查收藏状态失败: %v", err)
		}
		if exists {
			continue
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO quote.finv_quote_secu_fav (usc) VALUES ($1) ON CONFLICT (usc) DO NOTHING`, usc); err != nil {
			t.Fatalf("临时收藏 %s 失败: %v", usc, err)
		}
		added = append(added, usc)
	}
	t.Cleanup(func() {
		for _, usc := range added {
			if _, err := pool.Exec(context.Background(),
				`DELETE FROM quote.finv_quote_secu_fav WHERE usc = $1`, usc); err != nil {
				t.Logf("清理临时收藏 %s 失败: %v", usc, err)
			}
		}
	})

	collector := NewIndexFavSnapshotCollector(client, store, nil)
	result, err := collector.Collect(ctx, IndexFavSnapshotCollectOptions{Timeout: 90 * time.Second})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	t.Logf("收藏指数=%d codes=%d rows=%d 新增=%d 覆盖=%d 批次数=%d 未返回=%v 耗时=%dms",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, result.Batches, result.Missing, result.ElapsedMs)
	if result.CodesRequested == 0 {
		t.Skip("收藏表里没有美股/港股指数，跳过断言")
	}
	if result.RowsReceived == 0 {
		t.Fatalf("接口未返回任何指数快照（codes=%d）", result.CodesRequested)
	}

	// 落库校验：HSI（港股，HK.800000）与 DJI（美股，US..DJI）必须已写入且字段口径正确。
	for _, usc := range []string{"HSI", "DJI"} {
		var (
			marketType, marketLabel, instrumentType, provider string
			priceNominal                                      float64
			dtUpdate                                          *time.Time
		)
		err := pool.QueryRow(ctx, `
SELECT market_type, market_label, instrument_type, provider, price_nominal, gmt_update
  FROM cdeu.finv_quote_secu_realtime
 WHERE usc = $1`, usc).Scan(&marketType, &marketLabel, &instrumentType, &provider, &priceNominal, &dtUpdate)
		if err != nil {
			t.Fatalf("查询落库行 %s 失败: %v", usc, err)
		}
		wantMarket := map[string]string{"HSI": "1", "DJI": "2"}[usc]
		wantLabel := map[string]string{"HSI": "HK", "DJI": "US"}[usc]
		if marketType != wantMarket || marketLabel != wantLabel {
			t.Errorf("%s: market_type/label=%s/%s want %s/%s", usc, marketType, marketLabel, wantMarket, wantLabel)
		}
		if instrumentType != "6" {
			t.Errorf("%s: instrument_type=%s want 6（指数）", usc, instrumentType)
		}
		if provider != IndexFavProvider {
			t.Errorf("%s: provider=%s want %s", usc, provider, IndexFavProvider)
		}
		if priceNominal <= 0 {
			t.Errorf("%s: price_nominal=%v want > 0", usc, priceNominal)
		}
		t.Logf("%s 落库：market=%s/%s instrument_type=%s provider=%s price=%v",
			usc, marketType, marketLabel, instrumentType, provider, priceNominal)
	}
}
