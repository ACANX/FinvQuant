package dedup_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/database"

	. "github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	_ "github.com/acanx/finvquant/internal/meta/finv/quote/dedup/job"
)

const integrationUsc = "IT159934"

// TestServiceIntegration 端到端验证 10 个 Job 的编排（默认跳过）。
//
// 需要**可清空的测试库**（会执行全量迁移），并按需设置上传目录：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:55432/finv_migtest?sslmode=disable" \
//	  go test -p 1 ./internal/meta/finv/quote/dedup -run TestServiceIntegration -v
//
// 注意：与 internal/database 的迁移集成测试共用同一个测试库，批量运行时必须加 `-p 1` 串行。
func TestServiceIntegration(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("解析测试 DSN 失败: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,dedup,cdeu,archive,sys"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "..", "..", "..", "Deploy", "Migrations")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	root := t.TempDir()
	options := Options{
		Enabled:           true,
		Location:          location,
		TimeZone:          "Asia/Shanghai",
		OutputDir:         filepath.Join(root, "out"),
		WorkerID:          "it-service",
		PageSize:          1000,
		WindowStart:       0,
		WindowEnd:         0, // 起止相同 = 全天可执行
		Lease:             30 * time.Second,
		MaxAttempts:       2,
		UploadMode:        UploadModeDir,
		UploadDir:         filepath.Join(root, "upload"),
		LocalRetention:    0,
		MaturityThreshold: 0.95,
		PatchBudgetRatio:  0.5,
		Compression:       CompressionNone,
		FetchLimit:        100,
		MaxJobsPerRun:     5,
	}
	service, err := NewService(pool, options)
	if err != nil {
		t.Fatalf("创建归档服务失败: %v", err)
	}

	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		for _, statement := range []string{
			`DELETE FROM cdeu.finv_quote_cdeu_export_file_upload_log WHERE file_id IN (SELECT file_id FROM cdeu.finv_quote_cdeu_export_file WHERE usc = $1)`,
			`DELETE FROM cdeu.finv_quote_cdeu_export_file_patch WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_cdeu_export_file WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_cdeu_export_job WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_cdeu_quote_maturity_mon WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_cdeu_usc WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_secu_kline_min WHERE usc = $1`,
			`DELETE FROM export.finv_quote_secu_kline_min WHERE usc = $1`,
			`DELETE FROM cdeu.finv_quote_cdeu_job_run_log_export_file WHERE usc = $1`,
		} {
			if _, execErr := pool.Exec(cleanupCtx, statement, integrationUsc); execErr != nil {
				t.Logf("清理测试数据失败（可忽略）: %v", execErr)
			}
		}
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM cdeu.finv_quote_cdeu_scan_watermark WHERE watermark_name LIKE $1`,
			"POST_SEAL_PATCH_"+integrationUsc+"_%")
		// 当日预算与入口统计同样清理，保证测试可重复运行。
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_daily_budget WHERE biz_date = current_date`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_ingest_stats WHERE stat_date = current_date`)
		// CDEU 运行实例按日累积；测试库每次清空，保证完成条件断言可重复。
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_job_step_dynamic`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_job_dynamic`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_export_batch`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_job_step_run`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_job_run`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_process_run`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_phase_run`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_workflow_run`)
	}
	cleanup()
	defer cleanup()

	// ---- 准备数据 ----
	if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_usc
  (usc, category, day_expect_rows, region, market, provider, name, timezone, currency)
VALUES ($1, 'A_SHARE', 3, 'CN', 'SZ', 'FTMM', '集成测试证券', 'Asia/Shanghai', 'CNY')`,
		integrationUsc); err != nil {
		t.Fatalf("插入证券主数据失败: %v", err)
	}
	// V200051 起优先级已合并进 finv_dedup_usc，直接更新主表列。
	if _, err := pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_usc
   SET priority_rank = 1, sla_days = 5, max_wait_days = 7, priority_gmt_update = now()
 WHERE usc = $1`, integrationUsc); err != nil {
		t.Fatalf("设置证券优先级失败: %v", err)
	}
	insertMainRow := func(ts int64, clock int) {
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_secu_kline_min
  (usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
   change_price, change_ratio, last_close, provider)
VALUES ($1, $2, 20250102, $3, 1.1, 1.2, 1.0, 1.15, 1.12, 100, 115.5, 0.05, 4.5, 1.10, 'FTMM')
ON CONFLICT (ts, usc) DO NOTHING`, integrationUsc, ts, clock); err != nil {
			t.Fatalf("插入主表数据失败: %v", err)
		}
	}
	for index := 0; index < 3; index++ {
		insertMainRow(int64(1735779600+index*60), 93000+index)
	}

	// ---- 1) 成熟度 / 入口统计 / 预算 ----
	if _, err := service.Run(ctx, JobRefreshQuoteMaturity); err != nil {
		t.Fatalf("%s 失败: %v", JobRefreshQuoteMaturity, err)
	}
	if _, err := service.Run(ctx, JobCalcDailyIngestStats); err != nil {
		t.Fatalf("%s 失败: %v", JobCalcDailyIngestStats, err)
	}
	budgetResult, err := service.Run(ctx, JobCalcDailyExportBudget)
	if err != nil {
		t.Fatalf("%s 失败: %v", JobCalcDailyExportBudget, err)
	}
	budget, ok := budgetResult.Details.(Budget)
	if !ok {
		t.Fatalf("预算结果类型异常: %T", budgetResult.Details)
	}
	if budget.BudgetRows <= 0 {
		t.Fatalf("预算条数应大于 0，实际 %d", budget.BudgetRows)
	}
	// 测试库当日入口仅 3 行，按真实公式预算也是 3 行；这里放大当日预算，
	// 以便一次跑完导出 + 补丁链路（预算公式本身由 TestComputeBudget 覆盖）。
	if _, err := pool.Exec(ctx, `
UPDATE cdeu.finv_quote_cdeu_daily_budget
   SET budget_rows = 1000, budget_bytes = 1000000,
       consumed_rows = 0, consumed_bytes = 0, status = 'CALCULATED'
 WHERE biz_date = $1::date`, options.Today()); err != nil {
		t.Fatalf("放大测试预算失败: %v", err)
	}

	// ---- 2) 导出（LOAD → EXPORT → CLEANUP）----
	if _, err := service.Run(ctx, JobRunDueExportJobs); err != nil {
		t.Fatalf("%s 失败: %v", JobRunDueExportJobs, err)
	}

	meta := FileMeta{
		Usc:      integrationUsc,
		BizMonth: 202501,
		Region:   "CN",
		Market:   "SZ",
		Provider: "FTMM",
		Name:     "集成测试证券",
		Timezone: "Asia/Shanghai",
		Currency: "CNY",
	}
	folder := filepath.Join(options.OutputDir, integrationUsc, "202501")
	basePath := filepath.Join(folder, meta.BaseFileName())
	manifestPath := filepath.Join(folder, meta.ManifestFileName())
	assertMVsv(t, basePath, 3)

	// 任务状态与文件登记
	var jobStatus string
	var patchCount, baseVersion int
	var loadedRows int64
	if err := pool.QueryRow(ctx, `
SELECT status, patch_count, base_version, loaded_rows
  FROM cdeu.finv_quote_cdeu_export_job WHERE usc = $1 AND biz_month = 202501`,
		integrationUsc).Scan(&jobStatus, &patchCount, &baseVersion, &loadedRows); err != nil {
		t.Fatalf("读取任务状态失败: %v", err)
	}
	if jobStatus != JobStatusExported || baseVersion != 1 || loadedRows != 3 {
		t.Fatalf("任务状态异常：status=%s base_version=%d loaded_rows=%d", jobStatus, baseVersion, loadedRows)
	}

	// 中转表已清理
	stagingRows, _, err := service.Store().StagingSummary(ctx)
	if err != nil {
		t.Fatalf("读取中转表汇总失败: %v", err)
	}
	if stagingRows != 0 {
		t.Fatalf("导出后中转表应为空，实际 %d 行", stagingRows)
	}

	// Manifest 内容
	manifest := readManifest(t, manifestPath)
	if manifest.Base == nil || manifest.Base.RowCount != 3 || manifest.RowCount != 3 {
		t.Fatalf("Manifest 行数异常: %+v", manifest)
	}
	if len(manifest.Patches) != 0 {
		t.Fatalf("首次导出不应有补丁，实际 %d", len(manifest.Patches))
	}
	if manifest.Compression != CompressionNone || manifest.Delimiter != MvsvDelimiter {
		t.Fatalf("Manifest 格式字段异常: %+v", manifest)
	}

	// ---- 3) 上传（BASE 先于 MANIFEST）----
	if _, err := service.Run(ctx, JobUploadPendingExportFiles); err != nil {
		t.Fatalf("%s 失败: %v", JobUploadPendingExportFiles, err)
	}
	for _, name := range []string{meta.BaseFileName(), meta.ManifestFileName()} {
		if _, statErr := os.Stat(filepath.Join(options.UploadDir, name)); statErr != nil {
			t.Fatalf("上传目录缺少文件 %s: %v", name, statErr)
		}
	}
	var pendingUploads int64
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM cdeu.finv_quote_cdeu_export_file
 WHERE usc = $1 AND upload_status <> 'UPLOADED'`, integrationUsc).Scan(&pendingUploads); err != nil {
		t.Fatalf("统计上传状态失败: %v", err)
	}
	if pendingUploads != 0 {
		t.Fatalf("仍有未上传文件: %d", pendingUploads)
	}
	var uploadLogs int64
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM cdeu.finv_quote_cdeu_export_file_upload_log l
  JOIN cdeu.finv_quote_cdeu_export_file f ON f.file_id = l.file_id
 WHERE f.usc = $1 AND l.status = 'SUCCESS'`, integrationUsc).Scan(&uploadLogs); err != nil {
		t.Fatalf("统计上传审计日志失败: %v", err)
	}
	if uploadLogs < 2 {
		t.Fatalf("上传审计日志应至少 2 条，实际 %d", uploadLogs)
	}

	// ---- 4) 收口后补丁（L2_WARM）----
	insertMainRow(1735779780, 93003)

	if _, err := service.Run(ctx, JobScanPostSealPatches); err != nil {
		t.Fatalf("%s 失败: %v", JobScanPostSealPatches, err)
	}
	patchPath := filepath.Join(folder, meta.PatchFileName(1))
	if _, statErr := os.Stat(patchPath); statErr != nil {
		t.Fatalf("补丁文件未生成: %v", statErr)
	}
	assertMVsv(t, patchPath, 1)

	manifest = readManifest(t, manifestPath)
	if len(manifest.Patches) != 1 || manifest.Patches[0].RowCount != 1 {
		t.Fatalf("Manifest 补丁信息异常: %+v", manifest.Patches)
	}
	if manifest.RowCount != 4 {
		t.Fatalf("Manifest 总行数应为 4，实际 %d", manifest.RowCount)
	}

	var patchLevel, patchStatus string
	if err := pool.QueryRow(ctx, `
SELECT patch_level, status FROM cdeu.finv_quote_cdeu_export_file_patch
 WHERE job_id = (SELECT job_id FROM cdeu.finv_quote_cdeu_export_job WHERE usc = $1 AND biz_month = 202501)
 ORDER BY patch_id DESC LIMIT 1`, integrationUsc).Scan(&patchLevel, &patchStatus); err != nil {
		t.Fatalf("读取补丁任务失败: %v", err)
	}
	if patchLevel != PatchLevelL2Warm || patchStatus != PatchStatusDone {
		t.Fatalf("补丁任务异常：level=%s status=%s", patchLevel, patchStatus)
	}

	// ---- 5) 健康检查（含 MVsv 结构抽检）----
	healthResult, err := service.Run(ctx, JobCheckExportJobHealth)
	if err != nil {
		t.Fatalf("%s 失败: %v", JobCheckExportJobHealth, err)
	}
	metrics, ok := healthResult.Details.(*Metrics)
	if !ok {
		t.Fatalf("健康检查结果类型异常: %T", healthResult.Details)
	}
	if metrics.MvsvAnomalies != 0 {
		t.Fatalf("MVsv 结构抽检不应发现异常，实际 %d", metrics.MvsvAnomalies)
	}
	if metrics.PatchLast7Days < 1 {
		t.Fatalf("近 7 天补丁数应至少为 1，实际 %d", metrics.PatchLast7Days)
	}

	// ---- 6) 收口冻结与差异报告 ----
	if _, err := service.Run(ctx, JobMarkSealedJobsFrozen); err != nil {
		t.Fatalf("%s 失败: %v", JobMarkSealedJobsFrozen, err)
	}

	// ---- 7) 生命周期：网盘归档后才允许清理本地文件 ----
	fileRows, err := pool.Query(ctx, `
SELECT file_id FROM cdeu.finv_quote_cdeu_export_file
 WHERE usc = $1 AND file_type = 'BASE' AND export_status = 'EXPORTED'`, integrationUsc)
	if err != nil {
		t.Fatalf("读取 BASE 文件失败: %v", err)
	}
	var baseID int64
	if fileRows.Next() {
		if err := fileRows.Scan(&baseID); err != nil {
			t.Fatalf("扫描 BASE file_id 失败: %v", err)
		}
	}
	fileRows.Close()
	if baseID == 0 {
		t.Fatalf("未找到生效的 BASE 文件")
	}
	if err := service.Store().MarkNetworkArchived(ctx, baseID); err != nil {
		t.Fatalf("标记网盘归档失败: %v", err)
	}
	if _, err := service.Run(ctx, JobCleanExpiredExportArtifacts); err != nil {
		t.Fatalf("%s 失败: %v", JobCleanExpiredExportArtifacts, err)
	}
	if _, statErr := os.Stat(basePath); !os.IsNotExist(statErr) {
		t.Fatalf("网盘归档后本地 BASE 应被清理，实际仍存在（err=%v）", statErr)
	}

	// ---- 8) 超时回收 ----
	if _, err := service.Run(ctx, JobRecoverTimeoutExportJobs); err != nil {
		t.Fatalf("%s 失败: %v", JobRecoverTimeoutExportJobs, err)
	}

	// ---- 9) CDEU 结构化运行树 ----
	var cdeuStatus string
	if err := pool.QueryRow(ctx, `
SELECT status FROM cdeu.finv_quote_cdeu_workflow_run ORDER BY run_id DESC LIMIT 1`).Scan(&cdeuStatus); err != nil {
		t.Fatalf("读取 CDEU 流程运行失败: %v", err)
	}
	if cdeuStatus != CDEUStatusSuccess {
		t.Fatalf("CDEU 流程状态应为 SUCCESS，实际 %s", cdeuStatus)
	}
	var cdeuJobRuns int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cdeu.finv_quote_cdeu_job_run`).Scan(&cdeuJobRuns); err != nil {
		t.Fatalf("统计 CDEU 作业运行失败: %v", err)
	}
	if cdeuJobRuns != 15 {
		t.Fatalf("CDEU 作业运行应为 15 条，实际 %d", cdeuJobRuns)
	}
	var cdeuJobStepRuns int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step_run`).Scan(&cdeuJobStepRuns); err != nil {
		t.Fatalf("统计 CDEU JOB Step 运行失败: %v", err)
	}
	if cdeuJobStepRuns != 15 {
		t.Fatalf("CDEU JOB Step 运行应为 15 条，实际 %d", cdeuJobStepRuns)
	}
	overview, err := service.CDEUOverview(ctx, 0, 5)
	if err != nil {
		t.Fatalf("读取 CDEU 流程视图失败: %v", err)
	}
	if overview.Run == nil || overview.Run.Status != CDEUStatusSuccess {
		t.Fatalf("CDEU 最近运行应为 SUCCESS: %+v", overview.Run)
	}
	if len(overview.Phases) != 6 {
		t.Fatalf("CDEU 阶段数应为 6，实际 %d", len(overview.Phases))
	}
	viewJobs := 0
	for _, phase := range overview.Phases {
		for _, process := range phase.Processes {
			viewJobs += len(process.Jobs)
		}
	}
	dynamicJobs := 0
	for _, phase := range overview.Phases {
		for _, process := range phase.Processes {
			for _, job := range process.Jobs {
				if job.IsDynamic {
					dynamicJobs++
				}
			}
		}
	}
	if viewJobs < 10 || dynamicJobs == 0 {
		t.Fatalf("CDEU 视图作业数异常: total=%d dynamic=%d", viewJobs, dynamicJobs)
	}
	viewSteps := 0
	for _, phase := range overview.Phases {
		for _, process := range phase.Processes {
			for _, job := range process.Jobs {
				viewSteps += len(job.Steps)
			}
		}
	}
	if viewSteps == 0 {
		t.Fatal("CDEU 视图应至少包含一个静态 JOB Step")
	}
	var dynamicSteps int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step_dynamic`).Scan(&dynamicSteps); err != nil {
		t.Fatalf("统计动态 JOB Step 失败: %v", err)
	}
	if dynamicSteps < dynamicJobs || dynamicJobs < 1 {
		t.Fatalf("动态 JOB Step 数量不足: dynamic_jobs=%d steps=%d", dynamicJobs, dynamicSteps)
	}
	var uploadDynamicJobs, uploadDynamicSteps int
	if err := pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_dynamic WHERE job_category = 'UPLOAD'),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step_dynamic ds
         JOIN cdeu.finv_quote_cdeu_job_dynamic dj ON dj.dynamic_job_id = ds.dynamic_job_id
        WHERE dj.job_category = 'UPLOAD' AND ds.step_code = 'STEP_UPLOAD')`).
		Scan(&uploadDynamicJobs, &uploadDynamicSteps); err != nil {
		t.Fatalf("统计上传动态 JOB 失败: %v", err)
	}
	if uploadDynamicJobs == 0 || uploadDynamicSteps != uploadDynamicJobs {
		t.Fatalf("上传动态 JOB 配置不符: jobs=%d steps=%d", uploadDynamicJobs, uploadDynamicSteps)
	}
	var batchCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cdeu.finv_quote_cdeu_export_batch`).Scan(&batchCount); err != nil {
		t.Fatalf("统计证券导出批次清单失败: %v", err)
	}
	if batchCount == 0 {
		t.Fatal("证券导出批次清单不应为空")
	}

	// ---- 9) 指标接口 ----
	metrics, err = service.Metrics(ctx)
	if err != nil {
		t.Fatalf("读取指标失败: %v", err)
	}
	if metrics.UploadStatus[UploadStatusUploaded] < 2 {
		t.Fatalf("已上传文件数应至少 2，实际 %d", metrics.UploadStatus[UploadStatusUploaded])
	}
}

// assertMVsv 读取文件并校验行数（16 行头 + 1 行空行 + expected 行数据）与结构。
func assertMVsv(t *testing.T, path string, expected int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 MVsv 文件失败: %v", err)
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("MVsv 文件不应包含 CR")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != MvsvHeaderLines+1+expected {
		t.Fatalf("文件行数应为 %d，实际 %d", MvsvHeaderLines+1+expected, len(lines))
	}
	if lines[MvsvHeaderLines] != "" {
		t.Fatalf("第 17 行应为空行，实际 %q", lines[MvsvHeaderLines])
	}
	if !strings.Contains(lines[7], "# Count : ") {
		t.Fatalf("Count 行缺失: %q", lines[7])
	}
	if err := VerifyMVsv(path, CompressionNone, int64(expected)); err != nil {
		t.Fatalf("VerifyMVsv 失败: %v", err)
	}
}

// readManifest 解析 Manifest JSON。
func readManifest(t *testing.T, path string) Manifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 Manifest 失败: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("解析 Manifest 失败: %v", err)
	}
	return manifest
}
