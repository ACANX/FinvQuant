package cdeu

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// 本文件覆盖「跨 provider 快照落库」的冲突语义（需求 11287）与
// 「采集后自动补人民币指标」的日志文案（需求 11288），均不需要数据库。

// TestQuoteSecuRealtimeUpdateClauseKeepsExistingValues 断言 SET 子句是「有值才覆盖」。
//
// 背景：BDFS 采集不提供 rmb_* / total_shares / trust_aum 等列；旧实现是
// `col = EXCLUDED.col`（整行覆盖），每采一轮就把这些列写成 NULL，
// 表现为「点过人民币指标按钮后跑一次北交所采集，rmb 指标被清零」。
func TestQuoteSecuRealtimeUpdateClauseKeepsExistingValues(t *testing.T) {
	updates := quoteSecuRealtimeUpdateClause()
	if len(updates) == 0 {
		t.Fatal("SET 子句不能为空")
	}

	byColumn := make(map[string]string, len(updates))
	for _, item := range updates {
		parts := strings.SplitN(item, " = ", 2)
		if len(parts) != 2 {
			t.Fatalf("SET 子句格式不符: %q", item)
		}
		byColumn[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		if strings.HasPrefix(parts[1], "EXCLUDED.") {
			t.Fatalf("列 %s 仍是整行覆盖（%s），会把采集未提供的值写成 NULL", parts[0], parts[1])
		}
		if !strings.HasPrefix(parts[1], "COALESCE(EXCLUDED.") {
			t.Fatalf("列 %s 应为 COALESCE 口径，实际 %s", parts[0], parts[1])
		}
	}

	if _, ok := byColumn["usc"]; ok {
		t.Fatal("主键 usc 不应出现在 SET 子句里")
	}
	// 人民币计价五列必须都在（它们是本 bug 的主角，不能被漏掉或写错）
	for _, col := range []string{
		"rmb_exchange_rate", "rmb_market_value", "rmb_circulation_market_value",
		"rmb_trade_trunover", "rmb_price_nominal",
	} {
		expr, ok := byColumn[col]
		if !ok {
			t.Fatalf("SET 子句缺少 %s", col)
		}
		want := "COALESCE(EXCLUDED." + col + ", t." + col + ")"
		if expr != want {
			t.Fatalf("%s 的表达式应为 %s，实际 %s", col, want, expr)
		}
	}
	// 其余列同样走 COALESCE（BDFS 不提供 total_shares / trust_aum 等）
	for _, col := range []string{"total_shares", "trust_aum", "leverage", "stock_id", "provider"} {
		if expr, ok := byColumn[col]; !ok || !strings.Contains(expr, "COALESCE(") {
			t.Fatalf("%s 也应保留旧值（COALESCE），实际 %q", col, expr)
		}
	}
}

// TestRmbSyncLogMessage 采集后自动补全人民币指标的日志文案（需求 11288）。
func TestRmbSyncLogMessage(t *testing.T) {
	// 成功：INFO，带汇率/金额行数与耗时
	result := RMBMetricsSyncResult{RateUpdate: 12, AmountUpdate: 340}
	level, message := rmbSyncLogMessage("北交所股票", result, nil, 1500*time.Millisecond)
	if level != "INFO" {
		t.Fatalf("成功应为 INFO，实际 %s", level)
	}
	for _, want := range []string{"人民币计价指标已自动补全", "北交所股票", "汇率 12 行", "金额 340 行", "1500 ms"} {
		if !strings.Contains(message, want) {
			t.Fatalf("日志缺少 %q：%s", want, message)
		}
	}

	// 缺汇率币种：升级为 WARN 并列出币种
	result.MissingCurrencies = []string{"HKD", "JPY"}
	level, message = rmbSyncLogMessage("沪深京ETF", result, nil, time.Second)
	if level != "WARN" {
		t.Fatalf("缺汇率时应为 WARN，实际 %s", level)
	}
	if !strings.Contains(message, "缺汇率币种：HKD、JPY") {
		t.Fatalf("应列出缺汇率币种：%s", message)
	}

	// 失败：WARN，带原因，且不改变采集结果状态（由调用方保证）
	level, message = rmbSyncLogMessage("证券列表采集", RMBMetricsSyncResult{}, errors.New("db down"), time.Second)
	if level != "WARN" || !strings.Contains(message, "失败") || !strings.Contains(message, "db down") {
		t.Fatalf("失败日志不符：%s / %s", level, message)
	}
}
