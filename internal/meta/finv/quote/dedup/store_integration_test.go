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

// integrationUsc 集成测试专用证券编码（测试前后都会清理）。
const integrationUsc = "IT159934"

// TestStoreIntegration 在真实 PostgreSQL 上验证归档链路的 SQL 与行映射（默认跳过）。
//
// 需要一个**可清空的测试库**（会执行全量迁移）：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:55432/finv_migtest?sslmode=disable" \
//	  go test -p 1 ./internal/meta/finv/quote/dedup -run TestStoreIntegration -v
//
// 注意：与 internal/database 的迁移集成测试共用同一个测试库，批量运行时必须加 `-p 1` 串行。
func TestStoreIntegration(t *testing.T) {
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
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,dedup,archive"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool, filepath.Join("..", "..", "Deploy", "Migrations")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	store := NewStore(pool)
	cleanup := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		statements := []string{
			`DELETE FROM dedup.finv_dedup_export_file_upload_log WHERE file_id IN (SELECT file_id FROM dedup.finv_dedup_export_file WHERE usc = $1)`,
			`DELETE FROM dedup.finv_dedup_export_file_patch WHERE usc = $1`,
			`DELETE FROM dedup.finv_dedup_export_file WHERE usc = $1`,
			`DELETE FROM dedup.finv_dedup_export_job WHERE usc = $1`,
			`DELETE FROM dedup.finv_dedup_quote_maturity_mon WHERE usc = $1`,
			`DELETE FROM dedup.finv_dedup_usc_priority WHERE usc = $1`,
			`DELETE FROM dedup.finv_dedup_usc WHERE usc = $1`,
			`DELETE FROM dedup.finv_quote_secu_kline_min WHERE usc = $1`,
			`DELETE FROM export.finv_quote_secu_kline_min WHERE usc = $1`,
		}
		for _, statement := range statements {
			_, _ = pool.Exec(cleanupCtx, statement, integrationUsc)
		}
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM dedup.finv_dedup_scan_watermark WHERE watermark_name LIKE $1`,
			"POST_SEAL_PATCH_"+integrationUsc+"_%")
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM dedup.finv_dedup_scan_watermark WHERE watermark_name = 'IT_WATERMARK'`)
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM dedup.finv_dedup_job_run_log_export_file WHERE usc = $1`, integrationUsc)
	}
	cleanup()
	defer cleanup()

	// ---- 准备：证券主数据、优先级、主表 3 行（2025-01-02）----
	if _, err := pool.Exec(ctx, `
INSERT INTO dedup.finv_dedup_usc
  (usc, category, day_expect_rows, region, market, provider, name, timezone, currency)
VALUES ($1, 'A_SHARE', 3, 'CN', 'SZ', 'FTMM', '集成测试证券', 'Asia/Shanghai', 'CNY')`,
		integrationUsc); err != nil {
		t.Fatalf("插入证券主数据失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO dedup.finv_dedup_usc_priority (usc, priority_rank, sla_days, max_wait_days)
VALUES ($1, 1, 5, 7)`, integrationUsc); err != nil {
		t.Fatalf("插入证券优先级失败: %v", err)
	}
	for index := 0; index < 3; index++ {
		if _, err := pool.Exec(ctx, `
INSERT INTO dedup.finv_quote_secu_kline_min
  (usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
   change_price, change_ratio, last_close, provider)
VALUES ($1, $2, 20250102, $3, 1.1, 1.2, 1.0, 1.15, 1.12, 100, 115.5, 0.05, 4.5, 1.10, 'FTMM')
ON CONFLICT (ts, usc) DO NOTHING`,
			integrationUsc, int64(1735779600+index*60), 93000+index); err != nil {
			t.Fatalf("插入主表数据失败: %v", err)
		}
	}

	// ---- 成熟度：3/3 → COMPLETE ----
	now := time.Now()
	if _, err := store.RefreshMaturityIncremental(ctx, now.Add(-time.Hour), now.Add(time.Minute), 0.95); err != nil {
		t.Fatalf("刷新成熟度失败: %v", err)
	}
	var actualRows, expectedRows int
	var maturityStatus string
	if err := pool.QueryRow(ctx, `
SELECT actual_rows, expected_rows, maturity_status
  FROM dedup.finv_dedup_quote_maturity_mon WHERE usc = $1 AND trade_date = 20250102`,
		integrationUsc).Scan(&actualRows, &expectedRows, &maturityStatus); err != nil {
		t.Fatalf("读取成熟度失败: %v", err)
	}
	if actualRows != 3 || expectedRows != 3 || maturityStatus != MaturityComplete {
		t.Fatalf("成熟度不符：actual=%d expected=%d status=%s", actualRows, expectedRows, maturityStatus)
	}

	// ---- 候选生成（202501 已结束且全月 COMPLETE）----
	created, err := store.InsertCandidateJobs(ctx, 202601, 10)
	if err != nil {
		t.Fatalf("生成候选任务失败: %v", err)
	}
	if created != 1 {
		t.Fatalf("应生成 1 个候选任务，实际 %d", created)
	}

	// ---- 领取（行锁 + 租约 + RETURNING 行映射）----
	job, err := store.ClaimJob(ctx, "it-worker", 5*time.Minute)
	if err != nil {
		t.Fatalf("领取任务失败: %v", err)
	}
	if job == nil || job.Usc != integrationUsc {
		t.Fatalf("领取任务结果异常: %+v", job)
	}
	if job.Status != JobStatusLoading || job.PriorityRank != 1 {
		t.Fatalf("领取后状态/优先级不符: %+v", job)
	}
	if job.SealDate.Format("2006-01-02") != "2025-02-25" {
		t.Fatalf("seal_date 不符: %s", job.SealDate.Format("2006-01-02"))
	}

	// ---- LOAD 分页（keyset + ON CONFLICT 幂等）----
	page, err := store.LoadPage(ctx, integrationUsc, 20250102, 0, 100)
	if err != nil {
		t.Fatalf("装载分页失败: %v", err)
	}
	if page.PageRows != 3 || page.InsertedRows != 3 {
		t.Fatalf("装载行数不符：page=%d inserted=%d", page.PageRows, page.InsertedRows)
	}
	// 重复装载同一页应幂等（插入 0 行，页上界不变）。
	again, err := store.LoadPage(ctx, integrationUsc, 20250102, 0, 100)
	if err != nil {
		t.Fatalf("重复装载失败: %v", err)
	}
	if again.InsertedRows != 0 || again.MaxTS != page.MaxTS {
		t.Fatalf("重复装载不幂等：inserted=%d max_ts=%d/%d", again.InsertedRows, again.MaxTS, page.MaxTS)
	}

	if stagingRows, err := store.CountStaging(ctx, integrationUsc, 20250101, 20250131); err != nil || stagingRows != 3 {
		t.Fatalf("中转行数不符: %d err=%v", stagingRows, err)
	}
	if mainRows, err := store.CountMainDate(ctx, integrationUsc, 20250102); err != nil || mainRows != 3 {
		t.Fatalf("主表行数不符: %d err=%v", mainRows, err)
	}
	if rows, err := store.CountStagingDate(ctx, integrationUsc, 20250102); err != nil || rows != 3 {
		t.Fatalf("中转日行数不符: %d err=%v", rows, err)
	}
	if err := store.UpdateLoadProgress(ctx, job.JobID, 20250102, 0, 3, JobStatusLoaded, 5*time.Minute); err != nil {
		t.Fatalf("保存装载进度失败: %v", err)
	}

	// ---- 导出文件登记 ----
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
	if err := store.MarkJobExported(ctx, job.JobID, 3, 1); err != nil {
		t.Fatalf("标记导出完成失败: %v", err)
	}
	target := "cn/quotes/" + meta.BaseFileName()
	baseID, err := store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            integrationUsc,
		BizMonth:       202501,
		FileType:       FileTypeBase,
		Version:        1,
		FileName:       meta.BaseFileName(),
		FilePath:       "Data/ExportArchive/it/" + meta.BaseFileName(),
		TargetFileName: meta.BaseFileName(),
		UploadTarget:   &target,
		Format:         MvsvFormat,
		Compression:    CompressionNone,
		RowCount:       3,
		FileSize:       1024,
		Checksum:       "abc123",
		SealDate:       &job.SealDate,
	})
	if err != nil {
		t.Fatalf("登记 BASE 文件失败: %v", err)
	}
	// 重复登记同一版本：幂等（不新增记录）。
	if againID, err := store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            integrationUsc,
		BizMonth:       202501,
		FileType:       FileTypeBase,
		Version:        1,
		FileName:       meta.BaseFileName(),
		FilePath:       "Data/ExportArchive/it/" + meta.BaseFileName(),
		TargetFileName: meta.BaseFileName(),
		UploadTarget:   &target,
		Format:         MvsvFormat,
		Compression:    CompressionNone,
		RowCount:       3,
		FileSize:       1024,
		Checksum:       "abc123",
		SealDate:       &job.SealDate,
	}); err != nil || againID != baseID {
		t.Fatalf("重复登记 BASE 不幂等：id=%d/%d err=%v", againID, baseID, err)
	}

	manifestVersion, err := store.NextManifestVersion(ctx, integrationUsc, 202501)
	if err != nil || manifestVersion != 1 {
		t.Fatalf("首个 Manifest 版本应为 1，实际 %d err=%v", manifestVersion, err)
	}
	manifestTarget := "cn/quotes/" + meta.ManifestFileName()
	manifestID, err := store.RegisterFile(ctx, &File{
		JobID:          job.JobID,
		Usc:            integrationUsc,
		BizMonth:       202501,
		FileType:       FileTypeManifest,
		Version:        manifestVersion,
		FileName:       meta.ManifestFileName(),
		FilePath:       "Data/ExportArchive/it/" + meta.ManifestFileName(),
		TargetFileName: meta.ManifestFileName(),
		UploadTarget:   &manifestTarget,
		Format:         MvsvFormat,
		Compression:    CompressionNone,
		FileSize:       512,
		Checksum:       "def456",
		SealDate:       &job.SealDate,
	})
	if err != nil {
		t.Fatalf("登记 MANIFEST 失败: %v", err)
	}

	latestBase, err := store.LatestBaseFile(ctx, job.JobID)
	if err != nil || latestBase == nil || latestBase.FileName != meta.BaseFileName() {
		t.Fatalf("读取最新 BASE 失败: %+v err=%v", latestBase, err)
	}
	files, err := store.BaseAndPatchFiles(ctx, job.JobID)
	if err != nil || len(files) != 1 {
		t.Fatalf("BASE/PATCH 列表不符: %d err=%v", len(files), err)
	}

	// ---- 上传领取顺序：BASE 先于 MANIFEST（G-14）----
	picked, err := store.PickUploadFile(ctx)
	if err != nil || picked == nil || picked.FileID != baseID {
		t.Fatalf("应先领取 BASE：%+v err=%v", picked, err)
	}
	if err := store.FinishUpload(ctx, baseID, UploadStatusUploaded, true, &target, nil); err != nil {
		t.Fatalf("落定上传成功失败: %v", err)
	}
	if err := store.InsertUploadLog(ctx, baseID, 1, target, time.Now(), "SUCCESS", 1024, ""); err != nil {
		t.Fatalf("写上传审计日志失败: %v", err)
	}
	picked, err = store.PickUploadFile(ctx)
	if err != nil || picked == nil || picked.FileID != manifestID {
		t.Fatalf("BASE 上传后应领取 MANIFEST：%+v err=%v", picked, err)
	}
	if err := store.FinishUpload(ctx, manifestID, UploadStatusUploaded, true, &manifestTarget, nil); err != nil {
		t.Fatalf("落定 MANIFEST 上传失败: %v", err)
	}
	if remaining, err := store.PickUploadFile(ctx); err != nil || remaining != nil {
		t.Fatalf("不应再有待上传文件：%+v err=%v", remaining, err)
	}

	// ---- 补丁任务 ----
	if version, err := store.NextPatchVersion(ctx, job.JobID); err != nil || version != 1 {
		t.Fatalf("首个补丁序号应为 1，实际 %d err=%v", version, err)
	}
	patchFileID := baseID
	patchID, err := store.InsertPatchTask(ctx, &PatchTask{
		JobID:        job.JobID,
		Usc:          integrationUsc,
		BizMonth:     202501,
		PatchLevel:   PatchLevelL2Warm,
		PatchVersion: 1,
		SourceRows:   1,
		PatchRows:    1,
		Reason:       PatchReasonLateArrival,
		ArrivalStart: now.Add(-time.Hour),
		ArrivalEnd:   now,
		Status:       PatchStatusDone,
		PatchFileID:  &patchFileID,
	})
	if err != nil {
		t.Fatalf("登记补丁任务失败: %v", err)
	}
	if err := store.FinishPatchTask(ctx, patchID, PatchStatusDone, 1, &patchFileID, ""); err != nil {
		t.Fatalf("落定补丁任务失败: %v", err)
	}
	if version, err := store.NextPatchVersion(ctx, job.JobID); err != nil || version != 2 {
		t.Fatalf("第二个补丁序号应为 2，实际 %d err=%v", version, err)
	}

	// ---- 收口冻结与差异报告 ----
	if sealed, err := store.SealMaturity(ctx, 202501); err != nil || sealed < 1 {
		t.Fatalf("收口冻结失败: %d err=%v", sealed, err)
	}
	if frozenJobs, err := store.MarkJobsFrozen(ctx, now); err != nil {
		t.Fatalf("冻结任务失败: %v", err)
	} else if frozenJobs != 0 {
		t.Fatalf("已导出任务不应被冻结，实际 %d", frozenJobs)
	}
	if diff, err := store.FrozenDiffReport(ctx, 202501, 10); err != nil {
		t.Fatalf("读取差异报告失败: %v", err)
	} else if len(diff) != 0 {
		t.Fatalf("无缺失时差异报告应为空，实际 %+v", diff)
	}

	// ---- 指标与维护类查询 ----
	if _, err := store.JobStatusCounts(ctx); err != nil {
		t.Fatalf("任务状态聚合失败: %v", err)
	}
	if _, err := store.UploadStatusCounts(ctx); err != nil {
		t.Fatalf("上传状态聚合失败: %v", err)
	}
	if _, err := store.HeartbeatTimeoutCount(ctx); err != nil {
		t.Fatalf("心跳超时统计失败: %v", err)
	}
	if _, err := store.UploadRetryExhaustedCount(ctx, 8); err != nil {
		t.Fatalf("重试耗尽统计失败: %v", err)
	}
	if _, _, err := store.PatchStats(ctx); err != nil {
		t.Fatalf("补丁统计失败: %v", err)
	}
	if _, _, err := store.FrozenMissing(ctx, 202501); err != nil {
		t.Fatalf("FROZEN 缺失统计失败: %v", err)
	}
	if _, _, err := store.StagingSummary(ctx); err != nil {
		t.Fatalf("中转表汇总失败: %v", err)
	}
	if _, err := store.CleanupStaging(ctx, integrationUsc, 20250101, 20250131); err != nil {
		t.Fatalf("清理中转数据失败: %v", err)
	}
	if _, err := store.TruncateStagingIfEmpty(ctx); err != nil {
		t.Fatalf("TRUNCATE 中转表失败: %v", err)
	}
	if _, err := store.PurgeResidualStaging(ctx, 1000); err != nil {
		t.Fatalf("清理残留中转失败: %v", err)
	}

	// ---- 生命周期：网盘归档 → 本地清理 → 主表清理 ----
	if err := store.MarkNetworkArchived(ctx, baseID); err != nil {
		t.Fatalf("标记网盘归档失败: %v", err)
	}
	candidates, err := store.LocalCleanupCandidates(ctx, 0, 10)
	if err != nil {
		t.Fatalf("查询本地清理候选失败: %v", err)
	}
	foundBase := false
	for _, candidate := range candidates {
		if candidate.FileID == baseID {
			foundBase = true
		}
	}
	if !foundBase {
		t.Fatalf("BASE 应进入本地清理候选：%+v", candidates)
	}
	if err := store.MarkLocalDeleted(ctx, baseID); err != nil {
		t.Fatalf("标记本地清理失败: %v", err)
	}
	months, err := store.PurgeMainCandidates(ctx, 202601)
	if err != nil {
		t.Fatalf("查询主表清理候选失败: %v", err)
	}
	if len(months) != 1 || months[0] != 202501 {
		t.Fatalf("主表清理候选应为 [202501]，实际 %v", months)
	}
	if deleted, err := store.PurgeMainMonth(ctx, 202501, 100); err != nil || deleted != 3 {
		t.Fatalf("清理主表数据失败: deleted=%d err=%v", deleted, err)
	}

	// ---- 超时回收 ----
	if _, err := store.RecoverTimeoutJobs(ctx); err != nil {
		t.Fatalf("回收超时任务失败: %v", err)
	}
	if _, err := store.RecoverTimeoutUploads(ctx, 10*time.Minute); err != nil {
		t.Fatalf("回收超时上传失败: %v", err)
	}

	// ---- 状态重置（人工重试）----
	nextRun := now.Add(time.Minute)
	if err := store.RequeueJob(ctx, job.JobID, JobStatusFailed, "integration test", &nextRun); err != nil {
		t.Fatalf("标记任务失败失败: %v", err)
	}
	if err := store.ResetJobForRetry(ctx, job.JobID); err != nil {
		t.Fatalf("重置任务失败: %v", err)
	}
	if err := store.FinishUpload(ctx, baseID, UploadStatusFailed, false, &target, &nextRun); err != nil {
		t.Fatalf("标记上传失败失败: %v", err)
	}
	if err := store.ResetFileForRetry(ctx, baseID); err != nil {
		t.Fatalf("重置上传失败: %v", err)
	}

	// ---- 分页查询 ----
	if page, err := store.ListJobs(ctx, JobFilter{Usc: integrationUsc, PageSize: 10}); err != nil || page.Total < 1 {
		t.Fatalf("任务分页查询失败: total=%d err=%v", page.Total, err)
	}
	if page, err := store.ListFiles(ctx, FileFilter{Usc: integrationUsc, PageSize: 10}); err != nil || page.Total != 2 {
		t.Fatalf("文件分页查询失败: total=%d err=%v", page.Total, err)
	}

	// ---- 运行日志 ----
	jobID := job.JobID
	runID, err := store.StartRun(ctx, JobRunDueExportJobs, &jobID, integrationUsc, &job.BizMonth)
	if err != nil {
		t.Fatalf("登记运行日志失败: %v", err)
	}
	if err := store.FinishRun(ctx, runID, RunStatusSuccess, 3, 1024, ""); err != nil {
		t.Fatalf("结束运行日志失败: %v", err)
	}

	// ---- 水位 ----
	watermark := now.Truncate(time.Second)
	if err := store.UpsertWatermark(ctx, "IT_WATERMARK", watermark); err != nil {
		t.Fatalf("写入水位失败: %v", err)
	}
	if value, found, err := store.Watermark(ctx, "IT_WATERMARK"); err != nil || !found || !value.Equal(watermark) {
		t.Fatalf("读取水位不符: %v found=%v err=%v", value, found, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM dedup.finv_dedup_scan_watermark WHERE watermark_name = 'IT_WATERMARK'`); err != nil {
		t.Fatalf("清理测试水位失败: %v", err)
	}
}
