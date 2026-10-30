package dedup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/database"
)

// 默认值补全集成测试专用证券编码（测试前后都会清理）。
const (
	backfillUscTpl     = "ITB0001" // 命中 HK/HK 模板
	backfillUscKeep    = "ITB0002" // 已配置，补全后必须原值不变
	backfillUscNoMatch = "ITB0003" // 模板未命中，只补优先级默认值
)

// TestBackfillUscDefaults 在真实 PostgreSQL 上验证模板补全（默认跳过）。
//
//	go test -p 1 ./internal/meta/finv/quote/dedup -run TestBackfillUscDefaults -v
//
// 需要一个**可清空的测试库**（会执行全量迁移），DSN 见 TestStoreIntegration。
func TestBackfillUscDefaults(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("解析测试 DSN 失败: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,cdeu,archive,sys"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "..", "..", "..", "Deploy", "Migrations")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	store := NewStore(pool)
	cleanup := func(cleanupCtx context.Context) {
		for _, usc := range []string{backfillUscTpl, backfillUscKeep, backfillUscNoMatch} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, usc)
		}
	}
	cleanup(ctx)
	defer cleanup(context.Background())

	// 1. 缺 day_expect_rows 与优先级：按 HK/HK 模板补 331，优先级补 100 / 5 / 7。
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc (usc, category, region, market, provider, name, timezone, currency)
VALUES ($1, 'Stock', 'HK', 'HK', 'FTMM', '补全测试证券', 'Asia/Hong_Kong', 'HKD')`, backfillUscTpl); err != nil {
		t.Fatalf("插入待补全证券失败: %v", err)
	}

	// 2. 已配置的证券：补全后必须保持原值。
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc
  (usc, category, day_expect_rows, region, market, provider, name, timezone, currency,
   priority_rank, sla_days, max_wait_days, priority_gmt_update)
VALUES ($1, 'Stock', 999, 'CN', 'SH', 'FTMM', '保留值证券', 'Asia/Shanghai', 'CNY', 1, 2, 3, now())`,
		backfillUscKeep); err != nil {
		t.Fatalf("插入已配置证券失败: %v", err)
	}

	// 3. 模板未命中（region/market 无对应模板）：day_expect_rows 保持 NULL，优先级仍补默认值。
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc (usc, category, region, market, provider, name, timezone, currency)
VALUES ($1, 'Stock', 'ZZ', 'ZZ', 'FTMM', '未命中证券', 'UTC', 'USD')`, backfillUscNoMatch); err != nil {
		t.Fatalf("插入未命中证券失败: %v", err)
	}

	stat, err := store.BackfillUscDefaults(ctx)
	if err != nil {
		t.Fatalf("补全默认值失败: %v", err)
	}
	if stat.DayExpectRows < 1 || stat.Priority < 2 {
		t.Fatalf("补全行数不符：day_expect_rows=%d（预期 ≥1）priority=%d（预期 ≥2）",
			stat.DayExpectRows, stat.Priority)
	}

	type uscState struct {
		dayExpect      *int
		priorityRank   *int
		slaDays        *int
		maxWaitDays    *int
		priorityGmtSet bool
	}
	read := func(usc string) uscState {
		var state uscState
		if err := pool.QueryRow(ctx, `
SELECT day_expect_rows, priority_rank, sla_days, max_wait_days, priority_gmt_update IS NOT NULL
  FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, usc).Scan(
			&state.dayExpect, &state.priorityRank, &state.slaDays, &state.maxWaitDays,
			&state.priorityGmtSet); err != nil {
			t.Fatalf("读取证券 %s 失败: %v", usc, err)
		}
		return state
	}

	if got := read(backfillUscTpl); got.dayExpect == nil || *got.dayExpect != 331 {
		t.Fatalf("%s 的 day_expect_rows 应为模板值 331，实际 %v", backfillUscTpl, got.dayExpect)
	}
	if got := read(backfillUscTpl); got.priorityRank == nil || *got.priorityRank != defaultPriorityRank ||
		got.slaDays == nil || *got.slaDays != defaultSlaDays ||
		got.maxWaitDays == nil || *got.maxWaitDays != defaultMaxWaitDays {
		t.Fatalf("%s 的优先级/SLA 应补默认值 100/5/7，实际 %v/%v/%v",
			backfillUscTpl, got.priorityRank, got.slaDays, got.maxWaitDays)
	}
	// 自动补全不写 priority_gmt_update，用于区分「默认值」与「人工设置」。
	if got := read(backfillUscTpl); got.priorityGmtSet {
		t.Fatalf("%s 的 priority_gmt_update 不应被自动补全写入", backfillUscTpl)
	}

	if got := read(backfillUscKeep); got.dayExpect == nil || *got.dayExpect != 999 ||
		got.priorityRank == nil || *got.priorityRank != 1 {
		t.Fatalf("%s 的已配置值被覆盖：day_expect_rows=%v priority=%v",
			backfillUscKeep, got.dayExpect, got.priorityRank)
	}

	if got := read(backfillUscNoMatch); got.dayExpect != nil {
		t.Fatalf("%s 未命中模板时 day_expect_rows 应保持 NULL，实际 %v", backfillUscNoMatch, got.dayExpect)
	}

	// 4. 幂等：再次执行影响 0 行。
	stat, err = store.BackfillUscDefaults(ctx)
	if err != nil {
		t.Fatalf("重复补全失败: %v", err)
	}
	if stat.DayExpectRows != 0 || stat.Priority != 0 {
		t.Fatalf("重复补全应幂等（影响 0 行），实际 day_expect_rows=%d priority=%d",
			stat.DayExpectRows, stat.Priority)
	}
}
