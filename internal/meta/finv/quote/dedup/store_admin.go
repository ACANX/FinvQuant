package dedup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// 本文件实现归档运维台的数据访问：10 张表的分页查询 + 5 张可编辑表的管理员写入。
// 约束：所有 SQL 使用 schema 全限定名；写操作统一写一条管理审计（run log，job_kind=MANUAL_ADMIN_EDIT）。

// pagedList 通用「count + 分页 select」执行器。
func pagedList[T any](ctx context.Context, s *Store, table, columns string, conds []string, args []any, order string, f AdminFilter) (Page[T], error) {
	var page Page[T]
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if page.Total == 0 {
		page.List = []T{}
		return page, nil
	}
	pageSize := normalizePageSize(f.PageSize)
	offset := (normalizePage(f.Page) - 1) * pageSize
	query := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		columns, table, where, order, len(args)+1, len(args)+2)
	rows, err := s.pool.Query(ctx, query, append(append([]any{}, args...), pageSize, offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return page, err
	}
	page.List = list
	return page, nil
}

// appendUscCond 追加 usc 精确匹配条件。
func appendUscCond(conds []string, args []any, usc string) ([]string, []any) {
	if usc == "" {
		return conds, args
	}
	args = append(args, usc)
	return append(conds, fmt.Sprintf("usc = $%d", len(args))), args
}

// appendStatusCond 追加状态精确匹配条件。
func appendStatusCond(conds []string, args []any, status string) ([]string, []any) {
	if status == "" {
		return conds, args
	}
	args = append(args, status)
	return append(conds, fmt.Sprintf("status = $%d", len(args))), args
}

// appendDateRange 追加日期范围条件（列由调用方指定，值格式 yyyy-MM-dd）。
func appendDateRange(conds []string, args []any, column, from, to string) ([]string, []any) {
	if from != "" {
		args = append(args, from)
		conds = append(conds, fmt.Sprintf("%s >= $%d::date", column, len(args)))
	}
	if to != "" {
		args = append(args, to)
		conds = append(conds, fmt.Sprintf("%s <= $%d::date", column, len(args)))
	}
	return conds, args
}

// appendBizMonth 追加业务月份条件。
func appendBizMonth(conds []string, args []any, bizMonth int) ([]string, []any) {
	if bizMonth <= 0 {
		return conds, args
	}
	args = append(args, bizMonth)
	return append(conds, fmt.Sprintf("biz_month = $%d", len(args))), args
}

// ---------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------

// ListIngestStats 分页查询每日入口流量统计。
func (s *Store) ListIngestStats(ctx context.Context, f AdminFilter) (Page[IngestStat], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendDateRange(conds, args, "stat_date", f.DateFrom, f.DateTo)
	return pagedList[IngestStat](ctx, s, tableIngestStats,
		"stat_date, total_rows, total_bytes, gmt_update", conds, args, "stat_date DESC", f)
}

// ListBudgets 分页查询每日预算。
func (s *Store) ListBudgets(ctx context.Context, f AdminFilter) (Page[DailyBudget], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendDateRange(conds, args, "biz_date", f.DateFrom, f.DateTo)
	conds, args = appendStatusCond(conds, args, f.Status)
	return pagedList[DailyBudget](ctx, s, tableBudget,
		`biz_date, base_rows, base_bytes, p75_rows, backlog_ratio, sla_pressure, factor,
		 budget_rows, budget_bytes, hard_cap_rows, consumed_rows, consumed_bytes, status, gmt_create`,
		conds, args, "biz_date DESC", f)
}

// ListMaturity 分页查询成熟度。
func (s *Store) ListMaturity(ctx context.Context, f AdminFilter) (Page[MaturityRow], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendUscCond(conds, args, f.Usc)
	conds, args = appendStatusCond(conds, args, f.Status)
	conds, args = appendBizMonth(conds, args, f.BizMonth)
	return pagedList[MaturityRow](ctx, s, tableMaturity,
		`usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status,
		 gmt_first_seen, gmt_last_arrival, gmt_seale, gmt_update`,
		conds, args, "trade_date DESC, usc", f)
}

// ListWatermarks 查询全部扫描水位（数据量小，不分页）。
func (s *Store) ListWatermarks(ctx context.Context, f AdminFilter) (Page[WatermarkRow], error) {
	return pagedList[WatermarkRow](ctx, s, tableWatermark,
		"watermark_name, last_value, gmt_update", nil, nil, "watermark_name", f)
}

// ListPriorities 查询证券优先级（数据量小，不分页）。
func (s *Store) ListPriorities(ctx context.Context, f AdminFilter) (Page[PriorityRow], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendUscCond(conds, args, f.Usc)
	return pagedList[PriorityRow](ctx, s, tablePriority,
		"usc, priority_rank, sla_days, max_wait_days, gmt_update", conds, args, "priority_rank, usc", f)
}

// ListCalendar 分页查询交易日历。
func (s *Store) ListCalendar(ctx context.Context, f AdminFilter) (Page[CalendarRow], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendUscCond(conds, args, f.Usc)
	if f.BizMonth > 0 {
		year, month := f.BizMonth/100, f.BizMonth%100
		from := fmt.Sprintf("%04d-%02d-01", year, month)
		next := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		to := next.AddDate(0, 0, -1).Format("2006-01-02")
		conds, args = appendDateRange(conds, args, "to_date(trade_date::text, 'YYYYMMDD')", from, to)
	}
	return pagedList[CalendarRow](ctx, s, tableCalendar,
		"usc, trade_date, flag_trade_day", conds, args, "trade_date DESC, usc", f)
}

// ListUscs 分页查询证券主数据。
func (s *Store) ListUscs(ctx context.Context, f AdminFilter) (Page[UscRow], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendUscCond(conds, args, f.Usc)
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		conds = append(conds, fmt.Sprintf("(usc ILIKE $%d OR coalesce(name, '') ILIKE $%d)", len(args), len(args)))
	}
	return pagedList[UscRow](ctx, s, tableUsc,
		`usc, category, day_expect_rows, date_list, date_delist, flag_auto_discover,
		 region, market, provider, name, timezone, currency, gmt_create, gmt_update`,
		conds, args, "usc", f)
}

// ListRunLogs 分页查询 Job 运行日志。
func (s *Store) ListRunLogs(ctx context.Context, f AdminFilter) (Page[RunLogRow], error) {
	conds := []string{}
	args := []any{}
	if f.Kind != "" {
		args = append(args, f.Kind)
		conds = append(conds, fmt.Sprintf("job_kind = $%d", len(args)))
	}
	conds, args = appendStatusCond(conds, args, f.Status)
	if f.Usc != "" {
		args = append(args, f.Usc)
		conds = append(conds, fmt.Sprintf("usc = $%d", len(args)))
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		conds = append(conds, fmt.Sprintf("coalesce(error, '') ILIKE $%d", len(args)))
	}
	conds, args = appendDateRange(conds, args, "gmt_start", f.DateFrom, f.DateTo)
	return pagedList[RunLogRow](ctx, s, tableRunLog,
		`run_id, job_kind, job_id, usc, biz_month, gmt_start, gmt_finish, status,
		 processed_rows, processed_bytes, error`,
		conds, args, "gmt_start DESC", f)
}

// ListUploadLogs 分页查询上传审计。
func (s *Store) ListUploadLogs(ctx context.Context, f AdminFilter) (Page[UploadLogRow], error) {
	conds := []string{}
	args := []any{}
	if f.FileID > 0 {
		args = append(args, f.FileID)
		conds = append(conds, fmt.Sprintf("file_id = $%d", len(args)))
	}
	conds, args = appendStatusCond(conds, args, f.Status)
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		conds = append(conds, fmt.Sprintf("coalesce(error, '') ILIKE $%d", len(args)))
	}
	conds, args = appendDateRange(conds, args, "gmt_start", f.DateFrom, f.DateTo)
	return pagedList[UploadLogRow](ctx, s, tableUploadLog,
		`log_id, file_id, attempt, target_path, gmt_start, gmt_finish, status, uploaded_bytes, error`,
		conds, args, "gmt_start DESC", f)
}

// ListPatches 分页查询补丁任务。
func (s *Store) ListPatches(ctx context.Context, f AdminFilter) (Page[PatchRow], error) {
	conds := []string{}
	args := []any{}
	conds, args = appendUscCond(conds, args, f.Usc)
	conds, args = appendStatusCond(conds, args, f.Status)
	conds, args = appendBizMonth(conds, args, f.BizMonth)
	return pagedList[PatchRow](ctx, s, tablePatch,
		`patch_id, job_id, usc, biz_month, patch_level, patch_version, source_rows, patch_rows,
		 reason, status, patch_file_id, gmt_create, gmt_finish, last_error`,
		conds, args, "patch_id DESC", f)
}

// ---------------------------------------------------------------------
// 管理员写入
// ---------------------------------------------------------------------

// OverrideBudget 覆盖当日预算（不存在则插入一条最小记录）。
func (s *Store) OverrideBudget(ctx context.Context, in BudgetOverrideInput, bizDate time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableBudget+` (biz_date, base_rows, base_bytes, factor, budget_rows, hard_cap_rows, status)
VALUES ($1::date, 0, 0, $2, $3, $4, $5)
ON CONFLICT (biz_date) DO UPDATE SET
  factor = EXCLUDED.factor,
  budget_rows = EXCLUDED.budget_rows,
  hard_cap_rows = EXCLUDED.hard_cap_rows,
  status = EXCLUDED.status`,
		bizDate.Format("2006-01-02"), in.Factor, in.BudgetRows, in.HardCapRows, in.Status)
	return err
}

// ResetWatermark 重置扫描水位。
func (s *Store) ResetWatermark(ctx context.Context, name string, value time.Time) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableWatermark+` SET last_value = $2, gmt_update = now() WHERE watermark_name = $1`, name, value)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("水位 %s 不存在", name)
	}
	return nil
}

// UpsertPriority 新增或更新证券优先级。
func (s *Store) UpsertPriority(ctx context.Context, in PriorityUpsertInput) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tablePriority+` (usc, priority_rank, sla_days, max_wait_days, gmt_update)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (usc) DO UPDATE SET
  priority_rank = EXCLUDED.priority_rank,
  sla_days = EXCLUDED.sla_days,
  max_wait_days = EXCLUDED.max_wait_days,
  gmt_update = now()`,
		in.Usc, in.PriorityRank, in.SlaDays, in.MaxWaitDays)
	return err
}

// DeletePriority 删除证券优先级。
func (s *Store) DeletePriority(ctx context.Context, usc string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM `+tablePriority+` WHERE usc = $1`, usc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("优先级 %s 不存在", usc)
	}
	return nil
}

// UpsertCalendar 新增或更新交易日历标记。
func (s *Store) UpsertCalendar(ctx context.Context, in CalendarUpsertInput) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableCalendar+` (usc, trade_date, flag_trade_day)
VALUES ($1, $2, $3)
ON CONFLICT (usc, trade_date) DO UPDATE SET flag_trade_day = EXCLUDED.flag_trade_day`,
		in.Usc, in.TradeDate, in.FlagTradeDay)
	return err
}

// DeleteCalendar 删除一条交易日历。
func (s *Store) DeleteCalendar(ctx context.Context, usc string, tradeDate int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM `+tableCalendar+` WHERE usc = $1 AND trade_date = $2`, usc, tradeDate)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("交易日历 %s/%d 不存在", usc, tradeDate)
	}
	return nil
}

// UpsertUsc 新增或更新证券主数据（nil 字段保持原值）。
func (s *Store) UpsertUsc(ctx context.Context, in UscUpsertInput) error {
	if strings.TrimSpace(in.Usc) == "" {
		return errors.New("usc 不能为空")
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableUsc+` (usc, category, day_expect_rows, date_list, date_delist,
  flag_auto_discover, region, market, provider, name, timezone, currency, gmt_update)
VALUES ($1, $2, $3, $4::date, $5::date, COALESCE($6, true), $7, $8, $9, $10, $11, $12, now())
ON CONFLICT (usc) DO UPDATE SET
  category = COALESCE(EXCLUDED.category, `+tableUsc+`.category),
  day_expect_rows = COALESCE(EXCLUDED.day_expect_rows, `+tableUsc+`.day_expect_rows),
  date_list = COALESCE(EXCLUDED.date_list, `+tableUsc+`.date_list),
  date_delist = COALESCE(EXCLUDED.date_delist, `+tableUsc+`.date_delist),
  flag_auto_discover = COALESCE($6, `+tableUsc+`.flag_auto_discover),
  region = COALESCE(EXCLUDED.region, `+tableUsc+`.region),
  market = COALESCE(EXCLUDED.market, `+tableUsc+`.market),
  provider = COALESCE(EXCLUDED.provider, `+tableUsc+`.provider),
  name = COALESCE(EXCLUDED.name, `+tableUsc+`.name),
  timezone = COALESCE(EXCLUDED.timezone, `+tableUsc+`.timezone),
  currency = COALESCE(EXCLUDED.currency, `+tableUsc+`.currency),
  gmt_update = now()`,
		in.Usc, in.Category, in.DayExpectRows, in.DateList, in.DateDelist,
		in.FlagAutoDiscover, in.Region, in.Market, in.Provider, in.Name, in.Timezone, in.Currency)
	return err
}

// ResetPatchForRetry 把补丁任务重置为 PENDING。
func (s *Store) ResetPatchForRetry(ctx context.Context, patchID int64) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tablePatch+` SET status = 'PENDING', last_error = NULL WHERE patch_id = $1`, patchID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("补丁任务 %d 不存在", patchID)
	}
	return nil
}

// SetJobStatus 管理员变更任务状态（PAUSED / CANCELLED）。
func (s *Store) SetJobStatus(ctx context.Context, jobID int64, status, reason string) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableJob+` SET status = $2, last_error = $3, locked_by = NULL, locked_until = NULL, gmt_update = now()
WHERE job_id = $1`, jobID, status, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("导出任务 %d 不存在", jobID)
	}
	return nil
}

// AppendAdminAudit 记录一条管理员操作审计（写入运行日志表，job_kind=MANUAL_ADMIN_EDIT）。
func (s *Store) AppendAdminAudit(ctx context.Context, detail string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO `+tableRunLog+` (job_kind, gmt_start, gmt_finish, status, error)
VALUES ('MANUAL_ADMIN_EDIT', now(), now(), 'SUCCESS', $1)`, detail)
	return err
}
