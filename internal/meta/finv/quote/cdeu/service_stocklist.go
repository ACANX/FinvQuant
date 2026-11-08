package cdeu

import (
	"context"
	"log"
)

// 本文件是「实时行情列表采集」在 Service 层的薄封装：只做依赖装配，
// 采集流程本身在 quote_list_stock_snapshot_sync.go（单文件、可整体迁往 MetaExt 第三方包）。

// CollectStockList 按配置采集证券列表：读配置 → 分页拉取 → 逐页落库 → 回写统计。
//
// 采集范围来自配置表（cdeu.finv_cdeu_secu_realtime_sync_config），
// 采集控制参数（间隔、重试、超时、页数上限等）由 opts 运行时传入。
func (s *Service) CollectStockList(ctx context.Context, opts StockListCollectOptions) ([]StockListCollectResult, error) {
	collector := NewStockListCollector(s.store, s.stockListFetcher, s.store, log.Default())
	return collector.Collect(ctx, opts)
}

// SetStockListFetcher 替换列表拉取实现（单测注入用）；传 nil 恢复默认 moomoo 客户端。
func (s *Service) SetStockListFetcher(fetcher StockListFetcher) {
	if fetcher == nil {
		fetcher = NewStockListFetcher()
	}
	s.stockListFetcher = fetcher
}
