package mvsv

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatV2 MVSV-2 格式标识（多证券、单文件全表）。
const FormatV2 = "MVSV-2"

// V2Header 必填头部键（V2 不再要求单证券的 Exchange/StockId 等）。
var V2RequiredHeaders = []string{"Format", "Field", "Count", "Title"}

// buildHeaderV2 构建 MVSV-2 头部：按 # Field 列名动态映射。
func buildHeaderV2(values map[string]string) (*Header, error) {
	for _, key := range V2RequiredHeaders {
		if _, ok := values[key]; !ok {
			return nil, fmt.Errorf("缺少必填头部: %s", key)
		}
	}
	columns := strings.Split(values["Field"], "|")
	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("Field 不能为空")
	}
	seen := map[string]bool{}
	hasUSC, hasTS := false, false
	for _, column := range columns {
		if column == "" {
			return nil, fmt.Errorf("Field 含空列名: %s", values["Field"])
		}
		if seen[column] {
			return nil, fmt.Errorf("Field 存在重复列: %s", column)
		}
		seen[column] = true
		if column == "usc" {
			hasUSC = true
		}
		if column == "ts" {
			hasTS = true
		}
	}
	if !hasUSC || !hasTS {
		return nil, fmt.Errorf("MVSV-2 Field 必须包含 usc 与 ts 列")
	}
	count, err := strconv.Atoi(values["Count"])
	if err != nil || count < 0 {
		return nil, fmt.Errorf("Count 必须为非负整数")
	}
	return &Header{
		Values:  values,
		Count:   count,
		Format:  FormatV2,
		Columns: columns,
		TimeZone: func() string {
			if values["TimeZone"] != "" {
				return values["TimeZone"]
			}
			return values["EffectiveTimeZone"]
		}(),
	}, nil
}

// parseRowV2 按列名解析 MVSV-2 数据行。
func parseRowV2(line string, header *Header, lineNumber int) (Row, error) {
	fields := strings.Split(line, "|")
	if len(fields) != len(header.Columns) {
		return Row{}, fmt.Errorf("列数=%d，与 Field 列数 %d 不一致", len(fields), len(header.Columns))
	}
	row := Row{}
	for index, column := range header.Columns {
		raw := strings.TrimSpace(fields[index])
		switch column {
		case "usc":
			row.Usc = raw
		case "ts":
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return Row{}, fmt.Errorf("ts 必须为整数: %q", raw)
			}
			row.Ts = value
		case "date":
			row.Date = intOrNil(raw)
		case "time":
			row.Time = intOrNil(raw)
		case "open":
			row.Open = decimalOrNil(raw)
		case "high":
			row.High = decimalOrNil(raw)
		case "low":
			row.Low = decimalOrNil(raw)
		case "close":
			row.Close = decimalOrNil(raw)
		case "last_close":
			row.LastClose = decimalOrNil(raw)
		case "volume":
			row.Volume = numericTextOrNil(raw)
		case "turnover":
			row.Turnover = numericTextOrNil(raw)
		case "change_price":
			row.ChangePrice = decimalOrNil(raw)
		case "change_ratio":
			row.ChangeRatio = decimalOrNil(raw)
		case "paocd", "pacd":
			row.Paocd = decimalOrNil(raw)
		case "ext_field":
			row.ExtField = decimalOrNil(raw)
		case "remark":
			row.Remark = decimalOrNil(raw)
		case "provider":
			row.Provider = decimalOrNil(raw)
		case "gmt_create":
			row.GmtCreate = decimalOrNil(raw)
		case "gmt_update":
			row.GmtUpdate = decimalOrNil(raw)
		default:
			// 未知列忽略（向前兼容新增列）
		}
	}
	if row.Usc == "" {
		return Row{}, fmt.Errorf("第 %d 行: usc 不能为空", lineNumber)
	}
	return row, nil
}

// intOrNil 返回整数指针；空值或非法值返回 nil。
func intOrNil(raw string) *int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}
