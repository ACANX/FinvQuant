package cdeu

import (
	"strconv"
	"strings"
	"testing"
)

// 本文件是「待录入证券补录预填」（PendingSecuPrefill）的**纯逻辑回归**：
// 不需要数据库，只校验拼进 SQL 的表达式用了键表真实存在的列。
//
// 背景（需求 11286，2026-11-18 线上故障）：
// 预填 SQL 的 FROM 是键表 futu.finv_futu_secu_key（别名 t），该表只有 instrument，
// 没有 instrument_type；实现却沿用了快照表的 `t.instrument_type`，于是每次预填都报
// `column t.instrument_type does not exist (SQLSTATE 42703)`，
// 「证券参数补全 → 采集参数」与「待采证券录入」两个入口全部点不动。
func TestPendingSecuInstrumentTypeExprUsesKeyTableColumn(t *testing.T) {
	expr := pendingSecuInstrumentTypeExpr

	if strings.Contains(expr, "instrument_type") {
		t.Fatalf("预填表达式不得引用键表不存在的 instrument_type 列：\n%s", expr)
	}
	if !strings.Contains(expr, "btrim(t.instrument)") {
		t.Fatalf("预填表达式必须作用在键表的 t.instrument 列上：\n%s", expr)
	}
	// 归一后的目标值是整型，供采集表 instrument_type 直接比对
	if !strings.Contains(expr, "THEN 3") || !strings.Contains(expr, "ELSE -1") {
		t.Fatalf("预填表达式应把词表归一成整型、未知取 -1：\n%s", expr)
	}
}

// TestPendingSecuInstrumentTypeCasesInvertMapping 映射表必须与写入侧
// instrumentByInstrumentType 闭合（同一品种在两侧不能对不上，否则参考行取不到）。
//
// 校验方向：写入侧产出的每个词，反向映射表都必须映射回同一个 instrument_type。
// 反向的别名（sector / future / forex …）与数字形态不参与校验，它们只用于兼容历史数据。
func TestPendingSecuInstrumentTypeCasesInvertMapping(t *testing.T) {
	// 写入侧只覆盖这 5 种（其余默认 stock），故只校验它覆盖的范围
	for _, value := range []int{3, 4, 6, 7, 11} {
		word := instrumentByInstrumentType(value)
		found := false
		for _, item := range pendingSecuInstrumentTypeCases {
			if item.Key == word {
				found = true
				if item.Value != value {
					t.Fatalf("映射不互逆：写入侧 %d → %q，但反向映射表把它归到 %d",
						value, word, item.Value)
				}
				break
			}
		}
		if !found {
			t.Fatalf("反向映射表缺少写入侧词表 %q（instrument_type=%d）", word, value)
		}
	}
	_ = strconv.Atoi // 数字形态由 TestPendingSecuInstrumentTypeExprCoversCommonInstruments 覆盖
}

// TestPendingSecuInstrumentTypeExprCoversCommonInstruments 常见品种都必须被归一
// （否则 market_code 取不到参考值，前端必填校验必然失败 —— 需求 11164 的同一类问题）。
func TestPendingSecuInstrumentTypeExprCoversCommonInstruments(t *testing.T) {
	cases := map[string]int{
		"stock": 3, "STOCK": 3, "3": 3, "12": 3,
		"etf":     4,
		"index":   6,
		"sectors": 7, "sector": 7,
		"futures": 9, "future": 9,
		"forex":  10,
		"crypto": 11,
	}
	// 映射表里每个词都有定义，且大小写不敏感（SQL 用 lower(btrim())）
	for want, wantValue := range cases {
		key := strings.ToLower(want)
		found := false
		for _, item := range pendingSecuInstrumentTypeCases {
			if item.Key == key {
				found = true
				if item.Value != wantValue {
					t.Fatalf("%q 归一值应为 %d，实际 %d", want, wantValue, item.Value)
				}
				break
			}
		}
		if !found {
			t.Fatalf("映射表缺少品种 %q（会导致取不到参考行）", want)
		}
	}
	if !strings.Contains(pendingSecuInstrumentTypeExpr, "lower(btrim(") {
		t.Fatalf("表达式需对大小写/空白做归一：\n%s", pendingSecuInstrumentTypeExpr)
	}
}

// TestPendingSecuInstrumentTypeNormColumn 预填 SQL 必须同时给出归一列
// （instrument_type_norm），否则 Go 侧只能对词表做 Atoi，ETF/指数等品种会一律退化成股票。
func TestPendingSecuInstrumentTypeNormColumn(t *testing.T) {
	if !strings.Contains(pendingSecuInstrumentTypeExpr, "CASE") {
		t.Fatalf("归一表达式应为 CASE：\n%s", pendingSecuInstrumentTypeExpr)
	}
	// 归一列由同一表达式生成，保证「匹配参考行」与「预填默认值」用的是同一套映射
	built := buildPendingSecuInstrumentTypeExpr("t.instrument")
	if built != pendingSecuInstrumentTypeExpr {
		t.Fatalf("两次生成结果不一致（生成器必须确定性）")
	}
	if !strings.Contains(built, "'etf'") || !strings.Contains(built, "'crypto'") {
		t.Fatalf("归一表达式应覆盖 etf / crypto 等词表：\n%s", built)
	}
}
