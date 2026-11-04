package cdeu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// UpsertRealtimeStocks 以 stock_id 为键批量写入证券实时行情快照（需求 10957）。
//
// 语义（字段级覆盖，与 UpsertDayKlines 一致）：
//   - stock_id 不存在 → 插入；已存在 → 只覆盖「本次有值」的列
//     （COALESCE(EXCLUDED.x, t.x)），源端缺字段不会清空已有值；
//   - 空字符串视同未提供（数据库侧 btrim + nullif 后为 NULL），不会把旧值抹成空串；
//   - 例外：dt_update 未提供（省略或 null）时按「本次写入时刻」刷新为当前 Unix 毫秒，
//     不保留旧值——快照更新时间应反映最近一次写入；
//   - 整批在一条 INSERT ... ON CONFLICT 语句内完成（jsonb_to_recordset 展开），
//     无部分成功：语句失败即整批回滚。
//
// 调用前须已通过 ValidateRealtimeStockItems（stock_id 非空、批内唯一）。
func (s *Store) UpsertRealtimeStocks(ctx context.Context, items []RealtimeStockItem) (RealtimeStockUpsertStat, error) {
	var stat RealtimeStockUpsertStat
	if len(items) == 0 {
		return stat, nil
	}

	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.StockID == nil {
			return stat, fmt.Errorf("%w: 缺少 stock_id", ErrRealtimeStockInvalid)
		}
		payload = append(payload, map[string]any{
			"stock_id":                 *item.StockID,
			"usc":                      realtimeStockText(item.USC),
			"name":                     realtimeStockText(item.Name),
			"stock_code":               realtimeStockText(item.StockCode),
			"market_type":              realtimeStockText(item.MarketType),
			"market_label":             realtimeStockText(item.MarketLabel),
			"instrument_type":          realtimeStockText(item.InstrumentType),
			"currency":                 realtimeStockText(item.Currency),
			"market_value":             realtimeStockNumber(item.MarketValue),
			"total_shares":             realtimeStockNumber(item.TotalShares),
			"circulation_total_shares": realtimeStockNumber(item.CirculationTotalShares),
			"trade_trunover":           realtimeStockNumber(item.TradeTrunover),
			"trade_volumn":             realtimeStockNumber(item.TradeVolumn),
			"circulation_market_value": realtimeStockNumber(item.CirculationMarketValue),
			"dt_update":                realtimeStockInt(item.DtUpdate),
			"price_nominal":            realtimeStockNumber(item.PriceNominal),
			"change_ratio":             realtimeStockNumber(item.ChangeRatio),
			"trust_aum":                realtimeStockNumber(item.TrustAum),
			"trade_change_ratio":       realtimeStockNumber(item.TradeChangeRatio),
			"price_amplitude":          realtimeStockNumber(item.PriceAmplitude),
			"volumn_ratio":             realtimeStockNumber(item.VolumnRatio),
			"buysell_ratio":            realtimeStockNumber(item.BuysellRatio),
			"change_price":             realtimeStockNumber(item.ChangePrice),
			"price_direct":             realtimeStockText(item.PriceDirect),
			"leverage":                 realtimeStockNumber(item.Leverage),
			"page":                     realtimeStockInt(item.Page),
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return stat, fmt.Errorf("UpsertRealtimeStocks：序列化失败: %w", err)
	}

	// 列类型全部先取 text 再显式转 numeric，NULL 与空串都落到 NULL；
	// 数值不经 float64，NUMERIC(24,6) 的定点精度由数据库负责。
	err = s.pool.QueryRow(ctx, `
WITH src AS (
    SELECT x.stock_id                                             AS stock_id,
           nullif(btrim(x.usc), '')                               AS usc,
           nullif(btrim(x.name), '')                              AS name,
           nullif(btrim(x.stock_code), '')                        AS stock_code,
           nullif(btrim(x.market_type), '')                       AS market_type,
           nullif(btrim(x.market_label), '')                      AS market_label,
           nullif(btrim(x.instrument_type), '')                   AS instrument_type,
           nullif(btrim(x.currency), '')                          AS currency,
           nullif(x.market_value, '')::numeric(24,6)              AS market_value,
           nullif(x.total_shares, '')::numeric(24,6)              AS total_shares,
           nullif(x.circulation_total_shares, '')::numeric(24,6)  AS circulation_total_shares,
           nullif(x.trade_trunover, '')::numeric(24,6)            AS trade_trunover,
           nullif(x.trade_volumn, '')::numeric(24,6)              AS trade_volumn,
           nullif(x.circulation_market_value, '')::numeric(24,6)  AS circulation_market_value,
           COALESCE(x.dt_update, (extract(epoch FROM now()) * 1000)::bigint) AS dt_update,
           nullif(x.price_nominal, '')::numeric(20,6)             AS price_nominal,
           nullif(x.change_ratio, '')::numeric(20,6)              AS change_ratio,
           nullif(x.trust_aum, '')::numeric(24,6)                 AS trust_aum,
           nullif(x.trade_change_ratio, '')::numeric(20,6)        AS trade_change_ratio,
           nullif(x.price_amplitude, '')::numeric(20,6)           AS price_amplitude,
           nullif(x.volumn_ratio, '')::numeric(20,6)              AS volumn_ratio,
           nullif(x.buysell_ratio, '')::numeric(20,6)             AS buysell_ratio,
           nullif(x.change_price, '')::numeric(20,6)              AS change_price,
           nullif(btrim(x.price_direct), '')                      AS price_direct,
           nullif(x.leverage, '')::numeric(20,6)                  AS leverage,
           (x.page)::integer                                      AS page
      FROM jsonb_to_recordset($1::jsonb) AS x(
             stock_id bigint, usc text, name text, stock_code text, market_type text,
             market_label text, instrument_type text, currency text,
             market_value text, total_shares text, circulation_total_shares text,
             trade_trunover text, trade_volumn text, circulation_market_value text,
             dt_update bigint,
             price_nominal text, change_ratio text, trust_aum text,
             trade_change_ratio text, price_amplitude text, volumn_ratio text,
             buysell_ratio text, change_price text,
             price_direct text, leverage text,
             page bigint)
), upserted AS (
    INSERT INTO `+tableRealtimeStock+` AS t
        (usc, stock_id, name, stock_code, market_type, market_label, instrument_type,
         currency, market_value, total_shares, circulation_total_shares,
         trade_trunover, trade_volumn, circulation_market_value, dt_update,
         price_nominal, change_ratio, trust_aum, trade_change_ratio,
         price_amplitude, volumn_ratio, buysell_ratio, change_price,
         price_direct, leverage, page)
    SELECT src.usc, src.stock_id, src.name, src.stock_code, src.market_type,
           src.market_label, src.instrument_type, src.currency, src.market_value,
           src.total_shares, src.circulation_total_shares, src.trade_trunover,
           src.trade_volumn, src.circulation_market_value, src.dt_update,
           src.price_nominal, src.change_ratio, src.trust_aum, src.trade_change_ratio,
           src.price_amplitude, src.volumn_ratio, src.buysell_ratio, src.change_price,
           src.price_direct, src.leverage, src.page
      FROM src
    ON CONFLICT (stock_id) DO UPDATE SET
        usc                      = COALESCE(EXCLUDED.usc, t.usc),
        name                     = COALESCE(EXCLUDED.name, t.name),
        stock_code               = COALESCE(EXCLUDED.stock_code, t.stock_code),
        market_type              = COALESCE(EXCLUDED.market_type, t.market_type),
        market_label             = COALESCE(EXCLUDED.market_label, t.market_label),
        instrument_type          = COALESCE(EXCLUDED.instrument_type, t.instrument_type),
        currency                 = COALESCE(EXCLUDED.currency, t.currency),
        market_value             = COALESCE(EXCLUDED.market_value, t.market_value),
        total_shares             = COALESCE(EXCLUDED.total_shares, t.total_shares),
        circulation_total_shares = COALESCE(EXCLUDED.circulation_total_shares, t.circulation_total_shares),
        trade_trunover           = COALESCE(EXCLUDED.trade_trunover, t.trade_trunover),
        trade_volumn             = COALESCE(EXCLUDED.trade_volumn, t.trade_volumn),
        circulation_market_value = COALESCE(EXCLUDED.circulation_market_value, t.circulation_market_value),
        dt_update                = COALESCE(EXCLUDED.dt_update, t.dt_update),
        price_nominal            = COALESCE(EXCLUDED.price_nominal, t.price_nominal),
        change_ratio             = COALESCE(EXCLUDED.change_ratio, t.change_ratio),
        trust_aum                = COALESCE(EXCLUDED.trust_aum, t.trust_aum),
        trade_change_ratio       = COALESCE(EXCLUDED.trade_change_ratio, t.trade_change_ratio),
        price_amplitude          = COALESCE(EXCLUDED.price_amplitude, t.price_amplitude),
        volumn_ratio             = COALESCE(EXCLUDED.volumn_ratio, t.volumn_ratio),
        buysell_ratio            = COALESCE(EXCLUDED.buysell_ratio, t.buysell_ratio),
        change_price             = COALESCE(EXCLUDED.change_price, t.change_price),
        price_direct             = COALESCE(EXCLUDED.price_direct, t.price_direct),
        leverage                 = COALESCE(EXCLUDED.leverage, t.leverage),
        page                     = COALESCE(EXCLUDED.page, t.page)
    RETURNING (xmax = 0) AS inserted
)
SELECT count(*) FILTER (WHERE inserted)::bigint,
       count(*) FILTER (WHERE NOT inserted)::bigint
  FROM upserted`, raw).
		Scan(&stat.Inserted, &stat.Updated)
	if err != nil {
		// NUMERIC(24,6) 溢出、stock_id 越界等数据库侧拒绝统一包一层，
		// 由 handler 映射为 400（入参问题）而非 500。
		if isRealtimeStockDataError(err) {
			return stat, fmt.Errorf("%w: %v", ErrRealtimeStockInvalid, err)
		}
		return stat, fmt.Errorf("UpsertRealtimeStocks：写入实时行情快照失败: %w", err)
	}
	stat.Total = stat.Inserted + stat.Updated
	return stat, nil
}

// isRealtimeStockDataError 判断是否属于「入参本身不合法」的数据库错误码：
//   - 22003 numeric_value_out_of_range（NUMERIC 溢出）
//   - 22P02 invalid_text_representation（数值文本非法）
//   - 22007 invalid_datetime_format / 22008 datetime_field_overflow（时间戳越界）
//   - 23514 check_violation（如 page <= 0 违反 page_chk 约束）
//
// 这些错误说明调用方数据有问题，应当返回 400 而不是 500。
func isRealtimeStockDataError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "22003", "22P02", "22007", "22008", "23514":
		return true
	}
	return false
}

// realtimeStockListColumns 列表输出列：数值列一律 ::text，避免 NUMERIC 经 float64 丢精度。
const realtimeStockListColumns = `
       stock_id,
       COALESCE(usc, '')                            AS usc,
       COALESCE(name, '')                           AS name,
       COALESCE(stock_code, '')                     AS stock_code,
       COALESCE(market_type, '')                    AS market_type,
       COALESCE(market_label, '')                   AS market_label,
       COALESCE(instrument_type, '')                AS instrument_type,
       COALESCE(currency, '')                       AS currency,
       COALESCE(price_nominal::text, '')            AS price_nominal,
       COALESCE(change_price::text, '')             AS change_price,
       COALESCE(change_ratio::text, '')             AS change_ratio,
       COALESCE(price_direct, '')                   AS price_direct,
       COALESCE(market_value::text, '')             AS market_value,
       COALESCE(total_shares::text, '')             AS total_shares,
       COALESCE(circulation_total_shares::text, '') AS circulation_total_shares,
       COALESCE(trade_trunover::text, '')           AS trade_trunover,
       COALESCE(trade_volumn::text, '')             AS trade_volumn,
       COALESCE(circulation_market_value::text, '') AS circulation_market_value,
       COALESCE(trust_aum::text, '')                AS trust_aum,
       COALESCE(trade_change_ratio::text, '')       AS trade_change_ratio,
       COALESCE(price_amplitude::text, '')          AS price_amplitude,
       COALESCE(volumn_ratio::text, '')             AS volumn_ratio,
       COALESCE(buysell_ratio::text, '')            AS buysell_ratio,
       COALESCE(leverage::text, '')                 AS leverage,
       COALESCE(dt_update, 0)                       AS dt_update`

// ListRealtimeStocks 分页查询实时行情快照：关键字（usc / 代码 / 名称）、市场、币种筛选，
// 白名单列排序（NULL 一律排最后），返回总数与当前页。
func (s *Store) ListRealtimeStocks(ctx context.Context, f RealtimeStockListFilter) (int64, []RealtimeStockListRow, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	orderBy := f.OrderBy
	if orderBy == "" {
		orderBy = "market_value"
	}
	if !RealtimeStockOrderColumns[orderBy] {
		return 0, nil, fmt.Errorf("%w: orderBy 非法（%s）", ErrRealtimeStockInvalid, orderBy)
	}

	var conds []string
	var args []any
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(usc ILIKE $%d OR stock_code ILIKE $%d OR name ILIKE $%d)", idx, idx, idx))
	}
	if market := strings.TrimSpace(f.MarketLabel); market != "" && !strings.EqualFold(market, "all") {
		args = append(args, market)
		conds = append(conds, fmt.Sprintf("market_label = $%d", len(args)))
	}
	if currency := strings.TrimSpace(f.Currency); currency != "" && !strings.EqualFold(currency, "all") {
		args = append(args, currency)
		conds = append(conds, fmt.Sprintf("currency = $%d", len(args)))
	}
	// usc 为空 = 尚未录入证券主数据（新上市/未纳入采集），是补录三张表的依据（需求 10995）。
	if f.USCMissing {
		conds = append(conds, "usc IS NULL")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM "+tableRealtimeStock+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeStocks：统计失败: %w", err)
	}
	if total == 0 {
		return 0, []RealtimeStockListRow{}, nil
	}

	direction := "DESC"
	if !f.Desc {
		direction = "ASC"
	}
	pageArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	// 排序必须写成 t.<列>：输出列里有同名别名（market_value::text AS market_value），
	// 不限定表别名时 ORDER BY 会解析到 text 输出列，退化成字符串排序（"9994…" > "5060…"）。
	query := "SELECT " + realtimeStockListColumns + " FROM " + tableRealtimeStock + " t" + where +
		" ORDER BY t." + orderBy + " " + direction + " NULLS LAST, t.stock_id" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, query, pageArgs...)
	if err != nil {
		return 0, nil, fmt.Errorf("ListRealtimeStocks：查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]RealtimeStockListRow, 0, f.Limit)
	for rows.Next() {
		var row RealtimeStockListRow
		if err := rows.Scan(
			&row.StockID, &row.USC, &row.Name, &row.StockCode, &row.MarketType,
			&row.MarketLabel, &row.InstrumentType, &row.Currency, &row.PriceNominal,
			&row.ChangePrice, &row.ChangeRatio, &row.PriceDirect, &row.MarketValue,
			&row.TotalShares, &row.CirculationTotalShares, &row.TradeTrunover,
			&row.TradeVolumn, &row.CirculationMarketValue, &row.TrustAum,
			&row.TradeChangeRatio, &row.PriceAmplitude, &row.VolumnRatio,
			&row.BuysellRatio, &row.Leverage, &row.DtUpdate,
		); err != nil {
			return 0, nil, fmt.Errorf("ListRealtimeStocks：扫描失败: %w", err)
		}
		list = append(list, row)
	}
	return total, list, rows.Err()
}
