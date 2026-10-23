"""P2-013/014 场外基金申购/赎回状态机。

申购状态机（TechSpec 5.7）：`CREATED -> ACCEPTED -> WAITING_NAV -> CONFIRMED`，
允许从非终态进入 `REJECTED`，渠道规则允许时进入 `CANCELLED`；
受理时冻结资金，拒绝/取消/确认失败必须释放或退回资金并保留完整状态历史。

赎回状态机：`CREATED -> ACCEPTED -> WAITING_NAV -> SETTLEMENT -> COMPLETED`，
允许进入 `REJECTED`/`CANCELLED`；费用、到账和拒绝路径可重放。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum


class FundApplicationError(ValueError):
    """基金申请状态转换不满足契约。"""


class FundApplicationState(StrEnum):
    Created = "CREATED"
    Accepted = "ACCEPTED"
    WaitingNav = "WAITING_NAV"
    Confirmed = "CONFIRMED"
    Settlement = "SETTLEMENT"
    Completed = "COMPLETED"
    Rejected = "REJECTED"
    Cancelled = "CANCELLED"


class FundApplicationAction(StrEnum):
    Accept = "ACCEPT"  # 受理并冻结资金
    WaitNav = "WAIT_NAV"  # 等待适用净值
    Confirm = "CONFIRM"  # 份额确认
    Settle = "SETTLE"  # 结算（赎回）
    Complete = "COMPLETE"  # 到账完成（赎回）
    Reject = "REJECT"  # 拒绝并释放/退回资金
    Cancel = "CANCEL"  # 渠道规则允许时取消并释放/退回资金


# 终态：进入后不可再转换
_TERMINAL_STATES = {
    FundApplicationState.Confirmed,
    FundApplicationState.Completed,
    FundApplicationState.Rejected,
    FundApplicationState.Cancelled,
}

# 申购状态机转换表：state -> allowed actions
_SUBSCRIPTION_TRANSITIONS: dict[FundApplicationState, frozenset[FundApplicationAction]] = {
    FundApplicationState.Created: frozenset({FundApplicationAction.Accept, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
    FundApplicationState.Accepted: frozenset({FundApplicationAction.WaitNav, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
    FundApplicationState.WaitingNav: frozenset({FundApplicationAction.Confirm, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
}

# 赎回状态机转换表
_REDEMPTION_TRANSITIONS: dict[FundApplicationState, frozenset[FundApplicationAction]] = {
    FundApplicationState.Created: frozenset({FundApplicationAction.Accept, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
    FundApplicationState.Accepted: frozenset({FundApplicationAction.WaitNav, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
    FundApplicationState.WaitingNav: frozenset({FundApplicationAction.Settle, FundApplicationAction.Reject, FundApplicationAction.Cancel}),
    FundApplicationState.Settlement: frozenset({FundApplicationAction.Complete, FundApplicationAction.Reject}),
}

_ACTION_TARGET: dict[FundApplicationAction, FundApplicationState] = {
    FundApplicationAction.Accept: FundApplicationState.Accepted,
    FundApplicationAction.WaitNav: FundApplicationState.WaitingNav,
    FundApplicationAction.Confirm: FundApplicationState.Confirmed,
    FundApplicationAction.Settle: FundApplicationState.Settlement,
    FundApplicationAction.Complete: FundApplicationState.Completed,
    FundApplicationAction.Reject: FundApplicationState.Rejected,
    FundApplicationAction.Cancel: FundApplicationState.Cancelled,
}


@dataclass(frozen=True, slots=True)
class StateTransitionV1:
    """一次状态转换记录（可重放、可审计）。"""

    applicationId: str
    fromState: FundApplicationState
    action: FundApplicationAction
    toState: FundApplicationState
    ts: datetime
    detail: str = ""


class FundApplicationStateMachineV1:
    """可重放的基金申请状态机；状态历史完整保留。"""

    def __init__(
        self,
        applicationId: str,
        kind: str = "SUBSCRIPTION",
        createdTs: datetime | None = None,
    ) -> None:
        if not applicationId:
            raise FundApplicationError("申请 ID 不能为空")
        if kind not in ("SUBSCRIPTION", "REDEMPTION"):
            raise FundApplicationError("未知申请类型")
        self._applicationId = applicationId
        self._kind = kind
        self._state = FundApplicationState.Created
        self._history: list[StateTransitionV1] = []

    @property
    def state(self) -> FundApplicationState:
        return self._state

    @property
    def history(self) -> tuple[StateTransitionV1, ...]:
        """完整状态历史（可重放）。"""
        return tuple(self._history)

    def apply(self, action: FundApplicationAction, ts: datetime, detail: str = "") -> FundApplicationState:
        """应用一个动作；非法转换抛出 FundApplicationError。"""
        if self._state in _TERMINAL_STATES:
            raise FundApplicationError(f"终态 {self._state} 不可再转换")
        transitions = (
            _SUBSCRIPTION_TRANSITIONS
            if self._kind == "SUBSCRIPTION"
            else _REDEMPTION_TRANSITIONS
        )
        allowed = transitions.get(self._state)
        if allowed is None or action not in allowed:
            raise FundApplicationError(
                f"申请 {self._applicationId} 从 {self._state} 不允许动作 {action}"
            )
        target = _ACTION_TARGET[action]
        self._history.append(
            StateTransitionV1(self._applicationId, self._state, action, target, ts, detail)
        )
        self._state = target
        return self._state

    @staticmethod
    def isTerminal(state: FundApplicationState) -> bool:
        return state in _TERMINAL_STATES
