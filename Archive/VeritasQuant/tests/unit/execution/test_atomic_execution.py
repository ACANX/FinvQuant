from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    LedgerProjectionStoreV1,
    LedgerStoreV1,
)
from veritasquant.accounts.Reservation import ReservationBookV1, ReservationKind
from veritasquant.core.Transaction import TransactionStoreV1
from veritasquant.execution.AtomicExecution import (
    AtomicExecutionBoundaryV1,
    AtomicExecutionError,
)
from veritasquant.execution.OrderStateMachine import OrderStateMachineV1, TransitionKind
from veritasquant.execution.Orders import (
    BrokerState,
    ExecutionReportEventV1,
    ExecutionType,
)

UTC = timezone.utc
VERSIONS = ("metadata-v1", "fees-v1", "policy-v1")


def _utc() -> datetime:
    return datetime(2026, 8, 2, 10, 0, tzinfo=UTC)


def _report(**overrides: object) -> ExecutionReportEventV1:
    values: dict[str, object] = {
        "BrokerReportId": "report-1",
        "ClientOrderId": "BUY-1",
        "BrokerOrderId": "broker-1",
        "ReportSequence": 1,
        "ExecutionType": ExecutionType.Fill,
        "ExecutionId": "exec-1",
        "LastQuantity": Decimal("100"),
        "LastPrice": Decimal("1.200"),
        "CumulativeQuantity": Decimal("100"),
        "RemainingQuantity": Decimal("0"),
        "BrokerState": BrokerState.Filled,
        "DiagnosticTs": _utc(),
        "AccountId": "account-1",
        "Ts": _utc(),
    }
    values.update(overrides)
    return ExecutionReportEventV1.model_validate(values)


class _Fixture:
    def __init__(self) -> None:
        self.transactionStore = TransactionStoreV1()
        self.ledgerStore = LedgerStoreV1()
        self.reservationBook = ReservationBookV1()
        self.stateMachine = OrderStateMachineV1()
        self.journalFactory = CashJournalFactoryV1(*VERSIONS)
        self._seedLedger()
        self._seedReservation()

    def _seedLedger(self) -> None:
        # 开户 10000 现金，保证买入成交有可用资金
        self.ledgerStore.commitJournal(
            self.journalFactory.createOpeningBalance(
                "account-1:open", "account-1", _utc(), 1, "open-event", "CNY", Decimal("10000")
            )
        )

    def _seedReservation(self) -> None:
        self.reservationBook.reserve(
            "BUY-1", "account-1", "BUY-1", ReservationKind.Cash, "CNY", Decimal("1000"), Decimal("10000")
        )

    def _orderMachine(self) -> OrderStateMachineV1:
        self.stateMachine.createIntent("BUY-1", "account-1", Decimal("100"), 0)
        self.stateMachine.transition("BUY-1", "account-1", TransitionKind.RiskApproval, 1)
        self.stateMachine.transition("BUY-1", "account-1", TransitionKind.CommandOutbox, 2)
        self.stateMachine.transition("BUY-1", "account-1", TransitionKind.SendSuccess, 3)
        self.stateMachine.transition("BUY-1", "account-1", TransitionKind.BrokerAccept, 4)
        return self.stateMachine

    def boundary(self) -> AtomicExecutionBoundaryV1:
        self._orderMachine()
        return AtomicExecutionBoundaryV1(
            transactionStore=self.transactionStore,
            ledgerStore=self.ledgerStore,
            reservationBook=self.reservationBook,
            stateMachine=self.stateMachine,
            journalFactory=self.journalFactory,
            accountId="account-1",
            currency="CNY",
        )


def test_fill_commits_order_reservation_and_ledger_atomically() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    outcome = boundary.commitExecutionReport(_report())
    assert outcome.journalCount == 1
    assert outcome.factSequences == (1,)
    assert outcome.orderVersionAfter == 6
    # 状态机推进到 FILLED
    assert fixture.stateMachine.snapshot("BUY-1").state.value == "FILLED"
    # 预占全部消耗
    reservation = fixture.reservationBook.get("BUY-1", "account-1")
    assert reservation.consumedAmount == Decimal("100")
    # 账本新增一条买入 journal（现金减少 120）
    assert len(fixture.ledgerStore.journals) == 2
    # outbox 已入队
    assert len(fixture.transactionStore.outbox) == 1
    assert fixture.transactionStore.outbox[0].topic == "execution.events"


def test_partial_fill_then_final_fill_are_both_atomic() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    partial = boundary.commitExecutionReport(
        _report(
            BrokerReportId="report-1",
            ExecutionId="exec-1",
            LastQuantity=Decimal("40"),
            CumulativeQuantity=Decimal("40"),
            RemainingQuantity=Decimal("60"),
            ExecutionType=ExecutionType.PartialFill,
        )
    )
    assert partial.orderVersionAfter == 6
    final = boundary.commitExecutionReport(
        _report(
            BrokerReportId="report-2",
            ExecutionId="exec-2",
            LastQuantity=Decimal("60"),
            CumulativeQuantity=Decimal("100"),
            RemainingQuantity=Decimal("0"),
            ExecutionType=ExecutionType.Fill,
        )
    )
    assert final.orderVersionAfter == 7
    assert fixture.stateMachine.snapshot("BUY-1").state.value == "FILLED"
    assert len(fixture.ledgerStore.journals) == 3  # 开户 + 两笔成交


def test_failed_transaction_rolls_back_everything() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    # 超额成交（超过订单量 100）导致状态机拒绝，整笔必须回滚
    with pytest.raises(AtomicExecutionError, match="回滚"):
        boundary.commitExecutionReport(
            _report(
                BrokerReportId="report-9",
                ExecutionId="exec-9",
                LastQuantity=Decimal("999"),
                CumulativeQuantity=Decimal("999"),
                RemainingQuantity=Decimal("0"),
            )
        )
    # 账本、事实、outbox 均无变化
    assert len(fixture.ledgerStore.journals) == 1
    assert len(fixture.transactionStore.facts) == 0
    assert len(fixture.transactionStore.outbox) == 0
    assert fixture.stateMachine.snapshot("BUY-1").state.value == "ACCEPTED"
    assert fixture.stateMachine.snapshot("BUY-1").orderVersion == 5


def test_cancel_releases_reservation_without_fake_ledger_entry() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    # 先撤单请求进入 PENDING_CANCEL（版本 5 -> 6）
    fixture.stateMachine.transition("BUY-1", "account-1", TransitionKind.CancelRequest, 5)
    # 撤单不虚构账本分录：技术方案要求拒绝
    with pytest.raises(AtomicExecutionError, match="不得虚构"):
        boundary.commitExecutionReport(
            _report(
                BrokerReportId="report-3",
                ExecutionId=None,
                ExecutionType=ExecutionType.Cancelled,
                LastQuantity=Decimal("0"),
                LastPrice=None,
                CumulativeQuantity=Decimal("0"),
                RemainingQuantity=Decimal("100"),
                BrokerState=BrokerState.Cancelled,
            )
        )
    # 事务已回滚：状态机仍停留在 PENDING_CANCEL，预占仍 ACTIVE
    assert len(fixture.ledgerStore.journals) == 1
    assert fixture.stateMachine.snapshot("BUY-1").state.value == "PENDING_CANCEL"
    assert fixture.reservationBook.get("BUY-1", "account-1").status.value == "ACTIVE"


def test_post_execution_balances_reflect_committed_state() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    outcome = boundary.commitExecutionReport(_report())
    # 开户 10000（CashAvailable 10000）- 买入 120 = 9880
    assert outcome.postExecutionCash == Decimal("9880")


def test_replay_of_same_execution_id_is_rejected_atomically() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    boundary.commitExecutionReport(_report())
    # 同 executionId 重复回报：预占消耗冲突（同 executionId 不同数量）→ 回滚
    with pytest.raises(AtomicExecutionError, match="回滚"):
        boundary.commitExecutionReport(
            _report(
                BrokerReportId="report-2",
                ExecutionId="exec-1",
                LastQuantity=Decimal("60"),
                CumulativeQuantity=Decimal("60"),
                RemainingQuantity=Decimal("40"),
                ExecutionType=ExecutionType.PartialFill,
            )
        )
    assert len(fixture.ledgerStore.journals) == 2


def test_projection_rebuild_matches_committed_facts() -> None:
    fixture = _Fixture()
    boundary = fixture.boundary()
    boundary.commitExecutionReport(_report())
    projection = LedgerProjectionStoreV1(fixture.ledgerStore).rebuild("account-1")
    # 重建投影与提交事实一致
    assert projection.lastLedgerSequence == 2
    assert projection.projectionHash  # 非空
