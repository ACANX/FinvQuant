package dedup

import (
	"context"
	"errors"
	"strings"
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

// ExportDecisionMaterial 判定「某证券月能否导出」所需的原始素材（需求 10832）。
//
// 只做取数，不做判定：判定与原因描述放在 Service.ExplainExportDecision，
// 那里能同时拿到 Options（本地目录、上传布局）用于生成文件名与落地路径。
type ExportDecisionMaterial struct {
	Usc          string
	BizMonth     int
	Region       *string
	Market       *string
	Provider     *string
	Name         *string
	Timezone     *string
	Currency     *string
	DateDelist   *time.Time
	PriorityRank int
	// Days 该月成熟度记录覆盖的交易日数；MatureDays 其中已达 COMPLETE/FROZEN 的天数。
	Days       int
	MatureDays int
	// ExpectedRows 按日模板累计的应有行数；ActualRows 实际采集行数。
	ExpectedRows int64
	ActualRows   int64
	// UnmatureDays 未成熟的交易日（YYYYMMDD），用于在原因里指名道姓。
	UnmatureDays []int
	// JobID / JobStatus / BaseFileName 已存在的导出任务与已登记文件（可为空）。
	JobID        *int64
	JobStatus    *string
	BaseFileName *string
}

// MaturityGap 「行情已有数据但成熟度表没有记录」的月份（需求 10832）。
//
// 这是「流程跑完却没生成动态 JOB」最容易被忽略的一类原因：导出判定全程基于
// 成熟度表，一旦成熟度行没生成（去重未产出 / 刷新 JOB 未跑到），判定侧完全看不到
// 该月的数据，表现为「静默无候选」。
type MaturityGap struct {
	Usc      string
	BizMonth int
}

// ExportDecisionMaterials 读取全部 (usc, biz_month) 的判定素材。
//
// 与 GenerateExportBatch 的 BASE 分支同源（同一张成熟度表、同一张证券主数据），
// 区别是不套 WHERE 过滤——每一条都要带出排除原因给运维看。
func (s *Store) ExportDecisionMaterials(ctx context.Context) ([]ExportDecisionMaterial, error) {
	rows, err := s.pool.Query(ctx, `
SELECT m.usc,
       m.biz_month,
       u.region, u.market, u.provider, u.name, u.timezone, u.currency,
       u.date_delist,
       COALESCE(u.priority_rank, 100),
       count(*)::int,
       count(*) FILTER (WHERE m.maturity_status IN ('COMPLETE','FROZEN'))::int,
       COALESCE(sum(m.expected_rows), 0)::bigint,
       COALESCE(sum(m.actual_rows), 0)::bigint,
       COALESCE(array_agg(m.trade_date ORDER BY m.trade_date)
                FILTER (WHERE m.maturity_status NOT IN ('COMPLETE','FROZEN')), '{}')::int[],
       j.job_id, j.status, j.file_name
  FROM `+tableMaturity+` m
  JOIN `+tableUsc+` u ON u.usc = m.usc
  LEFT JOIN LATERAL (
        SELECT jj.job_id,
               jj.status,
               (SELECT ff.file_name
                  FROM `+tableFile+` ff
                 WHERE ff.job_id = jj.job_id AND ff.file_type = 'BASE'
                 ORDER BY ff.version DESC
                 LIMIT 1) AS file_name
          FROM `+tableJob+` jj
         WHERE jj.usc = m.usc AND jj.biz_month = m.biz_month
         ORDER BY jj.job_id DESC
         LIMIT 1) j ON true
 GROUP BY m.usc, m.biz_month, u.region, u.market, u.provider, u.name, u.timezone,
          u.currency, u.date_delist, u.priority_rank, j.job_id, j.status, j.file_name
 ORDER BY COALESCE(u.priority_rank, 100), m.biz_month, m.usc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	materials := make([]ExportDecisionMaterial, 0, 256)
	for rows.Next() {
		var item ExportDecisionMaterial
		if err := rows.Scan(&item.Usc, &item.BizMonth,
			&item.Region, &item.Market, &item.Provider, &item.Name, &item.Timezone, &item.Currency,
			&item.DateDelist, &item.PriorityRank,
			&item.Days, &item.MatureDays, &item.ExpectedRows, &item.ActualRows, &item.UnmatureDays,
			&item.JobID, &item.JobStatus, &item.BaseFileName); err != nil {
			return nil, err
		}
		materials = append(materials, item)
	}
	return materials, rows.Err()
}

// ExportMaturityGaps 找出「行情表已有某月数据、但成熟度表没有该月记录」的证券月。
//
// fromMonth 用于限制扫描范围（YYYYMM），避免对全量行情表做去重扫描。
func (s *Store) ExportMaturityGaps(ctx context.Context, fromMonth int, limit int) ([]MaturityGap, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT q.usc, q.date / 100 AS biz_month
  FROM quote.finv_quote_secu_kline_min q
 WHERE q.date >= $1
   AND NOT EXISTS (
       SELECT 1 FROM `+tableMaturity+` m
        WHERE m.usc = q.usc AND m.biz_month = q.date / 100)
 ORDER BY 2 DESC, 1
 LIMIT $2`, fromMonth*100+1, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gaps := make([]MaturityGap, 0, 32)
	for rows.Next() {
		var gap MaturityGap
		if err := rows.Scan(&gap.Usc, &gap.BizMonth); err != nil {
			return nil, err
		}
		gaps = append(gaps, gap)
	}
	return gaps, rows.Err()
}

// StockRow 指定业务月份下单个证券的存量清单行（需求 10835）。
type StockRow struct {
	Usc      string `json:"usc"`
	BizMonth int    `json:"biz_month"`
	// Name / Region / Market 来自证券主数据（cdeu.finv_quote_cdeu_usc）。
	Name   string `json:"name"`
	Region string `json:"region"`
	Market string `json:"market"`
	// StockRows 主表（cdeu.finv_quote_secu_kline_min）当前存量行数：真正会被导出的数据量。
	StockRows int64 `json:"stock_rows"`
	// StockDays 主表覆盖的交易日数；DateFrom / DateTo 为对应的首末交易日（YYYYMMDD）。
	StockDays int `json:"stock_days"`
	DateFrom  int `json:"date_from"`
	DateTo    int `json:"date_to"`
	// ExpectedRows / ActualRows 来自成熟度表，主表被清理后仍能看到该月规模。
	ExpectedRows int64 `json:"expected_rows"`
	ActualRows   int64 `json:"actual_rows"`
	// MaturityStatus 成熟度汇总：COMPLETE / FROZEN / PARTIAL / IMMATURE；无记录为空串。
	MaturityStatus string `json:"maturity_status"`
	UnmatureDays   int    `json:"unmature_days"`
	// ExportStatus 导出任务状态（无任务为空串），FileName 为已登记或预计的文件名。
	ExportStatus string `json:"export_status"`
	JobID        *int64 `json:"job_id,omitempty"`
	FileName     string `json:"file_name"`
	// EstBytes 主表存量的字节估算（样本均值 × 行数），FileEstimate 为是否来自估算。
	EstBytes int64 `json:"est_bytes"`
}

// StockPage 指定业务月份的存量清单分页结果。
type StockPage struct {
	BizMonth int        `json:"biz_month"`
	Total    int        `json:"total"`
	Items    []StockRow `json:"items"`
	// 汇总（全量，不受分页影响）。
	TotalStockRows int64 `json:"total_stock_rows"`
	TotalSecu      int   `json:"total_secu"`
	TotalEstBytes  int64 `json:"total_est_bytes"`
	// CleanedSecu 主表已无数据但成熟度表有记录的证券数（已导出并被清理）。
	CleanedSecu int `json:"cleaned_secu"`
}

// StockFilter 存量清单查询条件。
type StockFilter struct {
	BizMonth int
	Usc      string
	Limit    int
	Offset   int
}

// ExportStockPage 统计指定业务月份在**主表**（cdeu.finv_quote_secu_kline_min）里的存量清单，
// 用成熟度表补充「已被清理月份」的规模，用导出任务表标注「是否已导出」。
//
// 需求 10835：让运维在跑流程前就能看到该月会被导出的证券清单、每证券行数与大致文件大小。
// 数据源一律 `cdeu.*`（`dedup` / `dedup_frozen` 已于 V100086 删除）。
func (s *Store) ExportStockPage(ctx context.Context, filter StockFilter) (StockPage, error) {
	page := StockPage{BizMonth: filter.BizMonth, Items: []StockRow{}}
	if filter.BizMonth <= 0 {
		return page, nil
	}
	// 上限 1000：常见月份证券数（几十到几百）一次出完，避免在页面上反复翻页。
	if filter.Limit <= 0 || filter.Limit > 1000 {
		filter.Limit = 500
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	dateFrom := filter.BizMonth*100 + 1
	dateTo := filter.BizMonth*100 + 31
	uscPattern := ""
	if strings.TrimSpace(filter.Usc) != "" {
		uscPattern = "%" + strings.TrimSpace(filter.Usc) + "%"
	}

	// 汇总：以「主表有数据」或「成熟度有记录」的证券月为全集。
	if err := s.pool.QueryRow(ctx, `
WITH main AS (
    SELECT usc, count(*) AS rows, count(DISTINCT date) AS days
      FROM `+tableMain+`
     WHERE date BETWEEN $1 AND $2
     GROUP BY usc
), mat AS (
    SELECT usc, sum(actual_rows) AS actual
      FROM `+tableMaturity+`
     WHERE biz_month = $3
     GROUP BY usc
), all_usc AS (
    SELECT usc FROM main UNION SELECT usc FROM mat
)
SELECT count(*),
       COALESCE(sum(main.rows), 0)::bigint,
       count(*) FILTER (WHERE COALESCE(main.rows, 0) = 0 AND mat.actual IS NOT NULL)
  FROM all_usc
  LEFT JOIN main ON main.usc = all_usc.usc
  LEFT JOIN mat  ON mat.usc  = all_usc.usc
 WHERE ($4 = '' OR all_usc.usc ILIKE $4)`,
		dateFrom, dateTo, filter.BizMonth, uscPattern).
		Scan(&page.TotalSecu, &page.TotalStockRows, &page.CleanedSecu); err != nil {
		return page, err
	}

	// 字节估算：取主表该月最多 10000 行样本算平均行字节，再按总行数折算。
	var sampleRows int64
	var sampleBytes int64
	if err := s.pool.QueryRow(ctx, `
WITH sample AS (
    SELECT * FROM `+tableMain+`
     WHERE date BETWEEN $1 AND $2
     LIMIT 10000
)
SELECT count(*), COALESCE(sum(pg_column_size(sample.*)), 0)::bigint FROM sample`,
		dateFrom, dateTo).Scan(&sampleRows, &sampleBytes); err != nil {
		return page, err
	}
	var avgRowBytes int64
	if sampleRows > 0 {
		avgRowBytes = sampleBytes / sampleRows
	}
	page.TotalEstBytes = page.TotalStockRows * avgRowBytes

	rows, err := s.pool.Query(ctx, `
WITH main AS (
    SELECT usc,
           count(*) AS rows,
           count(DISTINCT date) AS days,
           min(date) AS date_from,
           max(date) AS date_to
      FROM `+tableMain+`
     WHERE date BETWEEN $1 AND $2
     GROUP BY usc
), mat AS (
    SELECT usc,
           COALESCE(sum(expected_rows), 0)::bigint AS expected_rows,
           COALESCE(sum(actual_rows), 0)::bigint   AS actual_rows,
           count(*) FILTER (WHERE maturity_status NOT IN ('COMPLETE', 'FROZEN')) AS unmature_days,
           bool_and(maturity_status IN ('COMPLETE', 'FROZEN')) AS all_mature,
           bool_or(maturity_status = 'FROZEN') AS any_frozen
      FROM `+tableMaturity+`
     WHERE biz_month = $3
     GROUP BY usc
), job AS (
    SELECT DISTINCT ON (usc) usc, job_id, status
      FROM `+tableJob+`
     WHERE biz_month = $3
     ORDER BY usc, job_id DESC
), base_file AS (
    SELECT DISTINCT ON (job_id) job_id, file_name, file_size
      FROM `+tableFile+`
     WHERE file_type = 'BASE'
     ORDER BY job_id, version DESC
), all_usc AS (
    SELECT usc FROM main UNION SELECT usc FROM mat
)
SELECT all_usc.usc,
       COALESCE(u.name, ''), COALESCE(u.region, ''), COALESCE(u.market, ''),
       COALESCE(main.rows, 0),
       COALESCE(main.days, 0),
       COALESCE(main.date_from, 0),
       COALESCE(main.date_to, 0),
       COALESCE(mat.expected_rows, 0),
       COALESCE(mat.actual_rows, 0),
       CASE WHEN mat.usc IS NULL THEN ''
            WHEN mat.all_mature AND mat.any_frozen THEN 'FROZEN'
            WHEN mat.all_mature THEN 'COMPLETE'
            WHEN mat.unmature_days > 0 THEN 'PARTIAL'
            ELSE 'IMMATURE' END,
       COALESCE(mat.unmature_days, 0),
       COALESCE(job.status, ''),
       job.job_id,
       COALESCE(base_file.file_name, '')
  FROM all_usc
  LEFT JOIN main      ON main.usc = all_usc.usc
  LEFT JOIN mat       ON mat.usc  = all_usc.usc
  LEFT JOIN `+tableUsc+` u ON u.usc = all_usc.usc
  LEFT JOIN job       ON job.usc  = all_usc.usc
  LEFT JOIN base_file ON base_file.job_id = job.job_id
 WHERE ($4 = '' OR all_usc.usc ILIKE $4)
 ORDER BY COALESCE(main.rows, 0) DESC, all_usc.usc
 LIMIT $5 OFFSET $6`,
		dateFrom, dateTo, filter.BizMonth, uscPattern, filter.Limit, filter.Offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()

	for rows.Next() {
		var item StockRow
		item.BizMonth = filter.BizMonth
		if err := rows.Scan(&item.Usc, &item.Name, &item.Region, &item.Market,
			&item.StockRows, &item.StockDays, &item.DateFrom, &item.DateTo,
			&item.ExpectedRows, &item.ActualRows, &item.MaturityStatus, &item.UnmatureDays,
			&item.ExportStatus, &item.JobID, &item.FileName); err != nil {
			return page, err
		}
		item.EstBytes = item.StockRows * avgRowBytes
		// 未导出时给出「如果现在导出会叫什么文件」，与真实导出的命名规则完全一致。
		if item.FileName == "" {
			item.FileName = FileMeta{
				Usc: item.Usc, BizMonth: filter.BizMonth,
				Region: item.Region, Market: item.Market,
				Provider: "FTMM",
			}.BaseFileName()
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	page.Total = page.TotalSecu
	return page, nil
}
