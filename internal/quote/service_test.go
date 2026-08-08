package quote

import (
	"strings"
	"testing"
	"time"
)

func TestBuildUpsertSQLFieldMode(t *testing.T) {
	sql := buildUpsertSQL(UpsertModeField)
	if !strings.Contains(sql, "ON CONFLICT (ts, secu_code) DO UPDATE SET") {
		t.Fatal("缺少 ON CONFLICT 子句（应为 (ts, secu_code)，V21 移除 market_code）")
	}
	if strings.Contains(sql, "market_code") {
		t.Fatal("upsert 不应再包含 market_code 列（V21 已移除）")
	}
	if !strings.Contains(sql, "close = COALESCE(EXCLUDED.close, finv_quote_secu_kline_min.close)") {
		t.Fatal("FIELD 模式应使用 COALESCE 字段级覆盖")
	}
	if !strings.Contains(sql, "RETURNING (xmax = 0) AS is_insert") {
		t.Fatal("缺少 RETURNING 区分新增/覆盖")
	}
	if strings.Contains(sql, "COALESCE") == false {
		t.Fatal("FIELD 模式必须包含 COALESCE")
	}
}

func TestBuildUpsertSQLRowMode(t *testing.T) {
	sql := buildUpsertSQL(UpsertModeRow)
	if strings.Contains(sql, "COALESCE") {
		t.Fatal("ROW 模式不应包含 COALESCE")
	}
	if !strings.Contains(sql, "close = EXCLUDED.close") {
		t.Fatal("ROW 模式应整行覆盖")
	}
}

func TestBuildUpsertSQLCoversAllUpdatableColumns(t *testing.T) {
	sql := buildUpsertSQL(UpsertModeField)
	for _, column := range []string{"date", `"time"`, "prev_close", "open", "high", "low",
		"close", "paocd", "volume", "turnover", "ext_field", "remark"} {
		if !strings.Contains(sql, column) {
			t.Fatalf("upsert 未覆盖列: %s", column)
		}
	}
}

func TestDateToTS(t *testing.T) {
	// 2026-07-16 00:00:00 UTC
	ts := DateToTS(20260716)
	// 用 Go 时间库验证
	expect := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	if ts != expect {
		t.Errorf("DateToTS(20260716) = %d, 期望 %d", ts, expect)
	}
}

func TestDateRangeToTS(t *testing.T) {
	// days=1：单日 2026-07-16 全天（00:00:00 ~ 23:59:59 UTC）
	start, end := DateRangeToTS(20260716, 1)
	dayStart := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	if start != dayStart {
		t.Errorf("days=1 start 应为当日 00:00:00（%d），实际 %d", dayStart, start)
	}
	if end != dayStart+86400-1 {
		t.Errorf("days=1 end 应为当日 23:59:59（%d），实际 %d", dayStart+86400-1, end)
	}

	// days=5：2026-07-16 往前 5 天连续范围（07-12 00:00:00 ~ 07-16 23:59:59 UTC）
	start5, end5 := DateRangeToTS(20260716, 5)
	start5Expect := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC).Unix()
	if start5 != start5Expect {
		t.Errorf("days=5 start 应为 07-12 00:00:00（%d），实际 %d", start5Expect, start5)
	}
	// end5 应为 07-16 23:59:59
	end5Expect := time.Date(2026, 7, 16, 23, 59, 59, 0, time.UTC).Unix()
	if end5 != end5Expect {
		t.Errorf("days=5 end 应为 07-16 23:59:59（%d），实际 %d", end5Expect, end5)
	}
}
