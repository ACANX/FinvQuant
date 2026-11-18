package cdeu

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveRealtimeFutureAndCryptoPage 期货/加密货币行情排行真实库验证（需求 11322 / 11323，默认跳过）。
//
// 需要可读 PG：
//
//	FINV_RANK_LIVE=1 \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveRealtimeFutureAndCryptoPage -v ./internal/meta/finv/quote/cdeu/
func TestLiveRealtimeFutureAndCryptoPage(t *testing.T) {
	if os.Getenv("FINV_RANK_LIVE") != "1" {
		t.Skip("未设置 FINV_RANK_LIVE=1，跳过行情排行真实库测试")
	}
	pool := testPool(t)
	store := NewStore(pool)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	futureTotal, futures, err := store.ListRealtimeFutures(ctx, RealtimeFutureListFilter{Limit: 5})
	if err != nil {
		t.Fatalf("期货排行查询失败: %v", err)
	}
	if futureTotal == 0 || len(futures) == 0 {
		t.Fatalf("期货排行无数据（total=%d）", futureTotal)
	}
	t.Logf("期货排行：total=%d 返回 %d 行；首行 stock_id=%s contract=%s name=%s 分类=%s/%s 现价=%s 涨跌幅=%s fields_json=%d 字节",
		futureTotal, len(futures), futures[0].StockID, futures[0].ContractCode, futures[0].Name,
		futures[0].FuturesType, futures[0].FuturesTypeName, futures[0].PriceNominal,
		futures[0].ChangeRatio, len(futures[0].FieldsJSON))

	types, err := store.ListRealtimeFutureTypes(ctx)
	if err != nil {
		t.Fatalf("期货分类下拉失败: %v", err)
	}
	for _, opt := range types {
		if opt.Label == "" {
			t.Fatalf("期货分类 %s 缺中文名", opt.Value)
		}
	}
	t.Logf("期货分类下拉：%d 项（%s=%s … %s=%s）", len(types),
		types[0].Value, types[0].Label, types[len(types)-1].Value, types[len(types)-1].Label)

	// 按分类过滤必须只返回该分类
	filteredTotal, filtered, err := store.ListRealtimeFutures(ctx,
		RealtimeFutureListFilter{FuturesType: "7", Limit: 50})
	if err != nil {
		t.Fatalf("期货按分类查询失败: %v", err)
	}
	for _, row := range filtered {
		if row.FuturesType != "7" {
			t.Fatalf("按 futures_type=7 过滤却返回 %s（%s）", row.FuturesType, row.ContractCode)
		}
	}
	t.Logf("期货分类过滤 futures_type=7（加密货币期货）：total=%d", filteredTotal)

	cryptoTotal, cryptos, err := store.ListRealtimeCryptos(ctx, RealtimeCryptoListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("加密排行查询失败: %v", err)
	}
	if cryptoTotal == 0 || len(cryptos) == 0 {
		t.Fatalf("加密排行无数据（total=%d）", cryptoTotal)
	}
	t.Logf("加密排行：total=%d 返回 %d 行；首行 usc=%s code=%s name=%s 现价=%s 涨跌幅=%s 涨跌额=%s provider=%s",
		cryptoTotal, len(cryptos), cryptos[0].USC, cryptos[0].Code, cryptos[0].Name,
		cryptos[0].PriceNominal, cryptos[0].ChangeRatio, cryptos[0].ChangePrice, cryptos[0].Provider)

	currencies, err := store.ListRealtimeCryptoQuoteCurrencies(ctx)
	if err != nil {
		t.Fatalf("加密计价币种下拉失败: %v", err)
	}
	t.Logf("加密计价币种：%v", currencies)

	if _, _, err := store.ListRealtimeCryptos(ctx, RealtimeCryptoListFilter{Suspension: "9"}); err == nil {
		t.Fatal("suspension=9 应报参数非法")
	}
}
