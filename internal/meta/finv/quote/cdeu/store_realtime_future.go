package cdeu

import (
	"context"
	"fmt"
	"strings"
)

// ListRealtimeFutures 分页查询期货实时行情快照（需求 11322）。
//
// 表 cdeu.finv_quote_realtime_future（主键 stock_id）。分类中文名来自配置表
// cdeu.finv_quote_cdeu_future_sync_config；配置表缺行时返回空串，前端降级显示「分类 N」。
// 数值列统一 ::text 输出，避免 NUMERIC(20,6) 在 JSON 里丢精度。
func (s *Store) ListRealtimeFutures(ctx context.Context, f RealtimeFutureListFilter) (int64, []RealtimeFutureListRow, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	orderBy := strings.TrimSpace(f.OrderBy)
	if orderBy == "" {
		orderBy = "change_ratio"
	}
	if !RealtimeFutureOrderColumns[orderBy] {
		return 0, nil, fmt.Errorf("%w: orderBy 非法（%s）", ErrRealtimeStockInvalid, orderBy)
	}

	var conds []string
	var args []any
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(t.contract_code ILIKE $%d OR t.name ILIKE $%d OR t.usc ILIKE $%d OR t.stock_id::text ILIKE $%d)",
			idx, idx, idx, idx))
	}
	if ft := strings.TrimSpace(f.FuturesType); ft != "" && !strings.EqualFold(ft, "all") {
		args = append(args, ft)
		conds = append(conds, fmt.Sprintf("t.futures_type = $%d", len(args)))
	}
	if market := strings.TrimSpace(f.MarketLabel); market != "" && !strings.EqualFold(market, "all") {
		args = append(args, market)
		conds = append(conds, fmt.Sprintf("t.market_label = $%d", len(args)))
	}
	if f.OnlyOnline {
		conds = append(conds, "t.flag_online = '1'")
	}

	fromClause := tableFutureList + ` t
 LEFT JOIN ` + tableFutureSyncConfig + ` c
        ON c.market_type = t.market_type AND c.futures_type = t.futures_type`
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM "+fromClause+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeFutures：统计失败: %w", err)
	}
	if total == 0 {
		return 0, []RealtimeFutureListRow{}, nil
	}

	direction := "DESC"
	if !f.Desc {
		direction = "ASC"
	}
	pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	query := `SELECT t.stock_id::text,
       COALESCE(t.usc, ''),
       COALESCE(t.futures_type, ''),
       COALESCE(c.futures_type_name, ''),
       COALESCE(t.market_type, ''),
       COALESCE(t.market_label, ''),
       COALESCE(t.contract_code, ''),
       COALESCE(t.name, ''),
       COALESCE(t.price_nominal::text, ''),
       COALESCE(t.change_ratio::text, ''),
       COALESCE(t.price_direct, ''),
       COALESCE(t.flag_online, ''),
       COALESCE(t.provider, ''),
       COALESCE(t.remark, ''),
       COALESCE(t.dt_update, 0),
       COALESCE(t.fields_json::text, ''),
       COALESCE(t.field_list_json::text, '')
  FROM ` + fromClause + where +
		" ORDER BY t." + orderBy + " " + direction + " NULLS LAST, t.stock_id" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, query, pageArgs...)
	if err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeFutures：查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]RealtimeFutureListRow, 0, f.Limit)
	for rows.Next() {
		var row RealtimeFutureListRow
		if err := rows.Scan(
			&row.StockID, &row.USC, &row.FuturesType, &row.FuturesTypeName,
			&row.MarketType, &row.MarketLabel, &row.ContractCode, &row.Name,
			&row.PriceNominal, &row.ChangeRatio, &row.PriceDirect, &row.FlagOnline,
			&row.Provider, &row.Remark, &row.DtUpdate,
			&row.FieldsJSON, &row.FieldListJSON,
		); err != nil {
			return 0, nil, fmt.Errorf("ListRealtimeFutures：扫描失败: %w", err)
		}
		list = append(list, row)
	}
	return total, list, rows.Err()
}

// ListRealtimeFutureTypes 期货分类下拉项（配置表 1~8，按 futures_type 升序）。
func (s *Store) ListRealtimeFutureTypes(ctx context.Context) ([]RealtimeFutureTypeOption, error) {
	rows, err := s.pool.Query(ctx, `
SELECT futures_type, COALESCE(futures_type_name, '')
  FROM `+tableFutureSyncConfig+`
 WHERE flag_enable = '1'
 ORDER BY futures_type`)
	if err != nil {
		return nil, fmt.Errorf("ListRealtimeFutureTypes：查询失败: %w", err)
	}
	defer rows.Close()

	options := make([]RealtimeFutureTypeOption, 0, 8)
	for rows.Next() {
		var opt RealtimeFutureTypeOption
		if err := rows.Scan(&opt.Value, &opt.Label); err != nil {
			return nil, fmt.Errorf("ListRealtimeFutureTypes：扫描失败: %w", err)
		}
		options = append(options, opt)
	}
	return options, rows.Err()
}
