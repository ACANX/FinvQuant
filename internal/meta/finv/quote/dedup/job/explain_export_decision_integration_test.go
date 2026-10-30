package job_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/acanx/finvquant/internal/database"
	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	_ "github.com/acanx/finvquant/internal/meta/finv/quote/dedup/job"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestExplainExportDecisionEndToEnd 端到端验证「导出决策明细」JOB（需求 10832）。
//
// 验证三条链路，缺一条运维就看不到明细：
//  1. JOB 能被调度入口 RunWithTrigger 真正执行（executor 注册、CDEU 登记、dispatch 都通）；
//  2. 逐条结论真正写进运行日志（前端「执行日志」弹窗读的就是这张表）；
//  3. result.Details 里带出完整逐条清单（同步调用 Job/Run 时随响应返回）。
//
// 需要一个可写的测试库（会执行全量迁移），DSN 见 FINV_TEST_PG_DSN。
func TestExplainExportDecisionEndToEnd(t *testing.T) {
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
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "..", "..", "..", "..", "Deploy", "Migrations")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载业务时区失败: %v", err)
	}
	options := dedup.Options{
		Location:     location,
		TimeZone:     "Asia/Shanghai",
		OutputDir:    t.TempDir(),
		UploadLayout: dedup.UploadLayoutBySecurity,
		UploadPrefix: "Data/Finv/SecuQuoteMon/FTMM",
	}
	service, err := dedup.NewService(pool, options)
	if err != nil {
		t.Fatalf("构造 Service 失败: %v", err)
	}

	// 造一条「必然可导出」的证券月：元数据齐全、已成熟、未导出过。
	// 否则当前库里 101 个证券月全部 ALREADY_EXPORTED，端到端永远覆盖不到
	// 「能导出时给出多少条、文件名叫什么、保存在哪里」这条最关键的链路。
	const (
		probeUsc      = "ITEXP_PROBE"
		probeBizMonth = 202501
		probeTradeDay = 20250102
		probeRows     = 1440
	)
	cleanupProbe := func(cleanupCtx context.Context) {
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_export_job WHERE usc = $1`, probeUsc)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc = $1`, probeUsc)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`, probeUsc)
	}
	cleanupProbe(ctx)
	defer cleanupProbe(context.Background())
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc
  (usc, category, region, market, provider, name, timezone, currency,
   day_expect_rows, priority_rank, sla_days, max_wait_days)
VALUES ($1, 'Stock', 'US', 'NSDQ', 'FTMM', '端到端探针', 'America/New_York', 'USD',
        $2, 10, 5, 7)`, probeUsc, probeRows); err != nil {
		t.Fatalf("插入探针证券失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_quote_maturity_mon
  (usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status)
VALUES ($1, $2, $3, $4, $4, 'COMPLETE')`, probeUsc, probeTradeDay, probeBizMonth, probeRows); err != nil {
		t.Fatalf("插入探针成熟度失败: %v", err)
	}

	startedAt := time.Now()
	result, runErr := service.RunWithTrigger(ctx, dedup.JobExplainExportDecision, dedup.CDEUTriggerManual)
	if runErr != nil {
		t.Fatalf("执行 %s 失败: %v", dedup.JobExplainExportDecision, runErr)
	}
	if result.Status != dedup.RunStatusSuccess {
		t.Fatalf("运行状态应为 SUCCESS，实际 %s（message=%s）", result.Status, result.Message)
	}
	t.Logf("运行结果 status=%s processed_rows=%d message=%s", result.Status, result.ProcessedRows, result.Message)

	// ② 逐条结论必须落进运行日志（前端「执行日志」弹窗数据源）。
	// Result 不回传 run_id，按「本次开始时间之后该 kind 的最新一次运行」定位。
	var runID int64
	if err := pool.QueryRow(ctx, `
SELECT run_id FROM cdeu.finv_quote_cdeu_job_run_log_export_file
 WHERE job_kind = $1 AND gmt_start >= $2
 ORDER BY run_id DESC LIMIT 1`, dedup.JobExplainExportDecision, startedAt).Scan(&runID); err != nil {
		t.Fatalf("查询本次运行记录失败: %v", err)
	}
	var lineCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM cdeu.finv_quote_cdeu_job_run_log_line
		  WHERE run_id = $1 AND message LIKE '%导出决策%'`, runID).Scan(&lineCount); err != nil {
		t.Fatalf("查询运行日志行失败: %v", err)
	}
	if lineCount == 0 {
		t.Fatalf("运行日志里没有任何「导出决策」明细行（run_id=%d），前端执行日志将看不到原因", runID)
	}
	t.Logf("run_id=%d 运行日志明细行数（含「导出决策」）= %d", runID, lineCount)

	rows, err := pool.Query(ctx,
		`SELECT level, message FROM cdeu.finv_quote_cdeu_job_run_log_line
		  WHERE run_id = $1 AND message LIKE '%导出决策%'
		  ORDER BY line_id LIMIT 5`, runID)
	if err != nil {
		t.Fatalf("读取运行日志明细失败: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var level, message string
		if err := rows.Scan(&level, &message); err != nil {
			t.Fatalf("扫描日志行失败: %v", err)
		}
		t.Logf("  [%s] %s", level, message)
	}

	// ③ result.Details 要带出完整清单。
	details, ok := result.Details.(map[string]any)
	if !ok {
		t.Fatalf("result.Details 类型异常: %T", result.Details)
	}
	for _, key := range []string{"current_month", "total", "exportable", "excluded", "by_reason", "items"} {
		if _, exists := details[key]; !exists {
			t.Fatalf("result.Details 缺少字段 %q，实际键=%v", key, keysOf(details))
		}
	}
	t.Logf("详情 total=%v exportable=%v excluded=%v by_reason=%v",
		details["total"], details["exportable"], details["excluded"], details["by_reason"])

	// 至少要有一条「可导出」项带出文件名与两端路径，否则运维拿不到「导出去叫什么、放哪里」。
	items, _ := details["items"].([]dedup.ExportDecisionItem)
	var probe *dedup.ExportDecisionItem
	for index := range items {
		if items[index].Usc == probeUsc && items[index].BizMonth == probeBizMonth {
			probe = &items[index]
			break
		}
	}
	if probe == nil {
		t.Fatalf("探针证券月 %s/%d 未出现在决策明细里（共 %d 条）", probeUsc, probeBizMonth, len(items))
	}
	if probe.Verdict != dedup.ExportVerdictExportable || probe.ReasonCode != dedup.ExportReasonOK {
		t.Fatalf("探针应判为可导出，实际 verdict=%s reason=%s 说明=%s",
			probe.Verdict, probe.ReasonCode, probe.ReasonDetail)
	}
	if probe.ExpectedRows != probeRows || probe.ActualRows != probeRows {
		t.Fatalf("探针行数不符：expected=%d actual=%d（期望 %d）",
			probe.ExpectedRows, probe.ActualRows, probeRows)
	}
	const wantFile = "US_NSDQ_ITEXP_PROBE_Min_FTMM_Mon_202501_Base.mvsv"
	if probe.FileName != wantFile {
		t.Fatalf("探针文件名不符：%s（期望 %s）", probe.FileName, wantFile)
	}
	if !strings.Contains(probe.RemotePath, "Data/Finv/SecuQuoteMon/FTMM/US_NSDQ/ITEXP_PROBE/"+wantFile) {
		t.Fatalf("探针远端路径不符：%s", probe.RemotePath)
	}
	if !strings.HasSuffix(probe.LocalPath, wantFile) {
		t.Fatalf("探针本地路径不符：%s", probe.LocalPath)
	}
	t.Logf("可导出探针: %s %d 应有行=%d 实际行=%d 文件=%s 本地=%s 远端=%s",
		probe.Usc, probe.BizMonth, probe.ExpectedRows, probe.ActualRows,
		probe.FileName, probe.LocalPath, probe.RemotePath)

	// 运行日志里也必须有这条「可导出」明细（否则前端弹窗只看到排除项）。
	var exportableLines int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM cdeu.finv_quote_cdeu_job_run_log_line
		  WHERE run_id = $1 AND message LIKE '%可导出 ITEXP_PROBE 202501%'`, runID).
		Scan(&exportableLines); err != nil {
		t.Fatalf("查询可导出明细日志失败: %v", err)
	}
	if exportableLines != 1 {
		t.Fatalf("运行日志缺少可导出明细行（期望 1 行，实际 %d 行）", exportableLines)
	}

	// 每个排除项都必须有非空原因描述，避免"含糊其辞"。
	for _, item := range items {
		if item.Verdict != dedup.ExportVerdictExcluded {
			continue
		}
		if strings.TrimSpace(item.ReasonDetail) == "" {
			t.Fatalf("排除项缺少原因描述: %+v", item)
		}
		if item.ReasonCode == dedup.ExportReasonOK {
			t.Fatalf("排除项原因码不应为 OK: %+v", item)
		}
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
