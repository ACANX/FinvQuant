package cdeu

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件是「缺省汇率」在 Service 层的薄封装（需求 10999）：
// 手动触发一次 moomoo get-fx-overview → 组装直采 + CNY 派生行 → 全量 upsert 到
// public.finv_fxcer，并提供整表查询给前端展示。
// 采集口径在 fx_cer_sync.go，写入与查询在 store_fx_cer.go。

// NewFxCERFetcher 创建汇率拉取实现（上游默认值即返回中文站口径，无需自定义头）。
//
// 同一个 *quoteapiv2.Client 同时实现 GetFxOverview 与 GetFxRateList，
// 因此两个采集来源共用一个实例。
func NewFxCERFetcher() *ClientBundle {
	return &ClientBundle{client: quoteapiv2.New(quoteapiv2.Config{})}
}

// ClientBundle 把上游客户端包一层，便于单测按需替换（同时实现两个拉取接口）。
//
// 之所以不直接返回 *quoteapiv2.Client：SetFxCERFetcher / SetFxCERRateListFetcher
// 分别注入时，单测可能只关心其中一个来源。
type ClientBundle struct {
	client *quoteapiv2.Client
}

// GetFxOverview 实现 FxCERFetcher。
func (b *ClientBundle) GetFxOverview(ctx context.Context, req quoteapiv2.GetFxOverviewRequest) (map[string]quoteapiv2.FxOverviewQuote, error) {
	return b.client.GetFxOverview(ctx, req)
}

// GetFxRateList 实现 FxCERRateListFetcher。
func (b *ClientBundle) GetFxRateList(ctx context.Context, req quoteapiv2.GetFxRateListRequest) (quoteapiv2.FxRateListPage, error) {
	return b.client.GetFxRateList(ctx, req)
}

// SetFxCERFetcher 替换汇率概览拉取实现（单测注入用）；传 nil 恢复默认 moomoo 客户端。
func (s *Service) SetFxCERFetcher(fetcher FxCERFetcher) {
	if fetcher == nil {
		fetcher = NewFxCERFetcher()
	}
	s.fxCerFetcher = fetcher
}

// SetFxCERRateListFetcher 替换汇率列表拉取实现（单测注入用）；传 nil 恢复默认 moomoo 客户端。
func (s *Service) SetFxCERRateListFetcher(fetcher FxCERRateListFetcher) {
	if fetcher == nil {
		fetcher = NewFxCERFetcher()
	}
	s.fxCerRateListFetcher = fetcher
}

// SyncFxCER 手动更新一次缺省汇率（同步执行，一次请求内完成）。
//
// source 指定采集来源（需求 11001）：
//   - FxCERSourceOverview（默认）：get-fx-overview，9 币种两两完整矩阵，矩阵模式使用；
//   - FxCERSourceRateList：get-fx-rate-list（listId=1、page=0、pageSize=500），
//     币种更多、可补概览缺失的货币对，表格模式使用。
//
// 两个来源写同一张表，互不删除对方数据：同货币对按「新 dt_quote 覆盖旧」更新，
// 因此可以先用概览铺满矩阵、再用列表补充长尾币种。
// 返回的 statistics 只统计**本次实际写入的行**：因「旧报价不覆盖新报价、派生不覆盖直采」
// 被跳过的行不计入 insert/update（见 Store.UpsertFxCER）。
func (s *Service) SyncFxCER(ctx context.Context, source string) (FxCERSyncResult, error) {
	switch strings.TrimSpace(source) {
	case "", FxCERSourceOverview:
		return s.syncFxCERFromOverview(ctx)
	case FxCERSourceRateList:
		return s.syncFxCERFromRateList(ctx)
	default:
		return FxCERSyncResult{}, fmt.Errorf("%w: 未知的采集来源 %q（仅支持 overview / rateList）", ErrFxCERInvalid, source)
	}
}

// syncFxCERFromOverview 用 get-fx-overview 更新（9 币种两两矩阵 + CNY 派生）。
func (s *Service) syncFxCERFromOverview(ctx context.Context) (FxCERSyncResult, error) {
	started := time.Now()
	result := FxCERSyncResult{Source: FxCERSourceOverview, Pages: 1}

	quotes, err := s.fxCerFetcher.GetFxOverview(ctx, quoteapiv2.GetFxOverviewRequest{
		// 带 `_` 毫秒时间戳，避开服务端/中间层缓存；它参与 quote-token 签名。
		TimestampMillis: strconv.FormatInt(time.Now().UnixMilli(), 10),
	})
	if err != nil {
		return result, fmt.Errorf("拉取汇率概览失败: %w", err)
	}

	rows, err := BuildFxCERRows(quotes)
	if err != nil {
		return result, err
	}
	result.RowsReceive = len(rows)
	for _, row := range rows {
		if row.FlagDerive == "1" {
			result.Derive++
		}
		if row.TsQuote > result.TsQuoteMax {
			result.TsQuoteMax = row.TsQuote
		}
	}

	stat, err := s.store.UpsertFxCER(ctx, rows)
	if err != nil {
		return result, err
	}
	result.Insert, result.Update = stat.Insert, stat.Update
	result.ElapsedMs = time.Since(started).Milliseconds()
	log.Printf("[cdeu][fxcer] 更新完成 source=%s pages=%d rows=%d skipped=%d derive=%d insert=%d update=%d ts_quote_max=%d elapsed_ms=%d",
		result.Source, result.Pages, result.RowsReceive, result.Skipped, result.Derive,
		result.Insert, result.Update, result.TsQuoteMax, result.ElapsedMs)
	return result, nil
}

// fetchFxRateList 拉取全部汇率列表条目：先试 pageSize=500 单页，失败降级为分页。
//
// 返回条目、实际请求页数；两个路径都不对条目去重以外的内容做加工。
func (s *Service) fetchFxRateList(ctx context.Context) ([]quoteapiv2.FxRateListItem, int, error) {
	first, err := s.fxCerRateListFetcher.GetFxRateList(ctx, quoteapiv2.GetFxRateListRequest{
		ListID:   quoteapiv2.DefaultFxListID,
		Page:     "0",
		PageSize: "500",
	})
	if err == nil {
		return fxRateListUnique(first.Items), 1, nil
	}
	log.Printf("[cdeu][fxcer] pageSize=500 拉取失败，降级为 pageSize=%d 分页：%v",
		fxRateListFallbackPageSize, err)

	collected := make([]quoteapiv2.FxRateListItem, 0, 320)
	pages := 0
	for page := 0; page < fxRateListFallbackMaxPages; page++ {
		if page > 0 {
			if sleepErr := fxCerSleep(ctx, fxRateListFallbackInterval); sleepErr != nil {
				return nil, pages, sleepErr
			}
		}
		pageData, pageErr := s.fxCerRateListFetcher.GetFxRateList(ctx, quoteapiv2.GetFxRateListRequest{
			ListID:   quoteapiv2.DefaultFxListID,
			Page:     strconv.Itoa(page),
			PageSize: strconv.Itoa(fxRateListFallbackPageSize),
		})
		if pageErr != nil {
			if page == 0 {
				return nil, pages, pageErr
			}
			// 已拿到部分数据：记录并返回已完成部分，避免整轮失败。
			log.Printf("[cdeu][fxcer] 第 %d 页拉取失败，使用已获取的 %d 条：%v", page, len(collected), pageErr)
			break
		}
		pages++
		collected = append(collected, pageData.Items...)
		if fxRateListPageEnded(len(pageData.Items), fxRateListFallbackPageSize) {
			break
		}
	}
	if len(collected) == 0 {
		return nil, pages, fmt.Errorf("降级分页未取到任何条目")
	}
	return fxRateListUnique(collected), pages, nil
}

// fxRateListUnique 按货币对代码去重（保留首次出现），避免分页边界重复写入同一行。
func fxRateListUnique(items []quoteapiv2.FxRateListItem) []quoteapiv2.FxRateListItem {
	seen := make(map[string]bool, len(items))
	unique := make([]quoteapiv2.FxRateListItem, 0, len(items))
	for _, item := range items {
		code := strings.ToUpper(strings.TrimSpace(item.StockCode))
		if code != "" && seen[code] {
			continue
		}
		seen[code] = true
		unique = append(unique, item)
	}
	return unique
}

// fxRateListPageEnded 判断本页是否已是最后一页（本页条目数不足一页即结束）。
func fxRateListPageEnded(itemsOnPage, pageSize int) bool {
	return itemsOnPage < pageSize
}

// fxCerSleep 可被 ctx 取消的等待。
func fxCerSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// 汇率列表分页兜底参数（大 pageSize 被上游拒绝时使用）。
const (
	// fxRateListFallbackPageSize 兜底分页大小：上游对 pageSize=300/500 会返回 code=500
	//（"Cannot read properties of undefined (reading 'spreadTableCode')"，且 data 为数组导致
	// MetaExt v0.1.2 解析阶段就报错），实测 50 稳定可用。
	fxRateListFallbackPageSize = 50
	// fxRateListFallbackMaxPages 兜底最多翻页数（50 × 20 = 1000 条，当前 total=282 足够）。
	fxRateListFallbackMaxPages = 20
	// fxRateListFallbackInterval 兜底翻页的页间间隔（避免触发限流）。
	fxRateListFallbackInterval = 300 * time.Millisecond
)

// syncFxCERFromRateList 用 get-fx-rate-list 更新（listId=1 全部汇率，币种更多）。
//
// 该接口不返回报价时间，dt_quote 取本次采集时刻；`_` 时间戳省略（实测非必需）。
//
// 采集策略：先按需求用 pageSize=500 一次取全；若上游拒绝较大 pageSize（返回 code=500 或
// data 为数组导致解析失败），自动降级为 pageSize=50 逐页拉取，保证功能可用。
func (s *Service) syncFxCERFromRateList(ctx context.Context) (FxCERSyncResult, error) {
	started := time.Now()
	now := started.UnixMilli()
	result := FxCERSyncResult{Source: FxCERSourceRateList}

	items, pages, err := s.fetchFxRateList(ctx)
	if err != nil {
		return result, fmt.Errorf("拉取汇率列表失败: %w", err)
	}
	result.Pages = pages
	rows, skipped, err := BuildFxCERRowsFromRateList(items, now)
	if err != nil {
		return result, err
	}
	result.Skipped = skipped
	result.RowsReceive = len(rows)
	for _, row := range rows {
		if row.FlagDerive == "1" {
			result.Derive++
		}
		if row.TsQuote > result.TsQuoteMax {
			result.TsQuoteMax = row.TsQuote
		}
	}

	stat, err := s.store.UpsertFxCER(ctx, rows)
	if err != nil {
		return result, err
	}
	result.Insert, result.Update = stat.Insert, stat.Update
	result.ElapsedMs = time.Since(started).Milliseconds()
	log.Printf("[cdeu][fxcer] 更新完成 source=%s rows=%d derive=%d insert=%d update=%d ts_quote_max=%d elapsed_ms=%d",
		result.Source, result.RowsReceive, result.Derive, result.Insert, result.Update,
		result.TsQuoteMax, result.ElapsedMs)
	return result, nil
}

// FxCERList 读取全部缺省汇率（88 行量级，不分页），供前端表格展示。
func (s *Service) FxCERList(ctx context.Context) ([]FxCERRow, error) {
	return s.store.LoadFxCER(ctx)
}
