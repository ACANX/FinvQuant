package dedup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/syscfg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 表名常量（全部 schema 全限定，见 Docs/DevSpec/DbSchemaSpec.md §3）。
const (
	tableMain        = "cdeu.finv_quote_secu_kline_min"
	tableStaging     = "export.finv_quote_secu_kline_min"
	tableIngestStats = "cdeu.finv_quote_cdeu_ingest_stats"
	tableBudget      = "cdeu.finv_quote_cdeu_daily_budget"
	tableWatermark   = "cdeu.finv_quote_cdeu_scan_watermark"
	tableMaturity    = "cdeu.finv_quote_cdeu_quote_maturity_mon"
	tableCalendar    = "cdeu.finv_quote_cdeu_trade_calendar"
	tableRunLog      = "cdeu.finv_quote_cdeu_job_run_log_export_file"
	tableRunLogLine  = "cdeu.finv_quote_cdeu_job_run_log_line"
	tableUsc         = "cdeu.finv_quote_cdeu_usc"
	tableJob         = "cdeu.finv_quote_cdeu_export_job"
	tableFile        = "cdeu.finv_quote_cdeu_export_file"
	tableUploadLog   = "cdeu.finv_quote_cdeu_export_file_upload_log"
	tablePatch       = "cdeu.finv_quote_cdeu_export_file_patch"
	tableBatch       = "cdeu.finv_quote_cdeu_export_batch"
	tableDynamicJob  = "cdeu.finv_quote_cdeu_job_dynamic"
	tableDynamicStep = "cdeu.finv_quote_cdeu_job_step_dynamic"
)

// jobColumns 任务查询列（顺序与 Job 结构体 db 标签一致）。
const jobColumns = `job_id, usc, biz_month, priority_rank, freeze_date, seal_date, status,
	load_cursor_date, load_cursor_ts, loaded_rows, base_version, patch_count, oldest_pending_date,
	locked_by, locked_until, gmt_heartbeat, attempt, gmt_next_run, last_error, gmt_create, gmt_update`

// fileColumns 文件查询列（顺序与 File 结构体 db 标签一致）。
const fileColumns = `file_id, job_id, usc, biz_month, file_type, version, file_name, file_path,
	target_file_name, upload_target, format, compression, row_count, file_size, checksum,
	manifest_path, seal_date, export_status, upload_status, upload_attempt, gmt_next_retry,
	gmt_upload_start, gmt_upload_finish, remote_checksum_verified, network_status,
	cleanup_status, gmt_create, gmt_update`

// jobColumnsAliased 带 `j.` 前缀的任务列（用于 UPDATE ... FROM 的 RETURNING，避免列名歧义）。
const jobColumnsAliased = `j.job_id, j.usc, j.biz_month, j.priority_rank, j.freeze_date, j.seal_date, j.status,
	j.load_cursor_date, j.load_cursor_ts, j.loaded_rows, j.base_version, j.patch_count, j.oldest_pending_date,
	j.locked_by, j.locked_until, j.gmt_heartbeat, j.attempt, j.gmt_next_run, j.last_error, j.gmt_create, j.gmt_update`

// fileColumnsAliased 带 `f.` 前缀的文件列（用于 UPDATE ... FROM 的 RETURNING，避免列名歧义）。
const fileColumnsAliased = `f.file_id, f.job_id, f.usc, f.biz_month, f.file_type, f.version, f.file_name, f.file_path,
	f.target_file_name, f.upload_target, f.format, f.compression, f.row_count, f.file_size, f.checksum,
	f.manifest_path, f.seal_date, f.export_status, f.upload_status, f.upload_attempt, f.gmt_next_retry,
	f.gmt_upload_start, f.gmt_upload_finish, f.remote_checksum_verified, f.network_status,
	f.cleanup_status, f.gmt_create, f.gmt_update`

// Store 归档链路的数据访问层（只做 SQL 与行映射，不含业务编排）。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建 Store。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool 返回底层连接池（供需要自行流式读取的调用方使用）。
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// ---------------------------------------------------------------------
// 扫描水位
// ---------------------------------------------------------------------

// Watermark 读取水位；不存在时返回 found=false。
func (s *Store) Watermark(ctx context.Context, name string) (time.Time, bool, error) {
	var value time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last_value FROM `+tableWatermark+` WHERE watermark_name = $1`, name).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return value, true, nil
}

// UpsertWatermark 写入/推进水位。
func (s *Store) UpsertWatermark(ctx context.Context, name string, value time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableWatermark+` (watermark_name, last_value, gmt_update)
VALUES ($1, $2, now())
ON CONFLICT (watermark_name) DO UPDATE SET last_value = EXCLUDED.last_value, gmt_update = now()`,
		name, value)
	return err
}

// ---------------------------------------------------------------------
// 运行日志
// ---------------------------------------------------------------------

// StartRun 登记一次 Job 运行，返回 run_id。
func (s *Store) StartRun(ctx context.Context, kind string, jobID *int64, usc string, bizMonth *int) (int64, error) {
	var uscArg *string
	if usc != "" {
		uscArg = &usc
	}
	var runID int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO `+tableRunLog+` (job_kind, job_id, usc, biz_month, status, stage, progress_percent, gmt_heartbeat)
VALUES ($1, $2, $3, $4, 'RUNNING', 'RUNNING', 0, now()) RETURNING run_id`, kind, jobID, uscArg, bizMonth).Scan(&runID)
	if err != nil {
		return runID, err
	}
	// 详细日志是辅助诊断信息，写入失败不影响 Job 主流程。
	_ = s.AppendRunLog(ctx, runID, "INFO", "RUNNING", "JOB 开始 · "+kind, nil)
	return runID, nil
}

// FinishRun 结束一次 Job 运行并记录处理量与结果。
//
// 结束时同步收口阶段与进度：成功记 DONE/100，失败与暂停沿用终态作为阶段，
// 便于总览页直接展示「最近一次跑到哪一步」。
func (s *Store) FinishRun(ctx context.Context, runID int64, status string, rows, bytes int64, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableRunLog+`
   SET gmt_finish = now(), status = $2, processed_rows = $3, processed_bytes = $4, error = $5,
       stage = CASE WHEN $2 = 'SUCCESS' THEN 'DONE' ELSE $2 END,
       progress_percent = CASE WHEN $2 = 'SUCCESS' THEN 100 ELSE progress_percent END,
       gmt_heartbeat = now()
 WHERE run_id = $1`, runID, status, rows, bytes, errArg)
	if err != nil {
		return err
	}
	level := "INFO"
	if status != RunStatusSuccess {
		level = "ERROR"
	}
	message := "JOB 完成 · " + status
	if errText != "" {
		message += " · " + errText
	}
	_ = s.AppendRunLog(ctx, runID, level, status, message, nil)
	return nil
}

// UpdateRunProgress 更新运行中的阶段、进度与心跳。
//
// 心跳用于前端判断「正常进行中 / 卡住」；percent 为负表示无法估算总量的不确定进度。
func (s *Store) UpdateRunProgress(ctx context.Context, runID int64, stage string, percent float64, detail string) error {
	var detailArg *string
	if detail != "" {
		detailArg = &detail
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableRunLog+`
   SET stage = $2, progress_percent = $3, progress_detail = $4, gmt_heartbeat = now()
 WHERE run_id = $1`, runID, stage, percent, detailArg)
	if err != nil {
		return err
	}
	message := detail
	if message == "" {
		message = stage
	}
	_ = s.AppendRunLog(ctx, runID, "INFO", stage, message, &percent)
	return nil
}

// AppendRunLog 追加一条 Job 运行详细日志；line 写入失败不应中断主流程。
func (s *Store) AppendRunLog(ctx context.Context, runID int64, level, stage, message string, percent *float64) error {
	if runID <= 0 || message == "" {
		return nil
	}
	var stageArg *string
	if stage != "" {
		stageArg = &stage
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableRunLogLine+` (run_id, level, stage, message, progress_percent)
VALUES ($1, $2, $3, $4, $5)`, runID, level, stageArg, message, percent)
	return err
}

// ---------------------------------------------------------------------
// 入口流量统计与每日预算
// ---------------------------------------------------------------------

// CalcIngestStats 按 gmt_create 聚合指定时间窗内的入口流量，写入每日统计（幂等覆盖）。
func (s *Store) CalcIngestStats(ctx context.Context, from, to time.Time, timeZone string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableIngestStats+` (stat_date, total_rows, total_bytes, gmt_update)
SELECT (t.gmt_create AT TIME ZONE $3)::date AS stat_date,
       count(*),
       COALESCE(sum(pg_column_size(t)), 0),
       now()
  FROM `+tableMain+` t
 WHERE t.gmt_create >= $1 AND t.gmt_create < $2
 GROUP BY 1
ON CONFLICT (stat_date) DO UPDATE
   SET total_rows = EXCLUDED.total_rows,
       total_bytes = EXCLUDED.total_bytes,
       gmt_update = now()`, from, to, timeZone)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// IngestBaseline 近 N 天入口统计基线（日均条数、日均字节数、条数 P75）。
type IngestBaseline struct {
	BaseRows  float64
	BaseBytes float64
	P75Rows   float64
}

// IngestStatsBaseline 计算近 N 天入口基线。
func (s *Store) IngestStatsBaseline(ctx context.Context, from, to time.Time) (IngestBaseline, error) {
	var baseline IngestBaseline
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(avg(total_rows), 0)::float8,
       COALESCE(avg(total_bytes), 0)::float8,
       COALESCE((percentile_cont(0.75) WITHIN GROUP (ORDER BY total_rows))::float8, 0)
  FROM `+tableIngestStats+`
 WHERE stat_date >= $1::date AND stat_date <= $2::date`, from, to).
		Scan(&baseline.BaseRows, &baseline.BaseBytes, &baseline.P75Rows)
	return baseline, err
}

// PendingRows 统计未完成导出的 COMPLETE/FROZEN 证券月行数（积压量）。
func (s *Store) PendingRows(ctx context.Context) (int64, error) {
	var rows int64
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(sum(m.actual_rows), 0)::bigint
  FROM `+tableMaturity+` m
  LEFT JOIN `+tableJob+` j ON j.usc = m.usc AND j.biz_month = m.biz_month
 WHERE m.maturity_status IN ('COMPLETE', 'FROZEN')
   AND (j.job_id IS NULL OR j.status <> 'EXPORTED')`).Scan(&rows)
	return rows, err
}

// SLAPressure 统计「N 天内到达 freeze_date 且未完成」的任务占比。
func (s *Store) SLAPressure(ctx context.Context, today time.Time, withinDays int) (float64, error) {
	var pressure float64
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(
         count(*) FILTER (WHERE j.freeze_date <= $1::date + $2::int)::float8
         / NULLIF(count(*), 0)::float8, 0)
  FROM `+tableJob+` j
 WHERE j.status IN ('WAITING', 'LOADING', 'LOADED', 'EXPORTING', 'PAUSED', 'FAILED', 'FROZEN')`,
		today, withinDays).Scan(&pressure)
	return pressure, err
}

// UpsertBudget 写入当日预算（重算时保留已消耗量，预算不低于已消耗）。
func (s *Store) UpsertBudget(ctx context.Context, budget Budget) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableBudget+`
  (biz_date, base_rows, base_bytes, p75_rows, backlog_ratio, sla_pressure, factor,
   budget_rows, budget_bytes, hard_cap_rows, consumed_rows, consumed_bytes, status, gmt_create)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0, 0, $11, now())
ON CONFLICT (biz_date) DO UPDATE
   SET base_rows = EXCLUDED.base_rows,
       base_bytes = EXCLUDED.base_bytes,
       p75_rows = EXCLUDED.p75_rows,
       backlog_ratio = EXCLUDED.backlog_ratio,
       sla_pressure = EXCLUDED.sla_pressure,
       factor = EXCLUDED.factor,
       budget_rows = GREATEST(EXCLUDED.budget_rows, `+tableBudget+`.consumed_rows),
       budget_bytes = EXCLUDED.budget_bytes,
       hard_cap_rows = EXCLUDED.hard_cap_rows`,
		budget.BizDate, budget.BaseRows, budget.BaseBytes, budget.P75Rows,
		budget.BacklogRatio, budget.SLAPressure, budget.Factor,
		budget.BudgetRows, budget.BudgetBytes, budget.HardCapRows, budget.Status)
	return err
}

// GetBudget 读取某日预算；不存在时返回 nil。
func (s *Store) GetBudget(ctx context.Context, bizDate time.Time) (*Budget, error) {
	var budget Budget
	err := s.pool.QueryRow(ctx, `
SELECT biz_date, base_rows, base_bytes, COALESCE(p75_rows, 0), COALESCE(backlog_ratio, 0)::float8,
       COALESCE(sla_pressure, 0)::float8, factor::float8, budget_rows, budget_bytes,
       hard_cap_rows, consumed_rows, consumed_bytes, status
  FROM `+tableBudget+` WHERE biz_date = $1::date`, bizDate).
		Scan(&budget.BizDate, &budget.BaseRows, &budget.BaseBytes, &budget.P75Rows,
			&budget.BacklogRatio, &budget.SLAPressure, &budget.Factor,
			&budget.BudgetRows, &budget.BudgetBytes, &budget.HardCapRows,
			&budget.ConsumedRows, &budget.ConsumedBytes, &budget.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

// AddBudgetConsumed 累加预算消耗；达到预算时置 DONE。
func (s *Store) AddBudgetConsumed(ctx context.Context, bizDate time.Time, rows, bytes int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableBudget+`
   SET consumed_rows = consumed_rows + $2,
       consumed_bytes = consumed_bytes + $3,
       status = CASE WHEN consumed_rows + $2 >= budget_rows THEN 'DONE' ELSE 'RUNNING' END
 WHERE biz_date = $1::date`, bizDate, rows, bytes)
	return err
}

// SetBudgetStatus 更新预算状态（RUNNING / DONE / CALCULATED）。
func (s *Store) SetBudgetStatus(ctx context.Context, bizDate time.Time, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE `+tableBudget+` SET status = $2 WHERE biz_date = $1::date`, bizDate, status)
	return err
}

// ---------------------------------------------------------------------
// 成熟度
// ---------------------------------------------------------------------

const maturityUpsertSuffix = `
ON CONFLICT (usc, trade_date) DO UPDATE
   SET expected_rows = EXCLUDED.expected_rows,
       actual_rows = EXCLUDED.actual_rows,
       biz_month = EXCLUDED.biz_month,
       maturity_status = CASE WHEN ` + tableMaturity + `.maturity_status = 'FROZEN'
                              THEN 'FROZEN' ELSE EXCLUDED.maturity_status END,
       gmt_first_seen = COALESCE(` + tableMaturity + `.gmt_first_seen, EXCLUDED.gmt_first_seen),
       gmt_last_arrival = GREATEST(COALESCE(` + tableMaturity + `.gmt_last_arrival, EXCLUDED.gmt_last_arrival),
                                   EXCLUDED.gmt_last_arrival),
       gmt_update = now()`

// maturityUpsertHead 生成成熟度 upsert 的公共前缀：cnt（逐日全量行数）→ exp（预期条数与交易日标记）。
//
// thresholdParam 为阈值占位符编号（调用方传参个数不同，避免出现未使用的 $n）。
func maturityUpsertHead(sourceCTE string, thresholdParam int) string {
	return `
WITH cnt AS (` + sourceCTE + `),
exp AS (
  SELECT c.usc, c.trade_date, c.actual_rows, c.gmt_last,
         COALESCE(u.day_expect_rows, 0) AS day_expect,
         COALESCE(tc.flag_trade_day, true) AS flag_trade_day
    FROM cnt c
    LEFT JOIN ` + tableUsc + ` u ON u.usc = c.usc
    LEFT JOIN ` + tableCalendar + ` tc ON tc.usc = c.usc AND tc.trade_date = c.trade_date
)
INSERT INTO ` + tableMaturity + `
  (usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status,
   gmt_first_seen, gmt_last_arrival, gmt_update)
SELECT e.usc, e.trade_date, e.trade_date / 100,
       CASE WHEN NOT e.flag_trade_day THEN 0 ELSE e.day_expect END,
       e.actual_rows,
       CASE
         WHEN NOT e.flag_trade_day THEN 'COMPLETE'
         WHEN e.day_expect <= 0 THEN 'PARTIAL'
         WHEN e.actual_rows::numeric >= e.day_expect::numeric * $` + strconv.Itoa(thresholdParam) + `::numeric THEN 'COMPLETE'
         ELSE 'PARTIAL'
       END,
       now(), e.gmt_last, now()
  FROM exp e` + maturityUpsertSuffix
}

// RefreshMaturityIncremental 按 gmt_update 水位增量刷新成熟度（J-04）。
func (s *Store) RefreshMaturityIncremental(ctx context.Context, since, until time.Time, threshold float64) (int64, error) {
	sql := maturityUpsertHead(`
  SELECT t.usc, t.trade_date, t.actual_rows, t.gmt_last
    FROM (
      SELECT DISTINCT m.usc, m.date AS trade_date
        FROM `+tableMain+` m
       WHERE m.gmt_update > $1 AND m.gmt_update <= $2 AND m.date IS NOT NULL
    ) d
    JOIN (
      SELECT m.usc, m.date AS trade_date, count(*)::int AS actual_rows, max(m.gmt_update) AS gmt_last
        FROM `+tableMain+` m
       WHERE m.gmt_update > $1 AND m.gmt_update <= $2 AND m.date IS NOT NULL
       GROUP BY m.usc, m.date
    ) t ON t.usc = d.usc AND t.trade_date = d.trade_date`, 3)
	return s.execMaturityUpsert(ctx, sql, since, until, thresholdString(threshold))
}

// RecalibrateMaturity 按交易日期范围全量校准成熟度（防止水位漏行导致长期偏差）。
func (s *Store) RecalibrateMaturity(ctx context.Context, dateFrom int, threshold float64) (int64, error) {
	sql := maturityUpsertHead(`
  SELECT m.usc, m.date AS trade_date, count(*)::int AS actual_rows, max(m.gmt_update) AS gmt_last
    FROM `+tableMain+` m
   WHERE m.date >= $1 AND m.date IS NOT NULL
   GROUP BY m.usc, m.date`, 2)
	return s.execMaturityUpsert(ctx, sql, dateFrom, thresholdString(threshold))
}

func (s *Store) execMaturityUpsert(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := s.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// thresholdString 把阈值转成 numeric 文本参数（避免 float8 → numeric 隐式转换缺失）。
func thresholdString(threshold float64) string {
	return fmt.Sprintf("%.6f", threshold)
}

// DiscoverUsc 基于主表扫描自动发现新 usc，并回填 provider（P1）。
func (s *Store) DiscoverUsc(ctx context.Context, since, until time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableUsc+` (usc, flag_auto_discover)
SELECT DISTINCT m.usc, true
  FROM `+tableMain+` m
 WHERE m.gmt_update > $1 AND m.gmt_update <= $2
   AND m.usc IS NOT NULL
ON CONFLICT (usc) DO NOTHING`, since, until)
	if err != nil {
		return 0, err
	}
	inserted := tag.RowsAffected()

	if _, err := s.pool.Exec(ctx, `
UPDATE `+tableUsc+` u
   SET provider = COALESCE(u.provider, src.provider), gmt_update = now()
  FROM (
    SELECT m.usc, max(m.provider) AS provider
      FROM `+tableMain+` m
     WHERE m.gmt_update > $1 AND m.gmt_update <= $2 AND m.provider IS NOT NULL
     GROUP BY m.usc
  ) src
 WHERE u.usc = src.usc AND (u.provider IS NULL OR u.provider = '')`, since, until); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// RefreshSecurityMetadata 从 finv_security / finv_exchange / finv_market / finv_region 刷新证券主数据。
//
// 取值口径（ACANX 2026-10-26 调整，见迁移 V100036）：
//   - market：取 finv_market.market_symbol（市场符号，如 SH / SZ / BJ / NSDQ / CFD）；
//     为空串时依次回退 market_flag、exchange_flag，避免字段未维护时值变成 NULL。
//     历史口径直接拄 market_flag（SSE-A / US-NSDQ 这种板块级标识），与归档
//     目录 {region}_{market} 需要的市场级符号不一致，故改从 market_symbol 取。
//   - region：以 finv_region 字典为准（LEFT JOIN 规范化，如 exchange.region 写成
//     USA 时按字典取 US）；字典无对应项时回退 exchange.region 原值，不丢信息。
//
// 覆盖策略：region / market / currency 是**派生值**，以主数据为准覆盖（新值为
// NULL 时保留旧值）；其余列（category/date_list/name/timezone）仍仅在缺失时
// 补齐——name/timezone 可能被人工维护（中文名、时区别名），不盲覆盖。
//
// 关于 currency：取自 finv_security.currency_type（如港股人民币柜台 81810 =
// CNY）。早期版本只补 NULL，导致主数据修正后 usc 长期停在旧币种，进而写错
// MVsv 文件头的 "# Currency" 行，故改为可覆盖。
func (s *Store) RefreshSecurityMetadata(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableUsc+`
    (usc, category, date_list, flag_auto_discover, region, market, name, timezone, currency, gmt_update)
SELECT s.usc,
       NULLIF(s.security_type, ''),
       CASE
           WHEN s.init_date BETWEEN 19500101 AND 21001231
           THEN to_date(s.init_date::text, 'YYYYMMDD')
           ELSE NULL
       END,
       false,
       COALESCE(r.region, NULLIF(e.region, '')),
       COALESCE(NULLIF(m.market_symbol, ''), NULLIF(m.market_flag, ''), NULLIF(e.exchange_flag, '')),
       COALESCE(NULLIF(s.security_name_cn, ''), NULLIF(s.security_name, '')),
       COALESCE(NULLIF(s.tz, ''), NULLIF(s.timezone, '')),
       NULLIF(s.currency_type, ''),
       now()
  FROM public.finv_security s
  LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
  LEFT JOIN public.finv_market m ON m.market_code = s.market_code
  LEFT JOIN public.finv_region r ON r.region = e.region
 WHERE COALESCE(s.flag_enable, '1') = '1'
ON CONFLICT (usc) DO UPDATE SET
    category = COALESCE(`+tableUsc+`.category, EXCLUDED.category),
    date_list = COALESCE(`+tableUsc+`.date_list, EXCLUDED.date_list),
    -- 派生列：以字典/主数据为准覆盖（新值为 NULL 时保留旧值）
    region = COALESCE(EXCLUDED.region, `+tableUsc+`.region),
    market = COALESCE(EXCLUDED.market, `+tableUsc+`.market),
    name = COALESCE(`+tableUsc+`.name, EXCLUDED.name),
    timezone = COALESCE(`+tableUsc+`.timezone, EXCLUDED.timezone),
    currency = COALESCE(EXCLUDED.currency, `+tableUsc+`.currency),
    gmt_update = now()
 WHERE EXCLUDED.region IS NOT NULL AND `+tableUsc+`.region IS DISTINCT FROM EXCLUDED.region
    OR EXCLUDED.market IS NOT NULL AND `+tableUsc+`.market IS DISTINCT FROM EXCLUDED.market
    OR EXCLUDED.currency IS NOT NULL AND `+tableUsc+`.currency IS DISTINCT FROM EXCLUDED.currency
    OR `+tableUsc+`.category IS NULL
    OR `+tableUsc+`.date_list IS NULL
    OR `+tableUsc+`.name IS NULL
    OR `+tableUsc+`.timezone IS NULL
    OR `+tableUsc+`.currency IS NULL`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// SealMaturity 把已到收口日（含）以前的月份成熟度统一冻结为 FROZEN（J-06）。
func (s *Store) SealMaturity(ctx context.Context, maxBizMonth int) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableMaturity+`
   SET maturity_status = 'FROZEN',
       gmt_seale = COALESCE(gmt_seale, now()),
       gmt_update = now()
 WHERE biz_month <= $1 AND maturity_status <> 'FROZEN'`, maxBizMonth)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// FrozenDiffReport 输出 FROZEN 证券月的差异报告（缺失日期 / 缺失行数 / 缺失比例）。
func (s *Store) FrozenDiffReport(ctx context.Context, minBizMonth int, limit int) ([]DiffRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT m.usc,
       m.biz_month,
       count(*) FILTER (WHERE COALESCE(tc.flag_trade_day, true))::bigint AS expected_days,
       count(*) FILTER (WHERE COALESCE(tc.flag_trade_day, true) AND m.actual_rows = 0)::bigint AS missing_days,
       COALESCE(sum(m.expected_rows), 0)::bigint AS expected_rows,
       COALESCE(sum(m.actual_rows), 0)::bigint AS actual_rows,
       COALESCE(sum(GREATEST(m.expected_rows - m.actual_rows, 0)), 0)::bigint AS missing_rows
  FROM `+tableMaturity+` m
  LEFT JOIN `+tableCalendar+` tc ON tc.usc = m.usc AND tc.trade_date = m.trade_date
 WHERE m.maturity_status = 'FROZEN' AND m.biz_month >= $1
 GROUP BY m.usc, m.biz_month
HAVING COALESCE(sum(GREATEST(m.expected_rows - m.actual_rows, 0)), 0) > 0
 ORDER BY missing_rows DESC
 LIMIT $2`, minBizMonth, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]DiffRow, 0, 16)
	for rows.Next() {
		var item DiffRow
		if err := rows.Scan(&item.Usc, &item.BizMonth, &item.ExpectedDays, &item.MissingDays,
			&item.ExpectedRows, &item.ActualRows, &item.MissingRows); err != nil {
			return nil, err
		}
		if item.ExpectedRows > 0 {
			item.MissingRatio = float64(item.MissingRows) / float64(item.ExpectedRows)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------
// 任务：候选生成、领取、进度、状态
// ---------------------------------------------------------------------

// GenerateExportBatch 生成当前批次的导出清单，写入 cdeu.finv_quote_cdeu_export_batch。
//
// BASE：按成熟度/元数据条件生成待导出证券月；
// INCR：已存在 BASE 文件的证券月作为增量候选。
// 该表作为后续 PROCESS_EXPORT_FILE_MON_BASE / PROCESS_EXPORT_FILE_MON_INCR 的输入。
func (s *Store) GenerateExportBatch(ctx context.Context, bizDate time.Time, currentMonth int) (int64, int64, error) {
	batchNo := bizDate.Format("20060102")
	if _, err := s.pool.Exec(ctx, `DELETE FROM `+tableBatch+` WHERE batch_no = $1`, batchNo); err != nil {
		return 0, 0, err
	}

	baseTag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableBatch+`
    (batch_no, biz_date, usc, biz_month, export_type, source_job_id, status)
SELECT $1, $2::date, m.usc, m.biz_month, 'BASE', NULL, 'READY'
  FROM `+tableMaturity+` m
  JOIN `+tableUsc+` u ON u.usc = m.usc
 WHERE m.biz_month < $3
   AND u.region IS NOT NULL AND u.market IS NOT NULL AND u.provider IS NOT NULL
   AND u.name IS NOT NULL AND u.timezone IS NOT NULL AND u.currency IS NOT NULL
   AND (u.date_delist IS NULL OR u.date_delist >= make_date(m.biz_month / 100, m.biz_month % 100, 1))
   AND NOT EXISTS (
       SELECT 1 FROM `+tableJob+` j
        WHERE j.usc = m.usc AND j.biz_month = m.biz_month
          AND j.status IN ('LOADING', 'LOADED', 'EXPORTING', 'EXPORTED'))
 GROUP BY m.usc, m.biz_month, u.priority_rank, u.sla_days
HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))
ON CONFLICT (batch_no, usc, biz_month, export_type) DO UPDATE SET
    biz_date = EXCLUDED.biz_date,
    status = 'READY',
    gmt_update = now()`, batchNo, bizDate, currentMonth)
	if err != nil {
		return 0, 0, err
	}

	incrTag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableBatch+`
    (batch_no, biz_date, usc, biz_month, export_type, source_job_id, status)
SELECT $1, $2::date, j.usc, j.biz_month, 'INCR', j.job_id, 'READY'
  FROM `+tableJob+` j
 WHERE j.status = 'EXPORTED'
   AND EXISTS (
       SELECT 1 FROM `+tableFile+` f
        WHERE f.job_id = j.job_id AND f.file_type = 'BASE' AND f.export_status = 'EXPORTED')
ON CONFLICT (batch_no, usc, biz_month, export_type) DO UPDATE SET
    biz_date = EXCLUDED.biz_date,
    source_job_id = EXCLUDED.source_job_id,
    status = 'READY',
    gmt_update = now()`, batchNo, bizDate)
	if err != nil {
		return baseTag.RowsAffected(), 0, err
	}
	return baseTag.RowsAffected(), incrTag.RowsAffected(), nil
}

// GenerateDynamicJobs 按批次清单生成单证券导出 JOB。
//
// BASE 条目挂在 PROCESS_EXPORT_FILE_MON_BASE，INCR 条目挂在
// PROCESS_EXPORT_FILE_MON_INCR；自 V200045 起唯一键为
// (workflow_run_id, usc, biz_month)，每次运行各自持有独立实例。
func (s *Store) GenerateDynamicJobs(ctx context.Context, batchNo string) (int64, error) {
	batchRunID := cdeuRunIDFromContext(ctx)
	tag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicJob+`
    (process_id, job_code, name, usc, biz_month, export_type, export_job_id,
     job_category, seq_no, status, workflow_run_id)
SELECT p.process_id,
       'JOB_EXPORT_' || b.export_type || '_' ||
       regexp_replace(b.usc, '[^A-Za-z0-9]', '_', 'g') || '_' || b.biz_month,
       'EXPORT_MON_' || b.export_type || '_' || b.usc || '_' || b.biz_month,
       b.usc, b.biz_month, b.export_type,
       COALESCE(b.source_job_id, j.job_id), 'EXPORT',
       row_number() OVER (PARTITION BY p.process_id ORDER BY b.usc, b.biz_month),
       'PENDING', NULLIF($2::bigint, 0)
  FROM `+tableBatch+` b
  JOIN cdeu.finv_quote_cdeu_process p
    ON p.process_code = CASE b.export_type
        WHEN 'BASE' THEN 'PROCESS_EXPORT_FILE_MON_BASE'
        ELSE 'PROCESS_EXPORT_FILE_MON_INCR'
       END
  LEFT JOIN `+tableJob+` j ON j.usc = b.usc AND j.biz_month = b.biz_month
 WHERE b.batch_no = $1
   AND b.export_type = 'BASE'
   AND (b.source_job_id IS NOT NULL OR j.job_id IS NOT NULL)
ON CONFLICT (workflow_run_id, usc, biz_month) WHERE job_category = 'EXPORT' DO NOTHING`,
		batchNo, batchRunID)
	if err != nil {
		return 0, err
	}
	if _, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicStep+`
    (dynamic_job_id, step_code, name, seq_no, status)
SELECT dj.dynamic_job_id, v.step_code, v.name, v.seq_no, 'PENDING'
  FROM `+tableDynamicJob+` dj
 CROSS JOIN (VALUES
    ('STEP_LOAD', '装载', 1),
    ('STEP_EXPORT', '文件生成', 2),
    ('STEP_REGISTER', '文件登记', 3),
    ('STEP_CLEANUP', '中转清理', 4)
 ) AS v(step_code, name, seq_no)
 WHERE dj.process_id IN (
     SELECT process_id FROM cdeu.finv_quote_cdeu_process
      WHERE process_code IN ('PROCESS_EXPORT_FILE_MON_BASE', 'PROCESS_EXPORT_FILE_MON_INCR'))
   AND dj.workflow_run_id IS NOT DISTINCT FROM NULLIF($1::bigint, 0)
ON CONFLICT DO NOTHING`, batchRunID); err != nil {
		return tag.RowsAffected(), err
	}
	return tag.RowsAffected(), nil
}

// dynamicJobSelectionSQL 生成「按 (usc, biz_month) 定位目标动态导出实例」的子查询。
//
// 自 V200045 起动态实例按运行隔离，定位规则：
//
//	优先匹配调用方上下文携带的 workflow run（runParam 为 0 时视为无）；
//	无匹配时回退到最新一条，兼容手动单实例执行。
//
// 子查询固定使用 $1=usc、$2=biz_month，运行 ID 占位由 runParam 指定。
func dynamicJobSelectionSQL(runParam string) string {
	return `(
    SELECT dj2.dynamic_job_id
      FROM cdeu.finv_quote_cdeu_job_dynamic dj2
     WHERE dj2.usc = $1 AND dj2.biz_month = $2 AND dj2.job_category = 'EXPORT'
       AND (` + runParam + `::bigint = 0 OR dj2.workflow_run_id = ` + runParam + `::bigint)
     ORDER BY (dj2.workflow_run_id = ` + runParam + `::bigint) DESC NULLS LAST,
              dj2.dynamic_job_id DESC
     LIMIT 1)`
}

// MarkDynamicJobStepStatus 更新动态 JOB 内指定 Step 状态。
//
// 自 V200045 起动态实例按运行隔离，目标实例由 dynamicJobSelectionSQL 定位。
func (s *Store) MarkDynamicJobStepStatus(ctx context.Context, usc string, bizMonth int,
	stepCode, status, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicStep+` ds
   SET status = $4,
       started_at = CASE WHEN $4 = 'RUNNING' THEN COALESCE(ds.started_at, now()) ELSE ds.started_at END,
       finished_at = CASE WHEN $4 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE ds.finished_at END,
       error = $5,
       gmt_update = now()
 WHERE ds.dynamic_job_id = `+dynamicJobSelectionSQL("$6")+`
   AND ds.step_code = $3`,
		usc, bizMonth, stepCode, status, errArg, cdeuRunIDFromContext(ctx))
	return err
}

// FailDynamicJobSteps 将动态 JOB 内未结束的 Step 统一置为失败。
func (s *Store) FailDynamicJobSteps(ctx context.Context, usc string, bizMonth int, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicStep+` ds
   SET status = 'FAILED', finished_at = now(), error = $3, gmt_update = now()
 WHERE ds.dynamic_job_id = `+dynamicJobSelectionSQL("$4")+`
   AND ds.status IN ('PENDING','RUNNING')`,
		usc, bizMonth, errArg, cdeuRunIDFromContext(ctx))
	return err
}

// CompleteDynamicJobSteps 将动态 JOB 内未结束的 Step 统一置为成功。
func (s *Store) CompleteDynamicJobSteps(ctx context.Context, usc string, bizMonth int) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicStep+` ds
   SET status = 'SUCCESS', finished_at = COALESCE(ds.finished_at, now()), error = NULL, gmt_update = now()
 WHERE ds.dynamic_job_id = `+dynamicJobSelectionSQL("$3")+`
   AND ds.status IN ('PENDING','RUNNING')`,
		usc, bizMonth, cdeuRunIDFromContext(ctx))
	return err
}

// GenerateUploadDynamicJobs 基于已导出且未完成上传的月归档文件生成上传动态 JOB。
//
// 自 V200045 起唯一键为 (workflow_run_id, file_id)，每次运行各自持有独立实例。
func (s *Store) GenerateUploadDynamicJobs(ctx context.Context) (int64, error) {
	uploadRunID := cdeuRunIDFromContext(ctx)
	tag, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicJob+`
    (process_id, job_code, name, usc, biz_month, export_type, export_job_id,
     job_category, file_id, file_type, file_version, seq_no, status, workflow_run_id)
SELECT p.process_id,
       'JOB_UPLOAD_' || f.file_type || '_V' || f.version || '_' || upper(f.usc) || '_' || f.biz_month,
       'UPLOAD_' || f.file_type || '_V' || f.version || '_' || f.usc || '_' || f.biz_month,
       f.usc, f.biz_month, NULL, f.job_id,
       'UPLOAD', f.file_id, f.file_type, f.version,
       row_number() OVER (PARTITION BY p.process_id ORDER BY f.file_id),
       -- 上传实例初始状态（需求 10738）：UPLOADING 是「上一次上传被中断」留下的脏状态，
       -- 必须回到 PENDING 重新排队。原实现映射为 RUNNING 且不写 gmt_start，会生成
       -- 一条永不执行、也不超时的僵尸实例（run 60 的 JOB_UPLOAD_BASE_V2_GCMAIN_202201）。
       CASE f.upload_status
           WHEN 'UPLOAD_FAILED' THEN 'FAILED'
           ELSE 'PENDING'
       END,
       NULLIF($1::bigint, 0)
  FROM `+tableFile+` f
  JOIN cdeu.finv_quote_cdeu_process p ON p.process_code = 'PROCESS_UPLOAD'
 WHERE f.export_status = 'EXPORTED'
   AND f.upload_status NOT IN ('UPLOADED', 'SKIPPED')
ON CONFLICT (workflow_run_id, file_id) WHERE job_category = 'UPLOAD' DO NOTHING`,
		uploadRunID)
	if err != nil {
		return 0, err
	}
	if _, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicStep+`
    (dynamic_job_id, step_code, name, seq_no, status)
SELECT dj.dynamic_job_id, 'STEP_UPLOAD', '文件上传', 1, 'PENDING'
  FROM `+tableDynamicJob+` dj
 WHERE dj.job_category = 'UPLOAD'
   AND dj.workflow_run_id IS NOT DISTINCT FROM NULLIF($1::bigint, 0)
ON CONFLICT DO NOTHING`, uploadRunID); err != nil {
		return tag.RowsAffected(), err
	}
	// 修正僵尸上传实例（需求 10738）：
	// 历史实现会把「上次上传被中断」的 UPLOADING 文件映射成 RUNNING 实例，
	// 且不写 gmt_start；这类实例永远不会被上传循环选中，也不会超时，
	// 在界面上就是「一直转圈」的 JOB。判定口径：RUNNING 但从未有 gmt_start。
	if _, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicJob+` dj
   SET status = 'PENDING',
       error = NULL,
       gmt_update = now()
 WHERE dj.job_category = 'UPLOAD'
   AND dj.status = 'RUNNING'
   AND dj.gmt_start IS NULL`); err != nil {
		return tag.RowsAffected(), err
	}
	return tag.RowsAffected(), nil
}

// MarkDynamicUploadJobStatus 更新指定文件的上传动态 JOB 状态。
func (s *Store) MarkDynamicUploadJobStatus(ctx context.Context, fileID int64,
	status, errText string, runLogID int64) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	var workflowRunID *int64
	err := s.pool.QueryRow(ctx, `
UPDATE `+tableDynamicJob+` dj
   SET status = $2,
       gmt_start = CASE WHEN $2 = 'RUNNING' THEN COALESCE(gmt_start, now()) ELSE gmt_start END,
       gmt_finish = CASE WHEN $2 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE gmt_finish END,
       error = $3,
       run_log_id = COALESCE(NULLIF($4::bigint, 0), run_log_id),
       workflow_run_id = COALESCE(workflow_run_id, NULLIF($5::bigint, 0)),
       gmt_update = now()
 WHERE dj.dynamic_job_id = (
       SELECT dj2.dynamic_job_id
         FROM `+tableDynamicJob+` dj2
        WHERE dj2.job_category = 'UPLOAD' AND dj2.file_id = $1
          AND ($5::bigint = 0 OR dj2.workflow_run_id = $5::bigint)
        ORDER BY (dj2.workflow_run_id = $5::bigint) DESC NULLS LAST,
                 dj2.dynamic_job_id DESC
        LIMIT 1)
 RETURNING workflow_run_id`, fileID, status, errArg, runLogID, cdeuRunIDFromContext(ctx)).Scan(&workflowRunID)
	if err != nil {
		return err
	}
	if workflowRunID != nil {
		_ = s.EvaluateCDEURunProcesses(ctx, *workflowRunID)
	}
	return nil
}

// MarkDynamicUploadJobStepStatus 更新上传动态 JOB 的 STEP_UPLOAD。
func (s *Store) MarkDynamicUploadJobStepStatus(ctx context.Context, fileID int64, status, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicStep+` ds
   SET status = $2,
       started_at = CASE WHEN $2 = 'RUNNING' THEN COALESCE(ds.started_at, now()) ELSE ds.started_at END,
       finished_at = CASE WHEN $2 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE ds.finished_at END,
       error = $3, gmt_update = now()
 WHERE ds.dynamic_job_id = (
       SELECT dj2.dynamic_job_id
         FROM `+tableDynamicJob+` dj2
        WHERE dj2.job_category = 'UPLOAD' AND dj2.file_id = $1
          AND ($4::bigint = 0 OR dj2.workflow_run_id = $4::bigint)
        ORDER BY (dj2.workflow_run_id = $4::bigint) DESC NULLS LAST,
                 dj2.dynamic_job_id DESC
        LIMIT 1)
   AND ds.step_code = 'STEP_UPLOAD'`, fileID, status, errArg, cdeuRunIDFromContext(ctx))
	return err
}

// MarkDynamicJobStatus 更新动态 JOB 最近一次运行状态。
func (s *Store) MarkDynamicJobStatus(ctx context.Context, usc string, bizMonth int,
	status, errText string, runLogID int64) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	var workflowRunID *int64
	err := s.pool.QueryRow(ctx, `
UPDATE `+tableDynamicJob+` dj
   SET status = $3,
       gmt_start = CASE WHEN $3 = 'RUNNING' THEN COALESCE(gmt_start, now()) ELSE gmt_start END,
       gmt_finish = CASE WHEN $3 IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED') THEN now() ELSE gmt_finish END,
       error = $4,
       run_log_id = COALESCE(NULLIF($5::bigint, 0), run_log_id),
       workflow_run_id = COALESCE(workflow_run_id, NULLIF($6::bigint, 0)),
       gmt_update = now()
 WHERE dj.dynamic_job_id = `+dynamicJobSelectionSQL("$6")+`
 RETURNING workflow_run_id`, usc, bizMonth, status, errArg, runLogID,
		cdeuRunIDFromContext(ctx)).Scan(&workflowRunID)
	if err != nil {
		return err
	}
	if workflowRunID != nil {
		_ = s.EvaluateCDEURunProcesses(ctx, *workflowRunID)
	}
	return nil
}

// EnsureDynamicExportJob 在实际检测到增量后创建单证券 INCR 动态 JOB。
// 批次清单只负责候选扫描，不能在确认 delta > 0 前生成 INCR 任务。
//
// 自 V200045 起唯一键为 (workflow_run_id, usc, biz_month)：
// 同一运行内重复调用为幂等 UPSERT（重置为 PENDING），
// 不同运行各自持有独立实例，不再把上一轮的记录改写成本轮。
func (s *Store) EnsureDynamicExportJob(ctx context.Context, processCode, usc string, bizMonth int,
	exportType string, exportJobID int64) error {
	jobCode := fmt.Sprintf("JOB_EXPORT_%s_%s_%d", exportType, usc, bizMonth)
	name := fmt.Sprintf("EXPORT_MON_%s_%s_%d", exportType, usc, bizMonth)
	runID := cdeuRunIDFromContext(ctx)
	if _, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicJob+`
    (process_id, job_code, name, usc, biz_month, export_type, export_job_id,
     job_category, seq_no, status, workflow_run_id)
SELECT p.process_id, $2, $3, $4, $5, $6, NULLIF($7, 0),
       'EXPORT', 1, 'PENDING', NULLIF($8, 0)
  FROM cdeu.finv_quote_cdeu_process p
 WHERE p.process_code = $1
ON CONFLICT (workflow_run_id, usc, biz_month) WHERE job_category = 'EXPORT' DO UPDATE SET
    export_job_id = COALESCE(EXCLUDED.export_job_id, `+tableDynamicJob+`.export_job_id),
    status = 'PENDING',
    error = NULL,
    gmt_start = NULL,
    gmt_finish = NULL,
    run_log_id = NULL,
    gmt_update = now()`,
		processCode, jobCode, name, usc, bizMonth, exportType, exportJobID, runID); err != nil {
		return err
	}
	// Step 写入拆为两步，避开「同时违反两个唯一约束」的并发冲突：
	//   cdeu_job_step_dynamic 上有 UNIQUE(dynamic_job_id, step_code) 和 UNIQUE(dynamic_job_id, seq_no)；
	//   ON CONFLICT 若只声明一个冲突目标，另一个被违反时仍会报 23505（历史 run 55 即因此失败）。
	//   因此：① 用不指定目标的 ON CONFLICT DO NOTHING 保证存在；② 单独 UPDATE 重置为 PENDING。
	if _, err := s.pool.Exec(ctx, `
INSERT INTO `+tableDynamicStep+`
    (dynamic_job_id, step_code, name, seq_no, status)
SELECT dj.dynamic_job_id, v.step_code, v.name, v.seq_no, 'PENDING'
  FROM `+tableDynamicJob+` dj
 CROSS JOIN (VALUES
    ('STEP_LOAD', '装载', 1),
    ('STEP_EXPORT', '文件生成', 2),
    ('STEP_REGISTER', '文件登记', 3),
    ('STEP_CLEANUP', '中转清理', 4)
 ) AS v(step_code, name, seq_no)
 WHERE dj.usc = $1 AND dj.biz_month = $2
   AND dj.job_category = 'EXPORT' AND dj.export_type = $3
   AND dj.workflow_run_id IS NOT DISTINCT FROM NULLIF($4::bigint, 0)
ON CONFLICT DO NOTHING`,
		usc, bizMonth, exportType, runID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableDynamicStep+` ds
   SET status = 'PENDING',
       error = NULL,
       started_at = NULL,
       finished_at = NULL,
       gmt_update = now()
  FROM `+tableDynamicJob+` dj
 WHERE ds.dynamic_job_id = dj.dynamic_job_id
   AND dj.usc = $1 AND dj.biz_month = $2
   AND dj.job_category = 'EXPORT' AND dj.export_type = $3
   AND dj.workflow_run_id IS NOT DISTINCT FROM NULLIF($4::bigint, 0)`,
		usc, bizMonth, exportType, runID)
	return err
}

// InsertCandidateJobs 从当前批次清单读取 BASE 条目并生成导出任务
// （幂等：同一 (usc, biz_month) 只建一次）。
func (s *Store) InsertCandidateJobs(ctx context.Context, batchNo string, currentMonth int, limit int) (int64, error) {
	rows, err := s.pool.Query(ctx, `
SELECT m.usc,
       m.biz_month,
       COALESCE(u.priority_rank, 100),
       COALESCE(u.sla_days, 5),
       MIN(m.trade_date)
  FROM `+tableBatch+` b
  JOIN `+tableMaturity+` m ON m.usc = b.usc AND m.biz_month = b.biz_month
  JOIN `+tableUsc+` u ON u.usc = m.usc
 WHERE b.batch_no = $1 AND b.export_type = 'BASE'
   AND m.biz_month < $2
   AND u.region IS NOT NULL AND u.market IS NOT NULL AND u.provider IS NOT NULL
   AND u.name IS NOT NULL AND u.timezone IS NOT NULL AND u.currency IS NOT NULL
   AND (u.date_delist IS NULL OR u.date_delist >= make_date(m.biz_month / 100, m.biz_month % 100, 1))
   AND NOT EXISTS (
       SELECT 1 FROM `+tableJob+` j
        WHERE j.usc = m.usc AND j.biz_month = m.biz_month
          AND j.status IN ('LOADING', 'LOADED', 'EXPORTING', 'EXPORTED'))
 GROUP BY m.usc, m.biz_month, u.priority_rank, u.sla_days
HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))
 ORDER BY COALESCE(u.priority_rank, 100), MIN(m.trade_date)
 LIMIT $3`, batchNo, currentMonth, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type candidate struct {
		usc          string
		bizMonth     int
		priorityRank int
		slaDays      int
		oldestDate   int
	}
	candidates := make([]candidate, 0, 64)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.usc, &item.bizMonth, &item.priorityRank, &item.slaDays, &item.oldestDate); err != nil {
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, item := range candidates {
		batch.Queue(`
INSERT INTO `+tableJob+`
  (usc, biz_month, priority_rank, freeze_date, seal_date, status, oldest_pending_date)
VALUES ($1, $2, $3, $4::date, $5::date, 'WAITING', $6)
ON CONFLICT (usc, biz_month) DO NOTHING`,
			item.usc, item.bizMonth, item.priorityRank,
			FreezeDateOf(item.bizMonth, item.slaDays), SealDateOf(item.bizMonth), item.oldestDate)
	}
	results := s.pool.SendBatch(ctx, batch)
	var inserted int64
	for range candidates {
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return inserted, err
		}
		inserted += tag.RowsAffected()
	}
	if err := results.Close(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// ClaimJob 领取一个到期任务：行锁 + 租约 + SKIP LOCKED（多 worker 安全）。
//
// 排序（I-02）：FROZEN > 临近 freeze_date（≤3 天）> priority_rank > oldest_pending_date。
func (s *Store) ClaimJob(ctx context.Context, worker string, lease time.Duration) (*Job, error) {
	rows, err := s.pool.Query(ctx, `
WITH picked AS (
  SELECT job_id
    FROM `+tableJob+`
   WHERE (status IN ('WAITING', 'PAUSED', 'FROZEN')
          AND (gmt_next_run IS NULL OR gmt_next_run <= now()))
      OR (status IN ('LOADING', 'LOADED', 'EXPORTING')
          AND (locked_until IS NULL OR locked_until < now()))
   ORDER BY CASE WHEN status = 'FROZEN' THEN 0
                 WHEN freeze_date <= current_date + 3 THEN 1
                 ELSE 2 END,
            priority_rank,
            COALESCE(oldest_pending_date, 99999999),
            job_id
   LIMIT 1
   FOR UPDATE SKIP LOCKED
)
UPDATE `+tableJob+` j
   SET status = 'LOADING',
       locked_by = $1,
       locked_until = now() + make_interval(secs => $2::double precision),
       gmt_heartbeat = now(),
       attempt = j.attempt + 1,
       gmt_update = now()
  FROM picked
 WHERE j.job_id = picked.job_id
RETURNING `+jobColumnsAliased, worker, lease.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Job])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ClaimJobByID 人工领取指定导出任务（用于动态 JOB 的单任务手动执行）。
func (s *Store) ClaimJobByID(ctx context.Context, jobID int64, worker string, lease time.Duration) (*Job, error) {
	rows, err := s.pool.Query(ctx, `
WITH picked AS (
  SELECT job_id
    FROM `+tableJob+`
   WHERE job_id = $1
     AND status IN ('WAITING', 'PAUSED', 'FROZEN', 'FAILED')
   FOR UPDATE SKIP LOCKED
)
UPDATE `+tableJob+` j
   SET status = 'LOADING',
       locked_by = $2,
       locked_until = now() + make_interval(secs => $3::double precision),
       gmt_heartbeat = now(),
       attempt = j.attempt + 1,
       gmt_update = now()
  FROM picked
 WHERE j.job_id = picked.job_id
RETURNING `+jobColumnsAliased, jobID, worker, lease.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Job])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// Heartbeat 续租任务锁。
func (s *Store) Heartbeat(ctx context.Context, jobID int64, lease time.Duration) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET gmt_heartbeat = now(),
       locked_until = now() + make_interval(secs => $2::double precision),
       gmt_update = now()
 WHERE job_id = $1`, jobID, lease.Seconds())
	return err
}

// GetJob 读取单个任务。
func (s *Store) GetJob(ctx context.Context, jobID int64) (*Job, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+jobColumns+` FROM `+tableJob+` WHERE job_id = $1`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Job])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateLoadProgress 保存装载游标与已装载行数（每页一个短事务后调用）。
func (s *Store) UpdateLoadProgress(ctx context.Context, jobID int64, cursorDate int, cursorTS int64, loadedRows int64, status string, lease time.Duration) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET load_cursor_date = $2,
       load_cursor_ts = $3,
       loaded_rows = $4,
       status = $5,
       gmt_heartbeat = now(),
       locked_until = now() + make_interval(secs => $6::double precision),
       gmt_update = now()
 WHERE job_id = $1`, jobID, cursorDate, cursorTS, loadedRows, status, lease.Seconds())
	return err
}

// ResetLoadCursor 重置装载游标（续跑前发现 loaded_rows 与实际行数不一致时使用）。
func (s *Store) ResetLoadCursor(ctx context.Context, jobID int64) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET load_cursor_date = NULL, load_cursor_ts = NULL, loaded_rows = 0, gmt_update = now()
 WHERE job_id = $1`, jobID)
	return err
}

// MarkJobExported 标记任务导出完成并落定 base 版本。
func (s *Store) MarkJobExported(ctx context.Context, jobID int64, loadedRows int64, baseVersion int) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET status = 'EXPORTED',
       loaded_rows = $2,
       base_version = $3,
       locked_by = NULL,
       locked_until = NULL,
       gmt_next_run = NULL,
       last_error = NULL,
       gmt_update = now()
 WHERE job_id = $1`, jobID, loadedRows, baseVersion)
	return err
}

// RequeueJob 释放任务并按需退避（失败 / 暂停）。
func (s *Store) RequeueJob(ctx context.Context, jobID int64, status string, lastError string, nextRun *time.Time) error {
	var errArg *string
	if lastError != "" {
		errArg = &lastError
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET status = $2,
       locked_by = NULL,
       locked_until = NULL,
       last_error = $3,
       gmt_next_run = $4,
       gmt_update = now()
 WHERE job_id = $1`, jobID, status, errArg, nextRun)
	return err
}

// UpdateJobPatchState 记录补丁数量与 base 版本变化。
func (s *Store) UpdateJobPatchState(ctx context.Context, jobID int64, baseVersion int, patchCount int) error {
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET base_version = $2, patch_count = $3, gmt_update = now()
 WHERE job_id = $1`, jobID, baseVersion, patchCount)
	return err
}

// MarkJobsFrozen 把到达 seal_date 仍未完成的任务标记为 FROZEN（优先强制处理）。
func (s *Store) MarkJobsFrozen(ctx context.Context, today time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET status = 'FROZEN', locked_by = NULL, locked_until = NULL, gmt_update = now()
 WHERE status IN ('WAITING', 'PAUSED', 'FAILED')
   AND seal_date <= $1::date`, today)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RecoverTimeoutJobs 把心跳/租约超时的 RUNNING 任务回收为 PAUSED（M-09）。
func (s *Store) RecoverTimeoutJobs(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET status = 'PAUSED',
       locked_by = NULL,
       locked_until = NULL,
       last_error = concat_ws(' ', NULLIF(last_error, ''), '[HEARTBEAT_TIMEOUT]'),
       gmt_update = now()
 WHERE status IN ('LOADING', 'LOADED', 'EXPORTING')
   AND (locked_until IS NULL OR locked_until < now())`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ResetJobForRetry 人工重试：FAILED / PAUSED / FROZEN → WAITING。
func (s *Store) ResetJobForRetry(ctx context.Context, jobID int64) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+`
   SET status = 'WAITING',
       gmt_next_run = now(),
       locked_by = NULL,
       locked_until = NULL,
       last_error = NULL,
       gmt_update = now()
 WHERE job_id = $1 AND status IN ('FAILED', 'PAUSED', 'FROZEN', 'WAITING')`, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("任务 %d 不存在或状态不允许重试", jobID)
	}
	return nil
}

// ListJobs 分页查询任务。
func (s *Store) ListJobs(ctx context.Context, filter JobFilter) (Page[Job], error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 6)
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Usc != "" {
		args = append(args, filter.Usc)
		conditions = append(conditions, fmt.Sprintf("usc = $%d", len(args)))
	}
	if filter.BizMonth > 0 {
		args = append(args, filter.BizMonth)
		conditions = append(conditions, fmt.Sprintf("biz_month = $%d", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var page Page[Job]
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+tableJob+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if page.Total == 0 {
		page.List = []Job{}
		return page, nil
	}

	pageSize := normalizePageSize(filter.PageSize)
	offset := (normalizePage(filter.Page) - 1) * pageSize
	args = append(args, pageSize, offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM %s%s ORDER BY job_id DESC LIMIT $%d OFFSET $%d`,
		jobColumns, tableJob, where, len(args)-1, len(args)), args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[Job])
	if err != nil {
		return page, err
	}
	page.List = list
	return page, nil
}

// ---------------------------------------------------------------------
// 中转表（LOAD / EXPORT / CLEANUP）
// ---------------------------------------------------------------------

// LoadPage 装载单页：一条语句内完成「取主表一页 → 幂等写入中转表 → 返回页游标」。
//
// 使用 keyset 分页（ts > cursor），禁止 OFFSET 深分页；冲突走 ON CONFLICT DO NOTHING（F-03）。
func (s *Store) LoadPage(ctx context.Context, usc string, date int, tsAfter int64, limit int) (LoadPageResult, error) {
	var result LoadPageResult
	err := s.pool.QueryRow(ctx, `
WITH page AS (
  SELECT ts, date, "time", open, high, low, close, paocd, volume, turnover,
         change_price, change_ratio, ext_field, last_close, remark, provider
    FROM `+tableMain+`
   WHERE usc = $1 AND date = $2 AND ts > $3
   ORDER BY ts
   LIMIT $4
), ins AS (
  INSERT INTO `+tableStaging+`
    (usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
     change_price, change_ratio, ext_field, last_close, remark, provider)
  SELECT $1, ts, date, "time", open, high, low, close, paocd, volume, turnover,
         change_price, change_ratio, ext_field, last_close, remark, provider
    FROM page
  ON CONFLICT (ts, usc) DO NOTHING
  RETURNING 1
)
SELECT COALESCE((SELECT max(ts) FROM page), $3)::bigint,
       (SELECT count(*) FROM page)::bigint,
       (SELECT count(*) FROM ins)::bigint`, usc, date, tsAfter, limit).
		Scan(&result.MaxTS, &result.PageRows, &result.InsertedRows)
	return result, err
}

// LoadPatchPage 把主表增量（since 非空）或整月（since 为 nil）的一页装入中转表。
//
// 与 LoadPage 的区别：额外按 gmt_create / gmt_update 过滤补丁增量，供补丁链路复用
// 与正常导出相同的中转表 + 写文件 + 清理流程，避免大数据量直接流式写文件。
func (s *Store) LoadPatchPage(ctx context.Context, usc string, date int, tsAfter int64, since *time.Time, limit int) (LoadPageResult, error) {
	var result LoadPageResult
	err := s.pool.QueryRow(ctx, `
WITH page AS (
  SELECT ts, date, "time", open, high, low, close, paocd, volume, turnover,
         change_price, change_ratio, ext_field, last_close, remark, provider
    FROM `+tableMain+`
   WHERE usc = $1 AND date = $2 AND ts > $3
     AND ($5::timestamptz IS NULL OR gmt_create > $5 OR gmt_update > $5)
   ORDER BY ts
   LIMIT $4
), ins AS (
  INSERT INTO `+tableStaging+`
    (usc, ts, date, "time", open, high, low, close, paocd, volume, turnover,
     change_price, change_ratio, ext_field, last_close, remark, provider)
  SELECT $1, ts, date, "time", open, high, low, close, paocd, volume, turnover,
         change_price, change_ratio, ext_field, last_close, remark, provider
    FROM page
  ON CONFLICT (ts, usc) DO NOTHING
  RETURNING 1
)
SELECT COALESCE((SELECT max(ts) FROM page), $3)::bigint,
       (SELECT count(*) FROM page)::bigint,
       (SELECT count(*) FROM ins)::bigint`, usc, date, tsAfter, limit, since).
		Scan(&result.MaxTS, &result.PageRows, &result.InsertedRows)
	return result, err
}

// CountStaging 统计中转表中该证券日期范围内的行数。
func (s *Store) CountStaging(ctx context.Context, usc string, from, to int) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableStaging+` WHERE usc = $1 AND date BETWEEN $2 AND $3`, usc, from, to).Scan(&count)
	return count, err
}

// CountStagingDate 统计中转表中该证券某交易日的行数（M-01 校验用）。
func (s *Store) CountStagingDate(ctx context.Context, usc string, date int) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM `+tableStaging+` WHERE usc = $1 AND date = $2`, usc, date).Scan(&count)
	return count, err
}

// CountMainDate 统计主表中该证券某交易日的行数（LOAD 前行数基线）。
func (s *Store) CountMainDate(ctx context.Context, usc string, date int) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM `+tableMain+` WHERE usc = $1 AND date = $2`, usc, date).Scan(&count)
	return count, err
}

// CountMainMonth 统计主表中该证券月份日期范围内的行数（L1_HOT 重写 base 用）。
func (s *Store) CountMainMonth(ctx context.Context, usc string, from, to int) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM `+tableMain+` WHERE usc = $1 AND date BETWEEN $2 AND $3`, usc, from, to).Scan(&count)
	return count, err
}

// CleanupStaging 按 usc + 日期范围清理中转数据（E-06）。
func (s *Store) CleanupStaging(ctx context.Context, usc string, from, to int) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM `+tableStaging+` WHERE usc = $1 AND date BETWEEN $2 AND $3`, usc, from, to)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// TruncateStagingIfEmpty 中转表为空时 TRUNCATE（复用序列/释放空间）。
func (s *Store) TruncateStagingIfEmpty(ctx context.Context) (bool, error) {
	var count int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+tableStaging).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if _, err := s.pool.Exec(ctx, `TRUNCATE `+tableStaging); err != nil {
		return false, err
	}
	return true, nil
}

// StagingSummary 返回中转表残留行数与证券数（M-06 指标）。
func (s *Store) StagingSummary(ctx context.Context) (int64, int64, error) {
	var rows, uscs int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*), count(DISTINCT usc) FROM `+tableStaging).Scan(&rows, &uscs)
	return rows, uscs, err
}

// PurgeResidualStaging 清理不属于任何活跃任务的残留中转数据（E-08 / L-03）。
func (s *Store) PurgeResidualStaging(ctx context.Context, limit int) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
DELETE FROM `+tableStaging+` s
 WHERE s.ctid IN (
   SELECT s2.ctid
     FROM `+tableStaging+` s2
    WHERE NOT EXISTS (
        SELECT 1 FROM `+tableJob+` j
         WHERE j.usc = s2.usc
           AND s2.date BETWEEN (j.biz_month * 100 + 1) AND (j.biz_month * 100 + 31)
           AND j.status IN ('WAITING', 'LOADING', 'LOADED', 'EXPORTING', 'PAUSED', 'FROZEN'))
    LIMIT $1)`, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// QueryStagingRows 流式读取中转表数据行（EXPORT 阶段，按 ts 排序）。
func (s *Store) QueryStagingRows(ctx context.Context, usc string, from, to int) (pgx.Rows, error) {
	return s.pool.Query(ctx, `
SELECT ts, date, "time",
       open::text, high::text, low::text, close::text,
       volume::text, turnover::text,
       change_price::text, change_ratio::text, last_close::text
  FROM `+tableStaging+`
 WHERE usc = $1 AND date BETWEEN $2 AND $3
 ORDER BY ts`, usc, from, to)
}

// QueryMainRows 流式读取主表数据行（L1_HOT 重写 base / 生成补丁）。
//
// since 为 nil 时读取整月；非 nil 时只取 gmt_create 或 gmt_update 晚于该时刻的行（增量）。
func (s *Store) QueryMainRows(ctx context.Context, usc string, from, to int, since *time.Time) (pgx.Rows, error) {
	return s.pool.Query(ctx, `
SELECT ts, date, "time",
       open::text, high::text, low::text, close::text,
       volume::text, turnover::text,
       change_price::text, change_ratio::text, last_close::text
  FROM `+tableMain+`
 WHERE usc = $1 AND date BETWEEN $2 AND $3
   AND ($4::timestamptz IS NULL OR gmt_create > $4 OR gmt_update > $4)
 ORDER BY ts`, usc, from, to, since)
}

// CountMainDelta 统计主表中晚于 since 到达/更新的行数（补丁来源行数）。
func (s *Store) CountMainDelta(ctx context.Context, usc string, from, to int, since time.Time) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableMain+`
 WHERE usc = $1 AND date BETWEEN $2 AND $3
   AND (gmt_create > $4 OR gmt_update > $4)`, usc, from, to, since).Scan(&count)
	return count, err
}

// ---------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------

// dynamicJobRetentionDays 历史运行动态实例的保留天数。
//
// 自 V200045 起动态 JOB 每次运行一条，会持续累积；
// 超过保留期且所属运行已终态的实例由 CLEAN_EXPIRED_EXPORT_ARTIFACTS 清理。
const dynamicJobRetentionDays = 30

// PurgeStaleDynamicJobRuns 删除超过保留期的历史运行动态实例及其 Step。
//
// 只处理同时满足以下条件的实例，当前运行与未终态实例一律不动：
//   - workflow_run_id 非空；
//   - 所属 cdeu_workflow_run 已终态且 gmt_create 早于保留期；
//   - 实例自身也已终态。
//
// 返回删除的 dynamic_job 行数。
func (s *Store) PurgeStaleDynamicJobRuns(ctx context.Context, keepDays, limit int) (int64, error) {
	if keepDays <= 0 {
		keepDays = dynamicJobRetentionDays
	}
	if limit <= 0 {
		limit = 5000
	}
	rows, err := s.pool.Query(ctx, `
SELECT dj.dynamic_job_id
  FROM `+tableDynamicJob+` dj
  JOIN cdeu.finv_quote_cdeu_workflow_run wr ON wr.run_id = dj.workflow_run_id
 WHERE dj.workflow_run_id IS NOT NULL
   AND wr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
   AND wr.gmt_create < now() - make_interval(days => $1)
   AND dj.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
 ORDER BY dj.dynamic_job_id
 LIMIT $2`, keepDays, limit)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if _, err := s.pool.Exec(ctx, `
DELETE FROM `+tableDynamicStep+` WHERE dynamic_job_id = ANY($1)`, ids); err != nil {
		return 0, err
	}
	tag, err := s.pool.Exec(ctx, `
DELETE FROM `+tableDynamicJob+` WHERE dynamic_job_id = ANY($1)
   AND workflow_run_id IS NOT NULL
   AND status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')`, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// cdeuTerminalRunStatusSQL 终态运行状态集合（用于历史运行数据清理的门槛判断）。
const cdeuTerminalRunStatusSQL = `('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')`

// PurgeHistoryRuns 按策略整体清理历史流程运行数据（需求 10723）。
//
// policy 取 syscfg.HistoryPurgeOff / HistoryPurgeDays / HistoryPurgeCount：
//   - Off   直接返回，不删任何记录；
//   - Days  删除「已终态 + 启动时间早于 keep 天前」的 workflow_run；
//   - Count 只保留最近 keep 个 workflow_run，其余删除。
//
// 安全约束：
//   - 只处理终态 run，PENDING/READY/RUNNING 一律跳过；
//   - 永远保留 run_id 最大的那一个（防 keep=1 时把当前运行删掉）；
//   - 孤立 run（cdeu_job_run 有 workflow_run_id 但 cdeu_workflow_run 已缺失）按
//     cdeu_job_run 的 gmt_create 一并判定，不依赖父表是否存在。
//
// 返回各子表的删除行数汇总。
func (s *Store) PurgeHistoryRuns(ctx context.Context, policy string, keep, limit int) (map[string]int64, error) {
	stats := map[string]int64{
		"runs": 0, "phase_runs": 0, "process_runs": 0, "job_runs": 0,
		"job_step_runs": 0, "dynamic_jobs": 0, "dynamic_steps": 0,
		"run_log_lines": 0, "run_log_files": 0,
	}
	if policy == syscfg.HistoryPurgeOff {
		return stats, nil
	}
	if keep <= 0 {
		keep = syscfg.HistoryPurgeKeepDefault
	}
	if limit <= 0 {
		limit = 200
	}

	runIDs, err := s.cdeuHistoryRunIDs(ctx, policy, keep, limit)
	if err != nil {
		return stats, err
	}
	orphanIDs, err := s.cdeuOrphanHistoryRunIDs(ctx, policy, keep, limit)
	if err != nil {
		return stats, err
	}
	if len(runIDs) == 0 && len(orphanIDs) == 0 {
		return stats, nil
	}

	// 孤儿 run 没有父行，单独清子表。
	if len(orphanIDs) > 0 {
		if err := s.purgeCdeuRunChildren(ctx, orphanIDs, stats, false); err != nil {
			return stats, err
		}
		stats["orphan_runs"] = int64(len(orphanIDs))
	}
	if len(runIDs) == 0 {
		return stats, nil
	}

	if err := s.purgeCdeuRunChildren(ctx, runIDs, stats, true); err != nil {
		return stats, err
	}
	tag, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE run_id = ANY($1)
   AND status IN `+cdeuTerminalRunStatusSQL+`
   AND run_id <> (SELECT max(run_id) FROM cdeu.finv_quote_cdeu_workflow_run)`, runIDs)
	if err != nil {
		return stats, err
	}
	stats["runs"] = tag.RowsAffected()
	return stats, nil
}

// cdeuHistoryRunIDs 选出待清理的 workflow_run。
func (s *Store) cdeuHistoryRunIDs(ctx context.Context, policy string, keep, limit int) ([]int64, error) {
	var (
		rows pgx.Rows
		err  error
	)
	switch policy {
	case syscfg.HistoryPurgeCount:
		// 按 run_id 倒序排名，保留前 keep 个。
		rows, err = s.pool.Query(ctx, `
WITH ranked AS (
    SELECT run_id, row_number() OVER (ORDER BY run_id DESC) AS rn
      FROM cdeu.finv_quote_cdeu_workflow_run
     WHERE status IN `+cdeuTerminalRunStatusSQL+`
)
SELECT run_id FROM ranked WHERE rn > $1 ORDER BY run_id LIMIT $2`, keep, limit)
	default: // HistoryPurgeDays
		rows, err = s.pool.Query(ctx, `
SELECT run_id
  FROM cdeu.finv_quote_cdeu_workflow_run
 WHERE status IN `+cdeuTerminalRunStatusSQL+`
   AND started_at < now() - make_interval(days => $1)
   AND run_id <> (SELECT max(run_id) FROM cdeu.finv_quote_cdeu_workflow_run)
 ORDER BY run_id
 LIMIT $2`, keep, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// cdeuOrphanHistoryRunIDs 选出「有运行子表但父 workflow_run 已缺失」的历史 run_id。
//
// 这类数据来自早期清理遗留（当前库中有 13 个，run_id 40–48、50 等），
// 由于父行不在，无法用 cdeu_workflow_run.started_at 判定，改用子表的 gmt_create。
func (s *Store) cdeuOrphanHistoryRunIDs(ctx context.Context, policy string, keep, limit int) ([]int64, error) {
	// Count 策略下孤儿按 run_id 排名，与其他 run 混排会错位；此处只对 Days 生效。
	if policy != syscfg.HistoryPurgeDays {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
SELECT jr.workflow_run_id
  FROM cdeu.finv_quote_cdeu_job_run jr
  LEFT JOIN cdeu.finv_quote_cdeu_workflow_run wr ON wr.run_id = jr.workflow_run_id
 WHERE wr.run_id IS NULL
   AND jr.workflow_run_id IS NOT NULL
   AND jr.gmt_create < now() - make_interval(days => $1)
   AND jr.status IN ('SUCCESS','PARTIAL','FAILED','PAUSED','CANCELLED','SKIPPED')
 GROUP BY jr.workflow_run_id
 ORDER BY jr.workflow_run_id
 LIMIT $2`, keep, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// purgeCdeuRunChildren 按子→父顺序删除某批 run 的全部子表记录。
//
// withParent=true 时还包含 cdeu_phase_run / cdeu_process_run（仅对存在的父 run 有效）。
func (s *Store) purgeCdeuRunChildren(ctx context.Context, runIDs []int64, stats map[string]int64, withParent bool) error {
	steps, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_step_dynamic
 WHERE dynamic_job_id IN (
       SELECT dynamic_job_id FROM cdeu.finv_quote_cdeu_job_dynamic WHERE workflow_run_id = ANY($1))`, runIDs)
	if err != nil {
		return err
	}
	stats["dynamic_steps"] += steps.RowsAffected()

	dyn, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_dynamic WHERE workflow_run_id = ANY($1)`, runIDs)
	if err != nil {
		return err
	}
	stats["dynamic_jobs"] += dyn.RowsAffected()

	stepRuns, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_step_run WHERE workflow_run_id = ANY($1)`, runIDs)
	if err != nil {
		return err
	}
	stats["job_step_runs"] += stepRuns.RowsAffected()

	jobRuns, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_run WHERE workflow_run_id = ANY($1)`, runIDs)
	if err != nil {
		return err
	}
	stats["job_runs"] += jobRuns.RowsAffected()

	if withParent {
		procRuns, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_process_run WHERE workflow_run_id = ANY($1)`, runIDs)
		if err != nil {
			return err
		}
		stats["process_runs"] += procRuns.RowsAffected()

		phaseRuns, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_phase_run WHERE workflow_run_id = ANY($1)`, runIDs)
		if err != nil {
			return err
		}
		stats["phase_runs"] += phaseRuns.RowsAffected()
	}

	// 运行日志：run_id 是日志批次 ID（= cdeu_job_run.run_log_id），不是 workflow_run_id。
	logs, err := s.pool.Exec(ctx, `
DELETE FROM cdeu.finv_quote_cdeu_job_run_log_line
 WHERE run_id IN (
       SELECT DISTINCT run_log_id FROM cdeu.finv_quote_cdeu_job_run
        WHERE workflow_run_id = ANY($1) AND run_log_id IS NOT NULL)`, runIDs)
	if err != nil {
		// 子表已删，日志清理失败不回滚，下次运行会因 job_run 已空而无法定位——记日志即可。
		log.Printf("[cdeu] 清理运行日志失败 runs=%d err=%v", len(runIDs), err)
		return nil
	}
	stats["run_log_lines"] += logs.RowsAffected()
	return nil
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	switch {
	case size <= 0:
		return 20
	case size > 200:
		return 200
	default:
		return size
	}
}
