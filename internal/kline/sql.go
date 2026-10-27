package kline

import (
	"fmt"
	"strings"
)

// Table 业务表（必须 schema 全限定，见 Docs/DevSpec/DbSchemaSpec.md）。
const Table = "cdeu.finv_quote_secu_kline_min"

// maxParamsPerStatement PostgreSQL 单条语句参数上限 65535，按 60000 留余量。
const maxParamsPerStatement = 60000

// ColumnTypes PATCH 的 VALUES 显式类型转换（全 NULL 行也能推断出列类型）。
var ColumnTypes = map[string]string{
	ColUsc:         "varchar",
	ColTs:          "bigint",
	ColDate:        "integer",
	ColTime:        "integer",
	ColOpen:        "numeric(20,6)",
	ColHigh:        "numeric(20,6)",
	ColLow:         "numeric(20,6)",
	ColClose:       "numeric(20,6)",
	ColVolume:      "numeric(20,6)",
	ColTurnover:    "numeric(30,8)",
	ColChangePrice: "numeric(20,6)",
	ColChangeRatio: "numeric(20,6)",
	ColPaocd:       "numeric(20,6)",
	ColExtField:    "text",
	ColRemark:      "text",
	ColProvider:    "varchar",
}

// quote 给标识符加双引号（time 等关键字必须加）。
func quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func quotedColumns(columns []string) string {
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		out = append(out, quote(column))
	}
	return strings.Join(out, ", ")
}

// selectColumnsSQL 查询列：numeric 统一 ::text，避免 float64 精度丢失。
func selectColumnsSQL() string {
	out := make([]string, 0, len(InsertColumns))
	for _, column := range InsertColumns {
		if NumericColumns[column] {
			out = append(out, quote(column)+"::text")
			continue
		}
		out = append(out, quote(column))
	}
	return strings.Join(out, ", ")
}

// chunkSize 单条语句最多容纳的行数（60000/16 ≈ 3750，与 Python 一致）。
func chunkSize() int {
	per := maxParamsPerStatement / len(InsertColumns)
	if per < 1 {
		return 1
	}
	return per
}

// placeholders 生成从 start 开始的 n 个占位符：$start, $start+1, ...
func placeholders(start, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("$%d", start+i))
	}
	return out
}

// insertSQL 构造多行 VALUES 插入语句。
func insertSQL(nRows int, suffix string) string {
	values := make([]string, 0, nRows)
	arg := 1
	for i := 0; i < nRows; i++ {
		row := "(" + strings.Join(placeholders(arg, len(InsertColumns)), ", ") + ")"
		values = append(values, row)
		arg += len(InsertColumns)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES %s%s",
		Table, quotedColumns(InsertColumns), strings.Join(values, ", "), suffix)
}

// upsertConflict 返回 upsert 的冲突处理后缀。
func upsertConflict(mode string) string {
	if mode != "update" {
		return " ON CONFLICT (usc, ts) DO NOTHING"
	}
	assignments := make([]string, 0, len(UpsertUpdateColumns))
	for _, column := range UpsertUpdateColumns {
		assignments = append(assignments, fmt.Sprintf("%s = EXCLUDED.%s", quote(column), quote(column)))
	}
	return " ON CONFLICT (usc, ts) DO UPDATE SET " + strings.Join(assignments, ", ") +
		", " + quote("gmt_update") + " = now()"
}

// upsertSQL 单行 upsert 语句（测试与快照用）。
func upsertSQL(mode string) string {
	return insertSQL(1, upsertConflict(mode))
}

// patchSQL 批量增量更新：按列集合分组，一条 UPDATE ... FROM (VALUES ...)。
func patchSQL(columns []string, nRows int) string {
	all := append([]string{ColUsc, ColTs}, columns...)
	values := make([]string, 0, nRows)
	arg := 1
	for i := 0; i < nRows; i++ {
		casts := make([]string, 0, len(all))
		for _, column := range all {
			casts = append(casts, fmt.Sprintf("$%d::%s", arg, ColumnTypes[column]))
			arg++
		}
		values = append(values, "("+strings.Join(casts, ", ")+")")
	}
	assignments := make([]string, 0, len(columns))
	for _, column := range columns {
		assignments = append(assignments, fmt.Sprintf("%s = v.%s", quote(column), quote(column)))
	}
	return fmt.Sprintf(
		"UPDATE %s AS t SET %s FROM (VALUES %s) AS v(%s) WHERE t.%s = v.%s AND t.%s = v.%s",
		Table,
		strings.Join(assignments, ", "),
		strings.Join(values, ", "),
		quotedColumns(all),
		quote(ColUsc), quote(ColUsc),
		quote(ColTs), quote(ColTs),
	)
}

// whereParts 累积 WHERE 子句与参数（占位符按参数数量自动编号）。
type whereParts struct {
	clauses []string
	args    []any
}

func (w *whereParts) eq(column string, value any) {
	w.clauses = append(w.clauses, fmt.Sprintf("%s = $%d", quote(column), len(w.args)+1))
	w.args = append(w.args, value)
}

func (w *whereParts) compare(column, operator string, value any) {
	w.clauses = append(w.clauses, fmt.Sprintf("%s %s $%d", quote(column), operator, len(w.args)+1))
	w.args = append(w.args, value)
}

// where 返回 " WHERE ..." 片段与参数（无条件下为空串）。
func (w *whereParts) where() (string, []any) {
	if len(w.clauses) == 0 {
		return "", w.args
	}
	return " WHERE " + strings.Join(w.clauses, " AND "), w.args
}

// listWhere 构造列表/计数过滤条件。
func listWhere(filter ListFilter) (string, []any) {
	var w whereParts
	if filter.Usc != nil {
		w.eq(ColUsc, *filter.Usc)
	}
	if filter.Provider != nil {
		w.eq(ColProvider, *filter.Provider)
	}
	if filter.DateFrom != nil {
		w.compare(ColDate, ">=", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		w.compare(ColDate, "<=", *filter.DateTo)
	}
	if filter.TsFrom != nil {
		w.compare(ColTs, ">=", *filter.TsFrom)
	}
	if filter.TsTo != nil {
		w.compare(ColTs, "<=", *filter.TsTo)
	}
	return w.where()
}

// countWhere 构造 /Count 过滤条件（仅 usc/provider/date 范围）。
func countWhere(filter CountFilter) (string, []any) {
	return listWhere(ListFilter{
		Usc:      filter.Usc,
		Provider: filter.Provider,
		DateFrom: filter.DateFrom,
		DateTo:   filter.DateTo,
	})
}

// queryWhere 构造 /Query 组合条件（keys AND uscs AND provider）。
func queryWhere(spec QuerySpec) (string, []any) {
	var w whereParts
	if len(spec.Keys) > 0 {
		placeholders := make([]string, 0, len(spec.Keys))
		for _, key := range spec.Keys {
			placeholders = append(placeholders,
				fmt.Sprintf("($%d, $%d)", len(w.args)+1, len(w.args)+2))
			w.args = append(w.args, key.Usc, key.Ts)
		}
		w.clauses = append(w.clauses, fmt.Sprintf("(%s, %s) IN (%s)",
			quote(ColUsc), quote(ColTs), strings.Join(placeholders, ", ")))
	}
	if len(spec.Uscs) > 0 {
		w.clauses = append(w.clauses, fmt.Sprintf("%s = ANY($%d)", quote(ColUsc), len(w.args)+1))
		w.args = append(w.args, spec.Uscs)
	}
	if spec.Provider != nil {
		w.eq(ColProvider, *spec.Provider)
	}
	return w.where()
}

// deleteWhere 构造删除条件（keys OR uscs）。
func deleteWhere(keys []Key, uscs []string) (string, []any) {
	var clauses []string
	var args []any
	if len(keys) > 0 {
		placeholders := make([]string, 0, len(keys))
		for _, key := range keys {
			placeholders = append(placeholders,
				fmt.Sprintf("($%d, $%d)", len(args)+1, len(args)+2))
			args = append(args, key.Usc, key.Ts)
		}
		clauses = append(clauses, fmt.Sprintf("(%s, %s) IN (%s)",
			quote(ColUsc), quote(ColTs), strings.Join(placeholders, ", ")))
	}
	if len(uscs) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s = ANY($%d)", quote(ColUsc), len(args)+1))
		args = append(args, uscs)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " OR "), args
}
