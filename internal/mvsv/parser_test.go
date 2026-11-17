package mvsv

import (
	"strings"
	"testing"
)

const sampleMvsv = `# Title : "US_NVDA_Min_V4_2026"
# Format : "MVSV-1"
# Field : "ts|dt|o|c|l|h|v|t|cp|cr|p"
# FieldType : "Int|Long|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal"
# FieldName : "Ts|DateTime|Open|Close|Low|High|Volume|Turnover|ChangePrice|ChangeRatio|PrevClose"
# 字段名称 : "时间戳(UTC)|日期时间|开盘价|收盘价|最低价|最高价|成交量|成交额|涨跌值|涨跌幅(%)|前一收盘价"
# Count : 3
# EffectiveTimeZone : "Asia/Shanghai"
# StockId : 202597
# Code : "NVDA"
# Market : "SSE"
# MarketCode : 1
# Exchange : "SSE"
# ExchangeCode : 11
# CurrencyCode : 1
# PriceAccuracy : 3
# LotSize : 100
# Name : "英伟达"
# Region : "US"
# Period : "Min"
# Dsv : 3

1785720600|20260803093000|7.001|7.002|6.999|7.003|100000|70010000000|0.001|0.000143|7.000
1785720660|20260803093100|7.002|7.001|7.000|7.003|80000|56010000000|0.000|-0.000143|7.002
1785720720|20260803093200|7.003|7.004|7.001|7.005|90000|63020000000|0.002|0.000286|7.001
`

func TestParseValid(t *testing.T) {
	result, err := Parse([]byte(sampleMvsv), "TEST.mvsv")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if result.Header.Count != 3 {
		t.Fatalf("Count=%d，期望 3", result.Header.Count)
	}
	if result.Header.Values["MarketCode"] != "1" {
		t.Fatalf("MarketCode=%s", result.Header.Values["MarketCode"])
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
	first := result.Rows[0]
	if first.SecuCode != "NVDA" || first.MarketCode != 1 {
		t.Fatalf("证券标识错误: %+v", first)
	}
	if first.Ts != 1785720600 {
		t.Fatalf("ts=%d", first.Ts)
	}
	if first.Date == nil || *first.Date != 20260803 {
		t.Fatalf("date=%v", first.Date)
	}
	if first.Time == nil || *first.Time != 93000 {
		t.Fatalf("time=%v", first.Time)
	}
	if first.Close == nil || *first.Close != "7.002" {
		t.Fatalf("close=%v", first.Close)
	}
	if first.Volume == nil || *first.Volume != 100000 {
		t.Fatalf("volume=%v", first.Volume)
	}
}

func TestParseMissingMarketCode(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# MarketCode : 1\n", "", 1)
	_, err := Parse([]byte(content), "TEST.mvsv")
	if err == nil || !strings.Contains(err.Error(), "MarketCode") {
		t.Fatalf("期望缺少 MarketCode 错误，实际: %v", err)
	}
}

func TestParseBadFormat(t *testing.T) {
	_, err := Parse([]byte("# Format : \"MVSV-1\"\nbroken\n"), "bad.mvsv")
	if err == nil {
		t.Fatal("期望解析失败")
	}
}

func TestParseTsTimezoneMismatch(t *testing.T) {
	content := strings.Replace(sampleMvsv,
		"1785720600|20260803093000", "1785720600|20260803103000", 1)
	_, err := Parse([]byte(content), "TEST.mvsv")
	if err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("期望 ts 与 dt 不一致错误，实际: %v", err)
	}
}

// 布局 B（12 列，pc 已过时）：ts|d|t|o|c|l|h|v|a|cp|cr|p
// d=8 位日期 + t=6 位时间，a=成交额；数据行末尾允许残留空段（旧 pc 位，如 "...|4332.1|"）
// ts=1767337200 → 2026-01-02 07:00 UTC → America/New_York（UTC-5）2026-01-02 02:00:00（20260102020000）
const sampleMvsvLayoutB = `# Title : "GCmain_Min_V3_2026_195279_2026072202"
# Format : "MVSV-1"
# Field : "ts|d|t|o|c|l|h|v|a|cp|cr|p"
# FieldType : "Int|Long|Long|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal"
# FieldName : "Ts|Date|Time|Open|Close|Low|High|Volume|Amount|ChangePrice|ChangeRatio|PrevClose"
# 字段名称 : "时间戳(UTC)|日期|时间|开盘价|收盘价|最低价|最高价|成交量|成交额|涨跌值|涨跌幅(%)|前一收盘价"
# Count : 3
# EffectiveTimeZone : "America/New_York"
# StockId : 70000294
# Code : "GCMain"
# Market : "COMEX"
# MarketCode : 1320
# Exchange : "COMEX"
# ExchangeCode : 33
# CurrencyCode : 55
# PriceAccuracy : 1
# LotSize : 100
# Name : "黄金期货主连 (2608)"
# Region : "US"
# Period : "Min"
# Dsv : 3

1767337200|20260102|020000|4340|4907.5|4319.7|5626.8|5926343|0|575.4|13.282242|4332.1|
1767337260|20260102|020100|4340|4343.9|4338.5|4349.5|314|0|11.8|0.272385|4332.1|
1767337320|20260102|020200|4342.7|4344|4342|4346.3|81|0|0.1|0.002302|4343.9|
`

func TestParseLayoutB(t *testing.T) {
	result, err := Parse([]byte(sampleMvsvLayoutB), "GCmain.mvsv")
	if err != nil {
		t.Fatalf("解析布局 B 失败: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
	first := result.Rows[0]
	if first.SecuCode != "GCMain" || first.MarketCode != 1320 {
		t.Fatalf("证券标识错误: %+v", first)
	}
	if first.Ts != 1767337200 {
		t.Fatalf("ts=%d", first.Ts)
	}
	// d+t 分开解析：date=20260102, time=020000
	if first.Date == nil || *first.Date != 20260102 {
		t.Fatalf("date=%v", first.Date)
	}
	if first.Time == nil || *first.Time != 20000 {
		t.Fatalf("time=%v", first.Time)
	}
	// a 列映射成交额
	if first.Turnover == nil || *first.Turnover != "0" {
		t.Fatalf("turnover=%v", first.Turnover)
	}
	if first.Close == nil || *first.Close != "4907.5" {
		t.Fatalf("close=%v", first.Close)
	}
	if first.PrevClose == nil || *first.PrevClose != "4332.1" {
		t.Fatalf("prev_close=%v", first.PrevClose)
	}
}

func TestParseUnsupportedLayout(t *testing.T) {
	// 核心列名不同（第 2 列应为 d 或 dt，此处为 xx）→ 不匹配任何核心布局
	content := strings.Replace(sampleMvsvLayoutB,
		"# Field : \"ts|d|t|o|c|l|h|v|a|cp|cr|p\"",
		"# Field : \"ts|xx|t|o|c|l|h|v|a|cp|cr|p\"", 1)
	_, err := Parse([]byte(content), "bad.mvsv")
	if err == nil || !strings.Contains(err.Error(), "Field 布局不支持") {
		t.Fatalf("期望 Field 布局不支持错误，实际: %v", err)
	}
}

// 多余列容忍：布局 B 核心后追加已过时的 pc 列（13 列）也应解析成功
// （用户约定 2026-08-06：只要核心字段存在，多余字段允许解析）
func TestParseLayoutBWithExtraPcColumn(t *testing.T) {
	content := strings.Replace(sampleMvsvLayoutB,
		"# Field : \"ts|d|t|o|c|l|h|v|a|cp|cr|p\"",
		"# Field : \"ts|d|t|o|c|l|h|v|a|cp|cr|p|pc\"", 1)
	result, err := Parse([]byte(content), "layoutb_pc.mvsv")
	if err != nil {
		t.Fatalf("含 pc 多余列的布局 B 解析失败: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
	// 数据行也带 pc 值（非空）时仍应正常解析
	contentWithValue := strings.Replace(sampleMvsvLayoutB,
		"1767337200|20260102|020000|4340|4907.5|4319.7|5626.8|5926343|0|575.4|13.282242|4332.1|",
		"1767337200|20260102|020000|4340|4907.5|4319.7|5626.8|5926343|0|575.4|13.282242|4332.1|999", 1)
	result2, err := Parse([]byte(contentWithValue), "layoutb_pc_val.mvsv")
	if err != nil {
		t.Fatalf("含 pc 值的解析失败: %v", err)
	}
	if result2.Rows[0].Close == nil || *result2.Rows[0].Close != "4907.5" {
		t.Fatalf("close=%v", result2.Rows[0].Close)
	}
}

// 行尾空段容忍：数据行末尾允许残留旧 pc 空段（"...|4332.1|"），解析器应截断后按 12 列处理
func TestParseLayoutBTrailingEmptySegment(t *testing.T) {
	// 第三行末尾多一个空段（旧 pc 位残留），其余行正常
	content := strings.Replace(sampleMvsvLayoutB,
		"1767337320|20260102|020200|4342.7|4344|4342|4346.3|81|0|0.1|0.002302|4343.9|",
		"1767337320|20260102|020200|4342.7|4344|4342|4346.3|81|0|0.1|0.002302|4343.9||", 1)
	result, err := Parse([]byte(content), "trailing.mvsv")
	if err != nil {
		t.Fatalf("容忍行尾空段解析失败: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
	last := result.Rows[2]
	if last.Close == nil || *last.Close != "4344" {
		t.Fatalf("末行 close=%v", last.Close)
	}
}

// 时区解析：TimeZone 优先于 EffectiveTimeZone（用户更正：以 TimeZone 为准，EffectiveTimeZone 仅参考）
func TestTimeZonePrecedence(t *testing.T) {
	// 同一 ts，TimeZone=Asia/Shanghai 与 EffectiveTimeZone=America/New_York 的本地时间不同。
	// ts=1785720600 → Asia/Shanghai 20260803093000；America/New_York 20260802213000
	content := strings.Replace(sampleMvsv,
		"# EffectiveTimeZone : \"Asia/Shanghai\"",
		"# EffectiveTimeZone : \"America/New_York\"\n# TimeZone : \"Asia/Shanghai\"", 1)
	result, err := Parse([]byte(content), "tz_precedence.mvsv")
	if err != nil {
		t.Fatalf("TimeZone 优先解析失败: %v", err)
	}
	if result.Header.TimeZone != "Asia/Shanghai" {
		t.Fatalf("TimeZone=%s，期望 Asia/Shanghai（应优先于 EffectiveTimeZone）", result.Header.TimeZone)
	}
}

// EffectiveTimeZone 缺失：非必填（用户更正），仅 TimeZone 存在即可正常解析
func TestMissingEffectiveTimeZone(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# EffectiveTimeZone : \"Asia/Shanghai\"\n", "", 1)
	// 此时头部已无 EffectiveTimeZone，但 TimeZone 也不存在 → 应跳过校验正常解析
	result, err := Parse([]byte(content), "no_etz.mvsv")
	if err != nil {
		t.Fatalf("缺少 EffectiveTimeZone 不应报错: %v", err)
	}
	if result.Header.TimeZone != "" {
		t.Fatalf("TimeZone=%s，期望空（两者都未提供）", result.Header.TimeZone)
	}
}

// 两者都缺失：不校验 ts 一致性，正常解析（头部少一行不影响数据）
func TestMissingBothTimeZones(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# EffectiveTimeZone : \"Asia/Shanghai\"\n", "", 1)
	result, err := Parse([]byte(content), "no_tz.mvsv")
	if err != nil {
		t.Fatalf("两者都缺失不应报错: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
}

// TimeZone 非法时报错（以 TimeZone 为准的校验）
func TestInvalidTimeZone(t *testing.T) {
	content := strings.Replace(sampleMvsv,
		"# EffectiveTimeZone : \"Asia/Shanghai\"",
		"# EffectiveTimeZone : \"Asia/Shanghai\"\n# TimeZone : \"Not/AZone\"", 1)
	_, err := Parse([]byte(content), "bad_tz.mvsv")
	if err == nil || !strings.Contains(err.Error(), "时区非法") {
		t.Fatalf("期望 TimeZone 非法错误，实际: %v", err)
	}
}

// Exchange / ExchangeCode / Market / MarketCode 四键必填（用户约定 2026-08-06）
func TestParseMissingExchangeCode(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# ExchangeCode : 11\n", "", 1)
	_, err := Parse([]byte(content), "no_exchangecode.mvsv")
	if err == nil || !strings.Contains(err.Error(), "ExchangeCode") {
		t.Fatalf("期望缺少 ExchangeCode 错误，实际: %v", err)
	}
}

func TestParseMissingExchange(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# Exchange : \"SSE\"\n", "", 1)
	_, err := Parse([]byte(content), "no_exchange.mvsv")
	if err == nil || !strings.Contains(err.Error(), "Exchange") {
		t.Fatalf("期望缺少 Exchange 错误，实际: %v", err)
	}
}

// 溯源类必填键（用户约定 2026-08-06）：Title/Region/Name/Period/Dsv/FieldType/FieldName/字段名称/StockId
func TestParseMissingTitle(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# Title : \"US_NVDA_Min_V4_2026\"\n", "", 1)
	_, err := Parse([]byte(content), "no_title.mvsv")
	if err == nil || !strings.Contains(err.Error(), "Title") {
		t.Fatalf("期望缺少 Title 错误，实际: %v", err)
	}
}

func TestParseMissingStockId(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# StockId : 202597\n", "", 1)
	_, err := Parse([]byte(content), "no_stockid.mvsv")
	if err == nil || !strings.Contains(err.Error(), "StockId") {
		t.Fatalf("期望缺少 StockId 错误，实际: %v", err)
	}
}

func TestParseMissingFieldType(t *testing.T) {
	content := strings.Replace(sampleMvsv, "# FieldType : \"Int|Long|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal\"\n", "", 1)
	_, err := Parse([]byte(content), "no_fieldtype.mvsv")
	if err == nil || !strings.Contains(err.Error(), "FieldType") {
		t.Fatalf("期望缺少 FieldType 错误，实际: %v", err)
	}
}
