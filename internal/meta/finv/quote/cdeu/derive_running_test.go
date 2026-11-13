package cdeu

import (
	"testing"
	"time"
)

// TestDeriveCDEURunning 校验「子级在跑而父级仍 PENDING」时派生为 RUNNING（需求 10919）。
func TestDeriveCDEURunning(t *testing.T) {
	started := time.Now().Add(-3 * time.Minute)
	// 1) 子流程有 RUNNING 的 JOB → 阶段 RUNNING
	phase := &CDEUPhaseView{
		Status: CDEUStatusPending,
		Processes: []CDEUProcessView{
			{Status: CDEUStatusPending, Jobs: []CDEUJobView{{Status: CDEUStatusRunning}}},
		},
	}
	deriveCDEURunning(phase)
	if phase.Status != CDEUStatusRunning {
		t.Fatalf("应派生为 RUNNING，得到 %s", phase.Status)
	}
	if phase.Processes[0].Status != CDEUStatusRunning {
		t.Fatalf("子流程也应派生为 RUNNING，得到 %s", phase.Processes[0].Status)
	}

	// 2) 子流程 RUNNING（无 JOB 明细）→ 阶段 RUNNING
	phase = &CDEUPhaseView{
		Status:    CDEUStatusPending,
		Processes: []CDEUProcessView{{Status: CDEUStatusRunning}},
	}
	deriveCDEURunning(phase)
	if phase.Status != CDEUStatusRunning {
		t.Fatalf("子流程 RUNNING 时阶段应 RUNNING，得到 %s", phase.Status)
	}

	// 3) 阶段已开始未结束 → RUNNING
	phase = &CDEUPhaseView{Status: CDEUStatusPending, StartedAt: &started}
	deriveCDEURunning(phase)
	if phase.Status != CDEUStatusRunning {
		t.Fatalf("已开始未结束应 RUNNING，得到 %s", phase.Status)
	}

	// 4) 子级全 PENDING 且阶段未开始 → 保持 PENDING
	phase = &CDEUPhaseView{
		Status:    CDEUStatusPending,
		Processes: []CDEUProcessView{{Status: CDEUStatusPending, Jobs: []CDEUJobView{{Status: CDEUStatusPending}}}},
	}
	deriveCDEURunning(phase)
	if phase.Status != CDEUStatusPending {
		t.Fatalf("未开始时应保持 PENDING，得到 %s", phase.Status)
	}

	// 5) 已终态不被改写
	phase = &CDEUPhaseView{
		Status:    CDEUStatusFailed,
		Processes: []CDEUProcessView{{Status: CDEUStatusRunning}},
	}
	deriveCDEURunning(phase)
	if phase.Status != CDEUStatusFailed {
		t.Fatalf("终态不应被改写，得到 %s", phase.Status)
	}
}
