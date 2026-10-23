"""订单全生命周期状态机与乐观版本控制（技术方案 4.6 节迁移表）。

状态固定为 NEW/PENDING_RISK/APPROVED/PENDING_SUBMIT/SUBMITTED/ACCEPTED/
PARTIALLY_FILLED/PENDING_CANCEL/CANCELLED/FILLED/REJECTED/EXPIRED/
RECONCILIATION_REQUIRED；任何非法边或 expected_version 冲突必须拒绝。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from enum import StrEnum

from veritasquant.execution.Orders import OrderState


class OrderStateMachineError(ValueError):
    """订单迁移不满足状态机或版本契约时抛出。"""


class TransitionKind(StrEnum):
    """状态机输入类别，对应技术方案迁移表的输入列。"""

    CreateIntent = "CREATE_INTENT"
    RiskApproval = "RISK_APPROVAL"
    RiskRejection = "RISK_REJECTION"
    CommandOutbox = "COMMAND_OUTBOX"
    SendSuccess = "SEND_SUCCESS"
    SendUnknown = "SEND_UNKNOWN"
    BrokerAccept = "BROKER_ACCEPT"
    BrokerReject = "BROKER_REJECT"
    IncrementalFill = "INCREMENTAL_FILL"
    CancelRequest = "CANCEL_REQUEST"
    CancelConfirmed = "CANCEL_CONFIRMED"
    Expiry = "EXPIRY"
    Reconciliation = "RECONCILIATION"
    LateFillVerified = "LATE_FILL_VERIFIED"


# 迁移表：当前状态 -> 输入 -> 下一状态；None 表示该输入对终态仅审计、状态不变。
_TRANSITIONS: dict[OrderState, dict[TransitionKind, OrderState]] = {
    OrderState.New: {
        TransitionKind.CreateIntent: OrderState.PendingRisk,
    },
    OrderState.PendingRisk: {
        TransitionKind.RiskApproval: OrderState.Approved,
        TransitionKind.RiskRejection: OrderState.Rejected,
    },
    OrderState.Approved: {
        TransitionKind.CommandOutbox: OrderState.PendingSubmit,
    },
    OrderState.PendingSubmit: {
        TransitionKind.SendSuccess: OrderState.Submitted,
        TransitionKind.SendUnknown: OrderState.ReconciliationRequired,
    },
    OrderState.Submitted: {
        TransitionKind.BrokerAccept: OrderState.Accepted,
        TransitionKind.BrokerReject: OrderState.Rejected,
    },
    OrderState.Accepted: {
        TransitionKind.IncrementalFill: OrderState.PartiallyFilled,
        TransitionKind.CancelRequest: OrderState.PendingCancel,
        TransitionKind.Expiry: OrderState.Expired,
        TransitionKind.Reconciliation: OrderState.ReconciliationRequired,
    },
    OrderState.PartiallyFilled: {
        TransitionKind.IncrementalFill: OrderState.PartiallyFilled,
        TransitionKind.CancelRequest: OrderState.PendingCancel,
        TransitionKind.Expiry: OrderState.Expired,
        TransitionKind.Reconciliation: OrderState.ReconciliationRequired,
    },
    OrderState.PendingCancel: {
        TransitionKind.IncrementalFill: OrderState.Filled,
        TransitionKind.CancelConfirmed: OrderState.Cancelled,
        TransitionKind.Expiry: OrderState.Cancelled,
        TransitionKind.Reconciliation: OrderState.ReconciliationRequired,
    },
    # 终态输入只做审计：不改变状态，也不允许回退。
    OrderState.Cancelled: {},
    OrderState.Filled: {},
    OrderState.Rejected: {},
    OrderState.Expired: {},
    OrderState.ReconciliationRequired: {
        TransitionKind.Reconciliation: OrderState.ReconciliationRequired,
    },
}


@dataclass(frozen=True, slots=True)
class OrderStateSnapshotV1:
    """订单在某一版本处的不可变状态事实。"""

    clientOrderId: str
    accountId: str
    state: OrderState
    orderVersion: int
    quantity: Decimal
    cumulativeQuantity: Decimal
    filledVersion: int
    cancelledQuantity: Decimal

    @property
    def remainingQuantity(self) -> Decimal:
        """剩余可成交量 = 订单量 - 累计成交量，永不下降回退。"""
        return self.quantity - self.cumulativeQuantity

    @property
    def isTerminal(self) -> bool:
        return self.state in (OrderState.Cancelled, OrderState.Filled, OrderState.Rejected, OrderState.Expired)


class OrderStateMachineV1:
    """按迁移表推进订单状态；每次迁移必须匹配 expected_version。"""

    def __init__(self) -> None:
        self._history: dict[str, list[OrderStateSnapshotV1]] = {}
        self._latest: dict[str, OrderStateSnapshotV1] = {}
        self._cancelledQuantities: dict[str, Decimal] = {}

    def createIntent(
        self,
        clientOrderId: str,
        accountId: str,
        quantity: Decimal,
        expectedAccountVersion: int,
    ) -> OrderStateSnapshotV1:
        """从合法意图创建订单：NEW -> PENDING_RISK，重复 intent 拒绝。"""
        if not clientOrderId or not accountId:
            raise OrderStateMachineError("订单必须包含账户和订单 ID")
        if not isinstance(quantity, Decimal) or quantity <= 0:
            raise OrderStateMachineError("订单数量必须为正 Decimal")
        if not isinstance(expectedAccountVersion, int) or expectedAccountVersion < 0:
            raise OrderStateMachineError("账户版本必须为非负整数")
        if clientOrderId in self._latest:
            raise OrderStateMachineError("clientOrderId 在账户内必须唯一")
        snapshot = OrderStateSnapshotV1(
            clientOrderId=clientOrderId,
            accountId=accountId,
            state=OrderState.PendingRisk,
            orderVersion=1,
            quantity=quantity,
            cumulativeQuantity=Decimal("0"),
            filledVersion=0,
            cancelledQuantity=Decimal("0"),
        )
        self._history[clientOrderId] = [snapshot]
        self._latest[clientOrderId] = snapshot
        return snapshot

    def transition(
        self,
        clientOrderId: str,
        accountId: str,
        kind: TransitionKind,
        expectedVersion: int,
        *,
        fillQuantity: Decimal = Decimal("0"),
        cancelQuantity: Decimal = Decimal("0"),
    ) -> OrderStateSnapshotV1:
        """执行一次带乐观版本检查的状态迁移；非法边或版本冲突拒绝。"""
        current = self._latest.get(clientOrderId)
        if current is None:
            raise OrderStateMachineError("未知订单，禁止状态迁移")
        if current.accountId != accountId:
            raise OrderStateMachineError("订单不得跨账户迁移")
        if expectedVersion != current.orderVersion:
            raise OrderStateMachineError(
                f"乐观版本冲突: 期望 {expectedVersion}，实际 {current.orderVersion}"
            )
        target = _TRANSITIONS[current.state].get(kind)
        if current.isTerminal:
            # 技术方案：终态收到的撤单确认/普通成交回报只做审计，不得回退状态；
            # 经核验的迟到成交（LateFillVerified）可审计迁移为 FILLED。
            if kind is TransitionKind.LateFillVerified and current.state is not OrderState.Filled:
                return self._applyTransition(
                    current, OrderState.Filled, current.cumulativeQuantity, current.cancelledQuantity
                )
            if kind in (
                TransitionKind.CancelConfirmed,
                TransitionKind.IncrementalFill,
                TransitionKind.LateFillVerified,
            ):
                return current
            raise OrderStateMachineError(
                f"非法状态迁移: {current.state.value} + {kind.value}"
            )
        if target is None:
            raise OrderStateMachineError(
                f"非法状态迁移: {current.state.value} + {kind.value}"
            )

        cumulative = current.cumulativeQuantity
        cancelled = current.cancelledQuantity
        if kind is TransitionKind.IncrementalFill:
            if not isinstance(fillQuantity, Decimal) or fillQuantity <= 0:
                raise OrderStateMachineError("增量成交必须为正 Decimal")
            cumulative += fillQuantity
            if cumulative > current.quantity:
                raise OrderStateMachineError("累计成交量不得超过订单量")
            # 技术方案：累计量等于订单量时迁移为 FILLED。
            if cumulative == current.quantity:
                target = OrderState.Filled
        if kind is TransitionKind.CancelConfirmed:
            if not isinstance(cancelQuantity, Decimal) or cancelQuantity < 0:
                raise OrderStateMachineError("撤单数量必须为非负 Decimal")
            cancelled = cancelQuantity
        if kind is TransitionKind.Expiry and current.state in (
            OrderState.Accepted,
            OrderState.PartiallyFilled,
            OrderState.PendingCancel,
        ):
            cancelled = current.remainingQuantity

        return self._applyTransition(current, target, cumulative, cancelled)

    def _applyTransition(
        self,
        current: OrderStateSnapshotV1,
        target: OrderState,
        cumulative: Decimal,
        cancelled: Decimal,
    ) -> OrderStateSnapshotV1:
        """构造新版本快照；状态变化时写入历史并更新最新。"""
        filled = current.filledVersion
        if target in (OrderState.Filled, OrderState.PartiallyFilled):
            filled = current.orderVersion + 1
        if target is OrderState.Filled:
            cumulative = current.quantity

        updated = OrderStateSnapshotV1(
            clientOrderId=current.clientOrderId,
            accountId=current.accountId,
            state=target,
            orderVersion=current.orderVersion + 1,
            quantity=current.quantity,
            cumulativeQuantity=cumulative,
            filledVersion=filled,
            cancelledQuantity=cancelled,
        )
        if updated.state != current.state:
            self._history[current.clientOrderId].append(updated)
            self._latest[current.clientOrderId] = updated
        return updated

    def snapshot(self, clientOrderId: str) -> OrderStateSnapshotV1:
        """返回订单最新状态快照。"""
        snapshot = self._latest.get(clientOrderId)
        if snapshot is None:
            raise OrderStateMachineError("未知订单")
        return snapshot

    def auditHistory(self, clientOrderId: str) -> tuple[OrderStateSnapshotV1, ...]:
        """返回订单全部版本的历史快照（含当前）。"""
        history = self._history.get(clientOrderId)
        if history is None:
            raise OrderStateMachineError("未知订单")
        return tuple(sorted(history, key=lambda item: item.orderVersion))

    def reconcile(self, clientOrderId: str, accountId: str, expectedVersion: int) -> OrderStateSnapshotV1:
        """外部活动状态进入对账状态；仅允许活动状态。"""
        current = self._latest.get(clientOrderId)
        if current is None:
            raise OrderStateMachineError("未知订单，禁止对账")
        if current.accountId != accountId:
            raise OrderStateMachineError("订单不得跨账户对账")
        if expectedVersion != current.orderVersion:
            raise OrderStateMachineError("对账版本冲突")
        if current.state in (OrderState.Cancelled, OrderState.Filled, OrderState.Rejected, OrderState.Expired):
            raise OrderStateMachineError("终态订单不需要对账")
        return self.transition(clientOrderId, accountId, TransitionKind.Reconciliation, expectedVersion)


def latestState(clientOrderId: str, snapshots: dict[str, OrderStateSnapshotV1]) -> OrderStateSnapshotV1:
    """从版本历史中取最新状态。"""
    versions = [item for key, item in snapshots.items() if key.split(":v")[0] == clientOrderId]
    if not versions:
        raise OrderStateMachineError("未知订单")
    return max(versions, key=lambda item: item.orderVersion)
