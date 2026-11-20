package cdeu

// 补全预填用的「时区」推断（需求 11165）。
//
// `public.finv_security` 有两列时区信息（口径见 Docs/DataDictMapping/FinvSecurity.md）：
//   - `timezone`：UTC 偏移字符串，如 `+08:00` / `-04:00`；
//   - `tz`：IANA 时区标识，如 `Asia/Shanghai` / `America/New_York`。
//
// 补全对话框原来这两列一律留空（"选填，留空即为空"），实际录入时人工并不知道该填什么，
// 结果新行没有时区信息。本文件按「市场 → 交易所 → 区域」三层推断缺省值：
//
//  1. **交易所级参考**：同 `exchange_code` 的已有 `finv_security` 行里 `(timezone, tz)` 的众数
//     —— 与库里既有数据口径完全一致（HK/CN/US 都能直接命中真实分布）；
//  2. **区域级参考**：同 `region` 的已有行众数（新交易所还没有自己的行时兜底）；
//  3. **内置区域映射**：库里完全没有参考行时按区域表给 IANA 时区，偏移按该 IANA 时区
//     **在当前时刻**的真实偏移推算（这样冬令时会给 `-05:00`，不会像历史数据那样恒为 `-04:00`）。
//
// 三层都拿不到（区域是 `N/A`、纯场外等）时保持留空，并明确告诉人工必须自己填。

import (
	"strings"
	"time"
)

// tzInference 时区推断结果。
type tzInference struct {
	// Timezone UTC 偏移字符串（`+08:00` / `-04:00`），空串表示推断不出。
	Timezone string
	// TZ IANA 时区标识（`Asia/Shanghai`）。
	TZ string
	// Source 预填来源（reference / dictionary / default）。
	Source string
	// Note 说明文字（写进字段提示，告诉人工这个值怎么来的、要不要核对）。
	Note string
}

// regionTZ 内置「区域 → IANA 时区」映射（需求 11165 的第三层兜底）。
//
// 区域取值来自 `public.finv_exchange.region`（实测 CN/HK/TW/US/JP/SG/IN/AU/KR/MY/CA/CC/FX/N/A）。
var regionTZ = map[string]string{
	"CN": "Asia/Shanghai",
	"HK": "Asia/Hong_Kong",
	"TW": "Asia/Taipei",
	"JP": "Asia/Tokyo",
	"KR": "Asia/Seoul",
	"SG": "Asia/Singapore",
	"MY": "Asia/Kuala_Lumpur",
	"IN": "Asia/Kolkata",
	"AU": "Australia/Sydney",
	"CA": "America/Toronto",
	"US": "America/New_York",
	// 加密与外汇是 24 小时连续交易，统一用 UTC
	"CC": "Etc/UTC",
	"FX": "Etc/UTC",
}

// inferSecurityTZ 按「交易所参考 → 区域参考 → 内置区域映射」推断时区缺省值。
//
// 参数都是数据库侧取回的值：
//   - exchangeTimezone / exchangeTZ：同 exchange_code 已有行的众数（可空）；
//   - regionTimezone / regionTZ：同 region 已有行的众数（可空）；
//   - region：`public.finv_exchange.region`（可空）。
//
// at 为推断时刻，用于把 IANA 时区换算成当前的 UTC 偏移（冬夏令时会不同）。
func inferSecurityTZ(exchangeTimezone, exchangeTZ, regionTimezone, regionTZName, region string,
	at time.Time) tzInference {
	if tz := strings.TrimSpace(exchangeTZ); tz != "" {
		return tzInference{
			Timezone: strings.TrimSpace(exchangeTimezone),
			TZ:       tz,
			Source:   PrefillSourceReference,
			Note:     "同交易所（exchange_code）已有证券行的众数，请核对",
		}
	}
	if tz := strings.TrimSpace(regionTZName); tz != "" {
		return tzInference{
			Timezone: strings.TrimSpace(regionTimezone),
			TZ:       tz,
			Source:   PrefillSourceReference,
			Note:     "同区域（region=" + region + "）已有证券行的众数，请核对",
		}
	}

	normalizedRegion := strings.ToUpper(strings.TrimSpace(region))
	tzName, ok := regionTZ[normalizedRegion]
	if !ok {
		return tzInference{
			Source: PrefillSourceManual,
			Note:   "区域 " + region + " 没有内置时区映射，库里也没有参考行，需人工填写",
		}
	}
	return tzInference{
		Timezone: offsetForZone(tzName, at),
		TZ:       tzName,
		Source:   PrefillSourceDictionary,
		Note: "按区域 " + normalizedRegion + " 映射到 " + tzName +
			"，偏移按该时区当前时刻推算（冬夏令时不同，请核对）",
	}
}

// offsetForZone 把 IANA 时区在当前时刻的偏移格式化成 `+08:00` / `-04:00`。
//
// 取不到时区数据（极小概率：容器缺 zoneinfo 且未内嵌 tzdata）时返回空串，
// 由调用方把字段标成「需人工填写」而不是写一个猜的偏移。
func offsetForZone(tzName string, at time.Time) string {
	location, err := time.LoadLocation(tzName)
	if err != nil {
		// 退一步：UTC 时区一定有，非法名字时至少给 +00:00 之外的答案没有意义，直接留空
		return ""
	}
	offset := at.In(location).Format("-07:00")
	if offset == "+00:00" || offset == "-00:00" {
		return "+00:00"
	}
	return offset
}

// fields 把推断结果整理成 `finv_security` 的两个预填字段（`timezone` 存偏移、`tz` 存 IANA 标识）。
func (t tzInference) fields() []PendingSecuPrefillField {
	note := strings.TrimSpace(t.Note)
	if note == "" {
		note = "选填，留空即为空"
	}
	return []PendingSecuPrefillField{
		{Key: "timezone", Label: "timezone 时区偏移", Value: t.Timezone, Source: t.Source,
			Note: note + "（`timezone` 存 UTC 偏移，如 +08:00 / -04:00）"},
		{Key: "tz", Label: "tz 时区标识", Value: t.TZ, Source: t.Source,
			Note: note + "（`tz` 存 IANA 标识，如 Asia/Shanghai / America/New_York）"},
	}
}

// warnings 推断不出时区时给一条风险提示（人工必须自己填）。
func (t tzInference) warnings(region string) []string {
	if t.TZ != "" || t.Timezone != "" {
		return nil
	}
	return []string{"时区推断不出来（区域 " + region + " 无内置映射、库里也没有参考行），timezone / tz 需人工填写"}
}
