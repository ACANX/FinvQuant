package mvsv

import "testing"

// 用户报错场景复现：13 列 Field（含已过时 pc）应能解析
func TestUser13ColumnPcFile(t *testing.T) {
	content := `# Title : "GCmain_Min_V3_2026_195279_2026072202"
# Format : "MVSV-1"
# Field : "ts|d|t|o|c|l|h|v|a|cp|cr|p|pc"
# FieldType : "Int|Long|Long|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal"
# FieldName : "Ts|Date|Time|Open|Close|Low|High|Volume|Amount|ChangePrice|ChangeRatio|PrevClose|Pc"
# 字段名称 : "时间戳(UTC)|日期|时间|开盘价|收盘价|最低价|最高价|成交量|成交额|涨跌值|涨跌幅(%)|前一收盘价|预留"
# Count : 3
# EffectiveTimeZone : "America/New_York"
# TimeZone : "America/New_York"
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
# DelistingFlag : 0
# ListedExchange : "COMEX"
# ListedBoard : ""
# Region : "US"
# Name : "黄金期货主连 (2608)"
# Period : "Min"
# Dsv : 3
# Year : 2026

1767394800|20260102|180000|4340.0|4338.8|4335.8|4343.0|300|0|6.7|0.154659|4332.1|999.0
1767394860|20260102|180100|4342.5|4342.4|4339.4|4345.5|315|0|10.3|0.23776|4332.1|999.0
1767394920|20260102|180200|4345.0|4346.0|4342.0|4349.0|330|0|13.9|0.320861|4332.1|999.0
`
	result, err := Parse([]byte(content), "GCmain_13col.mvsv")
	if err != nil {
		t.Fatalf("13 列（含 pc）文件解析失败: %v", err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("行数=%d，期望 3", len(result.Rows))
	}
	if result.Rows[0].Close == nil || *result.Rows[0].Close != "4338.8" {
		t.Fatalf("close=%v", result.Rows[0].Close)
	}
}
