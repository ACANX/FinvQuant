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

// SetFutureListFetcher 替换期货列表拉取实现（单测注入用）；传 nil 恢复默认 moomoo 客户端。
func (s *Service) SetFutureListFetcher(fetcher FutureListFetcher) {
	if fetcher == nil {
		fetcher = NewFutureListFetcher()
	}
	s.futureListFetcher = fetcher
}

// CollectFuturesList 按配置采集期货列表：读配置 → 分页拉取 → 逐页落库 → 回写统计（需求 11274）。
//
// 采集范围来自 cdeu.finv_quote_cdeu_future_sync_config，结果写 cdeu.finv_quote_realtime_future。
func (s *Service) CollectFuturesList(ctx context.Context, opts FutureListCollectOptions) ([]FutureListCollectResult, error) {
	collector := NewFutureListCollector(s.store, s.futureListFetcher, s.store, log.Default())
	return collector.Collect(ctx, opts)
}
