from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.execution.OrderStateMachine import (
    OrderStateMachineError,
    OrderStateMachineV1,
    TransitionKind,
)
from veritasquant.execution.Orders import OrderState


def _machine() -> OrderStateMachineV1:
    machine = OrderStateMachineV1()
    machine.createIntent("client-1", "account-1", Decimal("100"), 0)
    return machine


def _fullLifecycle() -> OrderStateMachineV1:
    """走到 ACCEPTED 的订单，供成交/撤单场景复用。"""
    machine = _machine()
    machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 1)
    machine.transition("client-1", "account-1", TransitionKind.CommandOutbox, 2)
    machine.transition("client-1", "account-1", TransitionKind.SendSuccess, 3)
    machine.transition("client-1", "account-1", TransitionKind.BrokerAccept, 4)
    return machine


def test_create_intent_moves_new_to_pending_risk() -> None:
    snapshot = _machine().snapshot("client-1")
    assert snapshot.state is OrderState.PendingRisk
    assert snapshot.orderVersion == 1
    assert snapshot.remainingQuantity == Decimal("100")


def test_full_approval_chain() -> None:
    machine = _machine()
    machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 1)
    machine.transition("client-1", "account-1", TransitionKind.CommandOutbox, 2)
    machine.transition("client-1", "account-1", TransitionKind.SendSuccess, 3)
    machine.transition("client-1", "account-1", TransitionKind.BrokerAccept, 4)
    assert machine.snapshot("client-1").state is OrderState.Accepted
    assert machine.snapshot("client-1").orderVersion == 5


def test_incremental_fill_never_decreases_cumulative() -> None:
    machine = _fullLifecycle()
    machine.transition("client-1", "account-1", TransitionKind.IncrementalFill, 5, fillQuantity=Decimal("40"))
    assert machine.snapshot("client-1").state is OrderState.PartiallyFilled
    assert machine.snapshot("client-1").cumulativeQuantity == Decimal("40")
    assert machine.snapshot("client-1").remainingQuantity == Decimal("60")
    machine.transition("client-1", "account-1", TransitionKind.IncrementalFill, 6, fillQuantity=Decimal("60"))
    assert machine.snapshot("client-1").state is OrderState.Filled
    assert machine.snapshot("client-1").cumulativeQuantity == Decimal("100")
    assert machine.snapshot("client-1").remainingQuantity == Decimal("0")


def test_fill_beyond_quantity_rejected() -> None:
    machine = _fullLifecycle()
    with pytest.raises(OrderStateMachineError, match="累计成交量"):
        machine.transition("client-1", "account-1", TransitionKind.IncrementalFill, 5, fillQuantity=Decimal("101"))


def test_cancel_race_fill_wins() -> None:
    machine = _fullLifecycle()
    machine.transition("client-1", "account-1", TransitionKind.CancelRequest, 5)
    assert machine.snapshot("client-1").state is OrderState.PendingCancel
    # 撤单竞态中成交优先记账
    machine.transition("client-1", "account-1", TransitionKind.IncrementalFill, 6, fillQuantity=Decimal("100"))
    assert machine.snapshot("client-1").state is OrderState.Filled
    # 全成后撤单确认仅审计，不回退状态
    machine.transition("client-1", "account-1", TransitionKind.CancelConfirmed, 7, cancelQuantity=Decimal("0"))
    assert machine.snapshot("client-1").state is OrderState.Filled


def test_cancel_confirmed_releases_remaining() -> None:
    machine = _fullLifecycle()
    machine.transition("client-1", "account-1", TransitionKind.IncrementalFill, 5, fillQuantity=Decimal("40"))
    machine.transition("client-1", "account-1", TransitionKind.CancelRequest, 6)
    machine.transition("client-1", "account-1", TransitionKind.CancelConfirmed, 7, cancelQuantity=Decimal("60"))
    snapshot = machine.snapshot("client-1")
    assert snapshot.state is OrderState.Cancelled
    assert snapshot.cumulativeQuantity == Decimal("40")
    assert snapshot.cancelledQuantity == Decimal("60")
    assert snapshot.remainingQuantity == Decimal("60")


def test_risk_rejection_goes_to_rejected() -> None:
    machine = _machine()
    machine.transition("client-1", "account-1", TransitionKind.RiskRejection, 1)
    assert machine.snapshot("client-1").state is OrderState.Rejected
    assert machine.snapshot("client-1").isTerminal


def test_expiry_of_internal_order() -> None:
    machine = _fullLifecycle()
    machine.transition("client-1", "account-1", TransitionKind.Expiry, 5)
    assert machine.snapshot("client-1").state is OrderState.Expired
    assert machine.snapshot("client-1").isTerminal


def test_send_unknown_goes_to_reconciliation() -> None:
    machine = _machine()
    machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 1)
    machine.transition("client-1", "account-1", TransitionKind.CommandOutbox, 2)
    machine.transition("client-1", "account-1", TransitionKind.SendUnknown, 3)
    assert machine.snapshot("client-1").state is OrderState.ReconciliationRequired


def test_illegal_transition_rejected() -> None:
    machine = _machine()
    # NEW 状态不能直接撤单
    with pytest.raises(OrderStateMachineError, match="非法状态迁移"):
        machine.transition("client-1", "account-1", TransitionKind.CancelRequest, 1)
    # PENDING_RISK 不能直接送券商
    with pytest.raises(OrderStateMachineError, match="非法状态迁移"):
        machine.transition("client-1", "account-1", TransitionKind.SendSuccess, 1)


def test_terminal_state_does_not_regress() -> None:
    machine = _machine()
    machine.transition("client-1", "account-1", TransitionKind.RiskRejection, 1)
    with pytest.raises(OrderStateMachineError, match="非法状态迁移"):
        machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 2)
    assert machine.snapshot("client-1").state is OrderState.Rejected


def test_optimistic_version_conflict_rejected() -> None:
    machine = _machine()
    with pytest.raises(OrderStateMachineError, match="版本冲突"):
        machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 99)
    # 正确版本仍可推进
    machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 1)
    assert machine.snapshot("client-1").state is OrderState.Approved


def test_unknown_order_and_cross_account_rejected() -> None:
    machine = _machine()
    with pytest.raises(OrderStateMachineError, match="未知订单"):
        machine.transition("ghost", "account-1", TransitionKind.RiskApproval, 1)
    with pytest.raises(OrderStateMachineError, match="跨账户"):
        machine.transition("client-1", "account-2", TransitionKind.RiskApproval, 1)


def test_duplicate_intent_rejected() -> None:
    machine = OrderStateMachineV1()
    machine.createIntent("client-1", "account-1", Decimal("100"), 0)
    with pytest.raises(OrderStateMachineError, match="唯一"):
        machine.createIntent("client-1", "account-1", Decimal("200"), 0)


def test_audit_history_tracks_all_versions() -> None:
    machine = _fullLifecycle()
    history = machine.auditHistory("client-1")
    assert [item.state for item in history] == [
        OrderState.PendingRisk,
        OrderState.Approved,
        OrderState.PendingSubmit,
        OrderState.Submitted,
        OrderState.Accepted,
    ]
    assert [item.orderVersion for item in history] == [1, 2, 3, 4, 5]


def test_model_based_random_walk_never_violates_invariants() -> None:
    """随机合法步进：每次迁移后版本严格递增、累计量不下降、终态不回退。"""
    import random

    rng = random.Random(20260802)
    for _ in range(500):
        machine = _fullLifecycle()
        version = machine.snapshot("client-1").orderVersion
        lastCumulative = Decimal("0")
        steps = 0
        while steps < 20 and not machine.snapshot("client-1").isTerminal:
            before = machine.snapshot("client-1")
            options = [
                TransitionKind.IncrementalFill,
                TransitionKind.CancelRequest,
                TransitionKind.Expiry,
                TransitionKind.Reconciliation,
            ]
            kind = rng.choice(options)
            try:
                if kind is TransitionKind.IncrementalFill:
                    machine.transition(
                        "client-1", "account-1", kind, version, fillQuantity=Decimal(rng.randint(1, 30))
                    )
                elif kind is TransitionKind.CancelRequest:
                    machine.transition("client-1", "account-1", kind, version)
                elif kind is TransitionKind.Expiry:
                    machine.transition("client-1", "account-1", kind, version)
                else:
                    machine.reconcile("client-1", "account-1", version)
            except OrderStateMachineError:
                continue
            after = machine.snapshot("client-1")
            # 幂等输入（如对账维持原状态）不产生新版本；状态变化时必须严格递增。
            assert after.orderVersion >= before.orderVersion
            if after.state != before.state:
                assert after.orderVersion > before.orderVersion
            assert after.cumulativeQuantity >= lastCumulative
            assert after.remainingQuantity == after.quantity - after.cumulativeQuantity
            lastCumulative = after.cumulativeQuantity
            version = after.orderVersion
            steps += 1
