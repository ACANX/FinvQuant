// Package calc 提供行情字段的十进制精确计算（避免 float64 精度损失）。
package calc

import (
	"math/big"
	"strings"
)

// Change 计算涨跌额与涨跌幅（百分比）。
//
// 规则（ACANX 2026-10-24）：
//  1. 优先用 last_close 作为参考价（前收盘价）；
//  2. last_close 缺失或为 0 时，用 open 作为参考价（有值且非 0）；
//  3. 两者都不可用（或 close 缺失）时返回 nil,nil，表示不写该字段；
//  4. change_price = close - ref；change_ratio = (close - ref) / ref * 100。
//
// 参数为数据库/文件的十进制文本，内部用 big.Rat 精确运算，结果保留 6 位小数
// （与 numeric(20,6) 精度一致）。
func Change(closePrice, lastClose, open *string) (*string, *string) {
	closeRat, ok := parse(closePrice)
	if !ok {
		return nil, nil
	}
	ref, ok := parse(lastClose)
	if !ok || ref.Sign() == 0 {
		ref, ok = parse(open)
		if !ok || ref.Sign() == 0 {
			return nil, nil
		}
	}
	diff := new(big.Rat).Sub(closeRat, ref)
	ratio := new(big.Rat).Quo(new(big.Rat).Mul(diff, big.NewRat(100, 1)), ref)
	changePrice := diff.FloatString(6)
	changeRatio := ratio.FloatString(6)
	return &changePrice, &changeRatio
}

// parse 解析十进制文本；空值/非法值返回 false。
func parse(value *string) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	text := strings.TrimSpace(*value)
	if text == "" {
		return nil, false
	}
	rat, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false
	}
	return rat, true
}
