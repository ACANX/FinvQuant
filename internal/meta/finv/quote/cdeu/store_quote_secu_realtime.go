package cdeu

import (
	"context"
	"fmt"
	"strings"
)

// 本文件是「跨 provider 证券实时行情快照」的落库层（需求 11262）：
//   - ResolveSecuUsc：把 (交易所, 证券代码) 批量解析成 public.finv_security.usc；
//   - SaveQuoteSecuRealtimeRows：按 usc 主键 upsert 到 cdeu.finv_quote_secu_realtime（V300209）。
//
// 表结构与参考表 cdeu.finv_cdeu_secu_realtime_stock 对齐（多 provider、stock_id 为 TEXT），
// 两表并存，本文件只写新表，不改参考表。

// tableQuoteSecuRealtime 跨 provider 实时行情快照表（V300209）。
const tableQuoteSecuRealtime = "cdeu.finv_quote_secu_realtime"

// quoteSecuRealtimeColumns 落库列清单（顺序与 upsert 参数一致）。
var quoteSecuRealtimeColumns = []string{
	"usc", "stock_id", "name", "stock_code", "market_type", "market_label",
	"instrument_type", "currency", "market_value", "total_shares",
	"circulation_total_shares", "trade_trunover", "trade_volumn",
	"circulation_market_value", "dt_update", "price_nominal", "change_ratio",
	"trust_aum", "trade_change_ratio", "price_amplitude", "volumn_ratio",
	"buysell_ratio", "change_price", "price_direct", "leverage", "page",
	"rmb_exchange_rate", "rmb_market_value", "rmb_circulation_market_value",
	"rmb_trade_trunover", "rmb_price_nominal", "remark", "provider",
}

// ResolveSecuUsc 批量解析 usc：按 (finv_exchange.exchange_flag, finv_security.security_code) 匹配。
//
// 未命中的键不会出现在返回 map 中（调用方据此统计跳过数量）。
// 空输入直接返回空 map，不查库。
func (s *Store) ResolveSecuUsc(ctx context.Context, keys []SecuUscKey) (map[SecuUscKey]string, error) {
	out := make(map[SecuUscKey]string, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	exchanges := make([]string, 0, len(keys))
	codes := make([]string, 0, len(keys))
	for _, key := range keys {
		exchanges = append(exchanges, strings.ToUpper(strings.TrimSpace(key.Exchange)))
		codes = append(codes, strings.TrimSpace(key.Code))
	}

	rows, err := s.pool.Query(ctx, `
SELECT e.exchange_flag, s.security_code, s.usc
  FROM public.finv_security s
  JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
 WHERE (e.exchange_flag, s.security_code) IN (
       SELECT * FROM unnest($1::text[], $2::text[])
 )`, exchanges, codes)
	if err != nil {
		return nil, fmt.Errorf("查询 public.finv_security 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exchange, code, usc string
		if err := rows.Scan(&exchange, &code, &usc); err != nil {
			return nil, fmt.Errorf("解析 usc 结果失败: %w", err)
		}
		out[SecuUscKey{Exchange: strings.ToUpper(exchange), Code: code}] = usc
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("查询 public.finv_security 失败: %w", err)
	}
	return out, nil
}

// SaveQuoteSecuRealtimeRows 按 usc 主键批量 upsert，返回新增 / 更新行数。
//
// 语义为「整行覆盖」：BDFS 每次返回的是完整快照，缺字段写 NULL（不保留上一轮值）；
// gmt_create / gmt_update 不进 SQL，由表默认值与触发器维护。
// 行数较多时由调用方分批（BdfsDbBatchSize），本方法只处理一批。
func (s *Store) SaveQuoteSecuRealtimeRows(ctx context.Context, rows []QuoteSecuRealtimeRow) (int64, int64, error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*len(quoteSecuRealtimeColumns))
	for i, row := range rows {
		if strings.TrimSpace(row.USC) == "" {
			return 0, 0, fmt.Errorf("第 %d 行缺少 usc", i+1)
		}
		base := i * len(quoteSecuRealtimeColumns)
		slots := make([]string, 0, len(quoteSecuRealtimeColumns))
		for j := range quoteSecuRealtimeColumns {
			slots = append(slots, fmt.Sprintf("$%d", base+j+1))
		}
		placeholders = append(placeholders, "("+strings.Join(slots, ", ")+")")
		args = append(args,
			row.USC, row.StockID, row.Name, row.StockCode, row.MarketType, row.MarketLabel,
			row.InstrumentType, row.Currency, row.MarketValue, row.TotalShares,
			row.CirculationTotalShares, row.TradeTrunover, row.TradeVolumn,
			row.CirculationMarketValue, row.DtUpdate, row.PriceNominal, row.ChangeRatio,
			row.TrustAum, row.TradeChangeRatio, row.PriceAmplitude, row.VolumnRatio,
			row.BuysellRatio, row.ChangePrice, row.PriceDirect, row.Leverage, row.Page,
			row.RmbExchangeRate, row.RmbMarketValue, row.RmbCirculationMarketValue,
			row.RmbTradeTrunover, row.RmbPriceNominal, row.Remark, row.Provider,
		)
	}

	updates := make([]string, 0, len(quoteSecuRealtimeColumns)-1)
	for _, col := range quoteSecuRealtimeColumns {
		if col == "usc" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}

	sql := "INSERT INTO " + tableQuoteSecuRealtime + " (" +
		strings.Join(quoteSecuRealtimeColumns, ", ") + ") VALUES " +
		strings.Join(placeholders, ", ") +
		" ON CONFLICT (usc) DO UPDATE SET " + strings.Join(updates, ", ") +
		" RETURNING (xmax = 0) AS inserted"

	rowsOut, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("写入 %s 失败: %w", tableQuoteSecuRealtime, err)
	}
	defer rowsOut.Close()

	var inserted, updated int64
	for rowsOut.Next() {
		var isInsert bool
		if err := rowsOut.Scan(&isInsert); err != nil {
			return inserted, updated, fmt.Errorf("解析写入结果失败: %w", err)
		}
		if isInsert {
			inserted++
		} else {
			updated++
		}
	}
	if err := rowsOut.Err(); err != nil {
		return inserted, updated, fmt.Errorf("写入 %s 失败: %w", tableQuoteSecuRealtime, err)
	}
	return inserted, updated, nil
}

// 编译期断言：*Store 同时满足 usc 解析与落库接口。
var (
	_ SecuUscResolver       = (*Store)(nil)
	_ QuoteSecuRealtimeSink = (*Store)(nil)
)
