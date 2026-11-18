package cdeu

import (
	"context"
	"fmt"
	"strings"
)

// ListRealtimeCryptos 分页查询加密货币行情快照（需求 11323）。
//
// 表 cdeu.finv_quote_secu_realtime_crypto（主键 usc）。数值列统一 ::text 输出；
// suspension 以 'true'/'false' 文本返回，前端渲染成「是/否」。
func (s *Store) ListRealtimeCryptos(ctx context.Context, f RealtimeCryptoListFilter) (int64, []RealtimeCryptoListRow, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	orderBy := strings.TrimSpace(f.OrderBy)
	if orderBy == "" {
		orderBy = "change_ratio"
	}
	if !RealtimeCryptoOrderColumns[orderBy] {
		return 0, nil, fmt.Errorf("%w: orderBy 非法（%s）", ErrRealtimeStockInvalid, orderBy)
	}

	var conds []string
	var args []any
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(t.usc ILIKE $%d OR t.code ILIKE $%d OR t.name ILIKE $%d)", idx, idx, idx))
	}
	if cur := strings.TrimSpace(f.QuoteCurrency); cur != "" && !strings.EqualFold(cur, "all") {
		args = append(args, cur)
		conds = append(conds, fmt.Sprintf("t.quote_currency = $%d", len(args)))
	}
	if status := strings.TrimSpace(f.SecStatus); status != "" && !strings.EqualFold(status, "all") {
		args = append(args, status)
		conds = append(conds, fmt.Sprintf("t.sec_status = $%d", len(args)))
	}
	switch strings.TrimSpace(f.Suspension) {
	case "", "all":
		// 不限
	case "1":
		conds = append(conds, "t.suspension IS TRUE")
	case "0":
		conds = append(conds, "t.suspension IS NOT TRUE")
	default:
		return 0, nil, fmt.Errorf("%w: suspension 只能为 0 或 1", ErrRealtimeStockInvalid)
	}

	fromClause := tableCryptoRealtime + " t"
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM "+fromClause+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeCryptos：统计失败: %w", err)
	}
	if total == 0 {
		return 0, []RealtimeCryptoListRow{}, nil
	}

	direction := "DESC"
	if !f.Desc {
		direction = "ASC"
	}
	pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	query := `SELECT COALESCE(t.usc, ''),
       COALESCE(t.code, ''),
       COALESCE(t.name, ''),
       COALESCE(t.quote_currency, ''),
       COALESCE(t.price_nominal::text, ''),
       COALESCE(t.price_open::text, ''),
       COALESCE(t.price_highest::text, ''),
       COALESCE(t.price_lowest::text, ''),
       COALESCE(t.price_last_close::text, ''),
       COALESCE(t.price_avg::text, ''),
       COALESCE(t.change_price::text, ''),
       COALESCE(t.change_ratio::text, ''),
       COALESCE(t.price_amplitude::text, ''),
       COALESCE(t.price_direct, ''),
       COALESCE(t.trade_volumn::text, ''),
       COALESCE(t.trade_trunover::text, ''),
       COALESCE(t.volumn_ratio::text, ''),
       COALESCE(t.bid_price::text, ''),
       COALESCE(t.ask_price::text, ''),
       COALESCE(t.bid_vol::text, ''),
       COALESCE(t.ask_vol::text, ''),
       COALESCE(t.buysell_ratio::text, ''),
       COALESCE(t.highest52weeks::text, ''),
       COALESCE(t.lowest52weeks::text, ''),
       COALESCE(t.highest_history::text, ''),
       COALESCE(t.lowest_history::text, ''),
       COALESCE(t.sec_status, ''),
       COALESCE(t.suspension::text, ''),
       COALESCE(t.data_date, ''),
       COALESCE(t.provider, ''),
       COALESCE(t.dt_update, 0)
  FROM ` + fromClause + where +
		" ORDER BY t." + orderBy + " " + direction + " NULLS LAST, t.usc" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, query, pageArgs...)
	if err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeCryptos：查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]RealtimeCryptoListRow, 0, f.Limit)
	for rows.Next() {
		var row RealtimeCryptoListRow
		if err := rows.Scan(
			&row.USC, &row.Code, &row.Name, &row.QuoteCurrency,
			&row.PriceNominal, &row.PriceOpen, &row.PriceHighest, &row.PriceLowest,
			&row.PriceLastClose, &row.PriceAvg, &row.ChangePrice, &row.ChangeRatio,
			&row.PriceAmplitude, &row.PriceDirect, &row.TradeVolumn, &row.TradeTrunover,
			&row.VolumnRatio, &row.BidPrice, &row.AskPrice, &row.BidVol, &row.AskVol,
			&row.BuysellRatio, &row.Highest52Weeks, &row.Lowest52Weeks,
			&row.HighestHistory, &row.LowestHistory, &row.SecStatus, &row.Suspension,
			&row.DataDate, &row.Provider, &row.DtUpdate,
		); err != nil {
			return 0, nil, fmt.Errorf("ListRealtimeCryptos：扫描失败: %w", err)
		}
		list = append(list, row)
	}
	return total, list, rows.Err()
}

// ListRealtimeCryptoQuoteCurrencies 计价币种下拉项（SELECT DISTINCT，按字母序）。
func (s *Store) ListRealtimeCryptoQuoteCurrencies(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT quote_currency
  FROM `+tableCryptoRealtime+`
 WHERE NULLIF(btrim(quote_currency), '') IS NOT NULL
 ORDER BY quote_currency`)
	if err != nil {
		return nil, fmt.Errorf("ListRealtimeCryptoQuoteCurrencies：查询失败: %w", err)
	}
	defer rows.Close()

	values := make([]string, 0, 4)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("ListRealtimeCryptoQuoteCurrencies：扫描失败: %w", err)
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
