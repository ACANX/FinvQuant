package cdeu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// tableDay 日 K 线表（在 cdeu schema，需求 10950；结构稳定后再扩展到 quote）。
const tableDay = "cdeu.finv_quote_secu_kline_day"

// ErrCollectParamsNotFound 目标 usc 在 futu.finv_futu_quote_collect 里没有可用的请求参数。
var ErrCollectParamsNotFound = errors.New("futu.finv_futu_quote_collect 中没有该 usc 的启用记录")

// FutuQuoteParams futu 采集表里的证券定位参数（原样透传给 moomoo 接口）。
//
// 五个定位参数不做语义换算：同一编号在 moomoo 不同接口含义不同
// （Dictionaries.md §3），透传 t_stock 侧登记的取值才是正确做法。
type FutuQuoteParams struct {
	USC               string `json:"usc"`
	StockID           int64  `json:"stock_id"`
	FutuSymbol        string `json:"futu_symbol"`
	MarketType        int    `json:"market_type"`
	MarketCode        int    `json:"market_code"`
	InstrumentType    int    `json:"instrument_type"`
	SubInstrumentType int    `json:"sub_instrument_type"`
	ReqSection        string `json:"req_section"`
	QuoteMarket       string `json:"quote_market"`
	TypeSymbol        string `json:"type_symbol"`
	FlagEnable        string `json:"flag_enable"`
}

// CollectQuoteParams 读取 usc 对应的 moomoo 请求参数；无启用记录时返回
// （nil, ErrCollectParamsNotFound）。
func (s *Store) CollectQuoteParams(ctx context.Context, usc string) (*FutuQuoteParams, error) {
	var p FutuQuoteParams
	err := s.pool.QueryRow(ctx, `
SELECT usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
       sub_instrument_type, COALESCE(req_section, ''), quote_market, type_symbol,
       flag_enable::text
  FROM `+tableFutuQuote+`
 WHERE usc = $1 AND flag_enable = '1'`, usc).
		Scan(&p.USC, &p.StockID, &p.FutuSymbol, &p.MarketType, &p.MarketCode,
			&p.InstrumentType, &p.SubInstrumentType, &p.ReqSection, &p.QuoteMarket,
			&p.TypeSymbol, &p.FlagEnable)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCollectParamsNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DayKlineRow 一行日 K 数据（numeric 一律用字符串承载，避免 float64 精度损失）。
//
// 只为「本次数据源能提供的列」赋值；nil 表示该列本次无值。
type DayKlineRow struct {
	Date        int
	Ts          *int64
	Open        *string
	High        *string
	Low         *string
	Close       *string
	LastClose   *string
	ChangePrice *string
	ChangeRatio *string
	Volume      *string
	Turnover    *string
	Provider    string
}

// DayKlineUpsertStat upsert 统计。
type DayKlineUpsertStat struct {
	Inserted int64 `json:"inserted"`
	Updated  int64 `json:"updated"`
}

// Total 处理行数。
func (s DayKlineUpsertStat) Total() int64 { return s.Inserted + s.Updated }

// UpsertDayKlines 以 (date, usc) 为键 upsert 日 K 数据。
//
// 语义：
//   - 新行插入；已存在行按字段覆盖；
//   - 覆盖只对「本次有值」的列生效（COALESCE(EXCLUDED.x, t.x)），
//     避免把其它来源已回填的 open_interest / pe_ratio 等列抹成 NULL；
//   - gmt_update 由触发器维护。
func (s *Store) UpsertDayKlines(ctx context.Context, usc string, rows []DayKlineRow) (DayKlineUpsertStat, error) {
	var stat DayKlineUpsertStat
	if len(rows) == 0 {
		return stat, nil
	}
	if usc == "" {
		return stat, errors.New("UpsertDayKlines：usc 不能为空")
	}

	payload := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row.Date <= 0 {
			return stat, fmt.Errorf("UpsertDayKlines：date 非法（%d）", row.Date)
		}
		payload = append(payload, map[string]any{
			"date":         row.Date,
			"ts":           row.Ts,
			"open":         row.Open,
			"high":         row.High,
			"low":          row.Low,
			"close":        row.Close,
			"last_close":   row.LastClose,
			"change_price": row.ChangePrice,
			"change_ratio": row.ChangeRatio,
			"volume":       row.Volume,
			"turnover":     row.Turnover,
			"provider":     row.Provider,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return stat, fmt.Errorf("UpsertDayKlines：序列化失败: %w", err)
	}

	// jsonb_to_recordset 展开整批数据：一次往返写完，避免逐行往返。
	// 列类型全部先取 text，再在本层显式转 numeric/bigint，NULL 与空串都落到 NULL。
	if err := s.pool.QueryRow(ctx, `
WITH src AS (
    SELECT $1::text AS usc, x.date, x.ts,
           nullif(x.open, '')::numeric(20,6)         AS open,
           nullif(x.high, '')::numeric(20,6)         AS high,
           nullif(x.low, '')::numeric(20,6)          AS low,
           nullif(x.close, '')::numeric(20,6)        AS close,
           nullif(x.last_close, '')::numeric(20,6)   AS last_close,
           nullif(x.change_price, '')::numeric(20,6) AS change_price,
           nullif(x.change_ratio, '')::numeric(20,6) AS change_ratio,
           nullif(x.volume, '')::numeric(20,6)       AS volume,
           nullif(x.turnover, '')::numeric(30,8)     AS turnover,
           nullif(x.provider, '')                    AS provider
      FROM jsonb_to_recordset($2::jsonb) AS x(
             usc text, date integer, ts bigint,
             open text, high text, low text, close text,
             last_close text, change_price text, change_ratio text,
             volume text, turnover text, provider text)
), upserted AS (
    INSERT INTO `+tableDay+` AS t
        (usc, date, ts, open, high, low, close, last_close,
         change_price, change_ratio, volume, turnover, provider)
    SELECT $1, src.date, src.ts, src.open, src.high, src.low, src.close, src.last_close,
           src.change_price, src.change_ratio, src.volume, src.turnover, src.provider
      FROM src
    ON CONFLICT (date, usc) DO UPDATE SET
        ts           = COALESCE(EXCLUDED.ts, t.ts),
        open         = COALESCE(EXCLUDED.open, t.open),
        high         = COALESCE(EXCLUDED.high, t.high),
        low          = COALESCE(EXCLUDED.low, t.low),
        close        = COALESCE(EXCLUDED.close, t.close),
        last_close   = COALESCE(EXCLUDED.last_close, t.last_close),
        change_price = COALESCE(EXCLUDED.change_price, t.change_price),
        change_ratio = COALESCE(EXCLUDED.change_ratio, t.change_ratio),
        volume       = COALESCE(EXCLUDED.volume, t.volume),
        turnover     = COALESCE(EXCLUDED.turnover, t.turnover),
        provider     = COALESCE(EXCLUDED.provider, t.provider),
        gmt_update   = now()
    RETURNING (xmax = 0) AS inserted
)
SELECT count(*) FILTER (WHERE inserted)::bigint,
       count(*) FILTER (WHERE NOT inserted)::bigint
  FROM upserted`, usc, raw).
		Scan(&stat.Inserted, &stat.Updated); err != nil {
		return stat, fmt.Errorf("UpsertDayKlines：写入日K失败: %w", err)
	}
	return stat, nil
}

// DayKlineStat 某 usc 的日 K 存量统计（供页面/校验使用）。
type DayKlineStat struct {
	USC       string `json:"usc"`
	Rows      int64  `json:"rows"`
	DateFrom  int    `json:"date_from"`
	DateTo    int    `json:"date_to"`
	Providers string `json:"providers"`
}

// DayKlineStats 查询某 usc 的日 K 存量统计。
func (s *Store) DayKlineStats(ctx context.Context, usc string) (DayKlineStat, error) {
	stat := DayKlineStat{USC: usc}
	err := s.pool.QueryRow(ctx, `
SELECT count(*),
       COALESCE(min(date), 0),
       COALESCE(max(date), 0),
       COALESCE(string_agg(DISTINCT provider, ','), '')
  FROM `+tableDay+` WHERE usc = $1`, usc).
		Scan(&stat.Rows, &stat.DateFrom, &stat.DateTo, &stat.Providers)
	if err != nil {
		return stat, err
	}
	return stat, nil
}
