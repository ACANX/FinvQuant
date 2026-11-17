"""P5-008 独立紧急停止、只减仓和恢复审批测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.EmergencyStop import (
    EmergencyStopControllerV1,
    EmergencyStopMode,
    ResumeChecklistV1,
    StopError,
)


def _checklist(**overrides: object) -> ResumeChecklistV1:
    values: dict[str, object] = {
        "controlHealthy": True,
        "ledgerBalanced": True,
        "reconciliationClean": True,
    }
    values.update(overrides)
    return ResumeChecklistV1(**values)


class TestEmergencyStopController:
    def test_initial_running(self) -> None:
        controller = EmergencyStopControllerV1()
        assert controller.mode is EmergencyStopMode.Running
        assert controller.stopped is False
        assert controller.canSubmitNewOrder() is True

    def test_stop_all_blocks_new_orders(self) -> None:
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("ops-oncall")
        assert controller.stopped is True
        assert controller.canSubmitNewOrder() is False
        assert controller.canSubmitReduceOnlyOrder() is False  # STOP_ALL 也不允许减仓
        assert len(controller.stopHistory()) == 1

    def test_reduce_only_allows_reduce_blocks_open(self) -> None:
        controller = EmergencyStopControllerV1()
        controller.triggerReduceOnly("risk-ops")
        assert controller.canSubmitNewOrder() is False
        assert controller.canSubmitReduceOnlyOrder() is True

    def test_resume_requires_stopped(self) -> None:
        controller = EmergencyStopControllerV1()
        with pytest.raises(StopError, match="无需恢复"):
            controller.requestResume()

    def test_resume_requires_clean_checklist(self) -> None:
        """恢复前控制/账本/对账全部校验。"""
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("ops")
        controller.requestResume()
        bad = _checklist(reconciliationClean=False)
        with pytest.raises(StopError, match="全部校验"):
            controller.approveResume(approverId="alice", checklist=bad, secondApproverId="bob")

    def test_resume_requires_dual_approval(self) -> None:
        """恢复需双人批准。"""
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("ops")
        controller.requestResume()
        with pytest.raises(StopError, match="双人批准"):
            controller.approveResume(approverId="alice", checklist=_checklist())

    def test_resume_after_dual_approval(self) -> None:
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("ops")
        controller.requestResume()
        controller.approveResume(
            approverId="alice", checklist=_checklist(), secondApproverId="bob"
        )
        assert controller.mode is EmergencyStopMode.Running
        assert controller.canSubmitNewOrder() is True
        assert len(controller.approvals()) == 1

    def test_approve_without_pending_rejected(self) -> None:
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("ops")
        with pytest.raises(StopError, match="停止状态下申请恢复"):
            controller.approveResume(
                approverId="alice", checklist=_checklist(), secondApproverId="bob"
            )

    def test_independent_channel(self) -> None:
        """紧急停止不依赖 GUI/通知：控制器无任何 GUI/通知依赖。"""
        controller = EmergencyStopControllerV1()
        controller.triggerStopAll("独立通道触发")
        assert controller.mode is EmergencyStopMode.StopAll
