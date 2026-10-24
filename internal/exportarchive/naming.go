package exportarchive

import (
	"fmt"
	"strings"
	"time"
)

// MVsv 文件格式契约常量（与方案稿 4.3 节逐条对应）。
const (
	// MvsvDelimiter 字段分隔符。
	MvsvDelimiter = "|"
	// MvsvFieldLine 文件头 # Field 行固定值（da=date、ti=time，避免与 t=turnover 混淆）。
	MvsvFieldLine = "ts|da|ti|o|h|l|c|v|t|cp|cr|lc"
	// MvsvFieldNameLine 文件头 # FieldName 行固定值。
	MvsvFieldNameLine = "Timestamp|Date|Time|Open|High|Low|Close|Volume|Turnover|ChangePrice|ChangeRatio|LastClose"
	// MvsvFieldTypeLine 文件头 # FieldType 行固定值。
	MvsvFieldTypeLine = "Int|Int|Int|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal"
	// MvsvHeaderLines 文件头条数（不含其后的空行）。
	MvsvHeaderLines = 16
	// MvsvSchemaVersion MVsv 契约版本号（变更列顺序时递增，并在 Manifest 标注）。
	MvsvSchemaVersion = "mvsv-1"
	// MvsvFormat 文件格式标识。
	MvsvFormat = "mvsv"
	// MvsvEncoding 文件编码（UTF-8 无 BOM）。
	MvsvEncoding = "utf-8"
	// CompressionNone 不压缩。
	CompressionNone = "none"
	// CompressionGzip gzip 压缩（扩展名仍为 .mvsv）。
	CompressionGzip = "gzip"
)

// MvsvColumns 数据行 12 列（顺序固定，变更须走版本号并在 Manifest 标注 schema_version）。
var MvsvColumns = []string{"ts", "da", "ti", "o", "h", "l", "c", "v", "t", "cp", "cr", "lc"}

// BaseFileName BASE 文件名：{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}.mvsv。
func (m FileMeta) BaseFileName() string {
	return fmt.Sprintf("%s_%s_%s_Min_%s_Mon_%d.mvsv",
		sanitizeFilePart(m.Region), sanitizeFilePart(m.Market), sanitizeFilePart(m.Usc),
		sanitizeFilePart(m.Provider), m.BizMonth)
}

// PatchFileName PATCH 文件名：{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Incr_{n}.mvsv。
func (m FileMeta) PatchFileName(n int) string {
	return fmt.Sprintf("%s_%s_%s_Min_%s_Mon_%d_Incr_%d.mvsv",
		sanitizeFilePart(m.Region), sanitizeFilePart(m.Market), sanitizeFilePart(m.Usc),
		sanitizeFilePart(m.Provider), m.BizMonth, n)
}

// ManifestFileName MANIFEST 文件名：{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Manifest.json。
func (m FileMeta) ManifestFileName() string {
	return fmt.Sprintf("%s_%s_%s_Min_%s_Mon_%d_Manifest.json",
		sanitizeFilePart(m.Region), sanitizeFilePart(m.Market), sanitizeFilePart(m.Usc),
		sanitizeFilePart(m.Provider), m.BizMonth)
}

// headerLines 返回 16 行文件头（按固定顺序）。
func (m FileMeta) headerLines(rowCount int64) []string {
	return []string{
		fmt.Sprintf("# 标题 : %s 分钟级行情数据", m.Usc),
		`# 字段名称 : "时间戳|日期|时间|开盘价|最高价|最低价|收盘价|成交量|成交额|涨跌值|涨跌幅|前收价"`,
		fmt.Sprintf("# DataProvider : %s", m.Provider),
		fmt.Sprintf("# Field : %q", MvsvFieldLine),
		fmt.Sprintf("# FieldName : %q", MvsvFieldNameLine),
		fmt.Sprintf("# FieldType : %q", MvsvFieldTypeLine),
		"# TypeKLine : Min",
		fmt.Sprintf("# Count : %d", rowCount),
		fmt.Sprintf("# Symbol : %s.%s", m.Market, m.Usc),
		fmt.Sprintf("# SecuCode : %s", m.Usc),
		fmt.Sprintf("# USC : %s", m.Usc),
		fmt.Sprintf("# Name : %s", m.Name),
		fmt.Sprintf("# Region : %s", m.Region),
		fmt.Sprintf("# Market : %s", m.Market),
		fmt.Sprintf("# TimeZone : %s", m.Timezone),
		fmt.Sprintf("# Currency : %s", m.Currency),
	}
}

// sanitizeFilePart 清理文件名片段：去掉路径分隔符与控制字符，避免跨目录写文件。
// 元数据为空时返回 UNKNOWN，保证文件名仍然符合 {region}_{market}_... 的段数约定。
func sanitizeFilePart(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return "UNKNOWN"
	}
	var builder strings.Builder
	for _, char := range part {
		switch char {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t':
			builder.WriteByte('_')
		default:
			if char < 0x20 {
				builder.WriteByte('_')
				continue
			}
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// trimNumeric 输出主表原始精度的十进制文本：去掉多余的尾零与小数点。
//
// 主表 numeric(20,6)/numeric(30,8) 经 ::text 输出为定点小数（如 8.797000），
// MVsv 要求「按主表原始精度输出、不使用科学计数法」，这里仅裁剪无意义的尾零，
// 不改变数值；空值（NULL）输出空串，对应「连续 || 之间无内容」。
func trimNumeric(value *string) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(*value)
	if text == "" {
		return ""
	}
	if !strings.Contains(text, ".") {
		return text
	}
	text = strings.TrimRight(text, "0")
	text = strings.TrimSuffix(text, ".")
	if text == "" || text == "-" || text == "-0" {
		return "0"
	}
	return text
}

// ManifestFile Manifest 中单个物理文件的描述。
type ManifestFile struct {
	FileName string `json:"file_name"`
	RowCount int64  `json:"row_count"`
	FileSize int64  `json:"file_size"`
	Checksum string `json:"checksum"`
	Version  int    `json:"version"`
	Reason   string `json:"reason,omitempty"`
}

// Manifest 月归档 Manifest（{region}_{market}_{usc}_Min_{provider}_Mon_{yyyyMM}_Manifest.json）。
type Manifest struct {
	Usc                  string         `json:"usc"`
	Name                 string         `json:"name"`
	Region               string         `json:"region"`
	Market               string         `json:"market"`
	Provider             string         `json:"provider"`
	TimeZone             string         `json:"time_zone"`
	Currency             string         `json:"currency"`
	BizMonth             int            `json:"biz_month"`
	SchemaVersion        string         `json:"schema_version"`
	Format               string         `json:"format"`
	Delimiter            string         `json:"delimiter"`
	Encoding             string         `json:"encoding"`
	Compression          string         `json:"compression"`
	HeaderLines          int            `json:"header_lines"`
	BlankLineAfterHeader bool           `json:"blank_line_after_header"`
	Field                string         `json:"field"`
	FieldName            string         `json:"field_name"`
	FieldType            string         `json:"field_type"`
	Columns              []string       `json:"columns"`
	Base                 *ManifestFile  `json:"base"`
	Patches              []ManifestFile `json:"patches,omitempty"`
	RowCount             int64          `json:"row_count"`
	Checksum             string         `json:"checksum"`
	SealDate             string         `json:"seal_date"`
	GeneratedAt          string         `json:"generated_at"`
}

// BuildManifest 生成 Manifest：row_count 为 base + patches 行数之和，
// checksum 取 base 文件 SHA256（补丁各自的 checksum 见 patches[]）。
func BuildManifest(meta FileMeta, sealDate time.Time, compression string, base ManifestFile, patches []ManifestFile, generatedAt time.Time) Manifest {
	if compression == "" {
		compression = CompressionNone
	}
	total := base.RowCount
	for _, patch := range patches {
		total += patch.RowCount
	}
	return Manifest{
		Usc:                  meta.Usc,
		Name:                 meta.Name,
		Region:               meta.Region,
		Market:               meta.Market,
		Provider:             meta.Provider,
		TimeZone:             meta.Timezone,
		Currency:             meta.Currency,
		BizMonth:             meta.BizMonth,
		SchemaVersion:        MvsvSchemaVersion,
		Format:               MvsvFormat,
		Delimiter:            MvsvDelimiter,
		Encoding:             MvsvEncoding,
		Compression:          compression,
		HeaderLines:          MvsvHeaderLines,
		BlankLineAfterHeader: true,
		Field:                MvsvFieldLine,
		FieldName:            MvsvFieldNameLine,
		FieldType:            MvsvFieldTypeLine,
		Columns:              append([]string(nil), MvsvColumns...),
		Base:                 &base,
		Patches:              patches,
		RowCount:             total,
		Checksum:             base.Checksum,
		SealDate:             sealDate.Format("2006-01-02"),
		GeneratedAt:          generatedAt.UTC().Format(time.RFC3339),
	}
}

// MonthRange 返回 yyyyMM 对应的首末交易日期整数（yyyyMMdd）。
//
// 末位按 31 日给上界，主表 date 的实际取值落在区间内即可（查询按 date BETWEEN 使用）。
func MonthRange(bizMonth int) (from int, to int) {
	from = bizMonth*100 + 1
	to = bizMonth*100 + 31
	return from, to
}

// MonthFirstLastDay 返回 yyyyMM 首日与末日（按自然月）。
func MonthFirstLastDay(bizMonth int) (first time.Time, last time.Time) {
	year, month := bizMonth/100, time.Month(bizMonth%100)
	first = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	last = first.AddDate(0, 1, -1)
	return first, last
}

// SealDateOf 返回归档月份的收口日期：次月 25 日。
func SealDateOf(bizMonth int) time.Time {
	_, last := MonthFirstLastDay(bizMonth)
	return time.Date(last.Year(), last.Month(), 25, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
}

// FreezeDateOf 返回最晚完成日期：seal_date + slaDays（到期强制收口并优先处理）。
func FreezeDateOf(bizMonth int, slaDays int) time.Time {
	if slaDays < 0 {
		slaDays = 0
	}
	return SealDateOf(bizMonth).AddDate(0, 0, slaDays)
}
