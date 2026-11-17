"""P5-008 独立紧急停止、只减仓和恢复审批。

对齐 TechSpec 8.2/13 阶段 5：
- 紧急停止不依赖 GUI/通知（独立通道触发）；
- 恢复前控制、账本和对账全部校验（恢复审批）。

- `EmergencyStopMode`：STOP_ALL / REDUCE_ONLY / RESUME；
- `EmergencyStopControllerV1`：独立紧急停止控制器（不依赖 GUI/通知；
  状态机：运行 -> 停止/只减仓 -> 恢复审批）；
- `ResumeApprovalV1`：恢复审批（控制/账本/对账全部校验 + 双人批准）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum


class StopError(ValueError):
    """紧急停止不满足契约时抛出。"""


class EmergencyStopMode(StrEnum):
    Running = "RUNNING"
    StopAll = "STOP_ALL"          # 停止所有新订单
    ReduceOnly = "REDUCE_ONLY"    # 只允许减仓
    ResumePending = "RESUME_PENDING"  # 恢复审批中


@dataclass(frozen=True, slots=True)
class ResumeChecklistV1:
    """恢复前校验清单。"""

    controlHealthy: bool
    ledgerBalanced: bool
    reconciliationClean: bool

    @property
    def allPassed(self) -> bool:
        return self.controlHealthy and self.ledgerBalanced and self.reconciliationClean


@dataclass(frozen=True, slots=True)
class ResumeApprovalV1:
    """恢复审批记录。"""

    approverId: str
    approvedAt: datetime
    checklist: ResumeChecklistV1
    authorizedBy: str | None = None  # 第二审批人（双人）

    def __post_init__(self) -> None:
        if not self.approverId:
            raise StopError("审批人不能为空")


class EmergencyStopControllerV1:
    """独立紧急停止控制器。

    紧急停止通过独立通道（非 GUI/通知）触发；恢复必须满足：
    控制健康 + 账本平衡 + 对账干净 + 双人审批。
    """

    def __init__(self) -> None:
        self._mode = EmergencyStopMode.Running
        self._approvals: list[ResumeApprovalV1] = []
        self._stopHistory: list[tuple[EmergencyStopMode, datetime, str]] = []

    @property
    def mode(self) -> EmergencyStopMode:
        return self._mode

    @property
    def stopped(self) -> bool:
        return self._mode in (EmergencyStopMode.StopAll, EmergencyStopMode.ReduceOnly)

    def triggerStopAll(self, triggeredBy: str) -> None:
        """独立通道触发停止所有（不依赖 GUI/通知）。"""
        self._mode = EmergencyStopMode.StopAll
        self._stopHistory.append((EmergencyStopMode.StopAll, datetime.now(timezone.utc), triggeredBy))

    def triggerReduceOnly(self, triggeredBy: str) -> None:
        """只减仓：允许平仓，禁止新开仓。"""
        self._mode = EmergencyStopMode.ReduceOnly
        self._stopHistory.append(
            (EmergencyStopMode.ReduceOnly, datetime.now(timezone.utc), triggeredBy)
        )

    def canSubmitNewOrder(self) -> bool:
        """新开仓：仅 RUNNING 允许。"""
        return self._mode is EmergencyStopMode.Running

    def canSubmitReduceOnlyOrder(self) -> bool:
        """减仓单：RUNNING 或 REDUCE_ONLY 允许。"""
        return self._mode in (EmergencyStopMode.Running, EmergencyStopMode.ReduceOnly)

    def requestResume(self) -> None:
        """申请恢复：进入审批中状态。"""
        if not self.stopped:
            raise StopError("未处于停止状态，无需恢复")
        self._mode = EmergencyStopMode.ResumePending

    def approveResume(
        self,
        *,
        approverId: str,
        checklist: ResumeChecklistV1,
        secondApproverId: str | None = None,
    ) -> None:
        """恢复审批：清单全过 + 双人批准。"""
        if self._mode is not EmergencyStopMode.ResumePending:
            raise StopError("必须先在停止状态下申请恢复")
        if not checklist.allPassed:
            raise StopError("恢复前控制/账本/对账必须全部校验通过")
        if secondApproverId is None:
            raise StopError("恢复必须双人批准")
        self._approvals.append(
            ResumeApprovalV1(
                approverId=approverId,
                approvedAt=datetime.now(timezone.utc),
                checklist=checklist,
                authorizedBy=secondApproverId,
            )
        )
        self._mode = EmergencyStopMode.Running

    def approvals(self) -> tuple[ResumeApprovalV1, ...]:
        return tuple(self._approvals)

    def stopHistory(self) -> tuple[tuple[EmergencyStopMode, datetime, str], ...]:
        return tuple(self._stopHistory)
