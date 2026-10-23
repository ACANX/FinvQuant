from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.accounts.Reservation import ReservationBookV1
from veritasquant.core.Transaction import TransactionStoreV1
from veritasquant.execution.OrderStateMachine import OrderStateMachineV1, TransitionKind
from veritasquant.execution.Orders import (
    OrderIntentV1,
    OrderSide,
    OrderType,
    PositionEffect,
    TimeInForce,
)
from veritasquant.risk.AtomicRisk import AtomicRiskBoundaryV1, AtomicRiskError
from veritasquant.risk.RiskEngine import RiskDecision, RiskDecisionEventV1

UTC = timezone.utc


def _intent() -> OrderIntentV1:
    return OrderIntentV1.model_validate(
        {
            "IntentId": "intent-1",
            "RunId": "run-1",
            "AccountId": "account-1",
            "SubaccountId": "strategy-1",
            "StrategyId": "strategy-1",
            "StrategyVersion": "1.0.0",
            "Symbol": "518880",
            "InstrumentMetadataVersion": "meta-v1",
            "Side": OrderSide.Buy,
            "PositionEffect": PositionEffect.Open,
            "OrderType": OrderType.Market,
            "Quantity": Decimal("100"),
            "TimeInForce": TimeInForce.Day,
            "Ts": datetime(2026, 8, 2, tzinfo=UTC),
            "CreatedFromEventId": "event-100",
            "ExpectedAccountVersion": 5,
        }
    )


def _decision(decision: RiskDecision = RiskDecision.Approved, quantity: Decimal = Decimal("100")) -> RiskDecisionEventV1:
    return RiskDecisionEventV1(
        decisionId="decision-1",
        requestEventId="intent-1",
        accountId="account-1",
        decision=decision,
        approvedQuantity=quantity,
        ruleIds=(),
        riskPolicyVersion="P1-RISK-POLICY-V1",
        accountSnapshotVersion=1,
        orderSnapshotVersion=1,
        positionSnapshotVersion=1,
        reasonCodes=(),
        decisionHash="0" * 64,
    )


class _Fixture:
    def __init__(self) -> None:
        self.transactionStore = TransactionStoreV1()
        self.reservationBook = ReservationBookV1()
        self.stateMachine = OrderStateMachineV1()
        self.stateMachine.createIntent("order-intent-1", "account-1", Decimal("100"), 0)

    def boundary(self) -> AtomicRiskBoundaryV1:
        return AtomicRiskBoundaryV1(
            transactionStore=self.transactionStore,
            reservationBook=self.reservationBook,
            stateMachine=self.stateMachine,
            accountId="account-1",
            currency="CNY",
        )


def test_approval_commits_decision_reservation_and_migration_atomically() -> None:
    fixture = _Fixture()
    outcome = fixture.boundary().commitApproval(_decision(), _intent(), Decimal("10000"))
    assert outcome.orderVersionAfter == 2
    assert outcome.factSequences == (1,)
    assert fixture.stateMachine.snapshot("order-intent-1").state.value == "APPROVED"
    reservation = fixture.reservationBook.get("order-intent-1", "account-1")
    assert reservation.reservedAmount == Decimal("100")
    assert len(fixture.transactionStore.outbox) == 1
    assert fixture.transactionStore.outbox[0].topic == "risk.decisions"


def test_approval_never_leaves_approved_without_reservation() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    # 可用资金不足 → 预占失败 → 整笔回滚：订单不得 APPROVED
    with pytest.raises(AtomicRiskError, match="回滚"):
        boundary.commitApproval(_decision(quantity=Decimal("5000")), _intent(), Decimal("100"))
    assert fixture.stateMachine.snapshot("order-intent-1").state.value == "PENDING_RISK"
    assert len(fixture.transactionStore.facts) == 0
    assert len(fixture.transactionStore.outbox) == 0


def test_rejection_commits_without_reservation() -> None:
    fixture = _Fixture()
    outcome = fixture.boundary().commitRejection(_decision(decision=RiskDecision.Rejected, quantity=Decimal("0")), _intent())
    assert fixture.stateMachine.snapshot("order-intent-1").state.value == "REJECTED"
    assert outcome.reservationId == ""
    assert len(fixture.transactionStore.outbox) == 1


def test_rejected_decision_cannot_use_approval_path() -> None:
    fixture = _Fixture()
    with pytest.raises(AtomicRiskError, match="批准或降量"):
        fixture.boundary().commitApproval(_decision(decision=RiskDecision.Rejected, quantity=Decimal("0")), _intent(), Decimal("10000"))


def test_failed_transaction_rolls_back_all_stores() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    # 状态机在 PENDING_RISK，先手工推进到 APPROVED 制造版本冲突
    fixture.stateMachine.transition("order-intent-1", "account-1", TransitionKind.RiskApproval, 1)
    with pytest.raises(AtomicRiskError, match="回滚"):
        boundary.commitApproval(_decision(), _intent(), Decimal("10000"))
    # 预占不得残留
    assert len(fixture.transactionStore.facts) == 0
    assert len(fixture.transactionStore.outbox) == 0
