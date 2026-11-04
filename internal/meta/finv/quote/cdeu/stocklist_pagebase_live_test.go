package cdeu

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestLiveStockListPageBase 实测 moomoo get-stock-list 的 page 起始值（0 还是 1）。
//
// 默认跳过（同其它 live 测试）。在可访问 moomoo 的环境执行：
//
//	FINV_MOOMOO_LIVE=1 go test -run TestLiveStockListPageBase -v ./internal/meta/finv/quote/cdeu/
//
// 2026-11 实测结论（marketType=1、pageSize=5）：
//   page=0 → 报错「moomoo page 不能小于 1，实际 0」
//   page=1 → HSBC HOLDINGS（pagination.page=1、total=3337）
// 即本接口 page 从 1 开始，采集侧的 StockListDefaultPageStart=1 正确。
//
// 判定方法：分别请求 page=0/1/2，比较首条 stockId。
//   - page=1 与 page=2 首条不同、且 page=0 与 page=1 首条相同：服务端把 0 与 1 当同一页（1 起，0 被容忍）
//   - page=0 与 page=1 首条不同：服务端起算为 0
func TestLiveStockListPageBase(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过真实接口测试")
	}
	client := quoteapiv2.New(quoteapiv2.Config{})

	for _, page := range []string{"0", "1", "2"} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		result, err := client.GetStockList(ctx, quoteapiv2.GetStockListRequest{
			MarketType: "1",
			PlateType:  "1",
			RankType:   "3",
			Page:       page,
			PageSize:   "5",
		})
		cancel()
		if err != nil {
			t.Logf("page=%s 请求失败: %v", page, err)
			continue
		}
		head := ""
		if len(result.Items) > 0 {
			head = fmt.Sprintf("%d %s", result.Items[0].StockID, result.Items[0].Name)
		}
		t.Logf("page=%s 返回 %d 条 | pagination.page=%d pageSize=%d pageCount=%d total=%d | 首条=%s",
			page, len(result.Items), result.Pagination.Page, result.Pagination.PageSize,
			result.Pagination.PageCount, result.Pagination.Total, head)
		if len(result.Items) > 1 {
			t.Logf("  第二条=%d %s", result.Items[1].StockID, result.Items[1].Name)
		}
	}
}
