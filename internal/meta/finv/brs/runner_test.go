package brs

import (
	"errors"
	"testing"
	"time"
)

// TestNextRuns 校验未来执行时间预览：每天 03:00:00 应产生间隔 1 天的 5 条。
func TestNextRuns(t *testing.T) {
	from := time.Date(2026, 11, 1, 4, 0, 0, 0, time.Local)
	got := nextRuns("0 0 3 * * *", from, 5)
	if len(got) != 5 {
		t.Fatalf("期望 5 条，实际 %d 条：%v", len(got), got)
	}
	if got[0] != "2026-11-02 03:00:00" {
		t.Errorf("首条应为 2026-11-02 03:00:00，实际 %s", got[0])
	}
	if got[4] != "2026-11-06 03:00:00" {
		t.Errorf("末条应为 2026-11-06 03:00:00，实际 %s", got[4])
	}
}

// TestNextRunsInvalid 非法表达式返回空切片而不是 panic。
func TestNextRunsInvalid(t *testing.T) {
	if got := nextRuns("0 0 3 * *", time.Now(), 5); len(got) != 0 {
		t.Errorf("非法表达式应返回空切片，实际 %v", got)
	}
}

// TestRunnerBusy 已有运行时再次触发应返回 *BusyError（不会真正启动执行）。
func TestRunnerBusy(t *testing.T) {
	r := NewRunner(nil, Options{})
	started := time.Now().Add(-time.Minute)
	r.mu.Lock()
	r.state.Status = StatusRunning
	r.state.Trigger = TriggerAuto
	r.state.StartedAt = &started
	r.mu.Unlock()

	if _, err := r.TriggerManual(); err == nil {
		t.Fatal("运行中再次触发应返回错误")
	} else {
		var busy *BusyError
		if !errors.As(err, &busy) {
			t.Fatalf("错误类型应为 *BusyError，实际 %T: %v", err, err)
		}
		if busy.RunningTrigger != TriggerAuto {
			t.Errorf("触发来源应为 auto，实际 %s", busy.RunningTrigger)
		}
	}
}

// TestRunnerStatusSinceSeq 校验日志增量拉取与 seq 单调递增。
func TestRunnerStatusSinceSeq(t *testing.T) {
	r := NewRunner(nil, Options{})
	r.appendLog("info", "first")
	r.appendLog("warn", "second")
	r.appendLog("error", "third")

	all := r.Status(0)
	if len(all.Logs) != 3 {
		t.Fatalf("期望 3 条日志，实际 %d", len(all.Logs))
	}
	if all.LastSeq != 3 {
		t.Errorf("LastSeq 应为 3，实际 %d", all.LastSeq)
	}
	for i := 1; i < len(all.Logs); i++ {
		if all.Logs[i].Seq <= all.Logs[i-1].Seq {
			t.Fatalf("seq 未递增：%v", all.Logs)
		}
	}

	incremental := r.Status(all.Logs[0].Seq)
	if len(incremental.Logs) != 2 || incremental.Logs[0].Message != "second" {
		t.Errorf("增量拉取结果不符：%v", incremental.Logs)
	}
}

// TestRunnerLogLimit 日志超过上限时丢弃最旧条目，seq 不回退。
func TestRunnerLogLimit(t *testing.T) {
	r := NewRunner(nil, Options{})
	for i := 0; i < maxLogEntries+5; i++ {
		r.appendLog("info", "line")
	}
	st := r.Status(0)
	if len(st.Logs) != maxLogEntries {
		t.Fatalf("期望保留 %d 条，实际 %d", maxLogEntries, len(st.Logs))
	}
	if st.LastSeq != int64(maxLogEntries+5) {
		t.Errorf("LastSeq 应为 %d，实际 %d", maxLogEntries+5, st.LastSeq)
	}
}
