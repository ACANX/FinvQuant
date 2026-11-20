package cdeu

// 采集参数（`futu.finv_futu_quote_collect`）补全的缺省推断（需求 11166）。
//
// 与规范证券参数一样，能由「市场 / 交易所 / 同市场已有配置行」推出来的字段一律给缺省值，
// 人工只做核对。本文件的三个纯函数负责可单测的那部分：
//
//   - normalizeInstrumentType：快照 `instrument_type` 是 TEXT 且 `12` 与 `3` 都是股票，
//     采集表存量以 `3` 为主 → 推缺省值时按 `3` 算（写库仍用快照原值，不擅自改下游拿到的品种码）；
//   - deriveMarketType：快照值 → 同市场已有配置行的众数 → 内置市场映射（HK=1 / US=2 / A股=4）；
//   - deriveReqSection：同市场同品种 `req_section` 的众数（没有参考行时留空，该列选填）。
//
// 背景：「按市场推 req_section」需要先落到数据上 —— 实测库里 26,529 行只有 2 行非空
// （`US` + `instrument_type=3` 的 `NVDA` / `SPCX` 填了 `1`），众数是空串，
// 所以当前这一列推出来仍是空；机制留着，等存量出现真实规律后会自动带出。

import (
	"fmt"
	"strings"
)

// normalizeInstrumentType 把快照的 instrument_type 归一到采集表的整型口径。
//
// `12` 与 `3` 都表示股票（采集表 26,494 行是 3、33 行是 12），推缺省值时统一按 3 算，
// 这样 `sub_instrument_type` 才会是 3002 而不是 12002。非法/缺失按股票（3）处理。
func normalizeInstrumentType(instrumentType int) int {
	switch instrumentType {
	case 4, 6, 7, 9, 10, 11:
		return instrumentType
	case 12:
		return 3
	default:
		return 3
	}
}

// deriveMarketType 推 market_type：快照值优先 → 同市场已有配置行众数 → 内置市场映射。
//
// 返回值和来源标记（快照 / 参考 / 字典）。快照值为 0 时才算「缺失」。
func deriveMarketType(snapshot int, refMode *int, marketLabel string) (int, string) {
	if snapshot > 0 {
		return snapshot, PrefillSourceSnapshot
	}
	if refMode != nil && *refMode > 0 {
		return *refMode, PrefillSourceReference
	}
	return marketTypeByLabel(marketLabel), PrefillSourceDictionary
}

// marketTypeByLabel 内置「市场标签 → 富途 market_type」映射（HK=1 / US=2 / A股=4，
// 外汇 FX=11 / 加密 CC=17 与 realtime_stock_enqueue.go 的口径保持一致）。
func marketTypeByLabel(marketLabel string) int {
	switch strings.ToUpper(strings.TrimSpace(marketLabel)) {
	case "HK":
		return 1
	case "US":
		return 2
	case "FX":
		return 11
	case "CC", "CRYPTO":
		return 17
	default:
		// A 股（SH / SZ / BJ）及其它未知市场
		return 4
	}
}

// marketTypeNote 给 market_type 字段写一句「这个值怎么来的」。
func marketTypeNote(source string, snapshot int, marketLabel string) string {
	switch source {
	case PrefillSourceSnapshot:
		return "取自快照 market_type"
	case PrefillSourceReference:
		return "同市场（" + marketLabel + "）已有配置行的众数，请核对"
	default:
		return fmt.Sprintf("按市场标签 %s 映射（HK=1 / US=2 / A股=4 / FX=11 / CC=17）", marketLabel)
	}
}

// deriveReqSection 推 req_section：同市场同品种已有配置行的众数。
//
// 该列在 `futu.finv_futu_quote_collect` 里是选填（实测 26,529 行只有 2 行非空），
// 所以推出来是空串很正常 —— 此时来源标「字典」而非「待填」，不要吓唬使用者必须填。
func deriveReqSection(refMode *string) (value, source, note string) {
	if refMode != nil {
		value = strings.TrimSpace(*refMode)
		if value != "" {
			return value, PrefillSourceReference,
				"同市场同品种已有配置行的众数（只有少数市场需要分节请求），请核对"
		}
		return "", PrefillSourceReference, "同市场同品种已有配置行的众数就是空（该列选填，多数市场不需要）"
	}
	return "", PrefillSourceDictionary, "同市场同品种没有可参照的配置行；该列选填，不知道就留空"
}

// instrumentTypeNote instrument_type 字段的说明。
//
// 快照的 `12` 与 `3` 都是股票（采集表 26,494 行是 3、33 行是 12）。这里**不擅自改成 3** ——
// 该列会被日K同步等下游当成富途品种码直接使用，改口径属于业务决策；只把事实写清楚，
// 让操作者自己决定是否与存量对齐。
func instrumentTypeNote(instrumentType int) string {
	if instrumentType == 12 {
		return "取自快照（12 与 3 同为股票）；采集表存量以 3 为主，如需与存量一致可改为 3，" +
			"推 sub_instrument_type 等缺省值时已按 3 计算"
	}
	return "取自快照 instrument_type"
}
