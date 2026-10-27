package dedup

import "testing"

func TestDecideCDEUStatus(t *testing.T) {
	children := func(statuses ...string) []cdeuChild {
		result := make([]cdeuChild, 0, len(statuses))
		for index, status := range statuses {
			result = append(result, cdeuChild{
				Code:     string(rune('A' + index)),
				Status:   status,
				Critical: true,
				Affects:  true,
			})
		}
		return result
	}

	tests := []struct {
		name     string
		rule     CDEUCompletionRule
		children []cdeuChild
		want     string
	}{
		{"全部成功", CDEUCompletionRule{Mode: "ALL"}, children(CDEUStatusSuccess, CDEUStatusSuccess), CDEUStatusSuccess},
		{"存在运行中", CDEUCompletionRule{Mode: "ALL"}, children(CDEUStatusSuccess, CDEUStatusRunning), CDEUStatusRunning},
		{"关键失败", CDEUCompletionRule{Mode: "ALL"}, children(CDEUStatusFailed, CDEUStatusSuccess), CDEUStatusFailed},
		{"非关键失败产生 PARTIAL", CDEUCompletionRule{Mode: "CRITICAL_ALL"}, []cdeuChild{
			{Code: "A", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "B", Status: CDEUStatusFailed, Critical: false, Affects: true},
		}, CDEUStatusPartial},
		{"至少一个", CDEUCompletionRule{Mode: "AT_LEAST", MinCount: intPtr(1)}, []cdeuChild{
			{Code: "A", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "B", Status: CDEUStatusFailed, Critical: false, Affects: true},
		}, CDEUStatusPartial},
		{"比例不足", CDEUCompletionRule{Mode: "AT_LEAST", MinRatio: floatPtr(0.8)}, []cdeuChild{
			{Code: "A", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "B", Status: CDEUStatusFailed, Critical: false, Affects: true},
		}, CDEUStatusFailed},
		{"任意一个", CDEUCompletionRule{Mode: "ANY_OF", Items: []string{"A"}}, []cdeuChild{
			{Code: "A", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "B", Status: CDEUStatusFailed, Critical: false, Affects: true},
		}, CDEUStatusPartial},
		{"忽略非参与项", CDEUCompletionRule{Mode: "ALL"}, []cdeuChild{
			{Code: "A", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "B", Status: CDEUStatusFailed, Critical: false, Affects: false},
		}, CDEUStatusSuccess},
		{"重复作业编码按实例计数", CDEUCompletionRule{Mode: "ALL"}, []cdeuChild{
			{Code: "JOB_EXPORT_LOAD", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "JOB_EXPORT_EXPORT", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "JOB_EXPORT_LOAD", Status: CDEUStatusSuccess, Critical: true, Affects: true},
			{Code: "JOB_EXPORT_EXPORT", Status: CDEUStatusSuccess, Critical: true, Affects: true},
		}, CDEUStatusSuccess},
		{"空集合默认为成功", CDEUCompletionRule{Mode: "ALL"}, nil, CDEUStatusSuccess},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _, _ := decideCDEUStatus(test.rule, test.children)
			if got != test.want {
				t.Fatalf("decideCDEUStatus() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestParseCDEUCompletionRuleDefault(t *testing.T) {
	rule := parseCDEUCompletionRule(nil)
	if rule.Mode != "ALL" {
		t.Fatalf("默认完成条件应为 ALL，实际 %s", rule.Mode)
	}
	bad := "{bad json"
	rule = parseCDEUCompletionRule(&bad)
	if rule.Mode != "ALL" {
		t.Fatalf("非法 JSON 应回退 ALL，实际 %s", rule.Mode)
	}
}

func intPtr(value int) *int           { return &value }
func floatPtr(value float64) *float64 { return &value }

// TestSubsequentJobDefinitions 校验手动续跑从当前 JOB 的内嵌阶段之后开始，且保持定义顺序。
func TestSubsequentJobDefinitions(t *testing.T) {
	definitions := []CDEUJobDefinition{
		{JobKind: JobCalcDailyIngestStats, JobCode: "JOB_CALC_INGEST_STATS"},
		{JobKind: JobRunDueExportJobs, JobCode: "JOB_FETCH_EXPORT_BATCH"},
		{JobKind: JobRunDueExportJobs, JobCode: "JOB_EXPORT_LOAD"},
		{JobKind: JobRunDueExportJobs, JobCode: "JOB_EXPORT_CLEANUP"},
		{JobKind: JobScanPostSealPatches, JobCode: "JOB_SCAN_PATCH"},
		{JobKind: JobUploadPendingExportFiles, JobCode: "JOB_UPLOAD_FILES"},
	}
	got := subsequentJobDefinitions(definitions, JobRunDueExportJobs)
	want := []string{JobScanPostSealPatches, JobUploadPendingExportFiles}
	if len(got) != len(want) {
		t.Fatalf("续跑 JOB 数量=%d，期望 %d", len(got), len(want))
	}
	for index := range want {
		if got[index].JobKind != want[index] {
			t.Fatalf("第 %d 个续跑 JOB=%s，期望 %s", index, got[index].JobKind, want[index])
		}
	}
	if len(subsequentJobDefinitions(definitions, "UNKNOWN")) != 0 {
		t.Fatal("未知 JOB 不应产生续跑清单")
	}
}
