package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/acanx/finvquant/internal/mvsv"
)

func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	outDir := fs.String("out", "/data/App/FinvQuant/Export/CdeuKline", "导出目录")
	where := fs.String("where", "", "附加过滤 SQL（可选）")
	noGmt := fs.Bool("no-gmt", false, "不导出 gmt_create/gmt_update")
	noGzip := fs.Bool("no-gzip", false, "不压缩")
	pgURL := fs.String("pg-url", "", "数据库连接串")
	_ = fs.Parse(args)

	ctx := context.Background()
	pool, err := openPool(ctx, *pgURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		return 1
	}
	defer pool.Close()

	whereClause := ""
	if trimmed := normalizeWhere(*where); trimmed != "" {
		whereClause = " WHERE " + trimmed
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "开启事务失败: %v\n", err)
		return 1
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var count int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM cdeu.finv_quote_secu_kline_min"+whereClause).Scan(&count); err != nil {
		fmt.Fprintf(os.Stderr, "统计行数失败: %v\n", err)
		return 1
	}

	columns := "usc, ts, date, \"time\", open::text, high::text, low::text, close::text, last_close::text, " +
		"volume::text, turnover::text, change_price::text, change_ratio::text, paocd::text, " +
		"ext_field, remark, provider, gmt_create::text, gmt_update::text"
	query := "SELECT " + columns + " FROM cdeu.finv_quote_secu_kline_min" + whereClause + " ORDER BY usc, date, ts"
	rows, err := tx.Query(ctx, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", err)
		return 1
	}
	defer rows.Close()

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		return 1
	}
	name := fmt.Sprintf("cdeu_kline_%s.mvsv", time.Now().Format("20060102_150405"))
	compression := mvsv.CompressionGzip
	if *noGzip {
		compression = mvsv.CompressionNone
	} else {
		name += ".gz"
	}
	dest := filepath.Join(*outDir, name)

	_, err = mvsv.WriteV2(dest, mvsv.V2Options{
		Title:      fmt.Sprintf("CdeuKline_%s", time.Now().Format("20060102_150405")),
		DataScope:  normalizeScope(*where),
		IncludeGmt: !*noGmt,
	}, compression, count, func(yield func(mvsv.Row) error) error {
		for rows.Next() {
			var row mvsv.Row
			var gmtCreate, gmtUpdate *string
			if err := rows.Scan(
				&row.Usc, &row.Ts, &row.Date, &row.Time,
				&row.Open, &row.High, &row.Low, &row.Close, &row.LastClose,
				&row.Volume, &row.Turnover, &row.ChangePrice, &row.ChangeRatio, &row.Paocd,
				&row.ExtField, &row.Remark, &row.Provider,
				&gmtCreate, &gmtUpdate,
			); err != nil {
				return err
			}
			if !*noGmt {
				row.GmtCreate, row.GmtUpdate = gmtCreate, gmtUpdate
			}
			if err := yield(row); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "导出失败: %v\n", err)
		return 1
	}

	if err := writeChecksum(dest, checksumFile(dest)); err != nil {
		fmt.Fprintf(os.Stderr, "写入校验文件失败: %v\n", err)
		return 1
	}
	fmt.Printf("导出完成：%s（%d 行）\n", dest, count)
	return 0
}

func normalizeWhere(where string) string {
	return trimSpace(where)
}

func normalizeScope(where string) string {
	if w := trimSpace(where); w != "" {
		return "where: " + w
	}
	return "all"
}
