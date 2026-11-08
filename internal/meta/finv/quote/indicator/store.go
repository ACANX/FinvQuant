package indicator

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 三张本地目标表的批量写入与统计（一次往返写三张表，SQL 见 sql.go）。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建写入器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// UpsertStat 一批写入的统计（只统计**实际改动**的行）。
type UpsertStat struct {
	IndInsert int // 指标表新增
	IndUpdate int // 指标表更新
	QcInsert  int // 采集属性表新增
	QcUpdate  int // 采集属性表更新
	QcSkip    int // 采集属性表因 (market_code, futu_symbol) 被别的 usc 占用而跳过
	KeyUpdate int // 证券键表补全 url / stock_id 的行数
}

// UpsertBatch 写入一批源行（payload 为上游原始 JSON 数组）。
//
// 三张表的写入语义见包注释；返回「实际新增 / 实际更新 / 跳过」计数，
// 完全一致的行不会被改写（因此也不在计数里）。
func (s *Store) UpsertBatch(ctx context.Context, payload []byte) (UpsertStat, error) {
	var stat UpsertStat
	if len(payload) == 0 {
		return stat, nil
	}
	err := s.pool.QueryRow(ctx, upsertBatchSQL, string(payload)).Scan(
		&stat.IndInsert, &stat.IndUpdate,
		&stat.QcInsert, &stat.QcUpdate, &stat.QcSkip,
		&stat.KeyUpdate,
	)
	if err != nil {
		return UpsertStat{}, fmt.Errorf("写入 %s / %s / %s 失败: %w",
			IndicatorTable, QuoteCollectTable, SecuKeyTable, err)
	}
	return stat, nil
}

// TableStats 三张目标表的当前行数（状态卡片用）。
func (s *Store) TableStats(ctx context.Context) ([]TableStat, error) {
	stats := make([]TableStat, 0, 3)
	for _, table := range []string{IndicatorTable, QuoteCollectTable, SecuKeyTable} {
		var rows int64
		// 表名来自本包常量（非用户输入），不存在注入风险。
		if err := s.pool.QueryRow(ctx, "SELECT count(*)::bigint FROM "+table).Scan(&rows); err != nil {
			return nil, fmt.Errorf("统计 %s 失败: %w", table, err)
		}
		stats = append(stats, TableStat{Table: table, Rows: rows})
	}
	return stats, nil
}
