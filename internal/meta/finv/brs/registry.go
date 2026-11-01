// Package brs 的备份登记（brs.brs_bak_rdb_table）管理：查询 / 新增 / 编辑 / 启用禁用。
//
// 与备份执行链（LoadTables）共用同一张登记表：本文件只做登记维护，
// 修改后下一轮 cron 备份（sys.sys_config 39001）即按新登记执行，无需重启。
package brs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 登记表的枚举取值（与迁移 V300060 / V300061 的 CHECK 约束一致）。
const (
	ScopeBoth   = "both"
	ScopeSchema = "schema"
	ScopeData   = "data"

	MethodPgDump = "pg_dump"
	MethodCopy   = "copy"

	CompressionNone = "none"
	CompressionGzip = "gzip"
	CompressionZstd = "zstd"

	// FlagEnable / FlagDisable 启用标志（char(1)）。
	FlagEnable  = "1"
	FlagDisable = "0"
)

// InputError 表示请求内容不合法（字段校验失败或唯一键冲突），对应 HTTP 400。
type InputError struct {
	msg string
}

// Error 实现 error 接口。
func (e *InputError) Error() string { return e.msg }

// inputErrorf 构造 InputError。
func inputErrorf(format string, a ...any) error {
	return &InputError{msg: fmt.Sprintf(format, a...)}
}

// RegistryRow 一条备份登记（JSON wire 字段为 snake_case，与库列名一致）。
type RegistryRow struct {
	IDN         int64      `json:"idn"`
	RdbType     string     `json:"rdb_type"`
	SchemaName  string     `json:"schema_name"`
	TableName   string     `json:"table_name"`
	FlagEnable  string     `json:"flag_enable"`
	Priority    int        `json:"priority"`
	BakScope    string     `json:"bak_scope"`
	BakMethod   string     `json:"bak_method"`
	Compression string     `json:"compression"`
	FlagUpload  string     `json:"flag_upload"`
	WhereClause *string    `json:"where_clause"`
	Remark      *string    `json:"remark"`
	Creator     *string    `json:"creator"`
	GmtCreate   *time.Time `json:"gmt_create"`
	GmtUpdate   *time.Time `json:"gmt_update"`
}

// Registry 备份登记管理服务。
type Registry struct {
	pool *pgxpool.Pool
}

// NewRegistry 创建备份登记管理服务。
func NewRegistry(pool *pgxpool.Pool) *Registry {
	return &Registry{pool: pool}
}

// List 分页查询备份登记。
//
// keyword 模糊匹配 schema_name / table_name / remark；flagEnable 为空表示不过滤，
// 否则只取 '1'（启用）或 '0'（停用）。排序：priority 倒序（与备份执行顺序一致）、idn 升序。
func (r *Registry) List(ctx context.Context, page, pageSize int, keyword, flagEnable string) ([]RegistryRow, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	if flagEnable != "" && flagEnable != FlagEnable && flagEnable != FlagDisable {
		return nil, 0, inputErrorf("flagEnable 只能为 0 或 1")
	}

	// 参数位置：$1 keyword 模式（空串不过滤）、$2 flagEnable、$3 limit、$4 offset
	pattern := ""
	if kw := strings.TrimSpace(keyword); kw != "" {
		pattern = "%" + kw + "%"
	}
	const query = `
SELECT idn, rdb_type, schema_name, table_name, flag_enable, priority, bak_scope,
       bak_method, compression, flag_upload, where_clause, remark, creator,
       gmt_create, gmt_update,
       count(*) OVER () AS total
  FROM brs.brs_bak_rdb_table
 WHERE ($1 = '' OR schema_name ILIKE $1 OR table_name ILIKE $1 OR COALESCE(remark, '') ILIKE $1)
   AND ($2 = '' OR flag_enable = $2)
 ORDER BY priority DESC, idn
 LIMIT $3 OFFSET $4`

	rows, err := r.pool.Query(ctx, query, pattern, flagEnable, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("查询备份登记失败: %w", err)
	}
	defer rows.Close()

	list := make([]RegistryRow, 0, pageSize)
	total := 0
	for rows.Next() {
		var row RegistryRow
		if err := rows.Scan(&row.IDN, &row.RdbType, &row.SchemaName, &row.TableName,
			&row.FlagEnable, &row.Priority, &row.BakScope, &row.BakMethod, &row.Compression,
			&row.FlagUpload, &row.WhereClause, &row.Remark, &row.Creator,
			&row.GmtCreate, &row.GmtUpdate, &total); err != nil {
			return nil, 0, fmt.Errorf("解析备份登记失败: %w", err)
		}
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("读取备份登记失败: %w", err)
	}
	return list, total, nil
}

// Get 按主键读取一条备份登记；不存在返回 pgx.ErrNoRows。
func (r *Registry) Get(ctx context.Context, idn int64) (RegistryRow, error) {
	const query = `
SELECT idn, rdb_type, schema_name, table_name, flag_enable, priority, bak_scope,
       bak_method, compression, flag_upload, where_clause, remark, creator,
       gmt_create, gmt_update
  FROM brs.brs_bak_rdb_table
 WHERE idn = $1`
	var row RegistryRow
	err := r.pool.QueryRow(ctx, query, idn).Scan(&row.IDN, &row.RdbType, &row.SchemaName,
		&row.TableName, &row.FlagEnable, &row.Priority, &row.BakScope, &row.BakMethod,
		&row.Compression, &row.FlagUpload, &row.WhereClause, &row.Remark, &row.Creator,
		&row.GmtCreate, &row.GmtUpdate)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegistryRow{}, pgx.ErrNoRows
	}
	if err != nil {
		return RegistryRow{}, fmt.Errorf("查询备份登记失败: %w", err)
	}
	return row, nil
}

// Save 新增（idn=0）或更新一条备份登记，返回主键。
//
// 校验在服务层完成，保证与库内 CHECK 约束一致的错误语义；
// 唯一键冲突（表重复登记 / 优先级被占用）转成可读错误。
func (r *Registry) Save(ctx context.Context, row RegistryRow) (int64, error) {
	row.RdbType = strings.TrimSpace(row.RdbType)
	if row.RdbType == "" {
		row.RdbType = "postgresql"
	}
	row.SchemaName = strings.TrimSpace(row.SchemaName)
	row.TableName = strings.TrimSpace(row.TableName)
	row.BakScope = strings.TrimSpace(row.BakScope)
	row.BakMethod = strings.TrimSpace(row.BakMethod)
	row.Compression = strings.TrimSpace(row.Compression)
	row.FlagEnable = strings.TrimSpace(row.FlagEnable)
	row.FlagUpload = strings.TrimSpace(row.FlagUpload)

	if row.SchemaName == "" || row.TableName == "" {
		return 0, inputErrorf("schema_name 与 table_name 不能为空")
	}
	if row.Priority < 0 {
		return 0, inputErrorf("priority 必须为非负整数")
	}
	if row.BakScope == "" {
		row.BakScope = ScopeBoth
	}
	switch row.BakScope {
	case ScopeBoth, ScopeSchema, ScopeData:
	default:
		return 0, inputErrorf("bak_scope 只能为 both / schema / data")
	}
	if row.BakMethod == "" {
		row.BakMethod = MethodPgDump
	}
	switch row.BakMethod {
	case MethodPgDump, MethodCopy:
	default:
		return 0, inputErrorf("bak_method 只能为 pg_dump / copy")
	}
	if row.Compression == "" {
		row.Compression = CompressionZstd
	}
	switch row.Compression {
	case CompressionNone, CompressionGzip, CompressionZstd:
	default:
		return 0, inputErrorf("compression 只能为 none / gzip / zstd")
	}
	if row.FlagEnable == "" {
		row.FlagEnable = FlagEnable
	}
	if row.FlagEnable != FlagEnable && row.FlagEnable != FlagDisable {
		return 0, inputErrorf("flag_enable 只能为 0 或 1")
	}
	if row.FlagUpload == "" {
		row.FlagUpload = FlagEnable
	}
	if row.FlagUpload != FlagEnable && row.FlagUpload != FlagDisable {
		return 0, inputErrorf("flag_upload 只能为 0 或 1")
	}

	if row.IDN == 0 {
		const insert = `
INSERT INTO brs.brs_bak_rdb_table
    (rdb_type, schema_name, table_name, flag_enable, priority, bak_scope, bak_method,
     compression, flag_upload, where_clause, remark, creator)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING idn`
		var idn int64
		err := r.pool.QueryRow(ctx, insert, row.RdbType, row.SchemaName, row.TableName,
			row.FlagEnable, row.Priority, row.BakScope, row.BakMethod, row.Compression,
			row.FlagUpload, row.WhereClause, row.Remark, row.Creator).Scan(&idn)
		if err != nil {
			return 0, wrapRegistryWriteError(err)
		}
		return idn, nil
	}

	const update = `
UPDATE brs.brs_bak_rdb_table SET
    rdb_type = $2, schema_name = $3, table_name = $4, flag_enable = $5, priority = $6,
    bak_scope = $7, bak_method = $8, compression = $9, flag_upload = $10,
    where_clause = $11, remark = $12
WHERE idn = $1`
	tag, err := r.pool.Exec(ctx, update, row.IDN, row.RdbType, row.SchemaName, row.TableName,
		row.FlagEnable, row.Priority, row.BakScope, row.BakMethod, row.Compression,
		row.FlagUpload, row.WhereClause, row.Remark)
	if err != nil {
		return 0, wrapRegistryWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return 0, pgx.ErrNoRows
	}
	return row.IDN, nil
}

// Toggle 切换启用状态（flag_enable：'1' 启用 / '0' 停用）。
func (r *Registry) Toggle(ctx context.Context, idn int64, flagEnable string) error {
	if idn <= 0 {
		return inputErrorf("idn 必须为正整数")
	}
	if flagEnable != FlagEnable && flagEnable != FlagDisable {
		return inputErrorf("flag_enable 只能为 0 或 1")
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE brs.brs_bak_rdb_table SET flag_enable = $2 WHERE idn = $1`, idn, flagEnable)
	if err != nil {
		return fmt.Errorf("更新启用状态失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// wrapRegistryWriteError 把唯一键冲突转成可读错误（按约束名区分原因）。
func wrapRegistryWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_brs_bak_rdb_table":
			return inputErrorf("该表已登记：同一 rdb_type + schema_name + table_name 只能有一条")
		case "uq_brs_bak_rdb_table_priority":
			return inputErrorf("优先级 priority 已被其他登记占用，请换一个非负整数")
		}
		return inputErrorf("唯一键冲突（%s）：%s", pgErr.ConstraintName, pgErr.Detail)
	}
	return fmt.Errorf("写入备份登记失败: %w", err)
}
