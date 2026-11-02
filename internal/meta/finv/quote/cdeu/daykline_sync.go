package cdeu

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/quoteapiv2"
)

// DayKlineProviderFTMM 日 K 数据写入 provider 列的取值。
// 与存量分钟线保持一致（FTMM 为本平台统一的行情供应商标识），不因取数通道不同而另起名字。
const DayKlineProviderFTMM = "FTMM"

// dayKlineUpsertBatch 单批 upsert 行数（控制 jsonb 体积，避免一次请求过大）。
const dayKlineUpsertBatch = 2000

// KlineFetcher K 线拉取能力（由 quoteapiv2.Client 实现；测试可注入假实现）。
type KlineFetcher interface {
	Kline(ctx context.Context, req quoteapiv2.KlineRequest) ([]quoteapiv2.Bar, error)
}

// SetKlineFetcher 替换 K 线拉取实现（单测注入用）；传 nil 恢复默认 moomoo 客户端。
func (s *Service) SetKlineFetcher(fetcher KlineFetcher) {
	if fetcher == nil {
		fetcher = quoteapiv2.New(quoteapiv2.Config{})
	}
	s.klineFetcher = fetcher
}

// DayKlineSyncRequest 日 K 同步请求。
type DayKlineSyncRequest struct {
	USC string `json:"usc"`
	// From / To 为 yyyyMMdd，0 表示不限（接口总是返回全量历史，区间在本地过滤）。
	From int `json:"from"`
	To   int `json:"to"`
}

// DayKlineSyncResult 日 K 同步结果（供接口/页面展示）。
type DayKlineSyncResult struct {
	USC      string          `json:"usc"`
	Params   FutuQuoteParams `json:"params"`
	Timezone string          `json:"timezone"`
	// Total 接口返回的 K 线根数（全量历史）。
	Total int `json:"total"`
	// InRange 落在 [From, To] 区间内的根数（实际 upsert 的行数）。
	InRange int `json:"in_range"`
	// Inserted / Updated upsert 统计（以 date+usc 为键）。
	Inserted int64 `json:"inserted"`
	Updated  int64 `json:"updated"`
	// DateFrom / DateTo 本次写入数据的日期范围（yyyyMMdd；无数据为 0）。
	DateFrom int `json:"date_from"`
	DateTo   int `json:"date_to"`
	// Provider 写入 provider 列的值。
	Provider string `json:"provider"`
}

// SyncDayKlineFromMoomoo 拉取指定 usc 的日 K 并 upsert 到 cdeu.finv_quote_secu_kline_day。
//
// 链路：futu.finv_futu_quote_collect（请求参数）→ moomoo get-kline（全量历史）
// → 本地按日期区间过滤 → 按 (date, usc) upsert。
//
// 日期归属：接口只给秒级时间戳，这里按 cdeu.finv_quote_cdeu_usc.timezone 换算成
// 市场本地日期（缺失/非法时回退 UTC）。跨夜盘（如 COMEX 18:00 起的电子盘）严格
// 按交易日历归属应另用日历校正，本方法只做本地日历日换算。
//
// 涨跌幅：接口 r 字段样本恒为 null，且口径未验证，这里不写 r，改用
// change_price / last_close * 100 计算百分数，与存量分钟线 change_ratio 口径一致。
func (s *Service) SyncDayKlineFromMoomoo(ctx context.Context, req DayKlineSyncRequest) (DayKlineSyncResult, error) {
	started := time.Now()
	result := DayKlineSyncResult{Provider: DayKlineProviderFTMM}
	usc := strings.TrimSpace(req.USC)
	log.Printf("[cdeu][daykline] sync start usc=%q from=%d to=%d", usc, req.From, req.To)
	if usc == "" {
		return result, fmt.Errorf("SyncDayKlineFromMoomoo：usc 不能为空")
	}
	result.USC = usc
	if req.From > 0 && req.To > 0 && req.From > req.To {
		return result, fmt.Errorf("SyncDayKlineFromMoomoo：from(%d) 不能大于 to(%d)", req.From, req.To)
	}

	params, err := s.store.CollectQuoteParams(ctx, usc)
	if err != nil {
		log.Printf("[cdeu][daykline] sync failed usc=%s step=collect_params err=%v elapsed_ms=%d",
			usc, err, time.Since(started).Milliseconds())
		return result, fmt.Errorf("读取 futu 采集参数失败（usc=%s）: %w", usc, err)
	}
	result.Params = *params
	log.Printf("[cdeu][daykline] params usc=%s futu_symbol=%s stock_id=%d market_type=%d market_code=%d instrument_type=%d sub_instrument_type=%d quote_market=%s type_symbol=%s",
		usc, params.FutuSymbol, params.StockID, params.MarketType, params.MarketCode,
		params.InstrumentType, params.SubInstrumentType, params.QuoteMarket, params.TypeSymbol)

	location, tzName := s.dayKlineLocation(ctx, usc)
	result.Timezone = tzName

	fetcher := s.klineFetcher
	if fetcher == nil {
		fetcher = quoteapiv2.New(quoteapiv2.Config{})
	}
	klineReq := quoteapiv2.KlineRequest{
		StockID:           strconv.FormatInt(params.StockID, 10),
		MarketType:        strconv.Itoa(params.MarketType),
		KlineType:         quoteapiv2.KlineTypeDay,
		MarketCode:        strconv.Itoa(params.MarketCode),
		InstrumentType:    strconv.Itoa(params.InstrumentType),
		SubInstrumentType: strconv.Itoa(params.SubInstrumentType),
	}
	log.Printf("[cdeu][daykline] fetch start usc=%s stockId=%s marketType=%s type=%s marketCode=%s instrumentType=%s subInstrumentType=%s tz=%s",
		usc, klineReq.StockID, klineReq.MarketType, klineReq.KlineType, klineReq.MarketCode,
		klineReq.InstrumentType, klineReq.SubInstrumentType, result.Timezone)
	fetchStarted := time.Now()
	bars, err := fetcher.Kline(ctx, klineReq)
	if err != nil {
		log.Printf("[cdeu][daykline] sync failed usc=%s step=fetch err=%v elapsed_ms=%d",
			usc, err, time.Since(started).Milliseconds())
		return result, fmt.Errorf("拉取 moomoo 日K失败（usc=%s）: %w", usc, err)
	}
	result.Total = len(bars)
	log.Printf("[cdeu][daykline] fetch done usc=%s bars=%d fetch_elapsed_ms=%d", usc, len(bars), time.Since(fetchStarted).Milliseconds())

	rows := make([]DayKlineRow, 0, len(bars))
	for _, bar := range bars {
		date := dayKlineDate(bar.Time, location)
		if date <= 0 {
			continue
		}
		if req.From > 0 && date < req.From {
			continue
		}
		if req.To > 0 && date > req.To {
			continue
		}
		ts := bar.Time
		row := DayKlineRow{
			Date:        date,
			Ts:          &ts,
			Open:        optionalNumeric(bar.Open),
			High:        optionalNumeric(bar.High),
			Low:         optionalNumeric(bar.Low),
			Close:       optionalNumeric(bar.Close),
			LastClose:   optionalPositive(bar.LastClose),
			ChangePrice: optionalNumeric(bar.ChangePrice),
			Volume:      optionalFloat(bar.Volume),
			Turnover:    optionalFloat(bar.Turnover),
			Provider:    DayKlineProviderFTMM,
		}
		row.ChangeRatio = dayKlineChangeRatio(bar)
		rows = append(rows, row)
		if result.DateFrom == 0 || date < result.DateFrom {
			result.DateFrom = date
		}
		if date > result.DateTo {
			result.DateTo = date
		}
	}
	result.InRange = len(rows)
	log.Printf("[cdeu][daykline] filter usc=%s total=%d in_range=%d date_from=%d date_to=%d tz=%s",
		usc, result.Total, result.InRange, result.DateFrom, result.DateTo, result.Timezone)

	for start := 0; start < len(rows); start += dayKlineUpsertBatch {
		end := start + dayKlineUpsertBatch
		if end > len(rows) {
			end = len(rows)
		}
		stat, err := s.store.UpsertDayKlines(ctx, usc, rows[start:end])
		if err != nil {
			log.Printf("[cdeu][daykline] sync failed usc=%s step=upsert offset=%d size=%d err=%v elapsed_ms=%d",
				usc, start, end-start, err, time.Since(started).Milliseconds())
			return result, err
		}
		result.Inserted += stat.Inserted
		result.Updated += stat.Updated
		log.Printf("[cdeu][daykline] upsert batch usc=%s offset=%d size=%d inserted=%d updated=%d",
			usc, start, end-start, stat.Inserted, stat.Updated)
	}
	log.Printf("[cdeu][daykline] sync done usc=%s total=%d in_range=%d inserted=%d updated=%d date_from=%d date_to=%d provider=%s elapsed_ms=%d",
		usc, result.Total, result.InRange, result.Inserted, result.Updated,
		result.DateFrom, result.DateTo, result.Provider, time.Since(started).Milliseconds())
	return result, nil
}

// dayKlineLocation 取 usc 的市场时区；缺失或非法时回退 UTC 并返回回退后的名称。
func (s *Service) dayKlineLocation(ctx context.Context, usc string) (*time.Location, string) {
	info, err := s.store.UscInfo(ctx, usc)
	if err != nil || info == nil || info.Timezone == nil {
		return time.UTC, "UTC"
	}
	name := strings.TrimSpace(*info.Timezone)
	if name == "" {
		return time.UTC, "UTC"
	}
	location, err := time.LoadLocation(name)
	if err != nil || location == nil {
		return time.UTC, "UTC"
	}
	return location, location.String()
}

// dayKlineDate 秒级时间戳 → 市场本地日期 yyyyMMdd。
func dayKlineDate(ts int64, location *time.Location) int {
	if ts <= 0 {
		return 0
	}
	at := time.Unix(ts, 0).In(location)
	return at.Year()*10000 + int(at.Month())*100 + at.Day()
}

// dayKlineChangeRatio 按 change_price / last_close * 100 计算涨跌幅（百分数，6 位小数），
// 与存量分钟线 change_ratio 的口径一致；last_close 缺失或为 0 时不写。
func dayKlineChangeRatio(bar quoteapiv2.Bar) *string {
	if bar.LastClose == 0 || strings.TrimSpace(bar.ChangePrice) == "" {
		return nil
	}
	changePrice, err := strconv.ParseFloat(strings.TrimSpace(bar.ChangePrice), 64)
	if err != nil {
		return nil
	}
	ratio := changePrice / bar.LastClose * 100
	if math.IsInf(ratio, 0) || math.IsNaN(ratio) {
		return nil
	}
	text := strconv.FormatFloat(ratio, 'f', 6, 64)
	return &text
}

// optionalNumeric 接口返回的字符串价格：空串 → nil（落库为 NULL）。
func optionalNumeric(value string) *string {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	return &text
}

// optionalPositive 前收盘价：<=0 视为缺失（接口对缺失量给 0）。
func optionalPositive(value float64) *string {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return nil
	}
	text := strconv.FormatFloat(value, 'f', 6, 64)
	return &text
}

// optionalFloat 数值量：0 是合法值（如样本成交量 0），按 0 写入。
func optionalFloat(value float64) *string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	text := strconv.FormatFloat(value, 'f', 6, 64)
	return &text
}
