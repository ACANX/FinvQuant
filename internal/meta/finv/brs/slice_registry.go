// 大表分片备份登记（brs.brs_bak_rdb_table_slice）的维护：列表 / 新增编辑 / 分片计划查询。
//
// 与执行链（RunSliceOnceWithHooks）共用同一张登记表：页面改完规则后，
// 下一次手动触发即按新规则执行，不需要重启服务。
package brs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 标识符校验：schema / 表名只允许字母、数字、下划线。
var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ListSliceRules 查询全部大表分片备份登记（含停用，priority 倒序）。
func ListSliceRules(ctx context.Context, pool *pgxpool.Pool) ([]SliceRule, error) {
	const query = `
SELECT idn, rdb_type, db, schema_name, table_name, flag_enable, priority,
       slice_mode, slice_key_sql, slice_where_tpl, slice_file_tpl,
       bak_scope, compression, min_rows, max_slices, redo_slices, flag_upload,
       remark, creator, gmt_create, gmt_update
  FROM brs.brs_bak_rdb_table_slice
 ORDER BY priority DESC, idn ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SliceRule, 0, 8)
	for rows.Next() {
		var rule SliceRule
		if err := rows.Scan(&rule.IDN, &rule.RdbType, &rule.DB, &rule.SchemaName, &rule.TableName,
			&rule.FlagEnable, &rule.Priority, &rule.SliceMode, &rule.SliceKeySQL, &rule.SliceWhereTpl,
			&rule.SliceFileTpl, &rule.BakScope, &rule.Compression, &rule.MinRows, &rule.MaxSlices,
			&rule.RedoSlices, &rule.FlagUpload, &rule.Remark, &rule.Creator,
			&rule.GmtCreate, &rule.GmtUpdate); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// SliceRuleInput 新增/编辑大表分片备份登记的请求体（字段为 snake_case）。
type SliceRuleInput struct {
	IDN           int64   `json:"idn"`             // 主键（新增时传 0，编辑时必填）
	DB            string  `json:"db"`              // 归属库，默认 finv
	SchemaName    string  `json:"schema_name"`     // schema 名
	TableName     string  `json:"table_name"`      // 表名
	FlagEnable    string  `json:"flag_enable"`     // 启用开关：1 启用 / 0 停用
	Priority      int     `json:"priority"`        // 执行顺序，越大越先
	SliceMode     string  `json:"slice_mode"`      // 分片维度：date/week/month/column/expr
	SliceKeySQL   *string `json:"slice_key_sql"`   // 分片枚举 SQL（column/expr 必填）
	SliceWhereTpl string  `json:"slice_where_tpl"` // where 模板，占位 {value}/{start}/{end}
	SliceFileTpl  string  `json:"slice_file_tpl"`  // 产物文件名模板，占位 {key}
	BakScope      string  `json:"bak_scope"`       // 备份范围：data / both / schema
	Compression   string  `json:"compression"`     // 压缩方式：none / gzip / zstd
	MinRows       int64   `json:"min_rows"`        // 分片行数下限
	MaxSlices     int     `json:"max_slices"`      // 分片数上限
	RedoSlices    int     `json:"redo_slices"`     // 末尾 N 个分片强制重做
	FlagUpload    string  `json:"flag_upload"`     // 上传开关：1 上传 / 0 仅本地
	Remark        *string `json:"remark"`          // 备注
}

// validate 校验登记内容，返回第一个不合法的原因。
func (in *SliceRuleInput) validate() error {
	in.DB = strings.TrimSpace(in.DB)
	in.SchemaName = strings.TrimSpace(in.SchemaName)
	in.TableName = strings.TrimSpace(in.TableName)
	in.SliceMode = strings.TrimSpace(in.SliceMode)
	in.SliceWhereTpl = strings.TrimSpace(in.SliceWhereTpl)
	in.SliceFileTpl = strings.TrimSpace(in.SliceFileTpl)
	in.BakScope = strings.TrimSpace(in.BakScope)
	in.Compression = strings.TrimSpace(in.Compression)

	if in.DB == "" {
		in.DB = "finv"
	}
	if !identPattern.MatchString(in.DB) {
		return inputErrorf("db 只能是字母、数字、下划线")
	}
	if !identPattern.MatchString(in.SchemaName) {
		return inputErrorf("schema_name 只能是字母、数字、下划线")
	}
	if !identPattern.MatchString(in.TableName) {
		return inputErrorf("table_name 只能是字母、数字、下划线")
	}
	if in.FlagEnable == "" {
		in.FlagEnable = FlagEnable
	}
	if in.FlagUpload == "" {
		in.FlagUpload = FlagEnable
	}
	if in.FlagEnable != FlagEnable && in.FlagEnable != FlagDisable {
		return inputErrorf("flag_enable 只能是 1 或 0")
	}
	if in.FlagUpload != FlagEnable && in.FlagUpload != FlagDisable {
		return inputErrorf("flag_upload 只能是 1 或 0")
	}
	switch in.SliceMode {
	case SliceModeDate, SliceModeWeek, SliceModeMonth:
	case SliceModeColumn, SliceModeExpr:
		if in.SliceKeySQL == nil || strings.TrimSpace(*in.SliceKeySQL) == "" {
			return inputErrorf("slice_mode=%s 时必须填写 slice_key_sql", in.SliceMode)
		}
	default:
		return inputErrorf("slice_mode 只能是 date / week / month / column / expr")
	}
	if in.SliceKeySQL != nil {
		query := strings.TrimSpace(*in.SliceKeySQL)
		if query != "" {
			lower := strings.ToLower(query)
			if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
				return inputErrorf("slice_key_sql 只允许 SELECT / WITH 查询")
			}
			if strings.Contains(query, ";") {
				return inputErrorf("slice_key_sql 不允许包含分号")
			}
		}
	}
	if in.SliceWhereTpl == "" {
		return inputErrorf("slice_where_tpl 不能为空")
	}
	hasRange := strings.Contains(in.SliceWhereTpl, "{start}") || strings.Contains(in.SliceWhereTpl, "{end}")
	if !strings.Contains(in.SliceWhereTpl, "{value}") && !hasRange {
		return inputErrorf("slice_where_tpl 至少包含 {value} 或 {start}/{end} 之一")
	}
	if in.SliceFileTpl == "" {
		in.SliceFileTpl = "{key}"
	}
	if !strings.Contains(in.SliceFileTpl, "{key}") {
		return inputErrorf("slice_file_tpl 必须包含 {key}")
	}
	// 分片备份只导出数据（CSV），表结构由迁移负责创建；因此只接受 data。
	if in.BakScope == "" {
		in.BakScope = ScopeData
	}
	if in.BakScope != ScopeData {
		return inputErrorf("bak_scope 只能是 data（分片备份只导数据，表结构由迁移创建）")
	}
	// 分片产物是 MVSV-2 文本文件，只支持 gzip（默认）或不压缩；pg_dump 的 zstd 不适用。
	if in.Compression == "" {
		in.Compression = CompressionGzip
	}
	switch in.Compression {
	case CompressionNone, CompressionGzip:
	default:
		return inputErrorf("compression 只能是 gzip / none（分片产物为 MVSV-2 文本）")
	}
	if in.Priority < 0 {
		return inputErrorf("priority 不能为负")
	}
	if in.MinRows < 0 {
		return inputErrorf("min_rows 不能为负")
	}
	if in.MaxSlices <= 0 || in.MaxSlices > 100000 {
		return inputErrorf("max_slices 取值范围 1~100000")
	}
	if in.RedoSlices < 0 || in.RedoSlices > in.MaxSlices {
		return inputErrorf("redo_slices 取值范围 0~max_slices")
	}
	return nil
}

// sliceRequiredColumns 分片导出所用的 MVSV-2 列（与 slice.go 的 sliceColumns 对应）。
var sliceRequiredColumns = []string{
	"usc", "ts", "date", "time", "open", "high", "low", "close", "last_close",
	"volume", "turnover", "change_price", "change_ratio", "vwap",
	"ext_field", "remark", "provider", "gmt_create", "gmt_update",
}

// validateSliceTable 校验目标表存在且列与 MVSV-2 布局一致（分片产物按该布局写出）。
//
// MVSV-2 是系统既有的行情交换格式（与 qiom export/import 一致），因此分片备份
// 只支持与 cdeu.finv_quote_secu_kline_min 同构的表。
func validateSliceTable(ctx context.Context, pool *pgxpool.Pool, schemaName, tableName string) error {
	var matched int
	err := pool.QueryRow(ctx, `
SELECT count(*)
  FROM information_schema.columns
 WHERE table_schema = $1 AND table_name = $2 AND column_name = ANY($3)`,
		schemaName, tableName, sliceRequiredColumns).Scan(&matched)
	if err != nil {
		return fmt.Errorf("校验目标表失败: %w", err)
	}
	if matched == 0 {
		return inputErrorf("目标表 %s.%s 不存在或不可读", schemaName, tableName)
	}
	if matched != len(sliceRequiredColumns) {
		return inputErrorf("目标表 %s.%s 与 MVSV-2 布局不一致（需与 cdeu.finv_quote_secu_kline_min 同构，缺少列 %d 个）",
			schemaName, tableName, len(sliceRequiredColumns)-matched)
	}
	return nil
}

// SaveSliceRule 新增或编辑一条大表分片备份登记。
func SaveSliceRule(ctx context.Context, pool *pgxpool.Pool, in SliceRuleInput, operator string) (SliceRule, error) {
	if err := in.validate(); err != nil {
		return SliceRule{}, err
	}
	if err := validateSliceTable(ctx, pool, in.SchemaName, in.TableName); err != nil {
		return SliceRule{}, err
	}
	var operatorPtr *string
	if strings.TrimSpace(operator) != "" {
		op := strings.TrimSpace(operator)
		operatorPtr = &op
	}

	const upsert = `
INSERT INTO brs.brs_bak_rdb_table_slice
    (rdb_type, db, schema_name, table_name, flag_enable, priority,
     slice_mode, slice_key_sql, slice_where_tpl, slice_file_tpl,
     bak_scope, compression, min_rows, max_slices, redo_slices, flag_upload, remark, creator)
VALUES ('postgresql', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT ON CONSTRAINT uq_brs_bak_rdb_table_slice DO UPDATE SET
    flag_enable     = EXCLUDED.flag_enable,
    priority        = EXCLUDED.priority,
    slice_mode      = EXCLUDED.slice_mode,
    slice_key_sql   = EXCLUDED.slice_key_sql,
    slice_where_tpl = EXCLUDED.slice_where_tpl,
    slice_file_tpl  = EXCLUDED.slice_file_tpl,
    bak_scope       = EXCLUDED.bak_scope,
    compression     = EXCLUDED.compression,
    min_rows        = EXCLUDED.min_rows,
    max_slices      = EXCLUDED.max_slices,
    redo_slices     = EXCLUDED.redo_slices,
    flag_upload     = EXCLUDED.flag_upload,
    remark          = EXCLUDED.remark
RETURNING idn`

	var idn int64
	err := pool.QueryRow(ctx, upsert,
		in.DB, in.SchemaName, in.TableName, in.FlagEnable, in.Priority,
		in.SliceMode, in.SliceKeySQL, in.SliceWhereTpl, in.SliceFileTpl,
		in.BakScope, in.Compression, in.MinRows, in.MaxSlices, in.RedoSlices, in.FlagUpload,
		in.Remark, operatorPtr).Scan(&idn)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return SliceRule{}, inputErrorf("该表已存在分片备份登记（%s.%s.%s）", in.DB, in.SchemaName, in.TableName)
		}
		return SliceRule{}, fmt.Errorf("保存大表分片备份登记失败: %w", err)
	}

	rules, err := ListSliceRules(ctx, pool)
	if err != nil {
		return SliceRule{}, err
	}
	for _, rule := range rules {
		if rule.IDN == idn {
			return rule, nil
		}
	}
	return SliceRule{}, fmt.Errorf("保存后未找到登记行（idn=%d）", idn)
}

// GetSliceRule 按主键读取一条登记。
func GetSliceRule(ctx context.Context, pool *pgxpool.Pool, idn int64) (SliceRule, error) {
	rules, err := ListSliceRules(ctx, pool)
	if err != nil {
		return SliceRule{}, err
	}
	for _, rule := range rules {
		if rule.IDN == idn {
			return rule, nil
		}
	}
	return SliceRule{}, inputErrorf("大表分片备份登记不存在（idn=%d）", idn)
}

// ToggleSliceRule 启用/停用一条登记。
func ToggleSliceRule(ctx context.Context, pool *pgxpool.Pool, idn int64, enable bool) (SliceRule, error) {
	flag := FlagDisable
	if enable {
		flag = FlagEnable
	}
	res, err := pool.Exec(ctx,
		`UPDATE brs.brs_bak_rdb_table_slice SET flag_enable = $2 WHERE idn = $1`, idn, flag)
	if err != nil {
		return SliceRule{}, fmt.Errorf("更新启用状态失败: %w", err)
	}
	if res.RowsAffected() == 0 {
		return SliceRule{}, inputErrorf("大表分片备份登记不存在（idn=%d）", idn)
	}
	return GetSliceRule(ctx, pool, idn)
}

// BuildSlicePlan 构建分片计划（供页面展示完成度）：枚举分片并逐个判断产物是否存在。
//
// 只用于展示，不产生文件；items 最多返回 limit 条（避免上千条记录撑爆响应），
// 计数始终按全量分片统计。
func BuildSlicePlan(ctx context.Context, pool *pgxpool.Pool, opts Options, rule SliceRule, limit int) (SlicePlan, error) {
	plan := SlicePlan{RuleID: rule.IDN, Table: rule.Table()}
	slices, err := ListSlices(ctx, pool, rule)
	if err != nil {
		return plan, err
	}
	store, env := resolveStoreAndEnv(ctx, pool, opts, Hooks{})

	plan.Total = len(slices)
	redoFrom := len(slices) - rule.RedoSlices
	if redoFrom < 0 {
		redoFrom = 0
	}
	plan.RedoFrom = redoFrom
	if limit <= 0 || limit > len(slices) {
		limit = len(slices)
	}
	for i, slice := range slices {
		if slice.Rows < rule.MinRows {
			plan.Skipped++
			continue
		}
		exists, _ := sliceExists(ctx, store, rule, opts, env, slice)
		// 重做窗口内的分片即使已存在也算「待处理」，与本轮执行口径一致。
		backed := exists && i < redoFrom
		if backed {
			plan.Done++
		} else {
			plan.Missing++
		}
		if len(plan.Items) < limit {
			plan.Items = append(plan.Items, SlicePlanItem{Key: slice.Key, Rows: slice.Rows, Backuped: backed})
		}
	}
	return plan, nil
}

// SliceRuleWithPlan 登记 + 当前分片计划（列表接口用，便于页面一屏展示完成度）。
type SliceRuleWithPlan struct {
	SliceRule
	Plan SlicePlan `json:"plan"` // 该表当前的分片完成度
}

// ListSliceRulesWithPlan 列表 + 每张表的分片完成度。
//
// items 只返回前 itemLimit 条；分片枚举失败时 plan.error 记录原因，不影响其它表展示。
func ListSliceRulesWithPlan(ctx context.Context, pool *pgxpool.Pool, opts Options, itemLimit int) ([]SliceRuleWithPlan, error) {
	rules, err := ListSliceRules(ctx, pool)
	if err != nil {
		return nil, err
	}
	out := make([]SliceRuleWithPlan, 0, len(rules))
	for _, rule := range rules {
		item := SliceRuleWithPlan{SliceRule: rule}
		plan, planErr := BuildSlicePlan(ctx, pool, opts, rule, itemLimit)
		if planErr != nil {
			// 单个登记的分片枚举失败不影响列表展示，但要留痕便于排查。
			log.Printf("[brs] 生成分片计划失败 %s: %v", rule.Table(), planErr)
			item.Plan = SlicePlan{RuleID: rule.IDN, Table: rule.Table()}
		} else {
			item.Plan = plan
		}
		out = append(out, item)
	}
	return out, nil
}
