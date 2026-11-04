package cdeu

import "testing"

// applyPhaseOrdering 复刻 attachCDEURun 末尾的阶段回填与派生逻辑，便于单测。
// 需求 10940：前置阶段未成功时，后续阶段不得回填为 SUCCESS。
func applyPhaseOrdering(phases []*CDEUPhaseView) {
	blocked := false
	for _, phase := range phases {
		if !blocked && phase.Status == CDEUStatusPending &&
			phase.TotalCount > 0 && phase.SuccessCount == phase.TotalCount {
			phase.Status = CDEUStatusSuccess
		}
		deriveCDEURunning(phase)
		if phase.Status != CDEUStatusSuccess {
			blocked = true
		}
	}
}

// TestPhaseOrderingBlocksLaterSuccess 上传仍在跑时，归档复核不得显示成功。
func TestPhaseOrderingBlocksLaterSuccess(t *testing.T) {
	upload := &CDEUPhaseView{
		PhaseCode: "PHASE_UPLOAD", SeqNo: 5, Status: CDEUStatusRunning,
		TotalCount: 1, SuccessCount: 0,
		Processes: []CDEUProcessView{{Status: CDEUStatusPending}},
	}
	review := &CDEUPhaseView{
		PhaseCode: "PHASE_REVIEW", SeqNo: 6, Status: CDEUStatusPending,
		TotalCount: 5, SuccessCount: 5,
		Processes: []CDEUProcessView{
			{Status: CDEUStatusSuccess},
			{Status: CDEUStatusSuccess},
		},
	}
	applyPhaseOrdering([]*CDEUPhaseView{upload, review})
	if review.Status != CDEUStatusPending {
		t.Fatalf("前置阶段 RUNNING 时后续阶段应为 PENDING，实际 %s", review.Status)
	}
}

// TestPhaseOrderingAllowsLaterSuccess 前置全部成功时，后续阶段可回填成功。
func TestPhaseOrderingAllowsLaterSuccess(t *testing.T) {
	export := &CDEUPhaseView{PhaseCode: "PHASE_EXPORT", SeqNo: 4, Status: CDEUStatusSuccess}
	review := &CDEUPhaseView{
		PhaseCode: "PHASE_REVIEW", SeqNo: 6, Status: CDEUStatusPending,
		TotalCount: 3, SuccessCount: 3,
	}
	applyPhaseOrdering([]*CDEUPhaseView{export, review})
	if review.Status != CDEUStatusSuccess {
		t.Fatalf("前置阶段全部成功时后续应回填 SUCCESS，实际 %s", review.Status)
	}
}

// TestPhaseOrderingBlocksOnFailed 前置阶段失败时，后续阶段不得回填成功。
func TestPhaseOrderingBlocksOnFailed(t *testing.T) {
	export := &CDEUPhaseView{PhaseCode: "PHASE_EXPORT", SeqNo: 4, Status: CDEUStatusFailed}
	upload := &CDEUPhaseView{
		PhaseCode: "PHASE_UPLOAD", SeqNo: 5, Status: CDEUStatusPending,
		TotalCount: 1, SuccessCount: 1,
	}
	applyPhaseOrdering([]*CDEUPhaseView{export, upload})
	if upload.Status != CDEUStatusPending {
		t.Fatalf("前置阶段 FAILED 时后续阶段应为 PENDING，实际 %s", upload.Status)
	}
}
