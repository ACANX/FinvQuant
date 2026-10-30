package dedup

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// strPtr / int64Ptr 构造测试用指针。
func strPtr(value string) *string { return &value }
func int64Ptr(value int64) *int64 { return &value }

// TestClassifyExportDecision 校验导出决策的判定顺序与原因描述（需求 10832）。
//
// 判定顺序必须与 GenerateExportBatch / InsertCandidateJobs 的 WHERE 顺序一致：
// 元数据完整性 → 退市 → 当月 → 成熟度 → 已导出 → 可导出。
// 原因描述要求「指名道姓」：缺哪个字段、早于哪一天、差哪几个交易日、已导出的 job_id 与文件名。
func TestClassifyExportDecision(t *testing.T) {
	options := Options{
		OutputDir:    "/data/App/FinvQuant/Export",
		UploadLayout: UploadLayoutBySecurity,
		UploadPrefix: "Data/Finv/SecuQuoteMon/FTMM",
	}
	service := &Service{options: options}
	const currentMonth = 202611

	base := ExportDecisionMaterial{
		Usc:          "NVDA",
		BizMonth:     202604,
		Region:       strPtr("US"),
		Market:       strPtr("NSDQ"),
		Provider:     strPtr("FTMM"),
		Name:         strPtr("英伟达"),
		Timezone:     strPtr("America/New_York"),
		Currency:     strPtr("USD"),
		PriorityRank: 10,
		Days:         21,
		MatureDays:   21,
		ExpectedRows: 20160,
		ActualRows:   20160,
	}

	t.Run("可导出时给出文件名与两端路径", func(t *testing.T) {
		item := service.classifyExportDecision(base, currentMonth)
		if item.Verdict != ExportVerdictExportable || item.ReasonCode != ExportReasonOK {
			t.Fatalf("应判为可导出，实际 verdict=%s reason=%s", item.Verdict, item.ReasonCode)
		}
		const wantFile = "US_NSDQ_NVDA_Min_FTMM_Mon_202604_Base.mvsv"
		if item.FileName != wantFile {
			t.Fatalf("文件名不符：%s（期望 %s）", item.FileName, wantFile)
		}
		// 本地路径用 filepath.Join 拼接，断言同样跨平台（Windows 开发机与 Linux 部署一致）。
		wantLocal := filepath.Join("/data/App/FinvQuant/Export", "NVDA", "202604", wantFile)
		if item.LocalPath != wantLocal {
			t.Fatalf("本地路径不符：%s（期望 %s）", item.LocalPath, wantLocal)
		}
		wantRemote := "Data/Finv/SecuQuoteMon/FTMM/US_NSDQ/NVDA/" + wantFile
		if item.RemotePath != wantRemote {
			t.Fatalf("远端路径不符：%s（期望 %s）", item.RemotePath, wantRemote)
		}
		if !strings.Contains(item.ReasonDetail, "20160") {
			t.Fatalf("原因描述应包含行数，实际：%s", item.ReasonDetail)
		}
	})

	t.Run("元数据缺失要指名到字段", func(t *testing.T) {
		material := base
		material.Provider = nil
		material.Currency = strPtr("")
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonMetadataIncomplete {
			t.Fatalf("应判为元数据不全，实际 %s", item.ReasonCode)
		}
		for _, want := range []string{"provider（数据商）", "currency（币种）"} {
			if !strings.Contains(item.ReasonDetail, want) {
				t.Fatalf("原因描述应包含 %q，实际：%s", want, item.ReasonDetail)
			}
		}
	})

	t.Run("退市要带上退市日期与月首日", func(t *testing.T) {
		delist := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
		material := base
		material.DateDelist = &delist
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonDelisted {
			t.Fatalf("应判为已退市，实际 %s", item.ReasonCode)
		}
		for _, want := range []string{"2026-02-10", "2026-04-01"} {
			if !strings.Contains(item.ReasonDetail, want) {
				t.Fatalf("原因描述应包含 %q，实际：%s", want, item.ReasonDetail)
			}
		}
	})

	t.Run("当月不可导出", func(t *testing.T) {
		material := base
		material.BizMonth = currentMonth
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonCurrentMonth {
			t.Fatalf("应判为当月，实际 %s", item.ReasonCode)
		}
		if !strings.Contains(item.ReasonDetail, "202611") {
			t.Fatalf("原因描述应包含月份，实际：%s", item.ReasonDetail)
		}
	})

	t.Run("未成熟要列出未成熟交易日", func(t *testing.T) {
		material := base
		material.MatureDays = 19
		material.UnmatureDays = []int{20260413, 20260414}
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonImmature {
			t.Fatalf("应判为未成熟，实际 %s", item.ReasonCode)
		}
		for _, want := range []string{"共 21 个交易日", "仅 19 天", "2026-04-13", "2026-04-14"} {
			if !strings.Contains(item.ReasonDetail, want) {
				t.Fatalf("原因描述应包含 %q，实际：%s", want, item.ReasonDetail)
			}
		}
	})

	t.Run("未成熟日期超过上限时给出总数", func(t *testing.T) {
		material := base
		material.MatureDays = 0
		days := make([]int, 0, 12)
		for day := 1; day <= 12; day++ {
			days = append(days, 20260400+day)
		}
		material.UnmatureDays = days
		item := service.classifyExportDecision(material, currentMonth)
		if !strings.Contains(item.ReasonDetail, "等 12 天") {
			t.Fatalf("应给出未成熟总天数，实际：%s", item.ReasonDetail)
		}
	})

	t.Run("已导出要给出 job_id 与文件名", func(t *testing.T) {
		material := base
		material.JobID = int64Ptr(101)
		material.JobStatus = strPtr(JobStatusExported)
		material.BaseFileName = strPtr("US_NSDQ_NVDA_Min_FTMM_Mon_202604_Base.mvsv")
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonAlreadyExported {
			t.Fatalf("应判为已导出，实际 %s", item.ReasonCode)
		}
		for _, want := range []string{"job_id=101", JobStatusExported, "US_NSDQ_NVDA_Min_FTMM_Mon_202604_Base.mvsv"} {
			if !strings.Contains(item.ReasonDetail, want) {
				t.Fatalf("原因描述应包含 %q，实际：%s", want, item.ReasonDetail)
			}
		}
	})

	t.Run("已导出但文件未登记也要说清楚", func(t *testing.T) {
		material := base
		material.JobID = int64Ptr(102)
		material.JobStatus = strPtr(JobStatusExporting)
		item := service.classifyExportDecision(material, currentMonth)
		if !strings.Contains(item.ReasonDetail, "（文件尚未登记）") {
			t.Fatalf("应说明文件未登记，实际：%s", item.ReasonDetail)
		}
	})

	t.Run("判定顺序：元数据优先于已导出", func(t *testing.T) {
		material := base
		material.Region = nil
		material.JobID = int64Ptr(103)
		material.JobStatus = strPtr(JobStatusExported)
		item := service.classifyExportDecision(material, currentMonth)
		if item.ReasonCode != ExportReasonMetadataIncomplete {
			t.Fatalf("元数据缺失应优先报告，实际 %s", item.ReasonCode)
		}
	})
}
