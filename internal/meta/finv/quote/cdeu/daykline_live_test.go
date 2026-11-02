package cdeu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/quote/moomoo"
)

// TestLiveGCMainDayKline 真实拉取 GCMain 日 K 并 upsert（需要可访问 www.moomoo.com 的网络）。
//
// 默认跳过：本机（宿主与容器）到 moomoo.com 的 TLS 握手被重置，无法在开发环境验证。
// 在可访问环境执行：
//
//	FINV_MOOMOO_LIVE=1 FINV_TEST_PG_DSN=... go test -run TestLiveGCMainDayKline -v ./internal/meta/finv/quote/cdeu/
//
// 该测试会真实写入 cdeu.finv_quote_secu_kline_day（GCMain 的日 K），不做清理：
// 它本身就是「同步一次真实数据」的最小可用入口。
func TestLiveGCMainDayKline(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过真实接口测试")
	}
	pool := testPool(t)
	service := &Service{
		store:        NewStore(pool),
		klineFetcher: moomoo.New(moomoo.Config{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := service.SyncDayKlineFromMoomoo(ctx, DayKlineSyncRequest{USC: "GCMain"})
	if err != nil {
		t.Fatalf("真实拉取 GCMain 日K失败: %v", err)
	}
	t.Logf("GCMain 日K同步：总数=%d 区间内=%d 新增=%d 更新=%d 日期=%d~%d 时区=%s",
		result.Total, result.InRange, result.Inserted, result.Updated,
		result.DateFrom, result.DateTo, result.Timezone)

	if result.Total < 1000 {
		t.Fatalf("GCMain 全量历史应有上千根日K，实际 %d", result.Total)
	}
	if result.DateFrom < 20000101 || result.DateTo < 20200101 {
		t.Fatalf("日期范围不合理：%+v", result)
	}
	stats, err := service.store.DayKlineStats(ctx, "GCMain")
	if err != nil {
		t.Fatalf("查询存量统计失败: %v", err)
	}
	if stats.Rows == 0 || stats.Providers != DayKlineProviderMoomoo {
		t.Fatalf("落库统计不符：%+v", stats)
	}
}
