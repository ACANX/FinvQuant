package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrateIntegration 在真实 PostgreSQL 上执行全量迁移（默认跳过）。
//
// 需要一个**可清空的测试库**（迁移会创建 schema 与种子数据）：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:55432/finv_migtest?sslmode=disable" \
//	  go test -p 1 ./internal/database -run TestMigrateIntegration -v
//
// 注意：本测试与 internal/meta/finv/quote/dedup 的集成测试共用同一个测试库，
// 同时运行会并发执行迁移；批量运行时必须加 `-p 1` 串行。
func TestMigrateIntegration(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 与 internal/database.NewPool 保持一致：迁移依赖 search_path 定位历史种子表。
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("解析测试 DSN 失败: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,dedup,archive"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("测试库不可达: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("测试库不可达: %v", err)
	}

	dir := filepath.Join("..", "..", "Deploy", "Migrations")
	if err := Migrate(ctx, pool, dir); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	// 幂等：重复执行不应报错，也不应重复应用。
	if err := Migrate(ctx, pool, dir); err != nil {
		t.Fatalf("重复执行迁移失败（幂等性）: %v", err)
	}

	// V41：12 张归档表 + 12 个索引全部就绪。
	tables := []string{
		"finv_dedup_ingest_stats",
		"finv_dedup_daily_budget",
		"finv_dedup_scan_watermark",
		"finv_dedup_usc_priority",
		"finv_dedup_quote_maturity_mon",
		"finv_dedup_trade_calendar",
		"finv_dedup_job_run_log_export_file",
		"finv_dedup_usc",
		"finv_dedup_export_job",
		"finv_dedup_export_file",
		"finv_dedup_export_file_upload_log",
		"finv_dedup_export_file_patch",
	}
	for _, table := range tables {
		var exists bool
		if err := pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'dedup' AND tablename = $1)`,
			table).Scan(&exists); err != nil {
			t.Fatalf("查询表 %s 失败: %v", table, err)
		}
		if !exists {
			t.Fatalf("迁移后缺少表 dedup.%s", table)
		}
	}

	indexes := []string{
		"idx_dedupquotematuritymon_month_status",
		"idx_dedupquotematuritymon_status_usc",
		"idx_dedupjobrunlogexportfile_kind_time",
		"idx_dedupjobrunlogexportfile_job",
		"idx_dedupexportjob_pick",
		"idx_dedupexportfile_upload",
		"idx_dedupexportfile_network",
		"idx_dedupexportfile_job",
		"idx_dedupexportfileuploadlog_file",
		"idx_dedupexportfilepatch_pending",
		"idx_dedupexportfilepatch_job",
		"idx_dedupexportfilepatch_file",
	}
	for _, index := range indexes {
		var exists bool
		if err := pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'dedup' AND indexname = $1)`,
			index).Scan(&exists); err != nil {
			t.Fatalf("查询索引 %s 失败: %v", index, err)
		}
		if !exists {
			t.Fatalf("迁移后缺少索引 dedup.%s", index)
		}
	}

	// 全库无外键（A-05）。
	var foreignKeys int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM pg_constraint c
  JOIN pg_class r ON r.oid = c.conrelid
  JOIN pg_namespace n ON n.oid = r.relnamespace
 WHERE c.contype = 'f'`).Scan(&foreignKeys); err != nil {
		t.Fatalf("统计外键失败: %v", err)
	}
	if foreignKeys != 0 {
		t.Fatalf("全库不应存在外键，实际 %d 个", foreignKeys)
	}

	// 既有两张业务表结构未被 V41 改动：主表主键仍为 (ts, usc)。
	var primaryKey string
	if err := pool.QueryRow(ctx, `
SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c
 WHERE c.conrelid = 'dedup.finv_quote_secu_kline_min'::regclass AND c.contype = 'p'`).Scan(&primaryKey); err != nil {
		t.Fatalf("读取主表主键失败: %v", err)
	}
	if primaryKey != "PRIMARY KEY (ts, usc)" {
		t.Fatalf("主表主键被改动: %s", primaryKey)
	}
}
