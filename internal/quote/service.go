// Package quote 提供历史分钟行情导入服务（MVSV → PostgreSQL 字段级覆盖）。
package quote

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/mvsv"
)

// UpsertMode 覆盖模式。
type UpsertMode string

const (
	// UpsertModeField 字段级覆盖：非 NULL 覆盖，NULL 保留旧值（COALESCE）。
	UpsertModeField UpsertMode = "FIELD"
	// UpsertModeRow 整行覆盖：NULL 也覆盖。
	UpsertModeRow UpsertMode = "ROW"
)

// UpsertResult 导入结果统计。
type UpsertResult struct {
	BatchID     string `json:"batch_id"`
	SecuCode    string `json:"secu_code"`
	MarketCode  int    `json:"market_code"`
	RecordCount int    `json:"record_count"`
	Inserted    int64  `json:"inserted"`
	Updated     int64  `json:"updated"`
	Mode        string `json:"mode"`
}

// Service 历史行情导入与查询服务。
type Service struct {
	pool *pgxpool.Pool
}

// NewService 创建服务。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Bar 查询返回的分钟级 K 线（涨跌额/涨跌幅由 close 与 prev_close 计算）。
type Bar struct {
	TS        int64    `json:"ts"`         // UTC 时间戳（秒，主键列，如 1754398800）
	Date      int      `json:"date"`       // 交易日期 yyyymmdd（多日查询时用于区分）
	Time      int      `json:"time"`       // 交易时间 hhmmss
	Open      *string  `json:"open"`       // 开盘价
	High      *string  `json:"high"`       // 最高价
	Low       *string  `json:"low"`        // 最低价
	Close     *string  `json:"close"`      // 收盘价
	Volume    *int64   `json:"volume"`     // 成交量（有值才展示）
	Turnover  *string  `json:"turnover"`   // 成交额（有值才展示）
	Change    *string  `json:"change"`     // 涨跌额（close - prev_close）
	ChangePct *string  `json:"change_pct"` // 涨跌幅 %
	Remark    *string  `json:"remark"`     // 备注
}

// QueryBars 按证券代码 + ts 时间范围查询分钟级 K 线（周期目前仅支持 Min=1 分钟）。
// startTS/endTS 为 UTC 秒，取 [startTS, endTS] 闭区间内全部记录（不分页），
// 适配单日数据量可变的 A 股/港股/美股/24h 电子盘（如 GCMain 全天约 2181 根），
// 避免固定 page_size 截断；拖动窗口即改变 ts 范围重新查询。
// 返回按 ts 升序的全部 bars 与总条数 total。
func (s *Service) QueryBars(ctx context.Context, secuCode, period string, startTS, endTS int64) ([]Bar, int, error) {
	if secuCode == "" {
		return nil, 0, fmt.Errorf("secu_code 不能为空")
	}
	// 周期目前仅支持 1 分钟（Min），其他周期暂不支持
	if period != "" && period != "Min" {
		return nil, 0, fmt.Errorf("周期 %s 暂不支持，目前仅支持 Min（1 分钟）", period)
	}
	if startTS <= 0 || endTS < startTS {
		return nil, 0, fmt.Errorf("时间范围非法：start_ts=%d end_ts=%d", startTS, endTS)
	}

	var total int
	if err := s.pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM finv_quote_secu_kline_min
WHERE secu_code = $1 AND ts BETWEEN $2 AND $3`, secuCode, startTS, endTS).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT ts, date, "time", open, high, low, close, volume, turnover, prev_close, remark
FROM finv_quote_secu_kline_min
WHERE secu_code = $1 AND ts BETWEEN $2 AND $3
ORDER BY ts ASC`, secuCode, startTS, endTS)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bars []Bar
	for rows.Next() {
		var bar Bar
		var prevClose *string
		if err := rows.Scan(&bar.TS, &bar.Date, &bar.Time, &bar.Open, &bar.High, &bar.Low, &bar.Close,
			&bar.Volume, &bar.Turnover, &prevClose, &bar.Remark); err != nil {
			return nil, 0, err
		}
		bar.Change, bar.ChangePct = computeChange(bar.Close, prevClose)
		bars = append(bars, bar)
	}
	return bars, total, rows.Err()
}

// computeChange 计算涨跌额与涨跌幅（close - prev_close）。
func computeChange(closeVal, prevClose *string) (*string, *string) {
	if closeVal == nil || prevClose == nil {
		return nil, nil
	}
	closeNum, err1 := strconv.ParseFloat(*closeVal, 64)
	prevNum, err2 := strconv.ParseFloat(*prevClose, 64)
	if err1 != nil || err2 != nil || prevNum == 0 {
		return nil, nil
	}
	change := closeNum - prevNum
	changePct := change / prevNum * 100
	changeStr := fmt.Sprintf("%.4f", change)
	pctStr := fmt.Sprintf("%.4f", changePct)
	return &changeStr, &pctStr
}

// DateToTS 将 yyyymmdd 转为 UTC 秒（当日 00:00）。
func DateToTS(date int) int64 {
	year := date / 10000
	month := (date / 100) % 100
	day := date % 100
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Unix()
}

// DateRangeToTS 将 yyyymmdd + 回溯天数（自然日）转为 ts 范围（UTC 秒，闭区间）。
// 结束 = 该日 23:59:59，开始 = (结束日 - days + 1) 日 00:00:00。
func DateRangeToTS(date, days int) (int64, int64) {
	endDate := DateToTS(date)
	startDate := endDate - int64(days-1)*86400
	return startDate, endDate + 86400 - 1
}

// ImportRows 将解析后的行情行批量 upsert 到 finv_quote_secu_kline_min。
// 字段级覆盖（FIELD）或整行覆盖（ROW）；发生覆盖时写入修正审计日志。
// remark 为表单备注：非空时写入每行的 remark 列。
func (s *Service) ImportRows(ctx context.Context, rows []mvsv.Row, mode UpsertMode, importedBy, source, remark string) (*UpsertResult, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("无可导入的行情记录")
	}
	if mode == "" {
		mode = UpsertModeField
	}
	if remark != "" {
		for index := range rows {
			value := remark
			rows[index].Remark = &value
		}
	}
	batchID := fmt.Sprintf("IMPORT_%s_%s", rows[0].SecuCode, time.Now().UTC().Format("20060102150405"))

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sql := buildUpsertSQL(mode)
	inserted, updated, err := s.upsertBatch(ctx, tx, sql, rows)
	if err != nil {
		return nil, err
	}

	if updated > 0 {
		if err := s.logRevision(ctx, tx, batchID, rows[0], updated, mode, importedBy); err != nil {
			return nil, err
		}
	}
	if err := s.registerBatch(ctx, tx, batchID, rows[0], len(rows), mode, importedBy, source); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	// market_code 已从主表移除（V21）：从 finv_security 字典关联获取，保持响应字段兼容
	marketCode := 0
	if err := s.pool.QueryRow(ctx,
		`SELECT market_code FROM finv_security WHERE usc = $1 OR security_code = $1 LIMIT 1`,
		rows[0].SecuCode).Scan(&marketCode); err != nil {
		// 字典未登记时返回 0（前端展示“未维护”），不影响导入结果
		marketCode = 0
	}
	return &UpsertResult{
		BatchID:     batchID,
		SecuCode:    rows[0].SecuCode,
		MarketCode:  marketCode,
		RecordCount: len(rows),
		Inserted:    inserted,
		Updated:     updated,
		Mode:        string(mode),
	}, nil
}

// buildUpsertSQL 生成字段级覆盖（COALESCE）或整行覆盖的 upsert SQL。
// 使用 RETURNING (xmax = 0) 区分新增与覆盖行。
func buildUpsertSQL(mode UpsertMode) string {
	updatable := []string{
		"date", `"time"`, "prev_close", "open", "high", "low", "close",
		"paocd", "volume", "turnover", "ext_field", "remark",
	}
	base := `
INSERT INTO finv_quote_secu_kline_min
    (secu_code, ts, date, "time", prev_close, open, high, low, close,
     paocd, volume, turnover, ext_field, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (ts, secu_code) DO UPDATE SET
`
	var assignments []string
	for _, column := range updatable {
		if mode == UpsertModeRow {
			assignments = append(assignments, fmt.Sprintf("    %s = EXCLUDED.%s", column, column))
		} else {
			assignments = append(assignments,
				fmt.Sprintf("    %s = COALESCE(EXCLUDED.%s, finv_quote_secu_kline_min.%s)", column, column, column))
		}
	}
	return base + strings.Join(assignments, ",\n") + "\nRETURNING (xmax = 0) AS is_insert"
}

// upsertBatch 分批执行 upsert 并统计新增/覆盖行数（RETURNING (xmax=0)）。
func (s *Service) upsertBatch(
	ctx context.Context,
	tx pgx.Tx,
	sql string,
	rows []mvsv.Row,
) (int64, int64, error) {
	const batchSize = 2000
	var inserted, updated int64
	for offset := 0; offset < len(rows); offset += batchSize {
		end := offset + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := &pgx.Batch{}
		for _, row := range rows[offset:end] {
			batch.Queue(sql, rowParams(row)...)
		}
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < end-offset; i++ {
			queryRows, err := results.Query()
			if err != nil {
				_ = results.Close()
				return 0, 0, fmt.Errorf("upsert 执行失败: %w", err)
			}
			for queryRows.Next() {
				var isInsert bool
				if scanErr := queryRows.Scan(&isInsert); scanErr != nil {
					queryRows.Close()
					_ = results.Close()
					return 0, 0, scanErr
				}
				if isInsert {
					inserted++
				} else {
					updated++
				}
			}
			queryRows.Close()
		}
		if err := results.Close(); err != nil {
			return 0, 0, err
		}
	}
	return inserted, updated, nil
}

func rowParams(row mvsv.Row) []any {
	return []any{
		row.SecuCode,
		row.Ts,
		row.Date,
		row.Time,
		row.PrevClose,
		row.Open,
		row.High,
		row.Low,
		row.Close,
		row.Paocd,
		row.Volume,
		row.Turnover,
		row.ExtField,
		row.Remark,
	}
}

func (s *Service) logRevision(
	ctx context.Context,
	tx pgx.Tx,
	batchID string,
	row mvsv.Row,
	affected int64,
	mode UpsertMode,
	importedBy string,
) error {
	_, err := tx.Exec(ctx, `
INSERT INTO finv_quote_revision_log
    (ingest_batch_id, market_code, secu_code, affected_rows, reason, revised_by, previous_summary, new_summary)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		batchID, row.MarketCode, row.SecuCode, affected,
		"MVSV-1 导入（同键覆盖）", importedBy,
		fmt.Sprintf(`{"mode":%q,"rows":%d}`, mode, affected),
		fmt.Sprintf(`{"mode":%q}`, mode),
	)
	return err
}

func (s *Service) registerBatch(
	ctx context.Context,
	tx pgx.Tx,
	batchID string,
	row mvsv.Row,
	recordCount int,
	mode UpsertMode,
	importedBy, source string,
) error {
	_, err := tx.Exec(ctx, `
INSERT INTO finv_quote_ingest_batches
    (ingest_batch_id, source, market_code, secu_code, data_version_id, file_count,
     record_count, upsert_mode, ts_precision, config_hash, imported_by, notes)
VALUES ($1, $2, $3, $4, $5, 1, $6, $7, 'Second', $8, $9, $10)`,
		batchID, source, row.MarketCode, row.SecuCode,
		fmt.Sprintf("mvsv-%d-%s", time.Now().Unix(), row.SecuCode),
		recordCount, mode,
		fmt.Sprintf("mvsv-upload-%d", time.Now().Unix()),
		importedBy, "MVSV-1 上传导入",
	)
	return err
}
