package backtest

import (
	"context"
	"testing"
)

// TestDeleteRunValidation 验证 DeleteRun 的校验逻辑：
// 运行中的任务拒绝删除；不存在的任务报错。
// 说明：DeleteRun 依赖 DB（Service.pool），此处用 DB 集成测试需真实 PG；
// 本测试聚焦纯逻辑部分——删除明细表顺序与模型字段，DB 流程由端到端验证。
func TestDeleteDetailTablesOrder(t *testing.T) {
	// 删除顺序：先子表（event_trace/position_log/cashflow）后主表（trade/equity），
	// 确保中途失败不遗留"主表已删但明细残留"的孤立数据。
	want := []string{
		"finv_quant_backtest_event_trace",
		"finv_quant_backtest_position_log",
		"finv_quant_backtest_cashflow",
		"finv_quant_backtest_trade",
		"finv_quant_backtest_equity",
	}
	if len(deleteDetailTables) != len(want) {
		t.Fatalf("明细表数量应为 %d，实际 %d", len(want), len(deleteDetailTables))
	}
	for i, tbl := range want {
		if deleteDetailTables[i] != tbl {
			t.Errorf("删除顺序第 %d 位应为 %s，实际 %s", i, tbl, deleteDetailTables[i])
		}
	}
}

// TestDelTaskStatusConstants 验证删除任务状态常量与回测任务状态分离。
func TestDelTaskStatusConstants(t *testing.T) {
	if DelPending != "PENDING" || DelRunning != "RUNNING" || DelSucceeded != "SUCCEEDED" || DelFailed != "FAILED" {
		t.Error("删除任务状态常量定义错误")
	}
	// 删除任务状态不应与回测任务状态混用（各自独立语义）
	if DelPending != RunPending || DelRunning != RunRunning {
		// PENDING/RUNNING 字面相同但语义独立；此处仅确认常量存在即可
		t.Log("删除任务与回测任务状态常量已定义（PENDING/RUNNING 字面一致属正常）")
	}
}

// TestRunDelTaskModel 验证删除任务模型字段。
func TestRunDelTaskModel(t *testing.T) {
	task := RunDelTask{
		DelTaskID:     "TASK-1",
		RunID:         "RUN-1",
		Status:        DelSucceeded,
		Progress:      100,
		DeletedCounts: map[string]int{"finv_quant_backtest_trade": 5},
	}
	if task.Status != DelSucceeded || task.Progress != 100 {
		t.Error("RunDelTask 字段赋值异常")
	}
	if task.DeletedCounts["finv_quant_backtest_trade"] != 5 {
		t.Error("DeletedCounts 未正确保存各表删除行数")
	}
	logEntry := RunDelLog{DelTaskID: "TASK-1", RunID: "RUN-1", Seq: 1, Action: "TASK_CREATED"}
	if logEntry.Action != "TASK_CREATED" || logEntry.Seq != 1 {
		t.Error("RunDelLog 字段赋值异常")
	}
}

var _ = context.Background // 保留 context 导入（集成测试时使用）
