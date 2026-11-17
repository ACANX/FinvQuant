"""风险决定、预占、订单迁移和 outbox 的原子提交边界。

技术方案：风险决定、活动控制、资源冻结、订单迁移和待发布命令原子提交；
崩溃注入后不出现"已发单无决定"或"已批准未预占"的状态。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.accounts.Reservation import ReservationBookV1, ReservationKind
from veritasquant.core.Transaction import TransactionStoreV1
from veritasquant.execution.OrderStateMachine import (
    OrderStateMachineError,
    OrderStateMachineV1,
    TransitionKind,
)
from veritasquant.execution.Orders import OrderIntentV1
from veritasquant.risk.RiskEngine import RiskDecision, RiskDecisionEventV1


class AtomicRiskError(ValueError):
    """风险原子提交失败时抛出；整笔回滚不留部分副作用。"""


@dataclass(frozen=True, slots=True)
class AtomicRiskOutcomeV1:
    """一次风险原子提交的完整结果。"""

    transactionId: str
    decision: RiskDecisionEventV1
    reservationId: str
    orderVersionAfter: int
    factSequences: tuple[int, ...]


class AtomicRiskBoundaryV1:
    """把风险决定、资源预占、订单迁移和 outbox 装入同一原子事务。"""

    def __init__(
        self,
        *,
        transactionStore: TransactionStoreV1,
        reservationBook: ReservationBookV1,
        stateMachine: OrderStateMachineV1,
        accountId: str,
        currency: str,
    ) -> None:
        self._transactionStore = transactionStore
        self._reservationBook = reservationBook
        self._stateMachine = stateMachine
        self._accountId = accountId
        self._currency = currency
        self._transactionCounter = 0

    def commitApproval(
        self,
        decision: RiskDecisionEventV1,
        intent: OrderIntentV1,
        availableCash: Decimal,
    ) -> AtomicRiskOutcomeV1:
        """原子提交：批准决定 + 预占 + 订单迁移 APPROVED + outbox。"""
        if decision.accountId != self._accountId or intent.accountId != self._accountId:
            raise AtomicRiskError("决定与意图必须属于同一账户")
        if decision.decision not in (RiskDecision.Approved, RiskDecision.Reduced):
            raise AtomicRiskError("只有批准或降量决定才能触发预占与迁移")

        self._transactionCounter += 1
        transactionId = f"risk-atomic-{self._transactionCounter}"
        transaction = self._transactionStore.begin()
        try:
            # 1) 预占资源（现金）
            if decision.approvedQuantity <= 0:
                raise AtomicRiskError("批准数量必须为正")
            orderId = self._orderIdFor(intent)
            reservation = self._reservationBook.reserve(
                reservationId=orderId,
                accountId=self._accountId,
                orderId=intent.intentId,
                kind=ReservationKind.Cash,
                unitId=self._currency,
                amount=decision.approvedQuantity,
                availableAmount=availableCash,
            )

            # 2) 订单状态迁移：PENDING_RISK -> APPROVED
            snapshot = self._stateMachine.snapshot(orderId)
            self._stateMachine.transition(orderId, self._accountId, TransitionKind.RiskApproval, snapshot.orderVersion)

            # 3) 领域事实 + outbox 同事务提交
            decisionHash = decision.decisionHash
            transaction.appendFact("RISK_DECISION", decisionHash)
            transaction.enqueue(
                messageId=f"outbox-{transactionId}",
                topic="risk.decisions",
                payloadHash=decisionHash,
            )
            facts = transaction.commit()

            after = self._stateMachine.snapshot(orderId)
            return AtomicRiskOutcomeV1(
                transactionId=transactionId,
                decision=decision,
                reservationId=reservation.reservationId,
                orderVersionAfter=after.orderVersion,
                factSequences=tuple(fact.sequence for fact in facts),
            )
        except (OrderStateMachineError, AtomicRiskError, ValueError) as error:
            transaction.rollback()
            raise AtomicRiskError(f"风险原子提交失败，整笔回滚: {error}") from error

    def commitRejection(
        self,
        decision: RiskDecisionEventV1,
        intent: OrderIntentV1,
    ) -> AtomicRiskOutcomeV1:
        """原子提交拒绝决定：订单迁移 REJECTED + outbox（无预占）。"""
        if decision.decision is not RiskDecision.Rejected:
            raise AtomicRiskError("只有拒绝决定才能走拒绝路径")
        self._transactionCounter += 1
        transactionId = f"risk-atomic-{self._transactionCounter}"
        transaction = self._transactionStore.begin()
        try:
            orderId = self._orderIdFor(intent)
            snapshot = self._stateMachine.snapshot(orderId)
            self._stateMachine.transition(orderId, self._accountId, TransitionKind.RiskRejection, snapshot.orderVersion)
            transaction.appendFact("RISK_DECISION", decision.decisionHash)
            transaction.enqueue(
                messageId=f"outbox-{transactionId}",
                topic="risk.decisions",
                payloadHash=decision.decisionHash,
            )
            facts = transaction.commit()
            after = self._stateMachine.snapshot(orderId)
            return AtomicRiskOutcomeV1(
                transactionId=transactionId,
                decision=decision,
                reservationId="",
                orderVersionAfter=after.orderVersion,
                factSequences=tuple(fact.sequence for fact in facts),
            )
        except (OrderStateMachineError, AtomicRiskError, ValueError) as error:
            transaction.rollback()
            raise AtomicRiskError(f"风险原子提交失败，整笔回滚: {error}") from error

    def _orderIdFor(self, intent: OrderIntentV1) -> str:
        """订单 ID：意图对应订单的 clientOrderId 等价键。"""
        return f"order-{intent.intentId}"
