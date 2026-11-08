package uscbackfill

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store 本地两张来源表的只读访问。
//
// 回填方向是「本地 → 上游」，本地只读不写：本包不会修改 futu 下的任何表。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建读取器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// MetaLocalStat 表 ① 来源（futu.finv_futu_quote_collect）的统计。
type MetaLocalStat struct {
	Rows        int64 // 表行数
	StockIDs    int64 // 去重后的 stock_id 个数
	Placeholder int64 // usc = futu_symbol 的行数（占位行，仅提示用）
}

// KeyLocalStat 表 ② 来源（futu.finv_futu_secu_key ⋈ quote_collect）的统计。
type KeyLocalStat struct {
	Rows       int64 // 本地证券键表行数
	Resolvable int64 // 能同时定出 usc 与 stock_id 的 ftmm_symbol 个数（去重）
}

// MetaLocalStat 统计表 ① 来源：行数、去重 stock_id、占位行数。
func (s *Store) MetaLocalStat(ctx context.Context) (MetaLocalStat, error) {
	var stat MetaLocalStat
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)::bigint,
		       count(DISTINCT stock_id)::bigint,
		       count(*) FILTER (WHERE usc = futu_symbol)::bigint
		  FROM `+LocalMetaTable).Scan(&stat.Rows, &stat.StockIDs, &stat.Placeholder)
	if err != nil {
		return MetaLocalStat{}, fmt.Errorf("统计 %s 失败: %w", LocalMetaTable, err)
	}
	return stat, nil
}

// KeyLocalStat 统计表 ② 来源：本地键表行数与可回填（能定出 usc + stock_id）的键数。
func (s *Store) KeyLocalStat(ctx context.Context) (KeyLocalStat, error) {
	var stat KeyLocalStat
	err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*)::bigint FROM `+LocalKeyTable+`),
		       (SELECT count(DISTINCT k.ftmm_symbol)::bigint
		          FROM `+LocalKeyTable+` k
		          JOIN `+LocalMetaTable+` q ON k.ftmm_symbol = `+KeySymbolExpr+`
		         WHERE COALESCE(btrim(q.usc), '') <> '')`).
		Scan(&stat.Rows, &stat.Resolvable)
	if err != nil {
		return KeyLocalStat{}, fmt.Errorf("统计 %s 可回填行数失败: %w", LocalKeyTable, err)
	}
	return stat, nil
}

// ReadLocalRows 读出表 ① 来源的全部行（usc / stock_id / futu_symbol），按 stock_id、usc 升序。
//
// 实测 24,418 行（约 2 MB），一次读进内存即可；排序保证同一 stock_id 的行相邻、
// 批次划分稳定（便于日志与复跑比对）。
func (s *Store) ReadLocalRows(ctx context.Context) ([]LocalRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT usc, stock_id, futu_symbol
		  FROM `+LocalMetaTable+`
		 WHERE stock_id IS NOT NULL
		 ORDER BY stock_id, usc`)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalMetaTable, err)
	}
	defer rows.Close()

	result := make([]LocalRow, 0, 32768)
	for rows.Next() {
		var row LocalRow
		if err := rows.Scan(&row.USC, &row.StockID, &row.FutuSymbol); err != nil {
			return nil, fmt.Errorf("解析 %s 行失败: %w", LocalMetaTable, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", LocalMetaTable, err)
	}
	return result, nil
}

// KeySymbolExpr 由本地采集属性表重建 ftmm_symbol 的表达式（需求 11057）。
//
// 上游键表的 ftmm_symbol = `{market}.{code}`，本地采集属性表把它拆成 market_label + futu_symbol
// （如 US.AAPL → market_label=US、futu_symbol=AAPL；CC.DASHTRY.FTUS → CC + DASHTRY.FTUS），
// 二者用 `market_label || '.' || futu_symbol` 可精确还原（实测命中 24,162 / 73,212 个键）。
const KeySymbolExpr = "q.market_label || '.' || q.futu_symbol"

// ReadLocalKeyRows 读出表 ② 来源：本地键表（决定候选键集）⋈ 采集属性表（提供 usc + stock_id）。
//
// 关联口径是**重建出的 ftmm_symbol**（见 KeySymbolExpr），比按 stock_id 关联覆盖更广
// （实测 24,162 行 vs 10,910 行，后者是前者的子集）；没有 stock_id 的键也能补上。
// 其余键本地无依据（实测 49,050 行），计入 KeyUnresolved 跳过。
func (s *Store) ReadLocalKeyRows(ctx context.Context) ([]LocalKeyRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT k.ftmm_symbol, q.usc, q.stock_id
		  FROM `+LocalKeyTable+` k
		  JOIN `+LocalMetaTable+` q ON k.ftmm_symbol = `+KeySymbolExpr+`
		 WHERE COALESCE(btrim(q.usc), '') <> ''
		 ORDER BY k.ftmm_symbol, q.usc`)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 可回填行失败: %w", LocalKeyTable, err)
	}
	defer rows.Close()

	result := make([]LocalKeyRow, 0, 16384)
	for rows.Next() {
		var row LocalKeyRow
		if err := rows.Scan(&row.FtmmSymbol, &row.USC, &row.StockID); err != nil {
			return nil, fmt.Errorf("解析 %s 行失败: %w", LocalKeyTable, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取 %s 可回填行失败: %w", LocalKeyTable, err)
	}
	return result, nil
}
