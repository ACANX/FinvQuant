// Package brs 提供数据库备份服务（Backup & Restore Service）的首期实现：
//
//   - 从 brs.brs_bak_rdb_table_full 读取需要备份的表（flag_enable='1'，priority 倒序）；
//   - 按 sys.sys_config 39001 的 6 段 cron（秒 分 时 日 月 周）每日触发；
//   - 对每张表执行 pg_dump（--table + -Fc + 按 compression 压缩）；
//   - 备份完成后可选上传对象存储（oss.Store 接口，首期为 nil 不上传）；
//   - 备份过程写服务日志（暂不落库）。
package brs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/base/cron"
	"github.com/acanx/finvquant/internal/meta/base/oss"
	// 对象存储适配器通过 init() 自行注册，必须 blank import 才会编译进二进制。
	// 早前没有引用导致 oss.New 报「未注册的对象存储 provider」，上传永远不可用。
	_ "github.com/acanx/finvquant/internal/meta/base/oss/local"
	_ "github.com/acanx/finvquant/internal/meta/base/oss/s3"
)

// CronConfigNo 每日备份 cron 的系统配置编号（6 段）。
const CronConfigNo = 39001

// EnvConfigNo 运行环境标识的系统配置编号（str_value，data_type='3'）。
// 上传到对象存储时作为 key 的一级目录：<prefix>/<env>/<yyyyMMdd>/<schema>.<table>.dump。
const EnvConfigNo = 1002

// defaultBackupDir 未配置 FINV_BRS_BACKUP_DIR 时的兜底备份根目录。
//
// 取 Native 安装根目录下的 BRS（<PREFIX>/BRS），**不放 <PREFIX>/Backup 下**：
// Backup 是数据库逻辑备份目录（Upgrade.sh 写入，root 0705/0750），BRS 需要运行用户
// 可写，两者混在同一个父目录里容易出现属主/权限互相干扰（2026-11-01 线上曾因
// /data/App 不可写导致 mkdir 失败，需求 10893/10901 的历史包袱）。
// 容器部署由 docker-compose 显式传 FINV_BRS_BACKUP_DIR，不受本默认值影响。
const defaultBackupDir = "/data/App/FinvQuant/BRS"

// Options 备份执行参数。
type Options struct {
	BackupDir string // 备份根目录（默认 <PREFIX>/Backup/BRS）
	Host      string
	Port      string
	User      string
	Password  string
	Database  string
	DumpBin   string // pg_dump 路径（默认 PATH 中查找）
	// LocalKeepDays 本地全量 dump 保留天数（<=0 表示不清理）；仅删除远端已存在的文件。
	LocalKeepDays int
	Store         oss.Store // 可选：对象存储上传实现（nil 不上传）
}

// Table 一条备份登记。
type Table struct {
	IDN         int64
	DB          string // 登记归属库（brs_bak_rdb_table_full.db），用作备份文件名前缀
	SchemaName  string
	TableName   string
	Priority    int
	BakScope    string
	BakMethod   string
	Compression string
	WhereClause string
	FlagUpload  string
}

// LoadTables 读取启用的备份登记（priority 倒序）。
func LoadTables(ctx context.Context, pool *pgxpool.Pool) ([]Table, error) {
	const query = `
SELECT idn, db, schema_name, table_name, priority, bak_scope, bak_method, compression,
       COALESCE(where_clause, ''), flag_upload
  FROM brs.brs_bak_rdb_table_full
 WHERE rdb_type = 'postgresql' AND flag_enable = '1'
 ORDER BY priority DESC, idn ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]Table, 0, 16)
	for rows.Next() {
		var table Table
		if err := rows.Scan(&table.IDN, &table.DB, &table.SchemaName, &table.TableName, &table.Priority,
			&table.BakScope, &table.BakMethod, &table.Compression, &table.WhereClause, &table.FlagUpload); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

// loadCronExpr 读取 39001 的 cron 表达式。
func loadCronExpr(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var expr string
	err := pool.QueryRow(ctx, `SELECT str_value FROM sys.sys_config WHERE config_no = $1`, CronConfigNo).Scan(&expr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(expr), nil
}

// Start 启动每日备份调度（阻塞直到 ctx 取消）。建议在独立 goroutine 中运行。
//
// 每秒检查一次 39001 的 6 段 cron 是否命中；cron 每 60 秒重新读取一次，
// 修改配置后最多 60 秒生效。非法表达式只记日志并跳过。
func Start(ctx context.Context, pool *pgxpool.Pool, opts Options) {
	if strings.TrimSpace(opts.BackupDir) == "" {
		opts.BackupDir = defaultBackupDir
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var (
		mu        sync.Mutex
		lastRead  time.Time
		schedule  *cron.Schedule
		exprCache string
		running   bool
	)
	getSchedule := func(now time.Time) *cron.Schedule {
		mu.Lock()
		defer mu.Unlock()
		if now.Sub(lastRead) < time.Minute && schedule != nil {
			return schedule
		}
		lastRead = now
		expr, err := loadCronExpr(ctx, pool)
		if err != nil {
			log.Printf("[brs] 读取 39001 cron 失败: %v", err)
			return schedule
		}
		if expr == exprCache && schedule != nil {
			return schedule
		}
		parsed, err := cron.Parse(expr)
		if err != nil {
			log.Printf("[brs] 39001 cron 非法（%q）: %v", expr, err)
			return schedule
		}
		exprCache = expr
		schedule = parsed
		log.Printf("[brs] 每日备份 cron 已加载: %q", expr)
		return schedule
	}

	log.Printf("[brs] 备份服务已启动（目录 %s，cron 配置 39001）", opts.BackupDir)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[brs] 备份服务停止")
			return
		case now := <-ticker.C:
			current := getSchedule(now)
			if current == nil || !current.Matches(now) {
				continue
			}
			mu.Lock()
			if running {
				mu.Unlock()
				continue
			}
			running = true
			mu.Unlock()

			go func() {
				defer func() {
					mu.Lock()
					running = false
					mu.Unlock()
				}()
				if err := RunOnce(ctx, pool, opts); err != nil {
					log.Printf("[brs] 本轮备份失败: %v", err)
				}
			}()
		}
	}
}

// Hooks 备份运行期回调：把进度与日志同时投递给内存状态（手动备份页面的实时展示）。
//
// 自动调度（cron）与手动触发共用同一套 Hooks，页面看到的行为一致。
type Hooks struct {
	// OnLog 每产生一条运行日志时调用；level 取 info / warn / error。
	OnLog func(level, message string)
	// OnProgress 每张表开始备份前调用：index 从 1 开始，done 为已完成张数。
	OnProgress func(index, done, total int, table Table)
	// OnSliceProgress 大表分片备份时每个分片开始前调用（done 为已完成分片数）。
	OnSliceProgress func(rule SliceRule, slice Slice, done, total int)
}

// emit 同时写服务日志（保持 [brs] 前缀不变）与运行期回调。
func (h Hooks) emit(level, format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	log.Printf("[brs] %s", message)
	if h.OnLog != nil {
		h.OnLog(level, message)
	}
}

// resolveStoreAndEnv 解析本次运行使用的对象存储与环境目录。
//
// 上传 key 的一级目录取 1002 的运行环境标识（需求 10897）：
// <prefix>/<env>/... 。环境标识缺失或非法时**不上传**，避免不同环境的备份混到同一路径下；
// 39002 未启用或配置无效时同样不上传（仅本地备份）。
func resolveStoreAndEnv(ctx context.Context, pool *pgxpool.Pool, opts Options, hooks Hooks) (oss.Store, string) {
	store := opts.Store
	if store == nil {
		loaded, err := loadOssStore(ctx, pool)
		if err != nil {
			hooks.emit("warn", "对象存储配置无效，跳过上传: %v", err)
		} else {
			store = loaded
			if store == nil {
				hooks.emit("info", "未启用对象存储上传（39002 enabled=false），仅本地备份")
			}
		}
	}
	if store == nil {
		return nil, ""
	}

	loaded, err := loadRuntimeEnv(ctx, pool)
	if err != nil {
		hooks.emit("warn", "读取运行环境标识（sys_config %d）失败，跳过上传: %v", EnvConfigNo, err)
		return nil, ""
	}
	if loaded == "" {
		hooks.emit("warn", "未配置运行环境标识（sys_config %d），跳过对象存储上传；本地备份保留", EnvConfigNo)
		return nil, ""
	}
	if !validEnvName(loaded) {
		hooks.emit("warn", "运行环境标识 %q 非法（只允许 2~32 位字母/数字/下划线/短横线），跳过上传", loaded)
		return nil, ""
	}
	hooks.emit("info", "上传环境目录：%s", loaded)
	return store, loaded
}

// loadRuntimeEnv 读取 1002 的运行环境标识（缺失返回空串，不报错）。
func loadRuntimeEnv(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var env string
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = $1`, EnvConfigNo).Scan(&env)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(env), nil
}

// validEnvName 校验环境标识可作为单级路径使用：2~32 位字母/数字/下划线/短横线。
func validEnvName(env string) bool {
	if len(env) < 2 || len(env) > 32 {
		return false
	}
	for _, r := range env {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// RunOnce 执行一轮备份（无进度回调）。
func RunOnce(ctx context.Context, pool *pgxpool.Pool, opts Options) error {
	return RunOnceWithHooks(ctx, pool, opts, Hooks{})
}

// RunOnceWithHooks 执行一轮备份：按 priority 倒序逐表 pg_dump，单表失败继续下一张。
func RunOnceWithHooks(ctx context.Context, pool *pgxpool.Pool, opts Options, hooks Hooks) error {
	tables, err := LoadTables(ctx, pool)
	if err != nil {
		hooks.emit("error", "读取备份登记失败: %v", err)
		return fmt.Errorf("读取备份登记失败: %w", err)
	}
	if len(tables) == 0 {
		hooks.emit("warn", "没有启用的备份表，跳过")
		return nil
	}

	day := time.Now().Format("20060102")
	dayDir := filepath.Join(opts.BackupDir, day)
	if err := os.MkdirAll(dayDir, 0o750); err != nil {
		hooks.emit("error", "创建备份目录失败: %v；请检查 FINV_BRS_BACKUP_DIR（当前 %s）是否存在且运行用户可写", err, opts.BackupDir)
		return fmt.Errorf("创建备份目录失败: %w（FINV_BRS_BACKUP_DIR=%s，请确认目录可写）", err, opts.BackupDir)
	}

	// 对象存储与环境标识：小表全量与大表分片共用同一套解析（39002 + 1002）。
	store, env := resolveStoreAndEnv(ctx, pool, opts, hooks)

	hooks.emit("info", "开始备份：%d 张表 → %s", len(tables), dayDir)
	var failed int
	for i, table := range tables {
		if hooks.OnProgress != nil {
			hooks.OnProgress(i+1, i, len(tables), table)
		}
		path, err := dumpTable(ctx, opts, dayDir, day, table, hooks)
		if err != nil {
			failed++
			hooks.emit("error", "备份失败 %s.%s（priority=%d）: %v", table.SchemaName, table.TableName, table.Priority, err)
			continue
		}
		if store != nil && table.FlagUpload == "1" {
			if err := upload(ctx, store, env, day, path, hooks); err != nil {
				hooks.emit("warn", "上传失败 %s: %v（本地文件保留）", path, err)
			}
		} else if store != nil {
			hooks.emit("info", "%s.%s 已关闭上传（flag_upload=0），仅保留本地文件", table.SchemaName, table.TableName)
		}
	}
	// 本地产物清理：只删「远端已存在」且超过保留期的 dump，避免目录每天累积。
	if _, err := cleanupLocalDumps(ctx, opts, store, env, opts.LocalKeepDays, hooks); err != nil {
		hooks.emit("warn", "本地清理失败（不影响本轮备份结果）: %v", err)
	}

	if failed > 0 {
		hooks.emit("error", "本轮备份结束：%d/%d 张表失败", failed, len(tables))
		return fmt.Errorf("%d/%d 张表备份失败", failed, len(tables))
	}
	hooks.emit("info", "本轮备份完成：%d 张表", len(tables))
	return nil
}

// dumpTable 对单表执行 pg_dump，返回文件路径。
func dumpTable(ctx context.Context, opts Options, dayDir, day string, table Table, hooks Hooks) (string, error) {
	if strings.TrimSpace(table.WhereClause) != "" {
		hooks.emit("warn", "%s.%s 配置了 where_clause，但 pg_dump 模式暂不支持，将备份整表",
			table.SchemaName, table.TableName)
	}

	dumpBin := strings.TrimSpace(opts.DumpBin)
	if dumpBin == "" {
		dumpBin = "pg_dump"
	}
	// 文件名格式 <db>.<schema>.<table>.dump（需求 10898）：
	// 加库名前缀，便于对象存储上区分不同库的同名表；不带时间戳（需求 10893），
	// 同一张表当天重复备份会覆盖上一次，属预期行为。
	dbName := strings.TrimSpace(table.DB)
	if dbName == "" {
		dbName = strings.TrimSpace(opts.Database)
	}
	fileName := fmt.Sprintf("%s.%s.%s.dump", dbName, table.SchemaName, table.TableName)
	filePath := filepath.Join(dayDir, fileName)

	args := []string{
		"--host", opts.Host,
		"--port", opts.Port,
		"--username", opts.User,
		"--dbname", opts.Database,
		"--table", table.SchemaName + "." + table.TableName,
		"--format", "custom",
		"--compress", compressSpec(table.Compression),
		"--file", filePath,
	}
	switch table.BakScope {
	case "schema":
		args = append(args, "--schema-only")
	case "data":
		args = append(args, "--data-only")
	}

	dumpCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(dumpCtx, dumpBin, args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+opts.Password)
	start := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w（%s）", err, truncateOutput(string(output)))
	}
	info, statErr := os.Stat(filePath)
	size := int64(0)
	if statErr == nil {
		size = info.Size()
	}
	hooks.emit("info", "备份成功 %s.%s（%s，%s，耗时 %s）", table.SchemaName, table.TableName,
		filePath, humanSize(size), time.Since(start).Round(time.Millisecond))
	return filePath, nil
}

// upload 把本地备份文件上传到对象存储。
//
// key 结构：<prefix>/<env>/<yyyyMMdd>/<schema>.<table>.dump
//
//	prefix 由 39002 的 prefix 决定（适配器拼接），env 为 1002 的运行环境标识，
//	day 为本地日期目录。
func upload(ctx context.Context, store oss.Store, env, day, path string, hooks Hooks) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	key := day + "/" + filepath.Base(path)
	if env != "" {
		key = env + "/" + key
	}
	if err := store.Put(ctx, key, file, info.Size(), "application/octet-stream"); err != nil {
		return err
	}
	hooks.emit("info", "已上传 %s", key)
	return nil
}

// compressSpec 把登记表的 compression 转为 pg_dump --compress 参数。
func compressSpec(compression string) string {
	switch strings.ToLower(strings.TrimSpace(compression)) {
	case "none":
		return "none"
	case "gzip":
		return "gzip:6"
	case "zstd", "":
		return "zstd:3"
	default:
		return "zstd:3"
	}
}

func truncateOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) > 500 {
		return output[:500] + "..."
	}
	return output
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// OssSettings 39002 的对象存储配置（JSON，凭证键 ak / sk，见迁移 V300082）。
//
// 兼容 V300082 之前的 access_key / secret_key 旧键。
type OssSettings struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	AK       string `json:"ak"`
	SK       string `json:"sk"`
	Prefix   string `json:"prefix"`
	LegacyAK string `json:"access_key,omitempty"`
	LegacySK string `json:"secret_key,omitempty"`
}

// AccessKey 返回生效的 Access Key（新键优先，旧键兜底）。
func (s OssSettings) AccessKey() string {
	if strings.TrimSpace(s.AK) != "" {
		return s.AK
	}
	return s.LegacyAK
}

// SecretKey 返回生效的 Secret Key（新键优先，旧键兜底）。
func (s OssSettings) SecretKey() string {
	if strings.TrimSpace(s.SK) != "" {
		return s.SK
	}
	return s.LegacySK
}

// loadOssStore 读取 39002 并构造对象存储客户端；未启用/缺配置返回 nil。
func loadOssStore(ctx context.Context, pool *pgxpool.Pool) (oss.Store, error) {
	var raw string
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(json_value::text, '') FROM sys.sys_config WHERE config_no = 39002`).Scan(&raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var settings OssSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("解析 39002 失败: %w", err)
	}
	if !settings.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(settings.Provider) == "" {
		settings.Provider = "s3"
	}
	if strings.TrimSpace(settings.Region) == "" {
		settings.Region = "auto"
	}
	return oss.New(oss.Config{
		Provider:  settings.Provider,
		Endpoint:  settings.Endpoint,
		Bucket:    settings.Bucket,
		Region:    settings.Region,
		AccessKey: settings.AccessKey(),
		SecretKey: settings.SecretKey(),
		Prefix:    settings.Prefix,
	})
}
