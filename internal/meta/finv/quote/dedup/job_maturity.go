package dedup

import "time"

// watermarkMaturity 成熟度增量扫描水位名。
const watermarkMaturity = "REFRESH_QUOTE_MATURITY"

// runRefreshQuoteMaturity 增量刷新成熟度并自动发现新 usc（J-01~J-05 / P1）。
// runMarkSealedJobsFrozen 收口冻结 + 差异报告（J-06 / J-07 / M-04）。
// firstDayOfMonth 返回 t 所在月份首日的 yyyyMMdd。
func firstDayOfMonth(t time.Time) int {
	return t.Year()*10000 + int(t.Month())*100 + 1
}

// maxSealedBizMonth 返回已过收口日（seal_date <= today）的最大归档月份。
func maxSealedBizMonth(today time.Time) int {
	month := today.Year()*100 + int(today.Month())
	for i := 0; i < 24; i++ {
		if !SealDateOf(month).After(today) {
			return month
		}
		month = addMonths(month, -1)
	}
	return month
}

// addMonths 在 yyyyMM 上加减月份。
func addMonths(month int, delta int) int {
	year, mon := month/100, time.Month(month%100)
	shifted := time.Date(year, mon, 1, 0, 0, 0, 0, time.UTC).AddDate(0, delta, 0)
	return shifted.Year()*100 + int(shifted.Month())
}
