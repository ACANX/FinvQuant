package cdeu

import (
	"testing"

	jobdef "github.com/acanx/finvquant/internal/meta/finv/quote/cdeu/jobmeta"
)

// TestJobRuntimesCoverAllMeta 防止「定义了 jobmeta 却忘记登记进 All()」。
//
// 需求 10939：PATCH_OUTDATED_USC 的 jobmeta 定义写好了，但没加入 jobmeta.All()，
// 于是 JobRuntimes() 里没有它，手动触发时报「未知 Job: PATCH_OUTDATED_USC」。
func TestJobRuntimesCoverAllMeta(t *testing.T) {
	kinds := map[string]bool{}
	for _, runtime := range JobRuntimes() {
		kinds[runtime.Kind] = true
	}
	for _, meta := range jobdef.All() {
		if !kinds[meta.Kind] {
			t.Errorf("Job 定义 %s 出现在 jobmeta.All()，但 JobRuntimes() 没有它：手动触发会报「未知 Job」", meta.Kind)
		}
	}
	if len(kinds) == 0 {
		t.Fatal("JobRuntimes() 为空")
	}
}

// TestPatchOutdatedUscRegistered 确认过时 USC 映射 JOB 已登记且带每日调度。
func TestPatchOutdatedUscRegistered(t *testing.T) {
	for _, runtime := range JobRuntimes() {
		if runtime.Kind != JobPatchOutdatedUsc {
			continue
		}
		if runtime.DailyAt != "05:00" {
			t.Errorf("DailyAt = %q，期望 05:00", runtime.DailyAt)
		}
		if runtime.Name == "" {
			t.Error("Name 不能为空")
		}
		return
	}
	t.Fatalf("JobRuntimes() 里没有 %s", JobPatchOutdatedUsc)
}
