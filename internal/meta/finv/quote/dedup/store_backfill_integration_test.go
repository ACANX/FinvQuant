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
	backfillUscTpl      = "ITB0001" // 命中 HK/HK 模板
	backfillUscKeep     = "ITB0002" // 已配置，补全后必须原值不变
	backfillUscNoMatch  = "ITB0003" // 模板未命中，只补优先级默认值
	backfillUscDisabled = "ITB0004" // 在 finv_security 里存在但 flag_enable='0'，仍应回填元数据
	backfillUscFutu     = "ITB0005" // 不在 finv_security，只能从 futu 镜像表兜底
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
		for _, usc := range []string{backfillUscTpl, backfillUscKeep, backfillUscNoMatch, backfillUscDisabled, backfillUscFutu} {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc = $1`, usc)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, usc)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.finv_security WHERE usc = $1`, usc)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM futu.finv_futu_quote_collect WHERE usc = $1`, usc)
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

	// 4. finv_security 里存在但 flag_enable='0'（V200062 导入的证券默认禁用）：
	//    usc 行的元数据仍应被回填（需求 10823：启用开关不限制元数据补全）。
	if _, err := pool.Exec(ctx, `
INSERT INTO public.finv_security
  (usc, exchange_code, market_code, security_type, security_code, security_name, security_name_cn,
   security_name_full, currency_type, provider, remark, init_date, timezone, tz, flag_enable)
VALUES ($1, 12, 1120, 'Stock', $1, '禁用证券', '禁用证券', '禁用证券', 'CNY', 'FTMM', 'it', 20000000, '+08:00', 'Asia/Shanghai', '0')`,
		backfillUscDisabled); err != nil {
		t.Fatalf("插入禁用证券主数据失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc (usc, flag_auto_discover) VALUES ($1, true)`, backfillUscDisabled); err != nil {
		t.Fatalf("插入禁用证券 usc 行失败: %v", err)
	}

	// 5. finv_security 未收录、只能从 futu 镜像表兜底的证券。
	if _, err := pool.Exec(ctx, `
INSERT INTO futu.finv_futu_quote_collect
  (usc, stock_id, futu_symbol, market_type, market_code, instrument_type, sub_instrument_type,
   quote_market, type_symbol, flag_enable, remark)
VALUES ($1, 1, $1, 1, 1, 3, 3002, 'HK', 'stock', '1', 'it')`, backfillUscFutu); err != nil {
		t.Fatalf("插入富途镜像行失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc (usc, flag_auto_discover) VALUES ($1, true)`, backfillUscFutu); err != nil {
		t.Fatalf("插入富途兜底 usc 行失败: %v", err)
	}

	// 6. 历史成熟度行：expected_rows 为 0（当初 day_expect_rows 未配置），
	//    补全 day_expect_rows 后必须被重算成模板值（需求 10826）。
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_quote_maturity_mon
  (usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status, gmt_first_seen, gmt_last_arrival)
VALUES ($1, 20260901, 202609, 0, 0, 'PARTIAL', now(), now())
ON CONFLICT (usc, trade_date) DO UPDATE SET expected_rows = 0, maturity_status = 'PARTIAL'`,
		backfillUscDisabled); err != nil {
		t.Fatalf("插入历史成熟度行失败: %v", err)
	}

	stat, err := store.BackfillUscDefaults(ctx, 0.95)
	if err != nil {
		t.Fatalf("补全默认值失败: %v", err)
	}
	if stat.DayExpectRows < 1 || stat.Priority < 2 {
		t.Fatalf("补全行数不符：day_expect_rows=%d（预期 ≥1）priority=%d（预期 ≥2）",
			stat.DayExpectRows, stat.Priority)
	}
	if stat.MetadataRows < 2 {
		t.Fatalf("元数据补全行数不符：%d（预期 ≥2，禁用证券 + 富途兜底）", stat.MetadataRows)
	}

	type uscState struct {
		dayExpect      *int
		priorityRank   *int
		slaDays        *int
		maxWaitDays    *int
		priorityGmtSet bool
		category       *string
		region         *string
		market         *string
		name           *string
		timezone       *string
		currency       *string
	}
	read := func(usc string) uscState {
		var state uscState
		if err := pool.QueryRow(ctx, `
SELECT day_expect_rows, priority_rank, sla_days, max_wait_days, priority_gmt_update IS NOT NULL,
       category, region, market, name, timezone, currency
  FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, usc).Scan(
			&state.dayExpect, &state.priorityRank, &state.slaDays, &state.maxWaitDays,
			&state.priorityGmtSet, &state.category, &state.region, &state.market,
			&state.name, &state.timezone, &state.currency); err != nil {
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

	// finv_security 里 flag_enable='0' 的证券：元数据仍被回填，且按 CN 区域兜底模板得到 241。
	disabled := read(backfillUscDisabled)
	if disabled.region == nil || *disabled.region != "CN" {
		t.Fatalf("%s 的 region 应为 CN（来自 exchange），实际 %v", backfillUscDisabled, disabled.region)
	}
	if disabled.category == nil || *disabled.category != "Stock" {
		t.Fatalf("%s 的 category 应为 Stock，实际 %v", backfillUscDisabled, disabled.category)
	}
	if disabled.name == nil || *disabled.name != "禁用证券" {
		t.Fatalf("%s 的 name 应回填，实际 %v", backfillUscDisabled, disabled.name)
	}
	if disabled.currency == nil || *disabled.currency != "CNY" || disabled.timezone == nil || *disabled.timezone != "Asia/Shanghai" {
		t.Fatalf("%s 的 currency/timezone 应回填，实际 %v/%v", backfillUscDisabled, disabled.currency, disabled.timezone)
	}
	if disabled.dayExpect == nil || *disabled.dayExpect != 241 {
		t.Fatalf("%s 的 day_expect_rows 应按 CN 区域兜底模板补 241（market 为空），实际 %v",
			backfillUscDisabled, disabled.dayExpect)
	}

	// 历史成熟度行的 expected_rows 必须跟着重算成 241（而不是继续停在 0）。
	var maturityExpected int
	var maturityStatus string
	if err := pool.QueryRow(ctx, `
SELECT expected_rows, maturity_status FROM cdeu.finv_quote_cdeu_quote_maturity_mon
 WHERE usc = $1 AND trade_date = 20260901`, backfillUscDisabled).Scan(&maturityExpected, &maturityStatus); err != nil {
		t.Fatalf("读取成熟度行失败: %v", err)
	}
	if maturityExpected != 241 {
		t.Fatalf("%s 的历史成熟度 expected_rows 应重算为 241，实际 %d", backfillUscDisabled, maturityExpected)
	}
	if maturityStatus != MaturityPartial {
		t.Fatalf("%s 的成熟度状态应为 PARTIAL（actual=0），实际 %s", backfillUscDisabled, maturityStatus)
	}

	// finv_security 未收录：从 futu 镜像表兜底 region/market/category，再按 HK/HK 模板补 331。
	futuRow := read(backfillUscFutu)
	if futuRow.region == nil || *futuRow.region != "HK" || futuRow.market == nil || *futuRow.market != "HK" {
		t.Fatalf("%s 的 region/market 应从富途镜像表兜底为 HK/HK，实际 %v/%v",
			backfillUscFutu, futuRow.region, futuRow.market)
	}
	if futuRow.category == nil || *futuRow.category != "Stock" {
		t.Fatalf("%s 的 category 应由 type_symbol 映射为 Stock，实际 %v", backfillUscFutu, futuRow.category)
	}
	if futuRow.dayExpect == nil || *futuRow.dayExpect != 331 {
		t.Fatalf("%s 的 day_expect_rows 应为 331，实际 %v", backfillUscFutu, futuRow.dayExpect)
	}

	// 6. 幂等：再次执行影响 0 行。
	stat, err = store.BackfillUscDefaults(ctx, 0.95)
	if err != nil {
		t.Fatalf("重复补全失败: %v", err)
	}
	if stat.DayExpectRows != 0 || stat.Priority != 0 || stat.MetadataRows != 0 || stat.DerivedRows != 0 || stat.MaturityRows != 0 {
		t.Fatalf("重复补全应幂等（影响 0 行），实际 metadata=%d derived=%d day_expect_rows=%d priority=%d maturity=%d",
			stat.MetadataRows, stat.DerivedRows, stat.DayExpectRows, stat.Priority, stat.MaturityRows)
	}
}
