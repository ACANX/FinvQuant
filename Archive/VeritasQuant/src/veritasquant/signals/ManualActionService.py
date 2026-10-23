"""P3-004 人工审核动作登记服务。

- `ManualActionServiceV1`：登记人工确认/忽略/成交动作；每个动作有身份、
  理由、ts、版本和审计；动作登记不直接修改内核或账本，只产生待执行意图
  （P3-005 授权命令消费）。
- 动作登记以 (signalReferenceId, actionId) 为唯一键；重复登记返回既有记录。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Protocol

from pydantic import ValidationError

from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    ManualExecutionV1,
    ManualReviewActionV1,
    SignalActionType,
    SignalReferenceV1,
    SignalStatus,
)


def _utcNowMs() -> datetime:
    """当前 UTC 时间截断到毫秒精度（项目时间契约）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class ManualActionError(ValueError):
    """人工动作登记不满足契约时抛出。"""


class ActionStore(Protocol):
    """人工动作持久化端口。"""

    def getAction(self, actionId: str) -> ManualReviewActionV1 | None: ...

    def getExecution(self, executionId: str) -> ManualExecutionV1 | None: ...

    def saveAction(self, action: ManualReviewActionV1) -> None: ...

    def saveExecution(self, execution: ManualExecutionV1) -> None: ...


@dataclass(slots=True)
class InMemoryActionStoreV1:
    """内存人工动作存储（模拟盘/测试）。"""

    _actions: dict[str, ManualReviewActionV1] = field(default_factory=dict)
    _executions: dict[str, ManualExecutionV1] = field(default_factory=dict)

    def getAction(self, actionId: str) -> ManualReviewActionV1 | None:
        return self._actions.get(actionId)

    def getExecution(self, executionId: str) -> ManualExecutionV1 | None:
        return self._executions.get(executionId)

    def saveAction(self, action: ManualReviewActionV1) -> None:
        if action.actionId in self._actions:
            raise ManualActionError(f"动作已存在: {action.actionId}")
        self._actions[action.actionId] = action

    def saveExecution(self, execution: ManualExecutionV1) -> None:
        if execution.executionId in self._executions:
            raise ManualActionError(f"成交登记已存在: {execution.executionId}")
        self._executions[execution.executionId] = execution

    def allActions(self) -> tuple[ManualReviewActionV1, ...]:
        return tuple(self._actions.values())

    def allExecutions(self) -> tuple[ManualExecutionV1, ...]:
        return tuple(self._executions.values())


class ManualActionServiceV1:
    """人工动作登记：身份、理由、ts、版本和审计字段完整；不直接改内核。"""

    def __init__(self, store: ActionStore) -> None:
        if store is None:
            raise ManualActionError("动作存储不能为空")
        self._store = store
        self._counter = 0

    def _nextId(self, prefix: str) -> str:
        self._counter += 1
        return f"{prefix}-{self._counter:06d}"

    def recordAction(
        self,
        *,
        signal: SignalReferenceV1,
        actionType: SignalActionType,
        operatorId: str,
        reason: str,
        ignoreReason: IgnoreReasonV1 | None = None,
        actedAt: datetime | None = None,
        actionId: str | None = None,
    ) -> ManualReviewActionV1:
        """登记人工动作。动作只登记意图，不改变信号状态（状态推进由 P3-005 执行）。"""
        if signal.status is SignalStatus.Expired:
            raise ManualActionError(f"信号已过期: {signal.signalReferenceId}")
        try:
            action = ManualReviewActionV1.create(
                actionId=actionId or self._nextId("act"),
                signalReferenceId=signal.signalReferenceId,
                actionType=actionType,
                operatorId=operatorId,
                reason=reason,
                ignoreReason=ignoreReason,
                actedAt=actedAt or _utcNowMs(),
                version=1,
                auditTrail=(f"signal={signal.signalReferenceId};operator={operatorId};type={actionType.value}",),
            )
        except ValidationError as error:
            raise ManualActionError(str(error.errors()[0].get("msg", "动作不满足契约"))) from error
        self._store.saveAction(action)
        return action

    def recordExecution(
        self,
        *,
        signal: SignalReferenceV1,
        action: ManualReviewActionV1,
        operatorId: str,
        direction: str,
        quantity: str,
        price: str,
        executedAt: datetime | None = None,
        deviationReason: IgnoreReasonV1 | None = None,
        note: str = "",
        executionId: str | None = None,
    ) -> ManualExecutionV1:
        """登记人工成交。仅登记，不写订单/账本（P3-005 授权命令执行）。"""
        if action.signalReferenceId != signal.signalReferenceId:
            raise ManualActionError("动作与信号不匹配")
        if action.actionType is not SignalActionType.RegisterExecution:
            raise ManualActionError("只有 REGISTER_EXECUTION 动作可以登记成交")
        execution = ManualExecutionV1.create(
            executionId=executionId or self._nextId("exec"),
            signalReferenceId=signal.signalReferenceId,
            actionId=action.actionId,
            operatorId=operatorId,
            executedAt=executedAt or _utcNowMs(),
            direction=direction,
            quantity=quantity,
            price=price,
            deviationReason=deviationReason,
            note=note,
        )
        self._store.saveExecution(execution)
        return execution

    def getAction(self, actionId: str) -> ManualReviewActionV1 | None:
        return self._store.getAction(actionId)

    def getExecution(self, executionId: str) -> ManualExecutionV1 | None:
        return self._store.getExecution(executionId)
