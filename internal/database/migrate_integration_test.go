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
	config.ConnConfig.RuntimeParams["search_path"] = "public,quote,quant,futu,export,sche,dedup,cdeu,archive,sys"
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

	// V100041：归档表 + 索引全部就绪（V200051 起 finv_dedup_usc_priority 已合并进 finv_dedup_usc，共 11 张）。
	//
	// 注意：dedup.finv_dedup_runtime_setting 不在清单内——该表于 V200041 删除
	// （导出低峰窗口改由 sys.sys_config 20006 承载），见 RequirementList 10653。
	tables := []string{
		"finv_dedup_ingest_stats",
		"finv_dedup_daily_budget",
		"finv_dedup_scan_watermark",
		"finv_dedup_quote_maturity_mon",
		"finv_dedup_trade_calendar",
		"finv_dedup_job_run_log_export_file",
		"finv_dedup_usc",
		"finv_dedup_export_job",
		"finv_dedup_export_file",
		"finv_dedup_export_file_upload_log",
		"finv_dedup_export_file_patch",
		"cdeu_workflow",
		"cdeu_phase",
		"cdeu_process",
		"cdeu_job",
		"cdeu_job_step",
		"cdeu_workflow_run",
		"cdeu_phase_run",
		"cdeu_process_run",
		"cdeu_job_run",
		"cdeu_job_step_run",
		"finv_dedup_export_batch",
		"cdeu_job_dynamic",
		"cdeu_job_step_dynamic",
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
		"idx_dedupjobrunlogexportfile_running",
		"idx_dedupexportjob_pick",
		"idx_dedupexportfile_upload",
		"idx_dedupexportfile_network",
		"idx_dedupexportfile_job",
		"idx_dedupexportfileuploadlog_file",
		"idx_dedupexportfilepatch_pending",
		"idx_dedupexportfilepatch_job",
		"idx_dedupexportfilepatch_file",
		"uq_cdeuworkflowrun_running",
		"idx_cdeuworkflowrun_biz",
		"idx_cdeu_job_run_process",
		"idx_cdeu_job_run_log",
		"idx_cdeu_job_step_job",
		"idx_cdeu_job_step_run_job",
		"idx_dedupexportbatch_lookup",
		"idx_dedupexportbatch_biz",
		"idx_cdeu_job_dynamic_process",
		"idx_cdeu_job_dynamic_status",
		"idx_cdeu_job_step_dynamic_job",
		"idx_cdeu_job_dynamic_file",
		"uq_cdeu_job_dynamic_export",
		"uq_cdeu_job_dynamic_upload",
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

	// V100042：运行日志增加阶段 / 进度 / 心跳列（供总览页展示执行进度）。
	columns := []string{"stage", "progress_percent", "progress_detail", "gmt_heartbeat"}
	for _, column := range columns {
		var exists bool
		if err := pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM information_schema.columns
 WHERE table_schema = 'dedup' AND table_name = 'finv_dedup_job_run_log_export_file' AND column_name = $1)`,
			column).Scan(&exists); err != nil {
			t.Fatalf("查询列 %s 失败: %v", column, err)
		}
		if !exists {
			t.Fatalf("迁移后缺少列 cdeu.finv_quote_cdeu_job_run_log_export_file.%s", column)
		}
	}

	// V100044–V100050：Workflow → Phase → Process → JOB → Step 定义与运行表。
	var workflowCount, phaseCount, processCount, jobCount, jobStepCount int
	if err := pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM cdeu.finv_quote_cdeu_workflow),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_phase),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_process),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step)`).
		Scan(&workflowCount, &phaseCount, &processCount, &jobCount, &jobStepCount); err != nil {
		t.Fatalf("统计 CDEU 初始数据失败: %v", err)
	}
	if workflowCount != 1 || phaseCount != 6 || processCount != 14 || jobCount != 15 || jobStepCount != 15 {
		t.Fatalf("CDEU 初始数据数量不符: workflow=%d phase=%d process=%d job=%d job_step=%d",
			workflowCount, phaseCount, processCount, jobCount, jobStepCount)
	}
	var opsPhase int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM cdeu.finv_quote_cdeu_phase WHERE phase_code = 'PHASE_OPS'`).Scan(&opsPhase); err != nil {
		t.Fatalf("查询 PHASE_OPS 失败: %v", err)
	}
	if opsPhase != 0 {
		t.Fatalf("PHASE_OPS 应已被 V100045 拆分删除，实际仍有 %d 行", opsPhase)
	}
	var collectName, dedupName string
	if err := pool.QueryRow(ctx, `
SELECT (SELECT name FROM cdeu.finv_quote_cdeu_phase WHERE phase_code = 'PHASE_COLLECT'),
       (SELECT name FROM cdeu.finv_quote_cdeu_phase WHERE phase_code = 'PHASE_DEDUP')`).
		Scan(&collectName, &dedupName); err != nil {
		t.Fatalf("读取阶段名称失败: %v", err)
	}
	if collectName != "历史行情收集" || dedupName != "历史行情去重" {
		t.Fatalf("阶段名称不符: collect=%s dedup=%s", collectName, dedupName)
	}
	var reviewSeq, reviewProcessCount int
	var reviewName string
	if err := pool.QueryRow(ctx, `
SELECT ph.seq_no, ph.name,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_process ps WHERE ps.phase_id = ph.phase_id)
  FROM cdeu.finv_quote_cdeu_phase ph
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ph.phase_code = 'PHASE_REVIEW'`).
		Scan(&reviewSeq, &reviewName, &reviewProcessCount); err != nil {
		t.Fatalf("读取归档复核阶段失败: %v", err)
	}
	if reviewSeq != 6 || reviewName != "归档复核" || reviewProcessCount != 1 {
		t.Fatalf("归档复核阶段配置不符: seq=%d name=%s processes=%d", reviewSeq, reviewName, reviewProcessCount)
	}
	var recoverPhase, healthPhase string
	if err := pool.QueryRow(ctx, `
SELECT ph.phase_code FROM cdeu.finv_quote_cdeu_process ps JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
 WHERE ps.process_code = 'PROCESS_RECOVER'`).Scan(&recoverPhase); err != nil {
		t.Fatalf("查询 PROCESS_RECOVER 阶段失败: %v", err)
	}
	if err := pool.QueryRow(ctx, `
SELECT ph.phase_code FROM cdeu.finv_quote_cdeu_process ps JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
 WHERE ps.process_code = 'PROCESS_HEALTH'`).Scan(&healthPhase); err != nil {
		t.Fatalf("查询 PROCESS_HEALTH 阶段失败: %v", err)
	}
	if recoverPhase != "PHASE_EXPORT" || healthPhase != "PHASE_UPLOAD" {
		t.Fatalf("旁路 Process 阶段拆分不符: recover=%s health=%s", recoverPhase, healthPhase)
	}
	var embeddedStages int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM cdeu.finv_quote_cdeu_job
 WHERE job_kind = 'RUN_DUE_EXPORT_JOBS'
   AND trigger_mode = 'EMBEDDED'
   AND execution_stage IN ('LOAD','EXPORT','REGISTER','CLEANUP')
   AND manual_rerun_enabled`).Scan(&embeddedStages); err != nil {
		t.Fatalf("统计导出内嵌阶段失败: %v", err)
	}
	if embeddedStages != 4 {
		t.Fatalf("导出内嵌阶段应登记 4 个可手动重跑 JOB，实际 %d", embeddedStages)
	}
	var sealPhase string
	var sealSeq int
	if err := pool.QueryRow(ctx, `
SELECT ph.phase_code, ps.seq_no
  FROM cdeu.finv_quote_cdeu_process ps
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ps.process_code = 'PROCESS_SEAL'`).Scan(&sealPhase, &sealSeq); err != nil {
		t.Fatalf("读取 PROCESS_SEAL 阶段失败: %v", err)
	}
	if sealPhase != "PHASE_DEDUP" || sealSeq != 1 {
		t.Fatalf("PROCESS_SEAL 应位于 PHASE_DEDUP seq=1，实际 %s seq=%d", sealPhase, sealSeq)
	}

	var historyProcessSeq int
	var historyProcessCritical, historyProcessAffects, historyProcessEnabled bool
	var historyProcessJobsCount int
	if err := pool.QueryRow(ctx, `
SELECT ps.seq_no, ps.critical, ps.affects_completion, ps.enabled,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job j WHERE j.process_id = ps.process_id)
  FROM cdeu.finv_quote_cdeu_process ps
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ph.phase_code = 'PHASE_COLLECT'
   AND ps.process_code = 'PROCESS_HISTORY_INGEST'`).
		Scan(&historyProcessSeq, &historyProcessCritical, &historyProcessAffects,
			&historyProcessEnabled, &historyProcessJobsCount); err != nil {
		t.Fatalf("读取历史行情收集占位 Process 失败: %v", err)
	}
	if historyProcessSeq != 1 || historyProcessCritical || !historyProcessAffects ||
		!historyProcessEnabled || historyProcessJobsCount != 0 {
		t.Fatalf("历史行情收集占位 Process 配置不符: seq=%d critical=%t affects=%t enabled=%t jobs=%d",
			historyProcessSeq, historyProcessCritical, historyProcessAffects,
			historyProcessEnabled, historyProcessJobsCount)
	}

	var decisionPhaseSeq int
	var decisionProcessCount int
	if err := pool.QueryRow(ctx, `
SELECT ph.seq_no,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_process ps WHERE ps.phase_id = ph.phase_id)
  FROM cdeu.finv_quote_cdeu_phase ph
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ph.phase_code = 'PHASE_DECISION'`).
		Scan(&decisionPhaseSeq, &decisionProcessCount); err != nil {
		t.Fatalf("读取导出决策阶段失败: %v", err)
	}
	if decisionPhaseSeq != 3 || decisionProcessCount != 4 {
		t.Fatalf("导出决策阶段配置不符: seq=%d processes=%d", decisionPhaseSeq, decisionProcessCount)
	}
	var decisionProcess string
	var decisionProcessSeq, decisionJobs int
	if err := pool.QueryRow(ctx, `
SELECT ps.process_code, ps.seq_no,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job j WHERE j.process_id = ps.process_id)
  FROM cdeu.finv_quote_cdeu_process ps
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ph.phase_code = 'PHASE_DECISION'
   AND ps.process_code = 'PROCESS_EXPORT_DECISION'`).
		Scan(&decisionProcess, &decisionProcessSeq, &decisionJobs); err != nil {
		t.Fatalf("读取导出决策 Process 失败: %v", err)
	}
	if decisionProcess != "PROCESS_EXPORT_DECISION" || decisionProcessSeq != 4 || decisionJobs != 0 {
		t.Fatalf("导出决策 Process 配置不符: process=%s seq=%d jobs=%d",
			decisionProcess, decisionProcessSeq, decisionJobs)
	}
	for _, expected := range []struct {
		code string
		seq  int
	}{{"PROCESS_INGEST_STATS", 1}, {"PROCESS_MATURITY", 2}, {"PROCESS_BUDGET", 3}} {
		var phaseCode string
		var seq int
		if err := pool.QueryRow(ctx, `
SELECT ph.phase_code, ps.seq_no
  FROM cdeu.finv_quote_cdeu_process ps
  JOIN cdeu.finv_quote_cdeu_phase ph ON ph.phase_id = ps.phase_id
  JOIN cdeu.finv_quote_cdeu_workflow w ON w.workflow_id = ph.workflow_id
 WHERE w.workflow_code = 'CDEU_DAILY' AND ps.process_code = $1`, expected.code).
			Scan(&phaseCode, &seq); err != nil {
			t.Fatalf("读取 Process %s 阶段失败: %v", expected.code, err)
		}
		if phaseCode != "PHASE_DECISION" || seq != expected.seq {
			t.Fatalf("Process %s 应位于 PHASE_DECISION seq=%d，实际 %s seq=%d",
				expected.code, expected.seq, phaseCode, seq)
		}
	}

	// 每个 JOB 至少一个 Step；导出内嵌阶段 JOB 使用 STEP_LOAD / STEP_EXPORT / STEP_REGISTER / STEP_CLEANUP。
	var jobsWithoutStep, exportStepCount int
	if err := pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM cdeu.finv_quote_cdeu_job j
         WHERE NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_job_step js WHERE js.job_id = j.job_id)),
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step js
         JOIN cdeu.finv_quote_cdeu_job j ON j.job_id = js.job_id
        WHERE j.job_kind = 'RUN_DUE_EXPORT_JOBS'
          AND js.step_code IN ('STEP_LOAD','STEP_EXPORT','STEP_REGISTER','STEP_CLEANUP'))`).
		Scan(&jobsWithoutStep, &exportStepCount); err != nil {
		t.Fatalf("统计 JOB Step 失败: %v", err)
	}
	if jobsWithoutStep != 0 || exportStepCount != 4 {
		t.Fatalf("JOB Step 配置不符: jobs_without_step=%d export_steps=%d", jobsWithoutStep, exportStepCount)
	}
	var metadataJobSeq, metadataJobStepCount int
	var metadataJobName string
	if err := pool.QueryRow(ctx, `
SELECT j.seq_no, j.name,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step js WHERE js.job_id = j.job_id)
  FROM cdeu.finv_quote_cdeu_job j
  JOIN cdeu.finv_quote_cdeu_process ps ON ps.process_id = j.process_id
 WHERE ps.process_code = 'PROCESS_EXPORT_FILE_MON_BASE'
   AND j.job_code = 'JOB_FETCH_SECURITY_METADATA'`).
		Scan(&metadataJobSeq, &metadataJobName, &metadataJobStepCount); err != nil {
		t.Fatalf("读取证券元数据获取 JOB 失败: %v", err)
	}
	if metadataJobSeq != 1 || metadataJobName != "证券元数据获取" || metadataJobStepCount != 1 {
		t.Fatalf("证券元数据获取 JOB 配置不符: seq=%d name=%s steps=%d",
			metadataJobSeq, metadataJobName, metadataJobStepCount)
	}
	var batchProcessSeq, batchJobSeq, batchJobStepCount int
	var batchProcessName, batchJobName string
	if err := pool.QueryRow(ctx, `
SELECT p.seq_no, p.name, j.seq_no, j.name,
       (SELECT count(*) FROM cdeu.finv_quote_cdeu_job_step js WHERE js.job_id = j.job_id)
  FROM cdeu.finv_quote_cdeu_process p
  JOIN cdeu.finv_quote_cdeu_job j ON j.process_id = p.process_id
 WHERE p.process_code = 'PROCESS_EXPORT_BATCH'
   AND j.job_code = 'JOB_FETCH_EXPORT_BATCH'`).
		Scan(&batchProcessSeq, &batchProcessName, &batchJobSeq, &batchJobName, &batchJobStepCount); err != nil {
		t.Fatalf("读取证券导出批次清单 Process 失败: %v", err)
	}
	if batchProcessSeq != 1 || batchProcessName != "证券导出批次清单获取" ||
		batchJobSeq != 1 || batchJobName != "证券导出批次清单获取" || batchJobStepCount != 1 {
		t.Fatalf("证券导出批次清单配置不符: processSeq=%d process=%s jobSeq=%d job=%s steps=%d",
			batchProcessSeq, batchProcessName, batchJobSeq, batchJobName, batchJobStepCount)
	}

	var workflowRule string
	if err := pool.QueryRow(ctx, `
SELECT completion_rule::text FROM cdeu.finv_quote_cdeu_workflow WHERE workflow_code = 'CDEU_DAILY' AND version = 1`).
		Scan(&workflowRule); err != nil {
		t.Fatalf("读取工作流完成条件失败: %v", err)
	}
	if workflowRule != `{"mode": "ALL"}` && workflowRule != `{"mode":"ALL"}` {
		t.Fatalf("工作流完成条件默认应为 ALL，实际 %s", workflowRule)
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

	// 既有两张业务表结构未被 V100041 改动：主表主键仍为 (ts, usc)。
	var primaryKey string
	if err := pool.QueryRow(ctx, `
SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c
 WHERE c.conrelid = 'cdeu.finv_quote_secu_kline_min'::regclass AND c.contype = 'p'`).Scan(&primaryKey); err != nil {
		t.Fatalf("读取主表主键失败: %v", err)
	}
	if primaryKey != "PRIMARY KEY (ts, usc)" {
		t.Fatalf("主表主键被改动: %s", primaryKey)
	}
}
