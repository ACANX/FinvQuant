package dedup

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// IngestSnapshot 采集侧（quote schema）的数据新鲜度快照，用于「采集就绪检查」。
type IngestSnapshot struct {
	LatestDate     int       // quote 表最大交易日（YYYYMMDD），无数据为 0
	LatestCreate   time.Time // quote 表最近一次写入时间
	TotalRows      int64     // quote 表总行数
	DistinctUsc    int       // quote 表涉及的证券数
	LagDays        int       // 最新交易日距今天数
	CurrentMonth   int       // 今天所属月份（YYYYMM）
	HasCurrentData bool      // 是否已有当月数据
}

// ExportConditionReport 「导出条件判定」的结果快照（只读，不落库）。
type ExportConditionReport struct {
	CurrentMonth      int      `json:"current_month"`
	CandidateMonths   int      `json:"candidate_months"`    // 满足 BASE 导出条件的证券月数
	AlreadyExported   int      `json:"already_exported"`    // 已导出/导出中而跳过的
	ImmatureMonths    int      `json:"immature_months"`     // 成熟度未达 COMPLETE/FROZEN 的
	IncompleteUsc     int      `json:"incomplete_usc"`      // 证券元数据不全的
	DelistedMonths    int      `json:"delisted_months"`     // 已退市而排除的
	CurrentMonthSkips int      `json:"current_month_skips"` // 当月不可导出的
	IncrCandidates    int      `json:"incr_candidates"`     // 已导出、可生成增量补丁的月份
	SampleCandidates  []string `json:"sample_candidates"`   // 前 10 个候选，便于人工核对
}

// QuoteIngestSnapshot 读取采集侧数据新鲜度（只读 quote schema，不影响加工侧的 dedup 副本）。
func (s *Store) QuoteIngestSnapshot(ctx context.Context, today time.Time) (IngestSnapshot, error) {
	var snapshot IngestSnapshot
	snapshot.CurrentMonth = today.Year()*100 + int(today.Month())

	// 性能：原实现用 max()+count(*)+count(DISTINCT usc) 一次扫全表，280 万行耗时 12.6 秒。
	// 拆成三条走索引/估算的短查询：
	//   ① 最新行（ORDER BY date DESC, ts DESC LIMIT 1）走 idx_finv_quote_date_usc_ts；
	//   ② 总行数取 pg_class.reltuples 估算（避免全表 count）；
	//   ③ 证券数取自加工侧 finv_dedup_usc，不扫行情表。
	if err := s.pool.QueryRow(ctx, `
SELECT m.date, m.gmt_create
  FROM quote.finv_quote_secu_kline_min m
 ORDER BY m.date DESC, m.ts DESC
 LIMIT 1`).Scan(&snapshot.LatestDate, &snapshot.LatestCreate); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return snapshot, err
		}
		// 空表：保持零值与 1970 初始时间，交由上层判定。
		snapshot.LatestDate = 0
	}
	if err := s.pool.QueryRow(ctx, `
SELECT GREATEST(COALESCE(c.reltuples, 0), 0)::bigint
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname = 'quote' AND c.relname = 'finv_quote_secu_kline_min'`).
		Scan(&snapshot.TotalRows); err != nil {
		return snapshot, err
	}
	if err := s.pool.QueryRow(ctx, `
SELECT count(*) FROM `+tableUsc).Scan(&snapshot.DistinctUsc); err != nil {
		return snapshot, err
	}
	if snapshot.LatestDate > 0 {
		latest := time.Date(snapshot.LatestDate/10000, time.Month(snapshot.LatestDate/100%100),
			snapshot.LatestDate%100, 0, 0, 0, 0, time.UTC)
		todayUTC := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		snapshot.LagDays = int(todayUTC.Sub(latest).Hours() / 24)
		if snapshot.LagDays < 0 {
			snapshot.LagDays = 0
		}
		snapshot.HasCurrentData = snapshot.LatestDate/100 >= snapshot.CurrentMonth
	}
	return snapshot, nil
}

// EvaluateExportConditions 只读评估「哪些证券月满足 BASE 导出条件」及排除原因分布。
//
// 判定口径与 GenerateExportBatch 的 BASE 分支完全一致，只是不写批次表——
// 目的是让「导出条件判定」成为一个可观测的独立步骤。
func (s *Store) EvaluateExportConditions(ctx context.Context, currentMonth int) (ExportConditionReport, error) {
	report := ExportConditionReport{CurrentMonth: currentMonth, SampleCandidates: []string{}}

	// 1) 逐条分类：每个 (usc, biz_month) 只落入一个原因桶。
	if err := s.pool.QueryRow(ctx, `
WITH grouped AS (
    SELECT m.usc AS usc,
           m.biz_month AS biz_month,
           bool_and(m.maturity_status IN ('COMPLETE','FROZEN')) AS mature,
           (u.region IS NOT NULL AND u.market IS NOT NULL AND u.provider IS NOT NULL
            AND u.name IS NOT NULL AND u.timezone IS NOT NULL AND u.currency IS NOT NULL) AS usc_ok,
           (u.date_delist IS NOT NULL
            AND u.date_delist < make_date(m.biz_month / 100, m.biz_month % 100, 1)) AS delisted,
           EXISTS (
               SELECT 1 FROM `+tableJob+` j
                WHERE j.usc = m.usc AND j.biz_month = m.biz_month
                  AND j.status IN ('LOADING','LOADED','EXPORTING','EXPORTED')) AS exported
      FROM `+tableMaturity+` m
      JOIN `+tableUsc+` u ON u.usc = m.usc
     GROUP BY m.usc, m.biz_month, u.region, u.market, u.provider, u.name, u.timezone,
              u.currency, u.date_delist
)
SELECT
    count(*) FILTER (WHERE NOT usc_ok)                                   AS incomplete_usc,
    count(*) FILTER (WHERE usc_ok AND delisted)                          AS delisted,
    count(*) FILTER (WHERE usc_ok AND NOT delisted AND biz_month >= $1)  AS current_month_skips,
    count(*) FILTER (WHERE usc_ok AND NOT delisted AND biz_month < $1 AND NOT mature) AS immature,
    count(*) FILTER (WHERE usc_ok AND NOT delisted AND biz_month < $1 AND mature AND exported) AS already,
    count(*) FILTER (WHERE usc_ok AND NOT delisted AND biz_month < $1 AND mature AND NOT exported) AS candidates
  FROM grouped`, currentMonth).
		Scan(&report.IncompleteUsc, &report.DelistedMonths, &report.CurrentMonthSkips,
			&report.ImmatureMonths, &report.AlreadyExported, &report.CandidateMonths); err != nil {
		return report, err
	}

	// 2) 增量候选：已导出且 BASE 文件已登记，等待补丁扫描的月份。
	if err := s.pool.QueryRow(ctx, `
SELECT count(*)
  FROM `+tableJob+` j
 WHERE j.status = 'EXPORTED'
   AND EXISTS (SELECT 1 FROM `+tableFile+` f
                WHERE f.job_id = j.job_id AND f.file_type = 'BASE' AND f.export_status = 'EXPORTED')`).
		Scan(&report.IncrCandidates); err != nil {
		return report, err
	}

	// 3) 候选样本（最多 10 条），便于人工核对判定结果。
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT m.usc || ':' || m.biz_month
  FROM `+tableMaturity+` m
  JOIN `+tableUsc+` u ON u.usc = m.usc
 WHERE m.biz_month < $1
   AND u.region IS NOT NULL AND u.market IS NOT NULL AND u.provider IS NOT NULL
   AND u.name IS NOT NULL AND u.timezone IS NOT NULL AND u.currency IS NOT NULL
   AND (u.date_delist IS NULL OR u.date_delist >= make_date(m.biz_month / 100, m.biz_month % 100, 1))
   AND NOT EXISTS (
       SELECT 1 FROM `+tableJob+` j
        WHERE j.usc = m.usc AND j.biz_month = m.biz_month
          AND j.status IN ('LOADING','LOADED','EXPORTING','EXPORTED'))
 GROUP BY m.usc, m.biz_month
HAVING bool_and(m.maturity_status IN ('COMPLETE','FROZEN'))
 ORDER BY 1
 LIMIT 10`, currentMonth)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return report, err
		}
		report.SampleCandidates = append(report.SampleCandidates, code)
	}
	return report, rows.Err()
}
