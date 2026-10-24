package dedup

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 表名常量（全部 schema 全限定，见 Docs/DevSpec/DbSchemaSpec.md §3）。
const (
	tableMain        = "dedup.finv_quote_secu_kline_min"
	tableStaging     = "export.finv_quote_secu_kline_min"
	tableIngestStats = "dedup.finv_dedup_ingest_stats"
	tableBudget      = "dedup.finv_dedup_daily_budget"
	tableWatermark   = "dedup.finv_dedup_scan_watermark"
	tablePriority    = "dedup.finv_dedup_usc_priority"
	tableMaturity    = "dedup.finv_dedup_quote_maturity_mon"
	tableCalendar    = "dedup.finv_dedup_trade_calendar"
	tableRunLog      = "dedup.finv_dedup_job_run_log_export_file"
	tableUsc         = "dedup.finv_dedup_usc"
	tableJob         = "dedup.finv_dedup_export_job"
	tableFile        = "dedup.finv_dedup_export_file"
	tableUploadLog   = "dedup.finv_dedup_export_file_upload_log"
	tablePatch       = "dedup.finv_dedup_export_file_patch"
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
INSERT INTO `+tableRunLog+` (job_kind, job_id, usc, biz_month)
VALUES ($1, $2, $3, $4) RETURNING run_id`, kind, jobID, uscArg, bizMonth).Scan(&runID)
	return runID, err
}

// FinishRun 结束一次 Job 运行并记录处理量与结果。
func (s *Store) FinishRun(ctx context.Context, runID int64, status string, rows, bytes int64, errText string) error {
	var errArg *string
	if errText != "" {
		errArg = &errText
	}
	_, err := s.pool.Exec(ctx, `
UPDATE `+tableRunLog+`
   SET gmt_finish = now(), status = $2, processed_rows = $3, processed_bytes = $4, error = $5
 WHERE run_id = $1`, runID, status, rows, bytes, errArg)
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

// InsertCandidateJobs 生成导出候选任务（幂等：同一 (usc, biz_month) 只建一次）。
//
// 候选条件：月份已结束（biz_month < 当前月）、该月所有已观测日均为 COMPLETE/FROZEN、
// 证券元数据齐备且未退市（或退市日晚于该月首日）。
func (s *Store) InsertCandidateJobs(ctx context.Context, currentMonth int, limit int) (int64, error) {
	rows, err := s.pool.Query(ctx, `
SELECT m.usc,
       m.biz_month,
       COALESCE(p.priority_rank, 100),
       COALESCE(p.sla_days, 5),
       MIN(m.trade_date)
  FROM `+tableMaturity+` m
  JOIN `+tableUsc+` u ON u.usc = m.usc
  LEFT JOIN `+tablePriority+` p ON p.usc = m.usc
 WHERE m.biz_month < $1
   AND u.region IS NOT NULL AND u.market IS NOT NULL AND u.provider IS NOT NULL
   AND u.name IS NOT NULL AND u.timezone IS NOT NULL AND u.currency IS NOT NULL
   AND (u.date_delist IS NULL OR u.date_delist >= make_date(m.biz_month / 100, m.biz_month % 100, 1))
   AND NOT EXISTS (
       SELECT 1 FROM `+tableJob+` j WHERE j.usc = m.usc AND j.biz_month = m.biz_month)
 GROUP BY m.usc, m.biz_month, p.priority_rank, p.sla_days
HAVING bool_and(m.maturity_status IN ('COMPLETE', 'FROZEN'))
 ORDER BY COALESCE(p.priority_rank, 100), MIN(m.trade_date)
 LIMIT $2`, currentMonth, limit)
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
