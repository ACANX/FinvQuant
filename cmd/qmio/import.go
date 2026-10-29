package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/mvsv"
)

const targetTable = "cdeu.finv_quote_secu_kline_min"

func runImport(args []string) int {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	mode := fs.String("mode", "upsert", "导入模式：upsert / append / replace")
	force := fs.Bool("force", false, "replace 模式必须显式确认")
	batchSize := fs.Int("batch", 1000, "每批行数")
	skipVerify := fs.Bool("skip-verify", false, "跳过 .sha256 校验")
	pgURL := fs.String("pg-url", "", "数据库连接串")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "用法: qmio import FILE [--mode upsert|append|replace] [--force]")
		return 2
	}
	path := fs.Arg(0)
	switch *mode {
	case "upsert", "append", "replace":
	default:
		fmt.Fprintf(os.Stderr, "非法 mode: %s\n", *mode)
		return 2
	}
	if *mode == "replace" && !*force {
		fmt.Fprintln(os.Stderr, "replace 会清空目标表，必须显式加 --force")
		return 2
	}
	if *batchSize <= 0 {
		*batchSize = 1000
	}

	if !*skipVerify {
		if err := verifyChecksum(path); err != nil {
			fmt.Fprintf(os.Stderr, "校验失败: %v（可用 --skip-verify 跳过）\n", err)
			return 1
		}
	}

	reader, closer, err := openMaybeGzip(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		return 1
	}
	defer closer.Close()

	ctx := context.Background()
	pool, err := openPool(ctx, *pgURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		return 1
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "开启事务失败: %v\n", err)
		return 1
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if *mode == "replace" {
		var before int64
		_ = tx.QueryRow(ctx, "SELECT count(*) FROM "+targetTable).Scan(&before)
		fmt.Printf("replace 模式：清空 %s（当前 %d 行）\n", targetTable, before)
		if _, err := tx.Exec(ctx, "TRUNCATE "+targetTable); err != nil {
			fmt.Fprintf(os.Stderr, "清空目标表失败: %v\n", err)
			return 1
		}
	}

	var columns []string
	var batch []mvsv.Row
	var affected int64
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		sql := buildInsertSQL(columns, len(batch), *mode)
		args := make([]any, 0, len(batch)*len(columns))
		for _, row := range batch {
			for _, column := range columns {
				args = append(args, rowValue(row, column))
			}
		}
		tag, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return err
		}
		affected += tag.RowsAffected()
		batch = batch[:0]
		return nil
	}

	header, err := mvsv.ReadAll(reader, path, func(h *mvsv.Header, row mvsv.Row) error {
		if columns == nil {
			if h.Format != mvsv.FormatV2 {
				return fmt.Errorf("仅支持 MVSV-2，实际 %s", h.Format)
			}
			columns = append([]string(nil), h.Columns...)
		}
		batch = append(batch, row)
		if len(batch) >= *batchSize {
			return flush()
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析/导入失败: %v\n", err)
		return 1
	}
	if err := flush(); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		return 1
	}
	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "提交失败: %v\n", err)
		return 1
	}
	fmt.Printf("导入完成：%s（format=%s，%d 行，影响 %d 行，mode=%s）\n",
		path, header.Format, header.Count, affected, *mode)
	return 0
}

func buildInsertSQL(columns []string, rows int, mode string) string {
	placeholders := make([]string, 0, rows)
	index := 1
	for i := 0; i < rows; i++ {
		parts := make([]string, 0, len(columns))
		for range columns {
			parts = append(parts, fmt.Sprintf("$%d", index))
			index++
		}
		placeholders = append(placeholders, "("+strings.Join(parts, ",")+")")
	}
	sql := "INSERT INTO " + targetTable + " (" + strings.Join(columns, ",") + ") VALUES " +
		strings.Join(placeholders, ",")
	switch mode {
	case "append":
		sql += " ON CONFLICT (usc, ts) DO NOTHING"
	case "replace":
		// 已 TRUNCATE，无需冲突处理
	case "upsert":
		sets := make([]string, 0, len(columns))
		hasGmt := false
		for _, column := range columns {
			if column == "usc" || column == "ts" {
				continue
			}
			sets = append(sets, column+"=EXCLUDED."+column)
			if column == "gmt_update" {
				hasGmt = true
			}
		}
		if !hasGmt {
			sets = append(sets, "gmt_update=now()")
		}
		sql += " ON CONFLICT (usc, ts) DO UPDATE SET " + strings.Join(sets, ",")
	}
	return sql
}

func rowValue(row mvsv.Row, column string) any {
	switch column {
	case "usc":
		return row.Usc
	case "ts":
		return row.Ts
	case "date":
		return intValue(row.Date)
	case "time":
		return intValue(row.Time)
	case "open":
		return textValue(row.Open)
	case "high":
		return textValue(row.High)
	case "low":
		return textValue(row.Low)
	case "close":
		return textValue(row.Close)
	case "last_close":
		return textValue(row.LastClose)
	case "volume":
		return textValue(row.Volume)
	case "turnover":
		return textValue(row.Turnover)
	case "change_price":
		return textValue(row.ChangePrice)
	case "change_ratio":
		return textValue(row.ChangeRatio)
	case "paocd":
		return textValue(row.Paocd)
	case "ext_field":
		return textValue(row.ExtField)
	case "remark":
		return textValue(row.Remark)
	case "provider":
		return textValue(row.Provider)
	case "gmt_create":
		return timeValue(row.GmtCreate)
	case "gmt_update":
		return timeValue(row.GmtUpdate)
	}
	return nil
}

func intValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func textValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func timeValue(value *string) any {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(*value)
	for _, layout := range []string{
		time.RFC3339Nano, time.RFC3339,
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return nil
}

// openMaybeGzip 按魔数自动识别 gzip。
func openMaybeGzip(path string) (io.Reader, io.Closer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	buffered := bufio.NewReader(file)
	magic, _ := buffered.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			_ = file.Close()
			return nil, nil, err
		}
		return gz, multiCloser{gz, file}, nil
	}
	return buffered, file, nil
}

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var first error
	for _, closer := range m {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func verifyChecksum(path string) error {
	checksumPath := checksumFile(path)
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 无校验文件时跳过
		}
		return err
	}
	expected := strings.Fields(string(data))
	if len(expected) == 0 {
		return fmt.Errorf("校验文件为空")
	}
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected[0], actual) {
		return fmt.Errorf("sha256 不一致（期望 %s，实际 %s）", expected[0], actual)
	}
	return nil
}
