package uscbackfill

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 本地源表（futu.finv_futu_quote_collect）的只读访问。
//
// 回填方向是「本地 → 上游」，本地只读不写：本包不会修改 futu 下的任何表。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建读取器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// LocalStat 本地源表统计（状态卡片）。
type LocalStat struct {
	Rows        int64 // 表行数
	StockIDs    int64 // 去重后的 stock_id 个数
	Placeholder int64 // usc = futu_symbol 的行数（占位行，仅提示用）
}

// LocalStat 统计本地源表：行数、去重 stock_id、占位行数。
func (s *Store) LocalStat(ctx context.Context) (LocalStat, error) {
	var stat LocalStat
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)::bigint,
		       count(DISTINCT stock_id)::bigint,
		       count(*) FILTER (WHERE usc = futu_symbol)::bigint
		  FROM `+LocalTable).Scan(&stat.Rows, &stat.StockIDs, &stat.Placeholder)
	if err != nil {
		return LocalStat{}, fmt.Errorf("统计 %s 失败: %w", LocalTable, err)
	}
	return stat, nil
}

// ReadLocalRows 读出本地源表全部行（usc / stock_id / futu_symbol），按 stock_id、usc 升序。
//
// 实测 24,418 行（约 2 MB），一次读进内存即可；上游回填按 stock_id 分批，
// 排序保证同一 stock_id 的行相邻、批次划分稳定（便于日志与复跑比对）。
func (s *Store) ReadLocalRows(ctx context.Context) ([]LocalRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT usc, stock_id, futu_symbol
		  FROM `+LocalTable+`
		 WHERE stock_id IS NOT NULL
		 ORDER BY stock_id, usc`)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalTable, err)
	}
	defer rows.Close()

	result := make([]LocalRow, 0, 32768)
	for rows.Next() {
		var row LocalRow
		if err := rows.Scan(&row.USC, &row.StockID, &row.FutuSymbol); err != nil {
			return nil, fmt.Errorf("解析 %s 行失败: %w", LocalTable, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalTable, err)
	}
	return result, nil
}
