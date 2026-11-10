package uscbackfill

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 本地来源表的只读访问。
//
// 回填方向是「本地 → 上游」，本地只读不写：本包不会修改 futu 下的任何表。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建读取器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// KeyLocalStat 本地证券键表（futu.finv_futu_secu_key）的统计。
type KeyLocalStat struct {
	Rows     int64 // 表行数
	WithUSC  int64 // usc 非空的行数（= 一次回推的待处理量）
	EmptyUSC int64 // usc 为空的行数（跳过）
}

// KeyLocalStat 统计本地证券键表：行数、usc 有值的行数、usc 为空的行数。
func (s *Store) KeyLocalStat(ctx context.Context) (KeyLocalStat, error) {
	var stat KeyLocalStat
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)::bigint,
		       count(*) FILTER (WHERE COALESCE(btrim(usc), '') <> '')::bigint,
		       count(*) FILTER (WHERE COALESCE(btrim(usc), '') = '')::bigint
		  FROM `+LocalKeyTable).Scan(&stat.Rows, &stat.WithUSC, &stat.EmptyUSC)
	if err != nil {
		return KeyLocalStat{}, fmt.Errorf("统计 %s 失败: %w", LocalKeyTable, err)
	}
	return stat, nil
}

// ReadLocalUSC 读出本地证券键表的 ftmm_symbol + usc + priority（只取 usc 非空的行），按 ftmm_symbol 升序。
//
// 实测 73,212 行（约 5 MB），一次读进内存即可；升序保证分批稳定，
// 便于日志与复跑比对。ftmm_symbol 是该表主键，天然唯一，无需归并去重。
func (s *Store) ReadLocalUSC(ctx context.Context) ([]LocalKeyRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ftmm_symbol, usc, priority,
		       COALESCE(btrim(instrument), ''),
		       COALESCE(btrim(market), ''),
		       COALESCE(btrim(code), ''),
		       stock_id,
		       COALESCE(btrim(status), ''),
		       flag_enable
		  FROM `+LocalKeyTable+`
		 WHERE COALESCE(btrim(usc), '') <> ''
		 ORDER BY ftmm_symbol`)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalKeyTable, err)
	}
	defer rows.Close()

	result := make([]LocalKeyRow, 0, 65536)
	for rows.Next() {
		var row LocalKeyRow
		if err := rows.Scan(&row.FtmmSymbol, &row.USC, &row.Priority,
			&row.Instrument, &row.Market, &row.Code, &row.StockID, &row.Status, &row.FlagEnable); err != nil {
			return nil, fmt.Errorf("解析 %s 行失败: %w", LocalKeyTable, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalKeyTable, err)
	}
	return result, nil
}
