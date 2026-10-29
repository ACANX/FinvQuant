package mvsv

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 压缩方式。
const (
	CompressionNone = "none"
	CompressionGzip = "gzip"
)

// WriteResult 文件写出结果。
type WriteResult struct {
	Path     string // 最终文件绝对路径
	Size     int64  // 文件字节数
	RowCount int64  // 实际写入的数据行数
	Checksum string // 文件内容 SHA256（小写十六进制；gzip 时为压缩后内容）
}

// V2Options MVSV-2 文件头配置。
type V2Options struct {
	Title      string
	DataScope  string
	TimeZone   string
	IncludeGmt bool
}

// V2Columns 按 IncludeGmt 返回数据区列名（与 cdeu.finv_quote_secu_kline_min 对齐）。
func V2Columns(includeGmt bool) []string {
	columns := []string{
		"usc", "ts", "date", "time", "open", "high", "low", "close", "last_close",
		"volume", "turnover", "change_price", "change_ratio", "paocd",
		"ext_field", "remark", "provider",
	}
	if includeGmt {
		columns = append(columns, "gmt_create", "gmt_update")
	}
	return columns
}

var v2FieldNames = map[string]string{
	"usc": "Usc", "ts": "Ts", "date": "Date", "time": "Time",
	"open": "Open", "high": "High", "low": "Low", "close": "Close",
	"last_close": "LastClose", "volume": "Volume", "turnover": "Turnover",
	"change_price": "ChangePrice", "change_ratio": "ChangeRatio",
	"paocd": "Paocd", "ext_field": "ExtField", "remark": "Remark",
	"provider": "Provider", "gmt_create": "GmtCreate", "gmt_update": "GmtUpdate",
}

var v2FieldNamesCN = map[string]string{
	"usc": "证券代码", "ts": "时间戳(UTC)", "date": "交易日期", "time": "交易时间",
	"open": "开盘价", "high": "最高价", "low": "最低价", "close": "收盘价",
	"last_close": "上一收盘价", "volume": "成交量", "turnover": "成交额",
	"change_price": "涨跌额", "change_ratio": "涨跌幅", "paocd": "累计均价",
	"ext_field": "扩展字段", "remark": "备注", "provider": "数据来源",
	"gmt_create": "创建时间", "gmt_update": "更新时间",
}

var v2FieldTypes = map[string]string{
	"usc": "String", "ts": "Int", "date": "Int", "time": "Int",
	"open": "Decimal", "high": "Decimal", "low": "Decimal", "close": "Decimal",
	"last_close": "Decimal", "volume": "Decimal", "turnover": "Decimal",
	"change_price": "Decimal", "change_ratio": "Decimal", "paocd": "Decimal",
	"ext_field": "String", "remark": "String", "provider": "String",
	"gmt_create": "Timestamp", "gmt_update": "Timestamp",
}

// WriteV2 流式写出 MVSV-2 文件（单文件全表，含 usc 列）。
//
// 写入流程：临时文件 → 校验实际行数 = expectedRows → 原子 rename；
// 返回文件大小与 SHA256（gzip 时为压缩后内容）。
func WriteV2(dest string, opts V2Options, compression string, expectedRows int64, source func(yield func(Row) error) error) (WriteResult, error) {
	if source == nil {
		return WriteResult{}, fmt.Errorf("导出数据源为空")
	}
	if expectedRows < 0 {
		expectedRows = 0
	}
	if compression != CompressionGzip {
		compression = CompressionNone
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return WriteResult{}, fmt.Errorf("创建导出目录失败: %w", err)
	}
	file, err := os.CreateTemp(dir, filepath.Base(dest)+".tmp-*")
	if err != nil {
		return WriteResult{}, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmp := file.Name()
	defer func() {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}()

	hash := sha256.New()
	var out io.Writer = io.MultiWriter(file, hash)
	var gz *gzip.Writer
	if compression == CompressionGzip {
		gz = gzip.NewWriter(out)
		out = gz
	}

	columns := V2Columns(opts.IncludeGmt)
	if err := writeV2Header(out, opts, len(columns), expectedRows); err != nil {
		return WriteResult{}, err
	}

	var written int64
	yield := func(row Row) error {
		line := renderV2Row(row, opts.IncludeGmt)
		if _, err := io.WriteString(out, line); err != nil {
			return err
		}
		written++
		return nil
	}
	if err := source(yield); err != nil {
		return WriteResult{}, err
	}
	if written != expectedRows {
		return WriteResult{}, fmt.Errorf("实际写入 %d 行，与预期 %d 行不一致", written, expectedRows)
	}

	if gz != nil {
		if err := gz.Close(); err != nil {
			return WriteResult{}, err
		}
	}
	if err := file.Close(); err != nil {
		return WriteResult{}, err
	}
	info, err := os.Stat(tmp)
	if err != nil {
		return WriteResult{}, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return WriteResult{}, err
	}
	tmp = ""
	return WriteResult{
		Path:     dest,
		Size:     info.Size(),
		RowCount: written,
		Checksum: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func writeV2Header(out io.Writer, opts V2Options, columnCount int, expectedRows int64) error {
	columns := V2Columns(opts.IncludeGmt)
	fieldTypes := make([]string, 0, columnCount)
	fieldNames := make([]string, 0, columnCount)
	fieldNamesCN := make([]string, 0, columnCount)
	for _, column := range columns {
		fieldTypes = append(fieldTypes, v2FieldTypes[column])
		fieldNames = append(fieldNames, v2FieldNames[column])
		fieldNamesCN = append(fieldNamesCN, v2FieldNamesCN[column])
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = fmt.Sprintf("CdeuKline_%s", time.Now().Format("20060102_150405"))
	}
	dataScope := strings.TrimSpace(opts.DataScope)
	if dataScope == "" {
		dataScope = "all"
	}
	timezone := strings.TrimSpace(opts.TimeZone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	lines := []string{
		fmt.Sprintf("# Title : %q", title),
		fmt.Sprintf("# Format : %q", FormatV2),
		fmt.Sprintf("# Field : %q", strings.Join(columns, "|")),
		fmt.Sprintf("# FieldType : %q", strings.Join(fieldTypes, "|")),
		fmt.Sprintf("# FieldName : %q", strings.Join(fieldNames, "|")),
		fmt.Sprintf("# 字段名称 : %q", strings.Join(fieldNamesCN, "|")),
		fmt.Sprintf("# Count : %d", expectedRows),
		fmt.Sprintf("# DataScope : %q", dataScope),
		fmt.Sprintf("# ExportTime : %q", time.Now().Format(time.RFC3339)),
		fmt.Sprintf("# TimeZone : %q", timezone),
		`# Region : ""`,
		`# Name : ""`,
		`# Period : "Min"`,
		`# Dsv : "1"`,
		`# Code : ""`,
		`# Exchange : ""`,
		"",
	}
	for _, line := range lines {
		if _, err := io.WriteString(out, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func renderV2Row(row Row, includeGmt bool) string {
	cells := []string{
		row.Usc,
		strconv.FormatInt(row.Ts, 10),
		intText(row.Date),
		intText(row.Time),
		textOf(row.Open),
		textOf(row.High),
		textOf(row.Low),
		textOf(row.Close),
		textOf(row.LastClose),
		textOf(row.Volume),
		textOf(row.Turnover),
		textOf(row.ChangePrice),
		textOf(row.ChangeRatio),
		textOf(row.Paocd),
		textOf(row.ExtField),
		textOf(row.Remark),
		textOf(row.Provider),
	}
	if includeGmt {
		cells = append(cells, textOf(row.GmtCreate), textOf(row.GmtUpdate))
	}
	return strings.Join(cells, "|") + "\n"
}

func intText(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
