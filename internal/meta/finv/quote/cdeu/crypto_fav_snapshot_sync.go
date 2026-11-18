package cdeu

// 收藏的加密货币行情快照采集（需求 11314）。
//
// 链路：quote.finv_quote_secu_fav（收藏表）→ 取加密货币 usc → `CC.<usc>` →
// moomoo OpenAPI `POST /api/v1.0/quote/snapshot`（一次最多 400 个）→
// 转换 → upsert 到 cdeu.finv_quote_secu_realtime_crypto（按 usc 覆盖）。
//
// 凭据（AppKey + Ed25519 私钥）只从环境变量读取，见 moomoo_openapi.go 的文件头说明。
//
// 失败语义：单个批次失败即返回错误（不写半批）；收藏表为空时返回「空跑」结果、不报错。

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// CryptoFavSnapshotCollectOptions 采集控制参数（零值按默认值补齐）。
type CryptoFavSnapshotCollectOptions struct {
	// Timeout 本次采集总超时，0 取 60s。
	Timeout time.Duration
	// BatchSize 单次请求的 code 数上限，0 取 MoomooSnapshotMaxCodes。
	BatchSize int
}

// normalize 补齐默认值。
func (o CryptoFavSnapshotCollectOptions) normalize() CryptoFavSnapshotCollectOptions {
	if o.Timeout <= 0 {
		o.Timeout = 60 * time.Second
	}
	if o.BatchSize <= 0 || o.BatchSize > MoomooSnapshotMaxCodes {
		o.BatchSize = MoomooSnapshotMaxCodes
	}
	return o
}

// CryptoFavSnapshotResult 一次采集的结果统计。
type CryptoFavSnapshotResult struct {
	// USCTotal 收藏表中加密货币数量。
	USCTotal int
	// CodesRequested 实际请求的 code 数（可能小于 USCTotal 去重后）。
	CodesRequested int
	// RowsReceived 接口返回的快照行数。
	RowsReceived int
	// RowsInsert / RowsUpdate 新增 / 覆盖行数。
	RowsInsert int64
	RowsUpdate int64
	// Batches 请求批次数。
	Batches int
	// ElapsedMs 总耗时。
	ElapsedMs int64
}

// CryptoFavSnapshotStore 采集器依赖的存储能力（Store 实现；测试可注入）。
type CryptoFavSnapshotStore interface {
	ListFavoriteCryptoUSCs(ctx context.Context) ([]string, error)
	SaveCryptoRealtimeRows(ctx context.Context, rows []CryptoRealtimeRow) (int64, int64, error)
}

// CryptoFavSnapshotCollector 收藏加密货币快照采集器。
type CryptoFavSnapshotCollector struct {
	fetcher MoomooSnapshotFetcher
	store   CryptoFavSnapshotStore
	logger  *log.Logger
}

// NewCryptoFavSnapshotCollector 创建采集器；logger 为 nil 时用 log.Default()。
func NewCryptoFavSnapshotCollector(
	fetcher MoomooSnapshotFetcher, store CryptoFavSnapshotStore, logger *log.Logger,
) *CryptoFavSnapshotCollector {
	if logger == nil {
		logger = log.Default()
	}
	return &CryptoFavSnapshotCollector{fetcher: fetcher, store: store, logger: logger}
}

// Collect 执行一次采集。
func (c *CryptoFavSnapshotCollector) Collect(
	ctx context.Context, opts CryptoFavSnapshotCollectOptions,
) (CryptoFavSnapshotResult, error) {
	var result CryptoFavSnapshotResult
	opts = opts.normalize()
	started := time.Now()

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	uscs, err := c.store.ListFavoriteCryptoUSCs(runCtx)
	if err != nil {
		return result, err
	}
	result.USCTotal = len(uscs)
	if len(uscs) == 0 {
		c.logger.Printf("[cdeu][crypto] 收藏表里没有加密货币，跳过采集")
		return result, nil
	}

	codes := make([]string, 0, len(uscs))
	seen := make(map[string]bool, len(uscs))
	for _, usc := range uscs {
		code := MoomooCryptoCodePrefix + strings.ToUpper(strings.TrimSpace(usc))
		if seen[code] {
			continue
		}
		seen[code] = true
		codes = append(codes, code)
	}
	result.CodesRequested = len(codes)

	for start := 0; start < len(codes); start += opts.BatchSize {
		if err := runCtx.Err(); err != nil {
			return result, err
		}
		end := start + opts.BatchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[start:end]
		snapshots, err := c.fetcher.GetQuoteSnapshots(runCtx, batch)
		if err != nil {
			return result, fmt.Errorf("拉取加密货币快照失败（第 %d 批）: %w", start/opts.BatchSize+1, err)
		}
		result.Batches++
		result.RowsReceived += len(snapshots)

		rows := make([]CryptoRealtimeRow, 0, len(snapshots))
		for _, snapshot := range snapshots {
			row, ok := cryptoRowFromSnapshot(snapshot)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			continue
		}
		inserted, updated, err := c.store.SaveCryptoRealtimeRows(runCtx, rows)
		result.RowsInsert += inserted
		result.RowsUpdate += updated
		if err != nil {
			return result, err
		}
		c.logger.Printf("[cdeu][crypto] 第 %d 批：codes=%d rows=%d 新增=%d 覆盖=%d",
			start/opts.BatchSize+1, len(batch), len(rows), inserted, updated)
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	c.logger.Printf("[cdeu][crypto] 采集完成：收藏加密货币=%d codes=%d rows=%d 新增=%d 覆盖=%d 批次数=%d 耗时=%dms",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, result.Batches, result.ElapsedMs)
	return result, nil
}

// cryptoRowFromSnapshot 把一条 moomoo 快照转成落库行。
//
// usc 取 code 去掉 `CC.` 前缀（BTCUSD）；涨跌额/涨跌幅由 last 与 prev_close 计算
// （接口的 amplitude 是振幅，不是涨跌幅）。
func cryptoRowFromSnapshot(snapshot MoomooSnapshot) (CryptoRealtimeRow, bool) {
	code := strings.TrimSpace(snapshot.Code)
	usc := strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(code), strings.ToUpper(MoomooCryptoCodePrefix)))
	if usc == "" || usc == strings.ToUpper(code) {
		return CryptoRealtimeRow{}, false
	}

	row := CryptoRealtimeRow{
		USC:            usc,
		Code:           code,
		Name:           snapshot.Name,
		QuoteCurrency:  cryptoQuoteCurrency(usc),
		PriceNominal:   cryptoNumber(snapshot.LastPrice),
		PriceOpen:      cryptoNumber(snapshot.OpenPrice),
		PriceHighest:   cryptoNumber(snapshot.HighPrice),
		PriceLowest:    cryptoNumber(snapshot.LowPrice),
		PriceLastClose: cryptoNumber(snapshot.PrevClosePrice),
		PriceAvg:       cryptoNumber(snapshot.AvgPrice),
		PriceAmplitude: cryptoNumber(snapshot.Amplitude),
		TradeVolumn:    cryptoNumber(snapshot.Volume),
		TradeTrunover:  cryptoNumber(snapshot.Turnover),
		VolumnRatio:    cryptoNumber(snapshot.VolumeRatio),
		BidPrice:       cryptoNumber(snapshot.BidPrice),
		AskPrice:       cryptoNumber(snapshot.AskPrice),
		BidVol:         cryptoNumber(snapshot.BidVol),
		AskVol:         cryptoNumber(snapshot.AskVol),
		BuysellRatio:   cryptoNumber(snapshot.BidAskRatio),
		Highest52Weeks: cryptoNumber(snapshot.Highest52WeeksPrice),
		Lowest52Weeks:  cryptoNumber(snapshot.Lowest52WeeksPrice),
		HighestHistory: cryptoNumber(snapshot.HighestHistoryPrice),
		LowestHistory:  cryptoNumber(snapshot.LowestHistoryPrice),
		SecStatus:      snapshot.SecStatus,
		Suspension:     snapshot.Suspension,
		DataDate:       snapshot.DataDate,
		Provider:       CryptoProvider,
	}
	if snapshot.UpdateTime > 0 {
		updateTime := snapshot.UpdateTime
		row.DtUpdate = &updateTime
	}

	// 涨跌额/涨跌幅：以上一收盘为基准；基准为 0 时留空（不写 0）。
	if snapshot.PrevClosePrice != 0 {
		change := snapshot.LastPrice - snapshot.PrevClosePrice
		ratio := change / snapshot.PrevClosePrice * 100
		row.ChangePrice = cryptoNumber(change)
		row.ChangeRatio = cryptoNumber(ratio)
		direct := "="
		switch {
		case change > 0:
			direct = "+"
		case change < 0:
			direct = "-"
		}
		row.PriceDirect = &direct
	}
	return row, true
}
