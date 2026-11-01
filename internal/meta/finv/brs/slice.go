// 大表增量（分片）备份：登记表 brs.brs_bak_rdb_table_slice 的读取、分片枚举与逐片导出。
//
// 与小表全量备份（brs.brs_bak_rdb_table + RunOnce）完全独立：
//
//	· 配置独立（另一张表，不改小表登记）；
//	· 产物独立（<BackupDir>/Slice/<db>.<schema>.<table>/<分片键>.dump）；
//	· 上传独立（<env>/<db>.<schema>.<table>/<分片键>.dump）。
//
// 增量语义（不新增状态表）：
//
//	· 分片键从库里枚举（如 SELECT DISTINCT date），逐个判断「本地文件或远端对象已存在」；
//	· 已存在则跳过；末尾 redo_slices 个分片强制重做（最近的日期数据仍在增长）；
//	· 因此重跑幂等：同一分片键覆盖同一文件/同一远端 key。
package brs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/base/oss"
	"github.com/acanx/finvquant/internal/meta/finv/quote/mvsv"
)

// 分片维度取值（与迁移 V300088 的 CHECK 约束一致）。
const (
	SliceModeDate   = "date"
	SliceModeWeek   = "week"
	SliceModeMonth  = "month"
	SliceModeColumn = "column"
	SliceModeExpr   = "expr"
)

// SliceDirName 分片备份在备份根目录下的子目录名。
const SliceDirName = "Slice"

// SliceRule 一条大表分片备份登记（对应 brs.brs_bak_rdb_table_slice 一行）。
type SliceRule struct {
	IDN           int64      `json:"idn"`             // 主键
	RdbType       string     `json:"rdb_type"`        // 数据库类型（首期固定 postgresql）
	DB            string     `json:"db"`              // 归属库，用作产物文件名前缀
	SchemaName    string     `json:"schema_name"`     // schema 名
	TableName     string     `json:"table_name"`      // 表名
	FlagEnable    string     `json:"flag_enable"`     // 启用开关：1 启用 / 0 停用
	Priority      int        `json:"priority"`        // 执行顺序，越大越先
	SliceMode     string     `json:"slice_mode"`      // 分片维度：date/week/month/column/expr
	SliceKeySQL   *string    `json:"slice_key_sql"`   // 分片枚举 SQL（空则按 slice_mode 用内置模板）
	SliceWhereTpl string     `json:"slice_where_tpl"` // where 模板，占位 {value}/{start}/{end}
	SliceFileTpl  string     `json:"slice_file_tpl"`  // 产物文件名模板，占位 {key}
	BakScope      string     `json:"bak_scope"`       // 备份范围：data / both / schema
	Compression   string     `json:"compression"`     // 压缩方式：none / gzip / zstd
	MinRows       int64      `json:"min_rows"`        // 分片行数下限，低于该值跳过
	MaxSlices     int        `json:"max_slices"`      // 分片数上限，超过则拒绝执行
	RedoSlices    int        `json:"redo_slices"`     // 末尾 N 个分片强制重做
	FlagUpload    string     `json:"flag_upload"`     // 上传开关：1 上传 / 0 仅本地
	Remark        *string    `json:"remark"`          // 备注
	Creator       *string    `json:"creator"`         // 创建人（sys_user.uid）
	GmtCreate     *time.Time `json:"gmt_create"`      // 创建时间
	GmtUpdate     *time.Time `json:"gmt_update"`      // 修改时间
}

// Slice 一个分片：键值 + 可选区间 + 行数。
type Slice struct {
	Key   string `json:"key"`   // 分片键（日期 20261018、周 2026W43、月 202610、列值等）
	Start string `json:"start"` // 区间起点（区间分片用；为空表示按 key 等值切分）
	End   string `json:"end"`   // 区间终点
	Rows  int64  `json:"rows"`  // 该分片在当前库中的行数
}

// SlicePlanItem 分片计划项（供页面展示完成度）。
type SlicePlanItem struct {
	Key      string `json:"key"`      // 分片键
	Rows     int64  `json:"rows"`     // 分片行数
	Backuped bool   `json:"backuped"` // 产物是否已存在（重做窗口内算未完成）
}

// SlicePlan 一张表的完整分片计划。
type SlicePlan struct {
	RuleID   int64           `json:"rule_id"`   // 登记主键
	Table    string          `json:"table"`     // db.schema.table
	Total    int             `json:"total"`     // 分片总数
	Done     int             `json:"done"`      // 已备份分片数
	Missing  int             `json:"missing"`   // 待备份分片数
	Skipped  int             `json:"skipped"`   // 行数低于 min_rows 而跳过的分片数
	RedoFrom int             `json:"redo_from"` // 重做窗口起点（该下标之后的分片每轮重做）
	Items    []SlicePlanItem `json:"items"`     // 明细（最多前 N 条）
}

// Table 返回 db.schema.table 形式的全名。
func (r SliceRule) Table() string {
	return r.DB + "." + r.SchemaName + "." + r.TableName
}

// LoadSliceRules 读取启用的大表分片备份登记（priority 倒序）。
func LoadSliceRules(ctx context.Context, pool *pgxpool.Pool) ([]SliceRule, error) {
	const query = `
SELECT idn, rdb_type, db, schema_name, table_name, flag_enable, priority,
       slice_mode, slice_key_sql, slice_where_tpl, slice_file_tpl,
       bak_scope, compression, min_rows, max_slices, redo_slices, flag_upload,
       remark, creator, gmt_create, gmt_update
  FROM brs.brs_bak_rdb_table_slice
 WHERE rdb_type = 'postgresql' AND flag_enable = '1'
 ORDER BY priority DESC, idn ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]SliceRule, 0, 4)
	for rows.Next() {
		var rule SliceRule
		if err := rows.Scan(&rule.IDN, &rule.RdbType, &rule.DB, &rule.SchemaName, &rule.TableName,
			&rule.FlagEnable, &rule.Priority, &rule.SliceMode, &rule.SliceKeySQL, &rule.SliceWhereTpl,
			&rule.SliceFileTpl, &rule.BakScope, &rule.Compression, &rule.MinRows, &rule.MaxSlices,
			&rule.RedoSlices, &rule.FlagUpload, &rule.Remark, &rule.Creator,
			&rule.GmtCreate, &rule.GmtUpdate); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// keySQL 返回分片枚举 SQL：未配置 slice_key_sql 时按 slice_mode 使用内置模板。
//
// 约定返回值固定 4 列：(分片键 text, 区间起点 text, 区间终点 text, 行数 bigint)。
// 注意别名不要用 key/rows 等关键字（PG 里 ROWS 是保留字）。
// start/end 供 where 模板的 {start}/{end} 使用；区间分片（周/月）必须给出区间，
// 这样 --where 才能写成 date >= start AND date <= end 走索引。
func (r SliceRule) keySQL() (string, error) {
	if r.SliceKeySQL != nil && strings.TrimSpace(*r.SliceKeySQL) != "" {
		return strings.TrimSpace(*r.SliceKeySQL), nil
	}
	table := r.SchemaName + "." + r.TableName
	switch r.SliceMode {
	case SliceModeDate:
		return fmt.Sprintf(`SELECT date::text AS slice_key, date::text AS slice_start, date::text AS slice_end, count(*) AS slice_rows
  FROM %s WHERE date IS NOT NULL GROUP BY date ORDER BY date`, table), nil
	case SliceModeWeek:
		return fmt.Sprintf(`SELECT to_char(to_date(date::text, 'YYYYMMDD'), 'IYYY"W"IW') AS slice_key,
       min(date)::text AS slice_start, max(date)::text AS slice_end, count(*) AS slice_rows
  FROM %s WHERE date IS NOT NULL GROUP BY 1 ORDER BY 1`, table), nil
	case SliceModeMonth:
		return fmt.Sprintf(`SELECT substr(date::text, 1, 6) AS slice_key,
       min(date)::text AS slice_start, max(date)::text AS slice_end, count(*) AS slice_rows
  FROM %s WHERE date IS NOT NULL GROUP BY 1 ORDER BY 1`, table), nil
	case SliceModeColumn:
		return "", fmt.Errorf("slice_mode=column 必须配置 slice_key_sql（返回 key/start/end/rows 四列）")
	case SliceModeExpr:
		return "", fmt.Errorf("slice_mode=expr 必须配置 slice_key_sql（返回 key/start/end/rows 四列）")
	default:
		return "", fmt.Errorf("未知 slice_mode：%s", r.SliceMode)
	}
}

// ListSlices 枚举分片（含每个分片的行数）。
func ListSlices(ctx context.Context, pool *pgxpool.Pool, rule SliceRule) ([]Slice, error) {
	query, err := rule.keySQL()
	if err != nil {
		return nil, err
	}
	if strings.Contains(query, ";") {
		return nil, fmt.Errorf("分片枚举 SQL 不允许包含分号")
	}
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("枚举分片失败: %w", err)
	}
	defer rows.Close()

	slices := make([]Slice, 0, 64)
	for rows.Next() {
		var item Slice
		var key, start, end *string
		if err := rows.Scan(&key, &start, &end, &item.Rows); err != nil {
			return nil, fmt.Errorf("解析分片结果失败: %w", err)
		}
		if key == nil {
			// 分片键为空的行无法归入任何分片（内置模板已排除；自定义 SQL 可能返回 NULL）。
			continue
		}
		item.Key = *key
		if start != nil {
			item.Start = *start
		}
		if end != nil {
			item.End = *end
		}
		slices = append(slices, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(slices) > rule.MaxSlices {
		return nil, fmt.Errorf("分片数 %d 超过上限 %d（检查 slice_key_sql 或调整 max_slices）",
			len(slices), rule.MaxSlices)
	}
	return slices, nil
}

// whereClause 渲染 pg_dump --where 条件。
func (r SliceRule) whereClause(s Slice) (string, error) {
	tpl := strings.TrimSpace(r.SliceWhereTpl)
	if tpl == "" {
		return "", fmt.Errorf("slice_where_tpl 为空")
	}
	if (strings.Contains(tpl, "{start}") || strings.Contains(tpl, "{end}")) && (s.Start == "" || s.End == "") {
		return "", fmt.Errorf("slice_where_tpl 使用了 {start}/{end}，但分片 %q 未返回区间值", s.Key)
	}
	return strings.NewReplacer(
		"{value}", s.Key,
		"{start}", s.Start,
		"{end}", s.End,
	).Replace(tpl), nil
}

// fileKey 渲染产物文件名（不含扩展名），只保留安全字符。
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (r SliceRule) fileKey(s Slice) string {
	tpl := strings.TrimSpace(r.SliceFileTpl)
	if tpl == "" {
		tpl = "{key}"
	}
	name := strings.ReplaceAll(tpl, "{key}", s.Key)
	name = unsafeFileChars.ReplaceAllString(strings.TrimSpace(name), "_")
	name = strings.Trim(name, "._-")
	if name == "" {
		name = "slice"
	}
	return name
}

// fileExt 返回产物扩展名（MVSV-2 文件，gzip 压缩时带 .gz）。
func (r SliceRule) fileExt() string {
	if strings.EqualFold(strings.TrimSpace(r.Compression), CompressionNone) {
		return ".mvsv"
	}
	return ".mvsv.gz"
}

// tableDir 返回该表分片产物的子目录名：<db>.<schema>.<table>。
func (r SliceRule) tableDir() string {
	return fmt.Sprintf("%s.%s.%s", r.DB, r.SchemaName, r.TableName)
}

// localPath 本地产物路径：<BackupDir>/Slice/<db>.<schema>.<table>/<分片键>.mvsv[.gz]。
func (r SliceRule) localPath(backupDir string, s Slice) string {
	return filepath.Join(backupDir, SliceDirName, r.tableDir(), r.fileKey(s)+r.fileExt())
}

// remoteKey 远端 key：<env>/<db>.<schema>.<table>/<分片键>.mvsv[.gz]（prefix 由适配器拼接）。
func (r SliceRule) remoteKey(env string, s Slice) string {
	key := r.tableDir() + "/" + r.fileKey(s) + r.fileExt()
	if env != "" {
		key = env + "/" + key
	}
	return key
}

// RunSliceOnceWithHooks 执行一轮大表分片备份：逐规则、逐分片导出并上传。
//
// 增量判定不依赖状态表：分片键由库枚举，本地文件或远端对象已存在即跳过；
// 末尾 redo_slices 个分片强制重做（最近的日期数据可能仍在增长）。
func RunSliceOnceWithHooks(ctx context.Context, pool *pgxpool.Pool, opts Options, hooks Hooks) error {
	rules, err := LoadSliceRules(ctx, pool)
	if err != nil {
		hooks.emit("error", "读取大表分片备份登记失败: %v", err)
		return fmt.Errorf("读取大表分片备份登记失败: %w", err)
	}
	if len(rules) == 0 {
		hooks.emit("warn", "没有启用的大表分片备份登记，跳过")
		return nil
	}

	store, env := resolveStoreAndEnv(ctx, pool, opts, hooks)

	var failed, total, done int
	for _, rule := range rules {
		slices, err := ListSlices(ctx, pool, rule)
		if err != nil {
			failed++
			hooks.emit("error", "枚举分片失败 %s: %v", rule.Table(), err)
			continue
		}
		hooks.emit("info", "%s：slice_mode=%s，共 %d 个分片", rule.Table(), rule.SliceMode, len(slices))

		dir := filepath.Join(opts.BackupDir, SliceDirName, rule.tableDir())
		if err := os.MkdirAll(dir, 0o750); err != nil {
			failed++
			hooks.emit("error", "创建分片目录失败 %s: %v；请检查 FINV_BRS_BACKUP_DIR 可写", dir, err)
			continue
		}

		redoFrom := len(slices) - rule.RedoSlices
		if redoFrom < 0 {
			redoFrom = 0
		}
		for i, slice := range slices {
			if slice.Rows < rule.MinRows {
				hooks.emit("info", "跳过 %s 分片 %s（%d 行 < min_rows=%d）", rule.Table(), slice.Key, slice.Rows, rule.MinRows)
				continue
			}
			total++
			// 增量：已存在且不在重做窗口内 → 跳过。
			if i < redoFrom {
				if exists, size := sliceExists(ctx, store, rule, opts, env, slice); exists {
					hooks.emit("info", "已备份，跳过 %s 分片 %s（%s）", rule.Table(), slice.Key, humanSize(size))
					continue
				}
			}
			if hooks.OnSliceProgress != nil {
				hooks.OnSliceProgress(rule, slice, done, total)
			}
			path, size, err := dumpSlice(ctx, pool, opts, rule, slice, hooks)
			if err != nil {
				failed++
				hooks.emit("error", "备份失败 %s 分片 %s: %v", rule.Table(), slice.Key, err)
				continue
			}
			done++
			if store != nil && rule.FlagUpload == "1" {
				if err := uploadSlice(ctx, store, rule, env, slice, path, hooks); err != nil {
					hooks.emit("warn", "上传失败 %s 分片 %s: %v（本地文件保留）", rule.Table(), slice.Key, err)
				}
			} else if store == nil {
				// store 为空时已在 resolveStoreAndEnv 里给出原因
			} else {
				hooks.emit("info", "%s 分片 %s 已关闭上传（flag_upload=0），仅保留本地文件", rule.Table(), slice.Key)
			}
			_ = size
		}
	}
	if failed > 0 {
		hooks.emit("error", "本轮分片备份结束：%d 个失败", failed)
		return fmt.Errorf("%d 个分片备份失败", failed)
	}
	hooks.emit("info", "本轮分片备份完成：处理 %d 个分片", done)
	return nil
}

// sliceExists 判断分片产物是否已存在（先本地文件，再远端对象）。
func sliceExists(ctx context.Context, store oss.Store, rule SliceRule, opts Options, env string, slice Slice) (bool, int64) {
	if info, err := os.Stat(rule.localPath(opts.BackupDir, slice)); err == nil {
		return true, info.Size()
	}
	if store != nil && rule.FlagUpload == "1" {
		if info, err := store.Stat(ctx, rule.remoteKey(env, slice)); err == nil {
			return true, info.Size
		}
	}
	return false, 0
}

// sliceColumns MVSV-2 的 19 列（与 internal/meta/finv/quote/mvsv.V2Columns(true) 一致）。
//
// 分片导出复用 qiom export 的列清单与类型转换：numeric 列统一 ::text，避免精度丢失。
const sliceColumns = "usc, ts, date, \"time\", open::text, high::text, low::text, close::text, " +
	"last_close::text, volume::text, turnover::text, change_price::text, change_ratio::text, vwap::text, " +
	"ext_field, remark, provider, gmt_create::text, gmt_update::text"

// dumpSlice 导出单个分片为 MVSV-2 文件（可 gzip），返回本地路径与文件大小。
//
// 不用 pg_dump：pg_dump 没有行级过滤选项（--where 不存在），无法按分片截取数据。
// 用只读 RepeatableRead 事务保证「统计行数」与「导出数据」来自同一快照，
// DumpImporter/WriteV2 会校验实际写入行数与统计值一致。
func dumpSlice(ctx context.Context, pool *pgxpool.Pool, opts Options, rule SliceRule, slice Slice, hooks Hooks) (string, int64, error) {
	where, err := rule.whereClause(slice)
	if err != nil {
		return "", 0, err
	}
	table := rule.SchemaName + "." + rule.TableName
	path := rule.localPath(opts.BackupDir, slice)

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return "", 0, fmt.Errorf("开启导出事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var count int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE "+where).Scan(&count); err != nil {
		return "", 0, fmt.Errorf("统计分片行数失败: %w", err)
	}

	rows, err := tx.Query(ctx, "SELECT "+sliceColumns+" FROM "+table+" WHERE "+where+" ORDER BY usc, ts")
	if err != nil {
		return "", 0, fmt.Errorf("查询分片数据失败: %w", err)
	}
	defer rows.Close()

	compression := mvsv.CompressionGzip
	if strings.EqualFold(strings.TrimSpace(rule.Compression), CompressionNone) {
		compression = mvsv.CompressionNone
	}

	start := time.Now()
	title := fmt.Sprintf("%s_%s", rule.TableName, rule.fileKey(slice))
	_, err = mvsv.WriteV2(path, mvsv.V2Options{
		Title:      title,
		DataScope:  "where: " + where,
		IncludeGmt: true,
	}, compression, count, func(yield func(mvsv.Row) error) error {
		for rows.Next() {
			var row mvsv.Row
			var gmtCreate, gmtUpdate *string
			if err := rows.Scan(
				&row.Usc, &row.Ts, &row.Date, &row.Time,
				&row.Open, &row.High, &row.Low, &row.Close, &row.LastClose,
				&row.Volume, &row.Turnover, &row.ChangePrice, &row.ChangeRatio, &row.Vwap,
				&row.ExtField, &row.Remark, &row.Provider,
				&gmtCreate, &gmtUpdate,
			); err != nil {
				return err
			}
			row.GmtCreate, row.GmtUpdate = gmtCreate, gmtUpdate
			if err := yield(row); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		_ = os.Remove(path)
		return "", 0, fmt.Errorf("写出 MVSV-2 分片失败: %w", err)
	}

	info, statErr := os.Stat(path)
	var size int64
	if statErr == nil {
		size = info.Size()
	}
	hooks.emit("info", "备份成功 %s 分片 %s（%d 行，%s，耗时 %s）",
		rule.Table(), slice.Key, count, humanSize(size), time.Since(start).Round(time.Millisecond))
	return path, size, nil
}

// uploadSlice 上传单个分片产物。
func uploadSlice(ctx context.Context, store oss.Store, rule SliceRule, env string, slice Slice, path string, hooks Hooks) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	key := rule.remoteKey(env, slice)
	if err := store.Put(ctx, key, file, info.Size(), "application/octet-stream"); err != nil {
		return err
	}
	hooks.emit("info", "已上传 %s", key)
	return nil
}
