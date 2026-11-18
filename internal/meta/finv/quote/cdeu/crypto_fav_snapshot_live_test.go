package cdeu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// TestLiveCryptoFavSnapshot 用真实 moomoo OpenAPI 拉取收藏的加密货币快照并落库（默认跳过）。
//
// 需要三样东西：
//
//	FINV_MOOMOO_APP_KEY       控制台创建的 AppKey ID
//	FINV_MOOMOO_PRIVATE_KEY   Ed25519 PKCS#8 私钥 PEM（可含字面 \n；**不要提交到仓库**）
//	FINV_TEST_PG_DSN          PostgreSQL 连接串
//
// 运行：
//
//	FINV_MOOMOO_LIVE=1 FINV_MOOMOO_APP_KEY=... FINV_MOOMOO_PRIVATE_KEY="$(cat key.pem)" \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveCryptoFavSnapshot -v ./internal/meta/finv/quote/cdeu/
//
// 测试会临时收藏 BTCUSD / ETHUSD（原本未收藏时），结束后删除自己加的收藏；
// 快照行留在 cdeu.finv_quote_secu_realtime_crypto 里（幂等，可重复执行）。
func TestLiveCryptoFavSnapshot(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过 moomoo OpenAPI 真实采集")
	}
	client, err := webapi.NewClientFromEnv()
	if err != nil {
		t.Fatalf("构造 moomoo 客户端失败: %v", err)
	}
	if client == nil {
		t.Skip("未配置 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY")
	}

	pool := testPool(t)
	store := NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 临时收藏两只加密货币；记录原本是否已收藏，结束时只删自己加的。
	targets := []string{"BTCUSD", "ETHUSD"}
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
	defer func() {
		for _, usc := range added {
			_, _ = pool.Exec(context.Background(),
				`DELETE FROM quote.finv_quote_secu_fav WHERE usc = $1`, usc)
		}
	}()

	collector := NewCryptoFavSnapshotCollector(client, store, nil)
	result, err := collector.Collect(ctx, CryptoFavSnapshotCollectOptions{})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if result.CodesRequested == 0 {
		t.Fatalf("收藏表里没有加密货币（应至少包含刚加入的 BTCUSD/ETHUSD）")
	}
	if result.RowsReceived == 0 || result.RowsInsert+result.RowsUpdate == 0 {
		t.Fatalf("未写入任何行: %+v", result)
	}

	var (
		price  *string
		code   string
		status string
	)
	if err := pool.QueryRow(ctx, `
SELECT price_nominal::text, code, provider
  FROM cdeu.finv_quote_secu_realtime_crypto WHERE usc = 'BTCUSD'`).Scan(&price, &code, &status); err != nil {
		t.Fatalf("查询 BTCUSD 落库结果失败: %v", err)
	}
	if price == nil || *price == "" || *price == "0" {
		t.Fatalf("BTCUSD 的 price_nominal 异常: %v", price)
	}
	if code != "CC.BTCUSD" || status != CryptoProvider {
		t.Fatalf("BTCUSD 落库的 code/provider 异常: code=%s provider=%s", code, status)
	}
	t.Logf("验证通过：收藏加密货币=%d codes=%d rows=%d 新增=%d 覆盖=%d BTCUSD价格=%s",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, *price)
}
