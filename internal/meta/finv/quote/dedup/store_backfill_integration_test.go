package dedup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestCDEUJobGate 校验调度侧的三级拓扑顺序门禁（需求 10829 / 10830）：
// 跨阶段、阶段内跨流程、流程内跨 JOB 都必须按 seq_no 依次执行。
//
// 需要一个**可清空的测试库**（会执行全量迁移），DSN 见 TestStoreIntegration。
func TestCDEUJobGate(t *testing.T) {
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
	// 用远期日期隔离，避免影响其它集成用例当天的活动运行。
	bizDate := time.Date(2099, 12, 30, 0, 0, 0, 0, time.UTC)
	cleanup := func(cleanupCtx context.Context) {
		_, _ = pool.Exec(cleanupCtx, `
DELETE FROM cdeu.finv_quote_cdeu_job_run
 WHERE workflow_run_id IN (SELECT run_id FROM cdeu.finv_quote_cdeu_workflow_run WHERE biz_date = $1)`, bizDate)
		_, _ = pool.Exec(cleanupCtx, `
DELETE FROM cdeu.finv_quote_cdeu_process_run
 WHERE workflow_run_id IN (SELECT run_id FROM cdeu.finv_quote_cdeu_workflow_run WHERE biz_date = $1)`, bizDate)
		_, _ = pool.Exec(cleanupCtx, `
DELETE FROM cdeu.finv_quote_cdeu_phase_run
 WHERE workflow_run_id IN (SELECT run_id FROM cdeu.finv_quote_cdeu_workflow_run WHERE biz_date = $1)`, bizDate)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM cdeu.finv_quote_cdeu_workflow_run WHERE biz_date = $1`, bizDate)
	}
	cleanup(ctx)
	defer cleanup(context.Background())

	var workflowID int64
	var workflowCode string
	var version int
	if err := pool.QueryRow(ctx, `
SELECT workflow_id, workflow_code, version FROM cdeu.finv_quote_cdeu_workflow
 WHERE enabled ORDER BY workflow_id LIMIT 1`).Scan(&workflowID, &workflowCode, &version); err != nil {
		t.Fatalf("读取工作流定义失败: %v", err)
	}
	var runID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_workflow_run
  (workflow_id, workflow_code, workflow_version, run_key, biz_date, trigger_type, status, started_at)
VALUES ($1, $2, $3, $4, $5, 'SCHEDULE', 'RUNNING', now()) RETURNING run_id`,
		workflowID, workflowCode, version, "it-gate-"+bizDate.Format("20060102"), bizDate).Scan(&runID); err != nil {
		t.Fatalf("插入工作流运行失败: %v", err)
	}

	at := func(started bool) *time.Time {
		if !started {
			return nil
		}
		now := time.Now()
		return &now
	}
	setPhase := func(phaseCode, status string, started bool) {
		_, _ = pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_phase_run
 WHERE workflow_run_id = $1
   AND phase_id = (SELECT phase_id FROM cdeu.finv_quote_cdeu_phase WHERE phase_code = $2)`, runID, phaseCode)
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_phase_run
  (workflow_run_id, phase_id, phase_code, seq_no, status, started_at)
SELECT $1, ph.phase_id, ph.phase_code, ph.seq_no, $3, $4
  FROM cdeu.finv_quote_cdeu_phase ph WHERE ph.phase_code = $2`,
			runID, phaseCode, status, at(started)); err != nil {
			t.Fatalf("设置阶段 %s 失败: %v", phaseCode, err)
		}
	}
	setProcess := func(processCode, status string, started bool) {
		_, _ = pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_process_run
 WHERE workflow_run_id = $1
   AND process_id = (SELECT process_id FROM cdeu.finv_quote_cdeu_process WHERE process_code = $2 AND enabled LIMIT 1)`,
			runID, processCode)
		// process_run.phase_run_id 非空：先确保所属阶段的 phase_run 存在。
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_phase_run (workflow_run_id, phase_id, phase_code, seq_no, status)
SELECT $1, ph.phase_id, ph.phase_code, ph.seq_no, 'RUNNING'
  FROM cdeu.finv_quote_cdeu_phase ph
  JOIN cdeu.finv_quote_cdeu_process p ON p.phase_id = ph.phase_id
 WHERE p.process_code = $2 AND p.enabled
   AND NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_phase_run pr
                    WHERE pr.workflow_run_id = $1 AND pr.phase_id = ph.phase_id)
 LIMIT 1`, runID, processCode); err != nil {
			t.Fatalf("准备流程 %s 的阶段运行失败: %v", processCode, err)
		}
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_process_run
  (workflow_run_id, process_id, process_code, phase_run_id, seq_no, status, started_at, batch_total)
SELECT $1, p.process_id, p.process_code, pr.phase_run_id, p.seq_no, $3, $4, 0
  FROM cdeu.finv_quote_cdeu_process p
  JOIN cdeu.finv_quote_cdeu_phase_run pr
    ON pr.phase_id = p.phase_id AND pr.workflow_run_id = $1
 WHERE p.process_code = $2 AND p.enabled LIMIT 1`,
			runID, processCode, status, at(started)); err != nil {
			t.Fatalf("设置流程 %s 失败: %v", processCode, err)
		}
	}
	setJobRun := func(jobCode, status string, started bool) {
		_, _ = pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_run
 WHERE workflow_run_id = $1
   AND job_id = (SELECT job_id FROM cdeu.finv_quote_cdeu_job WHERE job_code = $2 LIMIT 1)`, runID, jobCode)
		if _, err := pool.Exec(ctx, `
INSERT INTO cdeu.finv_quote_cdeu_job_run
  (workflow_run_id, process_id, job_id, job_code, job_kind, attempt, status, started_at)
SELECT $1, j.process_id, j.job_id, j.job_code, j.job_kind, 1, $3, $4
  FROM cdeu.finv_quote_cdeu_job j WHERE j.job_code = $2 LIMIT 1`,
			runID, jobCode, status, at(started)); err != nil {
			t.Fatalf("设置 JOB %s 失败: %v", jobCode, err)
		}
	}
	gate := func(kind string) (bool, string) {
		blocked, reason, err := store.CDEUJobGate(ctx, kind, bizDate)
		if err != nil {
			t.Fatalf("读取门禁失败 kind=%s: %v", kind, err)
		}
		return blocked, reason
	}

	// ① 跨阶段：PHASE_COLLECT 未开始 → PHASE_DECISION 的 JOB 被拦。
	setPhase("PHASE_COLLECT", CDEUStatusPending, false)
	if blocked, reason := gate("CALC_DAILY_INGEST_STATS"); !blocked || !strings.Contains(reason, "PHASE_COLLECT") {
		t.Fatalf("前序阶段未开始时应拦住 PHASE_DECISION 的 JOB，实际 blocked=%v reason=%s", blocked, reason)
	}

	// ② 阶段内跨流程：前序阶段全部 SUCCESS，但 PHASE_DECISION 内前序流程未开始 → 拦住。
	setPhase("PHASE_COLLECT", CDEUStatusSuccess, true)
	setPhase("PHASE_DEDUP", CDEUStatusSuccess, true)
	setProcess("PROCESS_INGEST_STATS", CDEUStatusPending, false)
	if blocked, reason := gate("REFRESH_QUOTE_MATURITY"); !blocked || !strings.Contains(reason, "PROCESS_INGEST_STATS") {
		t.Fatalf("同阶段内前序流程未完成时应拦住后序流程的 JOB，实际 blocked=%v reason=%s", blocked, reason)
	}

	// ③ 前序流程完成 → 放行。
	setProcess("PROCESS_INGEST_STATS", CDEUStatusSuccess, true)
	if blocked, reason := gate("REFRESH_QUOTE_MATURITY"); blocked {
		t.Fatalf("前序流程完成后不应被拦，实际 reason=%s", reason)
	}

	// ④ 流程内跨 JOB：PHASE_REVIEW 的 PURGE 流程里，前一个 JOB 未开始 → 后一个被拦。
	for _, code := range []string{"PHASE_DECISION", "PHASE_EXPORT", "PHASE_UPLOAD"} {
		setPhase(code, CDEUStatusSuccess, true)
	}
	setPhase("PHASE_REVIEW", CDEUStatusRunning, true)
	setProcess("PROCESS_RECOVER", CDEUStatusSuccess, true)
	setProcess("PROCESS_REVIEW", CDEUStatusSuccess, true)
	setProcess("PROCESS_PURGE", CDEUStatusRunning, true)
	setJobRun("JOB_PURGE_EXPORTED_MONTH", CDEUStatusPending, false)
	if blocked, reason := gate("PURGE_LOCAL_EXPORT_FILES"); !blocked || !strings.Contains(reason, "JOB_PURGE_EXPORTED_MONTH") {
		t.Fatalf("同流程内前序 JOB 未完成时应拦住后序 JOB，实际 blocked=%v reason=%s", blocked, reason)
	}
	setJobRun("JOB_PURGE_EXPORTED_MONTH", CDEUStatusSuccess, true)
	if blocked, reason := gate("PURGE_LOCAL_EXPORT_FILES"); blocked {
		t.Fatalf("前序 JOB 完成后不应被拦，实际 reason=%s", reason)
	}

	// ⑤ 前序已失败 → 拦住且原因写明失败。
	setPhase("PHASE_DEDUP", CDEUStatusFailed, true)
	if blocked, reason := gate("CALC_DAILY_INGEST_STATS"); !blocked || !strings.Contains(reason, "已失败") {
		t.Fatalf("前序阶段失败时应拦住后序，实际 blocked=%v reason=%s", blocked, reason)
	}
	setPhase("PHASE_DEDUP", CDEUStatusSuccess, true)

	// ⑥ 跨流程编排型 kind（RUN_DUE_EXPORT_JOBS 覆盖 6 个 JOB 定义）：只受阶段门禁约束，
	//    不被自己要推进的流程挡住（否则死锁）。
	setProcess("PROCESS_EXPORT_BATCH", CDEUStatusPending, false)
	if blocked, reason := gate("RUN_DUE_EXPORT_JOBS"); blocked {
		t.Fatalf("多定义 kind 不应被自身推进的流程阻塞，实际 reason=%s", reason)
	}

	// ⑦ 首个阶段（无前序）永不被拦。
	setPhase("PHASE_COLLECT", CDEUStatusPending, false)
	if blocked, reason := gate("CHECK_INGEST_READINESS"); blocked {
		t.Fatalf("首个阶段不应被拦，实际 reason=%s", reason)
	}
}
