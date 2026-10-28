// Command qmio 是 FinvQuant 的数据搬运小工具（Quick Market IO）。
//
// 子命令：
//
//	export  导出 cdeu.finv_quote_secu_kline_min 为 MVSV-2（默认 gzip）
//	import  导入 MVSV-2（默认 upsert）
//	verify  校验 MVSV 文件（格式/行数/sha256）
//
// 该二进制与 finvquant 安装在同一目录（<PREFIX>/bin/qmio）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/config"
)

const usage = `qmio — FinvQuant 数据搬运工具

用法:
  qmio export [参数]     导出 cdeu.finv_quote_secu_kline_min 为 MVSV-2
  qmio import FILE [参数] 导入 MVSV-2（默认 upsert）
  qmio verify FILE       校验 MVSV 文件

公共参数:
  --pg-url URL           覆盖数据库连接串（默认读取 FINV_PG_* 环境变量）

示例:
  qmio export --out /data/App/FinvQuant/Export/CdeuKline
  qmio import /data/App/FinvQuant/Export/CdeuKline/cdeu_kline_*.mvsv.gz
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	command := os.Args[1]
	args := os.Args[2:]
	switch command {
	case "export":
		os.Exit(runExport(args))
	case "import":
		os.Exit(runImport(args))
	case "verify":
		os.Exit(runVerify(args))
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n%s", command, usage)
		os.Exit(2)
	}
}

// openPool 建立数据库连接池。
func openPool(ctx context.Context, pgURL string) (*pgxpool.Pool, error) {
	dsn := strings.TrimSpace(pgURL)
	if dsn == "" {
		cfg := config.Load()
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			cfg.PgUser, cfg.PgPassword, cfg.PgHost, cfg.PgPort, cfg.PgDatabase)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("数据库不可达: %w", err)
	}
	return pool, nil
}

// stringFlag 定义可重复使用的字符串参数。
func stringFlag(fs *flag.FlagSet, name, fallback string) *string {
	return fs.String(name, fallback, "")
}
