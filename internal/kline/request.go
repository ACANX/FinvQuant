package kline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/acanx/finvquant/internal/calc"
)

// MaxSecondsTS 秒级时间戳上限（超过视为非秒级，默认拒绝）。
const MaxSecondsTS int64 = 100_000_000_000

// OptDec 可选十进制数值：区分「未传 / 显式 null / 有值」。
type OptDec struct {
	Set  bool
	Null bool
	Text string
}

// UnmarshalJSON 实现 json.Unmarshaler：只接受 JSON 数字或 null。
func (o *OptDec) UnmarshalJSON(data []byte) error {
	o.Set = true
	text := strings.TrimSpace(string(data))
	if text == "null" {
		o.Null = true
		return nil
	}
	if len(text) == 0 || !(text[0] == '-' || (text[0] >= '0' && text[0] <= '9')) {
		return fmt.Errorf("必须为数字")
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("必须为数字")
	}
	if _, ok := parseRat(number.String()); !ok {
		return fmt.Errorf("必须为数字")
	}
	o.Text = number.String()
	return nil
}

// Value 返回数值文本；显式 null 或未设置时返回 nil。
func (o OptDec) Value() *string {
	if !o.Set || o.Null {
		return nil
	}
	value := o.Text
	return &value
}

// OptInt 可选整数（date/time 等）。
type OptInt struct {
	Set   bool
	Null  bool
	Value int
}

// UnmarshalJSON 实现 json.Unmarshaler：只接受 JSON 整数或 null。
func (o *OptInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	text := strings.TrimSpace(string(data))
	if text == "null" {
		o.Null = true
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("必须为整数")
	}
	parsed, err := number.Int64()
	if err != nil {
		return fmt.Errorf("必须为整数")
	}
	o.Value = int(parsed)
	return nil
}

// Ptr 返回整数指针；显式 null 或未设置时返回 nil。
func (o OptInt) Ptr() *int {
	if !o.Set || o.Null {
		return nil
	}
	value := o.Value
	return &value
}

// OptStr 可选字符串。
type OptStr struct {
	Set  bool
	Null bool
	Text string
}

// UnmarshalJSON 实现 json.Unmarshaler：只接受 JSON 字符串或 null。
func (o *OptStr) UnmarshalJSON(data []byte) error {
	o.Set = true
	text := strings.TrimSpace(string(data))
	if text == "null" {
		o.Null = true
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("必须为字符串")
	}
	o.Text = value
	return nil
}

// Value 返回字符串指针；显式 null 或未设置时返回 nil。
func (o OptStr) Value() *string {
	if !o.Set || o.Null {
		return nil
	}
	value := o.Text
	return &value
}

// ItemIn 写入模型（/Add、/Batch、/Cdeu/Min/Upsert，亦用于 /Batch PATCH 的解析）。
//
// prev_close 为兼容字段：open 为空（未传或 null）时回填 open，自身不落库。
type ItemIn struct {
	Usc         *string `json:"usc"`
	Ts          *int64  `json:"ts"`
	Date        OptInt  `json:"date"`
	Time        OptInt  `json:"time"`
	Open        OptDec  `json:"open"`
	High        OptDec  `json:"high"`
	Low         OptDec  `json:"low"`
	Close       OptDec  `json:"close"`
	Volume      OptDec  `json:"volume"`
	Turnover    OptDec  `json:"turnover"`
	ChangePrice OptDec  `json:"change_price"`
	ChangeRatio OptDec  `json:"change_ratio"`
	Paocd       OptDec  `json:"paocd"`
	ExtField    OptStr  `json:"ext_field"`
	Remark      OptStr  `json:"remark"`
	Provider    OptStr  `json:"provider"`
	PrevClose   OptDec  `json:"prev_close"`
}

// BatchIn 批量写入请求体。
type BatchIn struct {
	Items []ItemIn `json:"items"`
}

// QueryIn /Query 请求体。
type QueryIn struct {
	Keys     []KeyPair `json:"keys"`
	Uscs     []string  `json:"uscs"`
	Provider *string   `json:"provider"`
	Limit    *int      `json:"limit"`
	Offset   *int      `json:"offset"`
}

// KeyPair (usc, ts) 组合键。
type KeyPair struct {
	Usc string `json:"usc"`
	Ts  int64  `json:"ts"`
}

// DeleteIn 删除请求体。
type DeleteIn struct {
	Keys []KeyPair `json:"keys"`
	Uscs []string  `json:"uscs"`
}

// DecodeJSON 严格解码（未知字段报错），并拒绝多余内容。
func DecodeJSON(data []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("请求体包含多余内容")
	}
	return nil
}

// unknownField 从 encoding/json 的未知字段错误中提取字段名。
func unknownField(err error) (string, bool) {
	const prefix = "json: unknown field "
	text := err.Error()
	index := strings.Index(text, prefix)
	if index < 0 {
		return "", false
	}
	name := strings.Trim(text[index+len(prefix):], `"`)
	return name, name != ""
}

// MapDecodeError 把 JSON 解码错误映射为目录错误（未知字段 / 格式错误）。
func MapDecodeError(err error) *Error {
	if field, ok := unknownField(err); ok {
		return New(CodeUnknownField).WithDebug("field", field)
	}
	return Wrap(CodeMalformedJSON, err).WithDebug("err_class", fmt.Sprintf("%T", err))
}

// normalizeTS 校验并归一 ts（DC-04：库内与契约均为秒级；ms 仅兼容开关）。
func normalizeTS(value int64, unit string) (int64, *Error) {
	if value < 0 {
		return 0, New(CodeOutOfRange).WithDebug("field", "ts").WithDebug("min", "0")
	}
	if value <= MaxSecondsTS {
		return value, nil
	}
	if unit == "ms" {
		return value / 1000, nil
	}
	return 0, New(CodeOutOfRange).WithDebug("field", "ts").
		WithDebug("min", "0").WithDebug("max", fmt.Sprint(MaxSecondsTS))
}

func validateKey(usc *string, ts *int64, unit string) (string, int64, *Error) {
	if usc == nil {
		return "", 0, New(CodeMissingParam).WithDebug("field", "usc")
	}
	if *usc == "" {
		return "", 0, New(CodeMissingParam).WithDebug("field", "usc")
	}
	if len(*usc) > 255 {
		return "", 0, New(CodeOutOfRange).WithDebug("field", "usc").WithDebug("max", "255")
	}
	if ts == nil {
		return "", 0, New(CodeMissingParam).WithDebug("field", "ts")
	}
	normalized, tsErr := normalizeTS(*ts, unit)
	if tsErr != nil {
		return "", 0, tsErr
	}
	return *usc, normalized, nil
}

func validateRange(name string, value, min, max int) *Error {
	if value < min || value > max {
		return New(CodeOutOfRange).WithDebug("field", name).
			WithDebug("min", fmt.Sprint(min)).WithDebug("max", fmt.Sprint(max))
	}
	return nil
}

// ToRow 校验并转换为落库行（应用 prev_close 兼容规则）。
func (item ItemIn) ToRow(unit string) (Row, *Error) {
	usc, ts, keyErr := validateKey(item.Usc, item.Ts, unit)
	if keyErr != nil {
		return Row{}, keyErr
	}
	row := Row{Usc: usc, Ts: ts}
	if item.Date.Set && !item.Date.Null {
		date := item.Date.Value
		if err := validateRange("date", date, 19500101, 21001231); err != nil {
			return Row{}, err
		}
		row.Date = &date
	}
	if item.Time.Set && !item.Time.Null {
		clock := item.Time.Value
		if err := validateRange("time", clock, 0, 235959); err != nil {
			return Row{}, err
		}
		row.Time = &clock
	}
	// prev_close 兼容：open 未传/为 null 时用 prev_close 回填（自身不落库）。
	open := item.Open
	if (!open.Set || open.Null) && item.PrevClose.Set && !item.PrevClose.Null {
		open = item.PrevClose
	}
	row.Open = open.Value()
	row.High = item.High.Value()
	row.Low = item.Low.Value()
	row.Close = item.Close.Value()
	row.Volume = item.Volume.Value()
	if row.Volume != nil && strings.HasPrefix(strings.TrimSpace(*row.Volume), "-") {
		return Row{}, New(CodeOutOfRange).WithDebug("field", "volume").WithDebug("min", "0")
	}
	row.Turnover = item.Turnover.Value()
	if row.Turnover != nil && strings.HasPrefix(strings.TrimSpace(*row.Turnover), "-") {
		return Row{}, New(CodeOutOfRange).WithDebug("field", "turnover").WithDebug("min", "0")
	}
	row.ChangePrice = item.ChangePrice.Value()
	row.ChangeRatio = item.ChangeRatio.Value()
	row.Paocd = item.Paocd.Value()
	row.ExtField = item.ExtField.Value()
	row.Remark = item.Remark.Value()
	row.Provider = item.Provider.Value()
	if row.Provider != nil && len(*row.Provider) > 255 {
		return Row{}, New(CodeOutOfRange).WithDebug("field", "provider").WithDebug("max", "255")
	}
	// 自动补全（仅当两列都未提供时）：API 不写 last_close，参考价回退 open。
	if row.ChangePrice == nil && row.ChangeRatio == nil {
		row.ChangePrice, row.ChangeRatio = calc.Change(row.Close, nil, row.Open)
	}
	return row, nil
}

// ToPatchItem 校验并转换为批量增量更新条目；仅保留显式传入的列。
func (item ItemIn) ToPatchItem(unit string) (PatchItem, *Error) {
	usc, ts, keyErr := validateKey(item.Usc, item.Ts, unit)
	if keyErr != nil {
		return PatchItem{}, keyErr
	}
	patch := PatchItem{Usc: usc, Ts: ts}
	open := item.Open
	if (!open.Set || open.Null) && item.PrevClose.Set && !item.PrevClose.Null {
		open = item.PrevClose
	}
	appendCol := func(column string, value *string) {
		patch.Columns = append(patch.Columns, column)
		patch.Values = append(patch.Values, value)
	}
	for _, field := range []struct {
		column string
		value  OptDec
	}{
		{ColOpen, open},
		{ColHigh, item.High},
		{ColLow, item.Low},
		{ColClose, item.Close},
		{ColVolume, item.Volume},
		{ColTurnover, item.Turnover},
		{ColChangePrice, item.ChangePrice},
		{ColChangeRatio, item.ChangeRatio},
		{ColPaocd, item.Paocd},
	} {
		if field.value.Set {
			appendCol(field.column, field.value.Value())
		}
	}
	if item.Date.Set {
		if item.Date.Null {
			appendCol(ColDate, nil)
		} else {
			if err := validateRange("date", item.Date.Value, 19500101, 21001231); err != nil {
				return PatchItem{}, err
			}
			text := fmt.Sprint(item.Date.Value)
			appendCol(ColDate, &text)
		}
	}
	if item.Time.Set {
		if item.Time.Null {
			appendCol(ColTime, nil)
		} else {
			if err := validateRange("time", item.Time.Value, 0, 235959); err != nil {
				return PatchItem{}, err
			}
			text := fmt.Sprint(item.Time.Value)
			appendCol(ColTime, &text)
		}
	}
	if item.ExtField.Set {
		appendCol(ColExtField, item.ExtField.Value())
	}
	if item.Remark.Set {
		appendCol(ColRemark, item.Remark.Value())
	}
	if item.Provider.Set {
		value := item.Provider.Value()
		if value != nil && len(*value) > 255 {
			return PatchItem{}, New(CodeOutOfRange).WithDebug("field", "provider").WithDebug("max", "255")
		}
		appendCol(ColProvider, value)
	}
	if patch.Columns == nil {
		patch.Columns = []string{}
		patch.Values = []*string{}
	}
	return patch, nil
}
