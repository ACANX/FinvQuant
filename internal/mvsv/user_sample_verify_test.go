package mvsv

import (
	"testing"
)

// GCmain 真实文件结构（12 列布局，行尾含旧 pc 空段）。
// 注意：d 列（本地日期）必须与 ts 在 EffectiveTimeZone 下换算一致，
// 否则一致性校验会报错（见 TestUserGCmainSampleTsMismatch）。
const userGCmainSample = `# Title : "GCmain_Min_V3_2026_195279_2026072202"
# Format : "MVSV-1"
# Field : "ts|d|t|o|c|l|h|v|a|cp|cr|p"
# FieldType : "Int|Long|Long|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal"
# FieldName : "Ts|Date|Time|Open|Close|Low|High|Volume|Amount|ChangePrice|ChangeRatio|PrevClose"
# 字段名称 : "时间戳(UTC)|日期|时间|开盘价|收盘价|最低价|最高价|成交量|成交额|涨跌值|涨跌幅(%)|前一收盘价"
# Count : 6
# EffectiveTimeZone : "America/New_York"
# StockId : 70000294
# FutuSymbol : "GCmain"
# Code : "GCMain"
# Market : "COMEX"
# MarketCode : 1320
# Exchange : "COMEX"
# ExchangeCode : 33
# PriceAccuracy : 1
# CurrencyCode : 55
# InstrumentType : 10
# InstrumentTypeV2 : 9
# LotSize : 100
# EngName : "Gold Futures (AUG6)"
# TimeZone : "America/New_York"
# Name : "黄金期货主连 (2608)"
# Region : "US"
# Period : "Min"
# Start : "202601010500"
# End : "202607210632"
# Size : 195279
# Dsv : 3
# Year : 2026

1767243600|20260101|000000|4340|4907.5|4319.7|5626.8|5926343|0|575.4|13.282242|4332.1|
1767308460|20260101|180100|4340|4343.9|4338.5|4349.5|314|0|11.8|0.272385|4332.1|
1767308520|20260101|180200|4342.7|4344|4342|4346.3|81|0|0.1|0.002302|4343.9|
1767308580|20260101|180300|4343.9|4344|4342.3|4345.6|38|0|0|0|4344|
1767308640|20260101|180400|4343.9|4340.3|4339.8|4343.9|98|0|-3.7|-0.085175|4344|
1767308700|20260101|180500|4339.5|4341.8|4337.3|4341.8|96|0|1.5|0.03456|4340.3|
`

func TestUserGCmainSample(t *testing.T) {
	result, err := Parse([]byte(userGCmainSample), "GCmain_user.mvsv")
	if err != nil {
		t.Fatalf("用户真实样例解析失败: %v", err)
	}
	if len(result.Rows) != 6 {
		t.Fatalf("行数=%d，期望 6", len(result.Rows))
	}
	first := result.Rows[0]
	if first.SecuCode != "GCMain" || first.MarketCode != 1320 {
		t.Fatalf("证券标识错误: %+v", first)
	}
	if first.Date == nil || *first.Date != 20260101 {
		t.Fatalf("date=%v", first.Date)
	}
	if first.Time == nil || *first.Time != 0 {
		t.Fatalf("time=%v (期望 0=000000)", first.Time)
	}
	if first.Turnover == nil || *first.Turnover != "0" {
		t.Fatalf("turnover=%v (a 列)", first.Turnover)
	}
	if first.PrevClose == nil || *first.PrevClose != "4332.1" {
		t.Fatalf("prev_close=%v", first.PrevClose)
	}
}

// d 列与 ts 不一致时（用户原始样例的日期拼写错误）应报一致性错误
func TestUserGCmainSampleTsMismatch(t *testing.T) {
	content := replaceOnce(userGCmainSample, "1767308460|20260101|180100", "1767308460|20260102|180100")
	_, err := Parse([]byte(content), "GCmain_bad.mvsv")
	if err == nil {
		t.Fatal("期望 ts 与 d/t 不一致错误（用户样例日期拼写错误场景）")
	}
}

func replaceOnce(s, old, new string) string {
	idx := indexOf(s, old)
	if idx < 0 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
