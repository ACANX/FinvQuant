// Package database 提供 PostgreSQL 18 连接池（pgx/v5）。
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/config"
)

// NewPool 建立 PG 连接池并 Ping 验证。
//
// search_path 固定为 public, quote, quant, futu, sche, cdeu, archive, sys：
//   - 业务表的 schema 归属：finv_quote_* → quote、finv_quant_* → quant、
//     finv_futu_* → futu、调度域 → sche、CDEU 工作流域 → cdeu、
//     归档域 → archive、系统域（用户/权限/审计等）→ sys，其余基础主数据表保留 public；
//   - `export` 与 `dedup` 已分别由 V200089（中转表迁入 cdeu 后删空壳）与 V200086
//     （需求 10834）删除，不再列入 search_path；
//   - 历史种子迁移（V200xxx）的 SQL 写的是不带 schema 的表名，需靠 search_path 解析；
//   - 这些 schema 尚未创建时（首次建库 V200001 起），PostgreSQL 会静默忽略不存在的 schema。
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_max_conns=10&search_path=public,quote,quant,futu,sche,cdeu,archive,sys",
		cfg.PgUser, cfg.PgPassword, cfg.PgHost, cfg.PgPort, cfg.PgDatabase,
	)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("创建 PG 连接池失败: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PG 连通性检查失败: %w", err)
	}
	return pool, nil
}
