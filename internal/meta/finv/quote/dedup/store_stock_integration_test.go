package dedup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestExportStockPage 验证「月份存量清单」的取数与统计口径（需求 10835）。
//
// 场景覆盖三类证券月：
//  1. 主表有存量 + 成熟度完整 + 未导出  → 存量行数、交易日、预计大小、预计文件名；
//  2. 主表已清理 + 成熟度有记录 + 已导出 → stock_rows=0、cleaned_secu 计数、导出状态与已登记文件名；
//  3. 成熟度未达标                      → maturity_status=PARTIAL 且 unmature_days>0。
//
// 需要一个可写的测试库（会执行全量迁移），DSN 见 FINV_TEST_PG_DSN。
func TestExportStockPage(t *testing.T) {
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

	const bizMonth = 209912
	const (
		uscStock   = "ITSTK_STOCK"
		uscCleaned = "ITSTK_CLEAN"
		uscPartial = "ITSTK_PART"
	)
	cleanup := func(cleanupCtx context.Context) {
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_secu_kline_min WHERE usc LIKE 'ITSTK_%'`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_export_file WHERE usc LIKE 'ITSTK_%'`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_export_job WHERE usc LIKE 'ITSTK_%'`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc LIKE 'ITSTK_%'`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc LIKE 'ITSTK_%'`)
	}
	cleanup(ctx)
	defer cleanup(context.Background())

	// 证券主数据
	for _, item := range []struct{ usc, name string }{
		{uscStock, "存量证券"}, {uscCleaned, "已清理证券"}, {uscPartial, "未成熟证券"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc (usc, category, region, market, provider, name, timezone, currency)
VALUES ($1, 'Stock', 'US', 'NSDQ', 'FTMM', $2, 'America/New_York', 'USD')`, item.usc, item.name); err != nil {
			t.Fatalf("插入证券 %s 失败: %v", item.usc, err)
		}
	}

	// ① 主表存量：3 个交易日 × 2 行
	for day := 1; day <= 3; day++ {
		for slot := 1; slot <= 2; slot++ {
			if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_secu_kline_min (usc, ts, date, open, close, provider)
VALUES ($1, $2, $3, 1, 2, 'FTMM')`,
				uscStock, int64(bizMonth)*100000+int64(day)*1000+int64(slot), bizMonth*100+day); err != nil {
				t.Fatalf("插入主表存量失败: %v", err)
			}
		}
	}

	// ③ 未成熟证券在主表也有 1 天数据（只到这 1 天，所以未成熟），否则它会被算成"已清理"。
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_secu_kline_min (usc, ts, date, open, close, provider)
VALUES ($1, $2, $3, 1, 2, 'FTMM')`,
		uscPartial, int64(bizMonth)*100000+1001, bizMonth*100+1); err != nil {
		t.Fatalf("插入未成熟证券主表数据失败: %v", err)
	}

	// 成熟度：① 完整 3 天；② 已清理但成熟度有记录；③ 有 1 天未达标
	maturity := []struct {
		usc       string
		status    string
		expected  int
		actual    int
		tradeDays []int
	}{
		{uscStock, MaturityComplete, 100, 100, []int{1, 2, 3}},
		{uscCleaned, MaturityFrozen, 200, 200, []int{1, 2, 3}},
		{uscPartial, MaturityPartial, 300, 150, []int{1, 2}},
	}
	for _, item := range maturity {
		for _, day := range item.tradeDays {
			if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_quote_maturity_mon
  (usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status)
VALUES ($1, $2, $3, $4, $5, $6)`,
				item.usc, bizMonth*100+day, bizMonth, item.expected, item.actual, item.status); err != nil {
				t.Fatalf("插入成熟度失败: %v", err)
			}
		}
	}
	// ③ 第 3 天未达标（覆盖 PARTIAL 的 unmature_days 统计）
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_quote_maturity_mon
  (usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status)
VALUES ($1, $2, $3, 300, 10, 'IMMATURE')`, uscPartial, bizMonth*100+3, bizMonth); err != nil {
		t.Fatalf("插入未成熟行失败: %v", err)
	}

	// ② 已导出的任务与文件（主表已清理的那只）
	var jobID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_export_job
  (usc, biz_month, priority_rank, freeze_date, seal_date, status, oldest_pending_date)
VALUES ($1, $2, 100, $3::date, $4::date, 'EXPORTED', $5) RETURNING job_id`,
		uscCleaned, bizMonth, "2100-01-31", "2100-01-25", bizMonth*100+1).Scan(&jobID); err != nil {
		t.Fatalf("插入导出任务失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_export_file
  (job_id, usc, biz_month, file_type, version, file_name, file_path, target_file_name,
   format, compression, row_count, file_size, checksum, export_status)
VALUES ($1, $2, $3, 'BASE', 1, 'US_NSDQ_ITSTK_CLEAN_Min_FTMM_Mon_209912_Base.mvsv',
        '/tmp/x.mvsv', 'US_NSDQ_ITSTK_CLEAN_Min_FTMM_Mon_209912_Base.mvsv',
        'mvsv', 'none', 600, 12345, 'x', 'EXPORTED')`,
		jobID, uscCleaned, bizMonth); err != nil {
		t.Fatalf("插入导出文件失败: %v", err)
	}

	store := NewStore(pool)
	filter := StockFilter{BizMonth: bizMonth, Limit: 50}
	page, err := store.ExportStockPage(ctx, filter)
	if err != nil {
		t.Fatalf("查询存量清单失败: %v", err)
	}
	if page.TotalSecu != 3 {
		t.Fatalf("证券数期望 3，实际 %d（items=%d）", page.TotalSecu, len(page.Items))
	}
	if page.TotalStockRows != 7 {
		t.Fatalf("总存量行数期望 7，实际 %d", page.TotalStockRows)
	}
	if page.CleanedSecu != 1 {
		t.Fatalf("已清理证券数期望 1，实际 %d", page.CleanedSecu)
	}
	if page.TotalEstBytes <= 0 {
		t.Fatalf("预计总大小应大于 0，实际 %d", page.TotalEstBytes)
	}

	byUsc := map[string]StockRow{}
	for _, item := range page.Items {
		byUsc[item.Usc] = item
	}

	stock := byUsc[uscStock]
	if stock.StockRows != 6 || stock.StockDays != 3 {
		t.Fatalf("存量证券口径不符：rows=%d days=%d（期望 6/3）", stock.StockRows, stock.StockDays)
	}
	if stock.EstBytes <= 0 {
		t.Fatalf("存量证券预计大小应大于 0")
	}
	const wantFile = "US_NSDQ_ITSTK_STOCK_Min_FTMM_Mon_209912_Base.mvsv"
	if stock.FileName != wantFile {
		t.Fatalf("未导出证券应给出预计文件名 %s，实际 %s", wantFile, stock.FileName)
	}
	if stock.MaturityStatus != MaturityComplete || stock.UnmatureDays != 0 {
		t.Fatalf("存量证券成熟度应为 COMPLETE 且 0 天未达标，实际 %s/%d",
			stock.MaturityStatus, stock.UnmatureDays)
	}
	if stock.ExportStatus != "" {
		t.Fatalf("存量证券不应有导出状态，实际 %s", stock.ExportStatus)
	}

	cleaned := byUsc[uscCleaned]
	if cleaned.StockRows != 0 {
		t.Fatalf("已清理证券主表行数应为 0，实际 %d", cleaned.StockRows)
	}
	if cleaned.ExportStatus != JobStatusExported || cleaned.JobID == nil || *cleaned.JobID != jobID {
		t.Fatalf("已清理证券应带出导出状态与 job_id，实际 status=%s job=%v", cleaned.ExportStatus, cleaned.JobID)
	}
	if cleaned.FileName != "US_NSDQ_ITSTK_CLEAN_Min_FTMM_Mon_209912_Base.mvsv" {
		t.Fatalf("已清理证券应带出已登记文件名，实际 %s", cleaned.FileName)
	}
	if cleaned.ActualRows != 600 {
		t.Fatalf("已清理证券实际行数期望 600，实际 %d", cleaned.ActualRows)
	}

	partial := byUsc[uscPartial]
	// 该证券三天成熟度记录分别是 PARTIAL / PARTIAL / IMMATURE，三者都不算 COMPLETE/FROZEN，
	// 所以「未达标天数」= 3，整体状态判定为 PARTIAL。
	if partial.MaturityStatus != MaturityPartial || partial.UnmatureDays != 3 {
		t.Fatalf("未成熟证券应为 PARTIAL/3 天未达标，实际 %s/%d",
			partial.MaturityStatus, partial.UnmatureDays)
	}

	// 分页与过滤
	paged, err := store.ExportStockPage(ctx, StockFilter{BizMonth: bizMonth, Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("分页查询失败: %v", err)
	}
	if len(paged.Items) != 1 || paged.TotalSecu != 3 {
		t.Fatalf("分页失效：items=%d total=%d", len(paged.Items), paged.TotalSecu)
	}
	filtered, err := store.ExportStockPage(ctx, StockFilter{BizMonth: bizMonth, Usc: "ITSTK_PART"})
	if err != nil {
		t.Fatalf("按证券过滤失败: %v", err)
	}
	if filtered.TotalSecu != 1 || filtered.Items[0].Usc != uscPartial {
		t.Fatalf("按证券过滤失效：total=%d", filtered.TotalSecu)
	}
	t.Logf("清单：证券=%d 存量行=%d 预计大小=%d 已清理=%d",
		page.TotalSecu, page.TotalStockRows, page.TotalEstBytes, page.CleanedSecu)
}
