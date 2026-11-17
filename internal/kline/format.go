package kline

import (
	"math/big"
	"strconv"
	"strings"
)

// parseRat 解析十进制文本为有理数（支持小数与科学计数法）。
func parseRat(text string) (*big.Rat, bool) {
	if strings.TrimSpace(text) == "" {
		return nil, false
	}
	return new(big.Rat).SetString(strings.TrimSpace(text))
}

// RatioText 把 change_ratio 输出为两位小数文本（ROUND_HALF_UP，四舍五入远离零）。
//
// 与 Python 实现保持一致：-0.004 → "-0.00"、-0.005 → "-0.01"、NULL → nil。
// 使用 big.Rat 做十进制精确运算，避免 float64 误差。
func RatioText(raw *string) *string {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	rat, ok := parseRat(text)
	if !ok {
		return nil
	}
	negative := strings.HasPrefix(text, "-")
	scaled := new(big.Rat).Mul(rat, big.NewRat(100, 1))

	num := new(big.Int).Abs(scaled.Num())
	den := scaled.Denom()
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	// 半值进位（HALF_UP）：2*remainder >= denominator 时进 1。
	if twice := new(big.Int).Lsh(remainder, 1); twice.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	digits := quotient.String()
	for len(digits) < 3 {
		digits = "0" + digits
	}
	out := digits[:len(digits)-2] + "." + digits[len(digits)-2:]
	if negative {
		out = "-" + out
	}
	return &out
}

// CellText 把 numeric 文本转为 /PageUni 单元格字符串：去掉无意义的尾随零，
// 并把 "-0"/"-0.000000" 归一为 "0"；空值保持 nil（JSON null）。
func CellText(raw *string) *string {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	if strings.ContainsAny(text, "eE") {
		if value, err := strconv.ParseFloat(text, 64); err == nil {
			text = strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "" || text == "-" || text == "-0" {
		text = "0"
	}
	return &text
}

// PageNumber 按 offset/limit 换算页码（从 1 开始；整除）。
func PageNumber(offset, limit int) int {
	if limit <= 0 {
		return 1
	}
	return offset/limit + 1
}

// SecondsText 把整型秒值转为字符串（/PageUni 用）。
func SecondsText(value int64) string {
	return strconv.FormatInt(value, 10)
}
