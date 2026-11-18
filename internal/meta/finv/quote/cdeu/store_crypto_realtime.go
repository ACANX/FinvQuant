package cdeu

// 加密货币实时快照的读写（需求 11314）：cdeu.finv_quote_secu_realtime_crypto。
//
// 写入入口只有「收藏的加密货币快照采集」（crypto_fav_snapshot_sync.go）；
// 本文件负责两件事：
//   - ListFavoriteCryptoUSCs：从 quote.finv_quote_secu_fav 取加密货币 usc（与收藏表联动）；
//   - SaveCryptoRealtimeRows：按 usc upsert 快照行。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// tableCryptoRealtime 加密货币快照表（需求 11314）。
const tableCryptoRealtime = "cdeu.finv_quote_secu_realtime_crypto"

// cryptoRealtimeColumns 可写列（gmt_create / gmt_update 交给默认值与触发器）。
var cryptoRealtimeColumns = []string{
	"usc", "code", "name", "quote_currency",
	"price_nominal", "price_open", "price_highest", "price_lowest", "price_last_close", "price_avg",
	"change_price", "change_ratio", "price_amplitude", "price_direct",
	"trade_volumn", "trade_trunover", "volumn_ratio",
	"bid_price", "ask_price", "bid_vol", "ask_vol", "buysell_ratio",
	"highest52weeks", "lowest52weeks", "highest_history", "lowest_history",
	"sec_status", "suspension", "data_date", "dt_update", "provider",
}

// CryptoRealtimeRow 一行加密货币快照。
type CryptoRealtimeRow struct {
	USC            string
	Code           string
	Name           string
	QuoteCurrency  string
	PriceNominal   *json.Number
	PriceOpen      *json.Number
	PriceHighest   *json.Number
	PriceLowest    *json.Number
	PriceLastClose *json.Number
	PriceAvg       *json.Number
	ChangePrice    *json.Number
	ChangeRatio    *json.Number
	PriceAmplitude *json.Number
	PriceDirect    *string
	TradeVolumn    *json.Number
	TradeTrunover  *json.Number
	VolumnRatio    *json.Number
	BidPrice       *json.Number
	AskPrice       *json.Number
	BidVol         *json.Number
	AskVol         *json.Number
	BuysellRatio   *json.Number
	Highest52Weeks *json.Number
	Lowest52Weeks  *json.Number
	HighestHistory *json.Number
	LowestHistory  *json.Number
	SecStatus      string
	Suspension     bool
	DataDate       string
	DtUpdate       *int64
	Provider       string
}

// ListFavoriteCryptoUSCs 返回收藏表里属于加密货币的 usc（按 usc 升序）。
//
// 判定条件取并集：交易所标志 CRYPTO / CC 或 security_type = 'Crypto'，
// 兼容 finv_security 里不同批次导入的行。
func (s *Store) ListFavoriteCryptoUSCs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT f.usc
  FROM quote.finv_quote_secu_fav f
  JOIN public.finv_security s ON s.usc = f.usc
  LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
 WHERE upper(btrim(COALESCE(e.exchange_flag, ''))) IN ('CRYPTO', 'CC')
    OR s.security_type = 'Crypto'
 ORDER BY f.usc`)
	if err != nil {
		return nil, fmt.Errorf("查询收藏的加密货币失败: %w", err)
	}
	defer rows.Close()

	uscs := make([]string, 0, 32)
	for rows.Next() {
		var usc string
		if err := rows.Scan(&usc); err != nil {
			return nil, fmt.Errorf("解析收藏的加密货币失败: %w", err)
		}
		uscs = append(uscs, usc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取收藏的加密货币失败: %w", err)
	}
	return uscs, nil
}

// SaveCryptoRealtimeRows 按 usc upsert 快照行（幂等）；返回新增与覆盖行数。
func (s *Store) SaveCryptoRealtimeRows(ctx context.Context, rows []CryptoRealtimeRow) (int64, int64, error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}

	placeholders := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*len(cryptoRealtimeColumns))
	for i, row := range rows {
		if strings.TrimSpace(row.USC) == "" {
			return 0, 0, fmt.Errorf("第 %d 行缺少 usc", i+1)
		}
		base := i * len(cryptoRealtimeColumns)
		slots := make([]string, 0, len(cryptoRealtimeColumns))
		for j := range cryptoRealtimeColumns {
			slots = append(slots, fmt.Sprintf("$%d", base+j+1))
		}
		placeholders = append(placeholders, "("+strings.Join(slots, ", ")+")")
		args = append(args,
			row.USC, row.Code, row.Name, row.QuoteCurrency,
			row.PriceNominal, row.PriceOpen, row.PriceHighest, row.PriceLowest,
			row.PriceLastClose, row.PriceAvg,
			row.ChangePrice, row.ChangeRatio, row.PriceAmplitude, row.PriceDirect,
			row.TradeVolumn, row.TradeTrunover, row.VolumnRatio,
			row.BidPrice, row.AskPrice, row.BidVol, row.AskVol, row.BuysellRatio,
			row.Highest52Weeks, row.Lowest52Weeks, row.HighestHistory, row.LowestHistory,
			row.SecStatus, row.Suspension, row.DataDate, row.DtUpdate, row.Provider,
		)
	}

	sets := make([]string, 0, len(cryptoRealtimeColumns)-1)
	for _, col := range cryptoRealtimeColumns {
		if col == "usc" {
			continue
		}
		sets = append(sets, col+" = EXCLUDED."+col)
	}
	// 目标表别名 t 与既有 upsert 同口径（保留写法一致，便于排查）。
	sql := "INSERT INTO " + tableCryptoRealtime + " AS t (" +
		strings.Join(cryptoRealtimeColumns, ", ") + ") VALUES " +
		strings.Join(placeholders, ", ") +
		" ON CONFLICT (usc) DO UPDATE SET " + strings.Join(sets, ", ") +
		" RETURNING (xmax = 0) AS inserted"

	rowsOut, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("写入 %s 失败: %w", tableCryptoRealtime, err)
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
		return inserted, updated, fmt.Errorf("写入 %s 失败: %w", tableCryptoRealtime, err)
	}
	return inserted, updated, nil
}

// cryptoNumber float64 → *json.Number（保留最短可逆文本，交给 NUMERIC 定点转换）。
func cryptoNumber(v float64) *json.Number {
	n := json.Number(strconv.FormatFloat(v, 'f', -1, 64))
	return &n
}

// cryptoQuoteCurrency 从 usc 后缀推断计价货币（BTCUSD → USD、ETHHKD → HKD）。
func cryptoQuoteCurrency(usc string) string {
	upper := strings.ToUpper(strings.TrimSpace(usc))
	for _, cur := range []string{"USDT", "USDC", "USD", "HKD", "EUR", "GBP", "SGD", "JPY", "CNY", "AUD", "TRY"} {
		if strings.HasSuffix(upper, cur) {
			return cur
		}
	}
	return ""
}
