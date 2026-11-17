package kline

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options 服务参数（来自环境变量）。
type Options struct {
	MaxBatchRows int    // 单次批量写入上限
	MaxListSize  int    // keys/uscs/PATCH 条目上限
	TSUnit       string // "s"（默认）/ "ms"（兼容换算）
}

// Service 分钟行情 K 线读写服务。
type Service struct {
	pool  *pgxpool.Pool
	opts  Options
	table string
}

// NewService 创建服务。
func NewService(pool *pgxpool.Pool, opts Options) *Service {
	if opts.MaxBatchRows <= 0 {
		opts.MaxBatchRows = 5000
	}
	if opts.MaxListSize <= 0 {
		opts.MaxListSize = 5000
	}
	if opts.TSUnit != "ms" {
		opts.TSUnit = "s"
	}
	return &Service{pool: pool, opts: opts, table: Table}
}

// rowArgs 按 InsertColumns 顺序展开参数。
func rowArgs(row Row) []any {
	return []any{
		row.Usc, row.Ts, row.Date, row.Time, row.Open, row.High, row.Low, row.Close,
		row.Volume, row.Turnover, row.ChangePrice, row.ChangeRatio, row.Paocd,
		row.ExtField, row.Remark, row.Provider,
	}
}

// rowTargets 按 selectColumnsSQL 的列顺序提供扫描目标。
func rowTargets(row *Row) []any {
	return []any{
		&row.Usc, &row.Ts, &row.Date, &row.Time, &row.Open, &row.High, &row.Low, &row.Close,
		&row.Volume, &row.Turnover, &row.ChangePrice, &row.ChangeRatio, &row.Paocd,
		&row.ExtField, &row.Remark, &row.Provider,
	}
}

func (s *Service) chunks(rows []Row) [][]Row {
	size := chunkSize()
	var out [][]Row
	for start := 0; start < len(rows); start += size {
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}
		out = append(out, rows[start:end])
	}
	return out
}

func (s *Service) rowsFromItems(items []ItemIn) ([]Row, *Error) {
	if len(items) == 0 {
		return nil, New(CodeMissingParam).WithDebug("field", "items")
	}
	if len(items) > s.opts.MaxBatchRows {
		return nil, New(CodeBatchTooLarge).
			WithDebug("limit", fmt.Sprint(s.opts.MaxBatchRows)).
			WithDebug("got", fmt.Sprint(len(items)))
	}
	rows := make([]Row, 0, len(items))
	for _, item := range items {
		row, err := item.ToRow(s.opts.TSUnit)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// checkDuplicate 请求内重复键拦截（写库前）。
func (s *Service) checkDuplicate(rows []Row) *Error {
	seen := make(map[Key]struct{}, len(rows))
	for _, row := range rows {
		key := Key{Usc: row.Usc, Ts: row.Ts}
		if _, ok := seen[key]; ok {
			return New(CodeDuplicateInRequest).
				WithDebug("first_dup", fmt.Sprintf("%s@%d", key.Usc, key.Ts)).
				WithDebug("count", fmt.Sprint(len(rows)))
		}
		seen[key] = struct{}{}
	}
	return nil
}

// NormalizeTS 按配置的 ts 单位归一入参（DC-04：默认秒级，ms 仅兼容换算）。
func (s *Service) NormalizeTS(value int64) (int64, *Error) {
	return normalizeTS(value, s.opts.TSUnit)
}

// Add 单条插入，返回落库后的整行。
func (s *Service) Add(ctx context.Context, item ItemIn) (Row, error) {
	row, verr := item.ToRow(s.opts.TSUnit)
	if verr != nil {
		return Row{}, verr
	}
	sql := insertSQL(1, " RETURNING "+selectColumnsSQL())
	var result Row
	if err := s.pool.QueryRow(ctx, sql, rowArgs(row)...).Scan(rowTargets(&result)...); err != nil {
		return Row{}, ClassifyDB(err)
	}
	return result, nil
}

// BatchInsert 批量插入（单事务，任一条冲突整体回滚）。
func (s *Service) BatchInsert(ctx context.Context, items []ItemIn) (int64, error) {
	rows, verr := s.rowsFromItems(items)
	if verr != nil {
		return 0, verr
	}
	if verr := s.checkDuplicate(rows); verr != nil {
		return 0, verr
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inserted int64
	for _, chunk := range s.chunks(rows) {
		args := make([]any, 0, len(chunk)*len(InsertColumns))
		for _, row := range chunk {
			args = append(args, rowArgs(row)...)
		}
		tag, execErr := tx.Exec(ctx, insertSQL(len(chunk), ""), args...)
		if execErr != nil {
			return 0, ClassifyDB(execErr)
		}
		inserted += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, ClassifyDB(err)
	}
	return inserted, nil
}

// Upsert 批量 upsert：mode=skip 冲突跳过；mode=update 冲突整行覆盖。
func (s *Service) Upsert(ctx context.Context, items []ItemIn, mode string) (int64, error) {
	if mode != "skip" && mode != "update" {
		return 0, New(CodeInvalidEnum).WithDebug("field", "onConflict").
			WithDebug("allowed", "skip,update")
	}
	rows, verr := s.rowsFromItems(items)
	if verr != nil {
		return 0, verr
	}
	// update 模式下一语句内重复键会触发 PostgreSQL「同一行被影响两次」，提前拦截；
	// skip 模式由 ON CONFLICT DO NOTHING 天然处理请求内重复（与旧服务一致）。
	if mode == "update" {
		if verr := s.checkDuplicate(rows); verr != nil {
			return 0, verr
		}
	}
	suffix := upsertConflict(mode)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var affected int64
	for _, chunk := range s.chunks(rows) {
		args := make([]any, 0, len(chunk)*len(InsertColumns))
		for _, row := range chunk {
			args = append(args, rowArgs(row)...)
		}
		tag, execErr := tx.Exec(ctx, insertSQL(len(chunk), suffix), args...)
		if execErr != nil {
			return 0, ClassifyDB(execErr)
		}
		affected += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, ClassifyDB(err)
	}
	return affected, nil
}

// Page 条件分页：返回满足条件的总记录数与当前页行。
func (s *Service) Page(ctx context.Context, filter ListFilter, limit, offset int) (int64, []Row, error) {
	if limit < 1 || limit > 1000 {
		return 0, nil, New(CodeOutOfRange).WithDebug("field", "limit").
			WithDebug("min", "1").WithDebug("max", "1000")
	}
	if offset < 0 {
		return 0, nil, New(CodeOutOfRange).WithDebug("field", "offset").WithDebug("min", "0")
	}
	where, args := listWhere(filter)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, nil, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var count int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+s.table+where, args...).Scan(&count); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	sql := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s, %s, %s LIMIT $%d OFFSET $%d",
		selectColumnsSQL(), s.table, where, quote(ColUsc), quote(ColDate), quote(ColTs), len(args)+1, len(args)+2)
	pgxRows, err := tx.Query(ctx, sql, pageArgs...)
	if err != nil {
		return 0, nil, ClassifyDB(err)
	}
	defer pgxRows.Close()

	items := make([]Row, 0, limit)
	for pgxRows.Next() {
		var row Row
		if err := pgxRows.Scan(rowTargets(&row)...); err != nil {
			return 0, nil, ClassifyDB(err)
		}
		items = append(items, row)
	}
	if err := pgxRows.Err(); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	return count, items, nil
}

// Query 按 keys / uscs / provider 组合查询（AND 语义）。
func (s *Service) Query(ctx context.Context, spec QuerySpec) (int64, []Row, error) {
	if len(spec.Keys) > s.opts.MaxListSize {
		return 0, nil, New(CodeListTooLarge).WithDebug("limit", fmt.Sprint(s.opts.MaxListSize)).
			WithDebug("got", fmt.Sprint(len(spec.Keys)))
	}
	if len(spec.Uscs) > s.opts.MaxListSize {
		return 0, nil, New(CodeListTooLarge).WithDebug("limit", fmt.Sprint(s.opts.MaxListSize)).
			WithDebug("got", fmt.Sprint(len(spec.Uscs)))
	}
	if len(spec.Keys) == 0 && len(spec.Uscs) == 0 && spec.Provider == nil {
		return 0, nil, New(CodeMissingParam).WithDebug("field", "keys|uscs|provider")
	}
	if spec.Limit < 1 || spec.Limit > 1000 {
		return 0, nil, New(CodeOutOfRange).WithDebug("field", "limit").
			WithDebug("min", "1").WithDebug("max", "1000")
	}
	if spec.Offset < 0 {
		return 0, nil, New(CodeOutOfRange).WithDebug("field", "offset").WithDebug("min", "0")
	}
	where, args := queryWhere(spec)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, nil, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var count int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+s.table+where, args...).Scan(&count); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	sql := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s, %s, %s LIMIT $%d OFFSET $%d",
		selectColumnsSQL(), s.table, where, quote(ColUsc), quote(ColDate), quote(ColTs), len(args)+1, len(args)+2)
	pgxRows, err := tx.Query(ctx, sql, append(args, spec.Limit, spec.Offset)...)
	if err != nil {
		return 0, nil, ClassifyDB(err)
	}
	defer pgxRows.Close()

	items := make([]Row, 0, spec.Limit)
	for pgxRows.Next() {
		var row Row
		if err := pgxRows.Scan(rowTargets(&row)...); err != nil {
			return 0, nil, ClassifyDB(err)
		}
		items = append(items, row)
	}
	if err := pgxRows.Err(); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, ClassifyDB(err)
	}
	return count, items, nil
}

// patchGroup 一组「显式更新列集合」相同的行。
type patchGroup struct {
	columns []string
	items   []PatchItem
}

// PatchBatch 批量增量更新（只更新显式传入的字段）。
func (s *Service) PatchBatch(ctx context.Context, items []ItemIn) (int64, error) {
	if len(items) > s.opts.MaxListSize {
		return 0, New(CodeListTooLarge).WithDebug("limit", fmt.Sprint(s.opts.MaxListSize)).
			WithDebug("got", fmt.Sprint(len(items)))
	}
	groups := make(map[string]*patchGroup, 4)
	order := make([]string, 0, 4)
	for _, item := range items {
		patch, verr := item.ToPatchItem(s.opts.TSUnit)
		if verr != nil {
			return 0, verr
		}
		if len(patch.Columns) == 0 {
			continue // 仅带定位键的条目跳过，不计入 updated
		}
		key := strings.Join(patch.Columns, ",")
		group, ok := groups[key]
		if !ok {
			group = &patchGroup{columns: append([]string(nil), patch.Columns...)}
			groups[key] = group
			order = append(order, key)
		}
		group.items = append(group.items, patch)
	}
	if len(groups) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, ClassifyDB(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updated int64
	for _, key := range order {
		group := groups[key]
		args := make([]any, 0, len(group.items)*(len(group.columns)+2))
		for _, patch := range group.items {
			args = append(args, patch.Usc, patch.Ts)
			for _, value := range patch.Values {
				args = append(args, value)
			}
		}
		tag, execErr := tx.Exec(ctx, patchSQL(group.columns, len(group.items)), args...)
		if execErr != nil {
			return 0, ClassifyDB(execErr)
		}
		updated += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, ClassifyDB(err)
	}
	return updated, nil
}

// DeleteBatch 批量删除（keys 与 uscs 之间为 OR 语义）。
func (s *Service) DeleteBatch(ctx context.Context, keys []Key, uscs []string) (int64, error) {
	if len(keys) > s.opts.MaxListSize {
		return 0, New(CodeListTooLarge).WithDebug("limit", fmt.Sprint(s.opts.MaxListSize)).
			WithDebug("got", fmt.Sprint(len(keys)))
	}
	if len(uscs) > s.opts.MaxListSize {
		return 0, New(CodeListTooLarge).WithDebug("limit", fmt.Sprint(s.opts.MaxListSize)).
			WithDebug("got", fmt.Sprint(len(uscs)))
	}
	if len(keys) == 0 && len(uscs) == 0 {
		return 0, New(CodeMissingParam).WithDebug("field", "keys|uscs")
	}
	where, args := deleteWhere(keys, uscs)
	tag, err := s.pool.Exec(ctx, "DELETE FROM "+s.table+where, args...)
	if err != nil {
		return 0, ClassifyDB(err)
	}
	return tag.RowsAffected(), nil
}

// Count 统计满足条件的记录总数。
func (s *Service) Count(ctx context.Context, filter CountFilter) (int64, error) {
	where, args := countWhere(filter)
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.table+where, args...).Scan(&total); err != nil {
		return 0, ClassifyDB(err)
	}
	return total, nil
}
