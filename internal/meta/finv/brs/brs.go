// Package brs 提供数据库备份服务（Backup & Restore Service）的首期实现：
//
//   - 从 brs.brs_bak_rdb_table 读取需要备份的表（flag_enable='1'，priority 倒序）；
//   - 按 sys.sys_config 39001 的 6 段 cron（秒 分 时 日 月 周）每日触发；
//   - 对每张表执行 pg_dump（--table + -Fc + 按 compression 压缩）；
//   - 备份完成后可选上传对象存储（oss.Store 接口，首期为 nil 不上传）；
//   - 备份过程写服务日志（暂不落库）。
package brs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// Options 备份执行参数。
type Options struct {
	BackupDir string // 备份根目录（默认 <PREFIX>/Backup/BRS）
	Host      string
	Port      string
	User      string
	Password  string
	Database  string
	DumpBin   string    // pg_dump 路径（默认 PATH 中查找）
	Store     oss.Store // 可选：对象存储上传实现（nil 不上传）
}

// Table 一条备份登记。
type Table struct {
	IDN         int64
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
SELECT idn, schema_name, table_name, priority, bak_scope, bak_method, compression,
       COALESCE(where_clause, ''), flag_upload
  FROM brs.brs_bak_rdb_table
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
		if err := rows.Scan(&table.IDN, &table.SchemaName, &table.TableName, &table.Priority,
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
		opts.BackupDir = "/data/App/FinvQuant/Backup/BRS"
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
}

// emit 同时写服务日志（保持 [brs] 前缀不变）与运行期回调。
func (h Hooks) emit(level, format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	log.Printf("[brs] %s", message)
	if h.OnLog != nil {
		h.OnLog(level, message)
	}
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
		hooks.emit("error", "创建备份目录失败: %v", err)
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

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
			if err := upload(ctx, store, day, path, hooks); err != nil {
				hooks.emit("warn", "上传失败 %s: %v（本地文件保留）", path, err)
			}
		} else if store != nil {
			hooks.emit("info", "%s.%s 已关闭上传（flag_upload=0），仅保留本地文件", table.SchemaName, table.TableName)
		}
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
	fileName := fmt.Sprintf("%s.%s_%s.dump", table.SchemaName, table.TableName, time.Now().Format("150405"))
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

// upload 把本地备份文件上传到对象存储（key 与本地日期目录同构）。
func upload(ctx context.Context, store oss.Store, day, path string, hooks Hooks) error {
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
