package exportarchive

import (
	"testing"
	"time"
)

// TestBuildAlerts 校验告警阈值（方案稿 5.3）。
func TestBuildAlerts(t *testing.T) {
	clean := Metrics{PatchLast7Days: 1, BudgetFactor: 1.2, FrozenMissingRatio: 0}
	if alerts := BuildAlerts(clean, Options{}); len(alerts) != 0 {
		t.Fatalf("健康指标不应产生告警，实际 %+v", alerts)
	}

	unhealthy := Metrics{
		BacklogDays:              8,
		BudgetFactor:             1.9,
		HeartbeatTimeouts:        1,
		UploadRetryExhausted:     2,
		HighFrequencyPatchGroups: 1,
		StagingRows:              10,
		FrozenMissingRatio:       0.06,
		MvsvAnomalies:            3,
	}
	alerts := BuildAlerts(unhealthy, Options{})
	want := map[string]bool{
		"BACKLOG_TOO_HIGH":       false,
		"BUDGET_FACTOR_HIGH":     false,
		"HEARTBEAT_TIMEOUT":      false,
		"UPLOAD_RETRY_EXHAUSTED": false,
		"PATCH_FREQUENCY_HIGH":   false,
		"STAGING_RESIDUAL":       false,
		"FROZEN_MISSING_HIGH":    false,
		"MVSV_STRUCTURE_INVALID": false,
	}
	for _, alert := range alerts {
		if _, ok := want[alert.Code]; !ok {
			t.Fatalf("出现未预期告警码 %s", alert.Code)
		}
		if alert.Level == "" || alert.Message == "" {
			t.Fatalf("告警项缺少 level/message: %+v", alert)
		}
		want[alert.Code] = true
	}
	for code, seen := range want {
		if !seen {
			t.Fatalf("缺少告警码 %s，实际 %+v", code, alerts)
		}
	}
}

// TestJobRuntimes 校验 10 个 Job 全部登记了调度配置且标识唯一。
func TestJobRuntimes(t *testing.T) {
	runtimes := JobRuntimes()
	if len(runtimes) != len(AllJobKinds()) {
		t.Fatalf("调度配置数量 %d 与 Job 数量 %d 不一致", len(runtimes), len(AllJobKinds()))
	}
	seen := map[string]bool{}
	for _, runtime := range runtimes {
		if runtime.Kind == "" || runtime.Schedule == "" {
			t.Fatalf("调度配置缺少 Kind/Schedule: %+v", runtime)
		}
		if seen[runtime.Kind] {
			t.Fatalf("Job 标识重复: %s", runtime.Kind)
		}
		seen[runtime.Kind] = true
		if runtime.DailyAt == "" && runtime.Interval <= 0 {
			t.Fatalf("Job %s 既无每日时刻也无间隔", runtime.Kind)
		}
	}
	for _, kind := range AllJobKinds() {
		if !seen[kind] {
			t.Fatalf("Job %s 未登记调度配置", kind)
		}
	}
}

// TestSealDateMonths 校验收口日期跨年。
func TestSealDateMonths(t *testing.T) {
	cases := map[int]string{
		202612: "2027-01-25",
		202601: "2026-02-25",
		202602: "2026-03-25",
	}
	for month, want := range cases {
		if got := SealDateOf(month).Format("2006-01-02"); got != want {
			t.Fatalf("SealDateOf(%d)=%s，期望 %s", month, got, want)
		}
	}
}

// TestNowUsesLocation 校验业务时区辅助方法。
func TestNowUsesLocation(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("缺少时区数据: %v", err)
	}
	options := Options{Location: location}
	now := options.Now()
	if now.Location().String() != "Asia/Shanghai" {
		t.Fatalf("Now() 时区应为 Asia/Shanghai，实际 %s", now.Location())
	}
	today := options.Today()
	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 {
		t.Fatalf("Today() 应为当天 00:00:00，实际 %s", today)
	}
	if month := options.CurrentMonth(); month < 200001 || month > 299912 {
		t.Fatalf("CurrentMonth() 取值异常: %d", month)
	}
}
