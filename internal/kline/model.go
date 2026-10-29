package kline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// 数据库列（顺序与表结构一致；API 侧不读写 gmt_create / gmt_update）。
const (
	ColUsc         = "usc"
	ColTs          = "ts"
	ColDate        = "date"
	ColTime        = "time"
	ColOpen        = "open"
	ColHigh        = "high"
	ColLow         = "low"
	ColClose       = "close"
	ColVolume      = "volume"
	ColTurnover    = "turnover"
	ColChangePrice = "change_price"
	ColChangeRatio = "change_ratio"
	ColLastClose   = "last_close"
	ColPaocd       = "paocd" // 库列名；对外 API 字段为 pacd
	ColExtField    = "ext_field"
	ColRemark      = "remark"
	ColProvider    = "provider"
)

// InsertColumns 写入列（gmt_* 由数据库默认值与触发器维护）。
var InsertColumns = []string{
	ColUsc, ColTs, ColDate, ColTime, ColOpen, ColHigh, ColLow, ColClose, ColLastClose,
	ColVolume, ColTurnover, ColChangePrice, ColChangeRatio, ColPaocd,
	ColExtField, ColRemark, ColProvider,
}

// UpsertUpdateColumns upsert 冲突时覆盖的业务列（usc/ts 为冲突键）。
var UpsertUpdateColumns = func() []string {
	out := make([]string, 0, len(InsertColumns)-2)
	for _, column := range InsertColumns {
		if column == ColUsc || column == ColTs {
			continue
		}
		out = append(out, column)
	}
	return out
}()

// CompactFieldMap 列表端点紧凑字段 → 表字段；map 的顺序即 column 顺序（见 CompactFields）。
var CompactFieldMap = []struct {
	Short  string
	Column string
}{
	{"usc", ColUsc},
	{"ts", ColTs},
	{"date", ColDate},
	{"time", ColTime},
	{"o", ColOpen},
	{"h", ColHigh},
	{"l", ColLow},
	{"c", ColClose},
	{"v", ColVolume},
	{"t", ColTurnover},
	{"cr", ColChangeRatio},
	{"cp", ColChangePrice},
	{"pa", ColPaocd},
	{"pv", ColProvider},
	{"ext", ColExtField},
}

// CompactFields 返回 /PageUni 的 column 列表（固定 15 项）。
func CompactFields() []string {
	out := make([]string, 0, len(CompactFieldMap))
	for _, item := range CompactFieldMap {
		out = append(out, item.Short)
	}
	return out
}

// NumericColumns 数据库 numeric 列（读取时统一 ::text，避免精度丢失）。
var NumericColumns = map[string]bool{
	ColOpen: true, ColHigh: true, ColLow: true, ColClose: true,
	ColVolume: true, ColTurnover: true, ColChangePrice: true,
	ColChangeRatio: true, ColLastClose: true, ColPaocd: true,
}

// Row 落库 / 查询行；所有 numeric 值以字符串承载，避免 float64 精度损失。
type Row struct {
	Usc         string
	Ts          int64
	Date        *int
	Time        *int
	Open        *string
	High        *string
	Low         *string
	Close       *string
	LastClose   *string
	Volume      *string
	Turnover    *string
	ChangePrice *string
	ChangeRatio *string
	Pacd        *string
	ExtField    *string
	Remark      *string
	Provider    *string
}

// Key (usc, ts) 复合主键。
type Key struct {
	Usc string
	Ts  int64
}

// ListItem /Page 的紧凑输出项（数值为 JSON 数字，cr 为两位小数文本）。
type ListItem struct {
	Usc  string          `json:"usc"`
	Ts   int64           `json:"ts"`
	Date *int            `json:"date"`
	Time *int            `json:"time"`
	O    json.RawMessage `json:"o"`
	H    json.RawMessage `json:"h"`
	L    json.RawMessage `json:"l"`
	C    json.RawMessage `json:"c"`
	V    json.RawMessage `json:"v"`
	T    json.RawMessage `json:"t"`
	Cr   *string         `json:"cr"`
	Cp   json.RawMessage `json:"cp"`
	Pa   json.RawMessage `json:"pa"`
	Pv   *string         `json:"pv"`
	Ext  *string         `json:"ext"`
}

// numericJSON 把 numeric 文本转为 JSON 数字字面量（去尾零）；空值输出 null。
func numericJSON(value *string) json.RawMessage {
	if value == nil {
		return json.RawMessage("null")
	}
	text := strings.TrimSpace(*value)
	if _, ok := parseRat(text); !ok {
		raw, _ := json.Marshal(text)
		return raw
	}
	cleaned := cleanNumericLiteral(text)
	return json.RawMessage(cleaned)
}

// cleanNumericLiteral 去掉 numeric 文本的尾随零（保留符号与整数部分）。
func cleanNumericLiteral(text string) string {
	cleaned := text
	if strings.ContainsAny(cleaned, "eE") {
		if rat, ok := parseRat(cleaned); ok {
			// 科学计数法在数据库 numeric 文本中不会出现，这里仅做兜底展开。
			cleaned = rat.FloatString(20)
		}
	}
	if strings.Contains(cleaned, ".") {
		cleaned = strings.TrimRight(cleaned, "0")
		cleaned = strings.TrimRight(cleaned, ".")
	}
	if cleaned == "" || cleaned == "-" || cleaned == "-0" {
		cleaned = "0"
	}
	return cleaned
}

// ListItem 转换为 /Page 输出项（不输出 remark 与 gmt_*）。
func (r Row) ListItem() ListItem {
	return ListItem{
		Usc:  r.Usc,
		Ts:   r.Ts,
		Date: r.Date,
		Time: r.Time,
		O:    numericJSON(r.Open),
		H:    numericJSON(r.High),
		L:    numericJSON(r.Low),
		C:    numericJSON(r.Close),
		V:    numericJSON(r.Volume),
		T:    numericJSON(r.Turnover),
		Cr:   RatioText(r.ChangeRatio),
		Cp:   numericJSON(r.ChangePrice),
		Pa:   numericJSON(r.Pacd),
		Pv:   r.Provider,
		Ext:  r.ExtField,
	}
}

// UniRow 转换为 /PageUni 的 row（每项为字符串或 null，顺序同 CompactFields）。
func (r Row) UniRow() []*string {
	intText := func(value *int) *string {
		if value == nil {
			return nil
		}
		text := strconv.Itoa(*value)
		return &text
	}
	tsText := strconv.FormatInt(r.Ts, 10)
	return []*string{
		&r.Usc, &tsText, intText(r.Date), intText(r.Time),
		CellText(r.Open), CellText(r.High), CellText(r.Low), CellText(r.Close),
		CellText(r.Volume), CellText(r.Turnover), RatioText(r.ChangeRatio),
		CellText(r.ChangePrice), CellText(r.Pacd), r.Provider, r.ExtField,
	}
}

// MarshalJSON 输出完整字段（/Add、/Query 使用；不含 gmt_*）。
func (r Row) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	writeJSONField(&buf, "usc", r.Usc, true)
	writeKey(&buf, "ts")
	buf.WriteString(strconv.FormatInt(r.Ts, 10))
	writeOptionalInt(&buf, "date", r.Date)
	writeOptionalInt(&buf, "time", r.Time)
	writeNumericField(&buf, "open", r.Open)
	writeNumericField(&buf, "high", r.High)
	writeNumericField(&buf, "low", r.Low)
	writeNumericField(&buf, "close", r.Close)
	writeNumericField(&buf, "last_close", r.LastClose)
	writeNumericField(&buf, "volume", r.Volume)
	writeNumericField(&buf, "turnover", r.Turnover)
	writeNumericField(&buf, "change_price", r.ChangePrice)
	writeNumericField(&buf, "change_ratio", r.ChangeRatio)
	writeNumericField(&buf, "pacd", r.Pacd)
	writeOptionalString(&buf, "ext_field", r.ExtField)
	writeOptionalString(&buf, "remark", r.Remark)
	writeOptionalString(&buf, "provider", r.Provider)
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func writeJSONField(buf *bytes.Buffer, key string, value string, first bool) {
	if !first {
		buf.WriteByte(',')
	}
	raw, _ := json.Marshal(value)
	buf.WriteString(strconv.Quote(key))
	buf.WriteByte(':')
	buf.Write(raw)
}

func writeKey(buf *bytes.Buffer, key string) {
	buf.WriteByte(',')
	buf.WriteString(strconv.Quote(key))
	buf.WriteByte(':')
}

func writeOptionalInt(buf *bytes.Buffer, key string, value *int) {
	writeKey(buf, key)
	if value == nil {
		buf.WriteString("null")
		return
	}
	buf.WriteString(strconv.Itoa(*value))
}

func writeNumericField(buf *bytes.Buffer, key string, value *string) {
	writeKey(buf, key)
	if value == nil {
		buf.WriteString("null")
		return
	}
	text := strings.TrimSpace(*value)
	if _, ok := parseRat(text); !ok {
		raw, _ := json.Marshal(text)
		buf.Write(raw)
		return
	}
	buf.WriteString(cleanNumericLiteral(text))
}

func writeOptionalString(buf *bytes.Buffer, key string, value *string) {
	writeKey(buf, key)
	if value == nil {
		buf.WriteString("null")
		return
	}
	raw, _ := json.Marshal(*value)
	buf.Write(raw)
}

// ListFilter 列表端点过滤条件（/Page、/PageUni 共用）。
type ListFilter struct {
	Usc      *string
	Provider *string
	DateFrom *int
	DateTo   *int
	TsFrom   *int64
	TsTo     *int64
}

// QuerySpec /Query 请求条件。
type QuerySpec struct {
	Keys     []Key
	Uscs     []string
	Provider *string
	Limit    int
	Offset   int
}

// CountFilter /Count 过滤条件。
type CountFilter struct {
	Usc      *string
	Provider *string
	DateFrom *int
	DateTo   *int
}

// PatchItem 批量增量更新条目：Columns 为本次显式传入的列（有序），Values 与之一一对应，
// nil 表示显式写入 NULL（区别于「未传」）。
type PatchItem struct {
	Usc     string
	Ts      int64
	Columns []string
	Values  []*string
}

// DebugString 便于日志输出。
func (p PatchItem) DebugString() string {
	return fmt.Sprintf("%s@%d:%s", p.Usc, p.Ts, strings.Join(p.Columns, ","))
}

// APIPrefix 分钟行情 K 线 API 的统一前缀（需求 10782 起统一到 CDEU）。
const APIPrefix = "/API/V1/Meta/Finv/Quote/Cdeu/Min"

// LegacyAPIPrefix 旧 Python 服务前缀；自需求 10782 起不再注册，仅保留常量供引用/文档追溯。
const LegacyAPIPrefix = "/API/V1/Meta/Finv/Quote/Secu/KLine/Min"

// CdeuAPIPrefix 与 APIPrefix 相同，保留别名以兼容既有引用（需求 10777/10781）。
const CdeuAPIPrefix = APIPrefix

// CdeuUpsertPath 分钟行情批量 Upsert 的完整路径。
const CdeuUpsertPath = CdeuAPIPrefix + "/Upsert"

// CdeuPagePath 分钟行情条件分页（对象数组）的完整路径。
const CdeuPagePath = CdeuAPIPrefix + "/Page"

// CdeuPageUniPath 分钟行情条件分页（column/row）的完整路径。
const CdeuPageUniPath = CdeuAPIPrefix + "/PageUni"
