package kline

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NullCounts 关键列的空值行数（数据完整性统计）。
type NullCounts struct {
	Date        int64 `json:"date"`
	Open        int64 `json:"open"`
	High        int64 `json:"high"`
	Low         int64 `json:"low"`
	Close       int64 `json:"close"`
	LastClose   int64 `json:"last_close"`
	ChangePrice int64 `json:"change_price"`
	ChangeRatio int64 `json:"change_ratio"`
	Volume      int64 `json:"volume"`
	Turnover    int64 `json:"turnover"`
	Provider    int64 `json:"provider"`
}

// StatsSummary 过滤范围内的汇总统计。
type StatsSummary struct {
	Rows      int64      `json:"rows"`
	UscCount  int64      `json:"usc_count"`
	DateFrom  *int       `json:"date_from"`
	DateTo    *int       `json:"date_to"`
	DateCount int64      `json:"date_count"`
	VolumeSum *string    `json:"volume_sum"`
	TurnSum   *string    `json:"turnover_sum"`
	CloseMin  *string    `json:"close_min"`
	CloseMax  *string    `json:"close_max"`
	CloseAvg  *string    `json:"close_avg"`
	Nulls     NullCounts `json:"null_counts"`
}

// DailyStat 按交易日期聚合的一行。
type DailyStat struct {
	Date      int     `json:"date"`
	Rows      int64   `json:"rows"`
	VolumeSum *string `json:"volume_sum"`
	TurnSum   *string `json:"turnover_sum"`
	CloseMin  *string `json:"close_min"`
	CloseMax  *string `json:"close_max"`
	CloseAvg  *string `json:"close_avg"`
}

// UscStat 按证券聚合的一行（按行数倒序，最多 20 条）。
type UscStat struct {
	Usc  string `json:"usc"`
	Rows int64  `json:"rows"`
}

// StatsResult /Stats 的完整返回。
type StatsResult struct {
	Summary StatsSummary `json:"summary"`
	Daily   []DailyStat  `json:"daily"`
	TopUsc  []UscStat    `json:"top_usc"`
}

// Stats 统计过滤范围内的去重分钟行情（只读事务内完成，保证三组结果口径一致）。
func (s *Service) Stats(ctx context.Context, filter ListFilter) (StatsResult, error) {
	result := StatsResult{Daily: []DailyStat{}, TopUsc: []UscStat{}}
	where, args := listWhere(filter)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	summarySQL := fmt.Sprintf(`
SELECT count(*), min(date), max(date),
       COALESCE(sum(volume), 0)::text, COALESCE(sum(turnover), 0)::text,
       round(min(close), 6)::text, round(max(close), 6)::text, round(avg(close), 6)::text,
       count(*) FILTER (WHERE date IS NULL),
       count(*) FILTER (WHERE open IS NULL),
       count(*) FILTER (WHERE high IS NULL),
       count(*) FILTER (WHERE low IS NULL),
       count(*) FILTER (WHERE close IS NULL),
       count(*) FILTER (WHERE last_close IS NULL),
       count(*) FILTER (WHERE change_price IS NULL),
       count(*) FILTER (WHERE change_ratio IS NULL),
       count(*) FILTER (WHERE volume IS NULL),
       count(*) FILTER (WHERE turnover IS NULL),
       count(*) FILTER (WHERE provider IS NULL)
FROM %s%s`, s.table, where)
	summary := &result.Summary
	if err := tx.QueryRow(ctx, summarySQL, args...).Scan(
		&summary.Rows, &summary.DateFrom, &summary.DateTo,
		&summary.VolumeSum, &summary.TurnSum,
		&summary.CloseMin, &summary.CloseMax, &summary.CloseAvg,
		&summary.Nulls.Date, &summary.Nulls.Open, &summary.Nulls.High, &summary.Nulls.Low,
		&summary.Nulls.Close, &summary.Nulls.LastClose, &summary.Nulls.ChangePrice,
		&summary.Nulls.ChangeRatio, &summary.Nulls.Volume, &summary.Nulls.Turnover,
		&summary.Nulls.Provider,
	); err != nil {
		return result, ClassifyDB(err)
	}

	// 按日期聚合（排除 date 为空的行，避免 GROUP BY NULL 影响「交易日数」口径）
	dailyWhere := where
	if dailyWhere == "" {
		dailyWhere = " WHERE date IS NOT NULL"
	} else {
		dailyWhere += " AND date IS NOT NULL"
	}
	dailySQL := fmt.Sprintf(`
SELECT date, count(*), COALESCE(sum(volume), 0)::text, COALESCE(sum(turnover), 0)::text,
       round(min(close), 6)::text, round(max(close), 6)::text, round(avg(close), 6)::text
FROM %s%s
GROUP BY date
ORDER BY date ASC`, s.table, dailyWhere)
	dailyRows, err := tx.Query(ctx, dailySQL, args...)
	if err != nil {
		return result, ClassifyDB(err)
	}
	for dailyRows.Next() {
		var item DailyStat
		if err := dailyRows.Scan(&item.Date, &item.Rows, &item.VolumeSum, &item.TurnSum,
			&item.CloseMin, &item.CloseMax, &item.CloseAvg); err != nil {
			dailyRows.Close()
			return result, ClassifyDB(err)
		}
		result.Daily = append(result.Daily, item)
	}
	if err := dailyRows.Err(); err != nil {
		dailyRows.Close()
		return result, ClassifyDB(err)
	}
	dailyRows.Close()

	topSQL := fmt.Sprintf(`
SELECT usc, count(*)
FROM %s%s
GROUP BY usc
ORDER BY count(*) DESC, usc ASC
LIMIT 20`, s.table, where)
	topRows, err := tx.Query(ctx, topSQL, args...)
	if err != nil {
		return result, ClassifyDB(err)
	}
	for topRows.Next() {
		var item UscStat
		if err := topRows.Scan(&item.Usc, &item.Rows); err != nil {
			topRows.Close()
			return result, ClassifyDB(err)
		}
		result.TopUsc = append(result.TopUsc, item)
	}
	if err := topRows.Err(); err != nil {
		topRows.Close()
		return result, ClassifyDB(err)
	}
	topRows.Close()

	// 交易日数与证券数从已聚合结果推导（避免全表 count(DISTINCT ...) 排序开销）；
	// 证券数在 Top 20 被截断时再补一次分组计数。
	summary.DateCount = int64(len(result.Daily))
	summary.UscCount = int64(len(result.TopUsc))
	if len(result.TopUsc) >= 20 {
		var uscCount int64
		if err := tx.QueryRow(ctx,
			fmt.Sprintf("SELECT count(*) FROM (SELECT usc FROM %s%s GROUP BY usc) t", s.table, where),
			args...).Scan(&uscCount); err != nil {
			return result, ClassifyDB(err)
		}
		summary.UscCount = uscCount
	}

	if err := tx.Commit(ctx); err != nil {
		return result, ClassifyDB(err)
	}
	return result, nil
}

// DateBounds 过滤范围内的交易日期边界（min/max date）。
type DateBounds struct {
	DateFrom *int `json:"date_from"`
	DateTo   *int `json:"date_to"`
}

// Bounds 查询过滤范围内的最小/最大交易日期（走 (usc,date,ts) 索引端点，耗时毫秒级）。
func (s *Service) Bounds(ctx context.Context, filter ListFilter) (DateBounds, error) {
	where, args := listWhere(filter)
	var bounds DateBounds
	err := s.pool.QueryRow(ctx,
		"SELECT min(date), max(date) FROM "+s.table+where, args...).
		Scan(&bounds.DateFrom, &bounds.DateTo)
	if err != nil {
		return bounds, ClassifyDB(err)
	}
	return bounds, nil
}
