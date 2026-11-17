"""P4-006 开盘/盘中/收盘及重连对账测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.Reconciliation import (
    AccountPositionV1,
    BrokerReconcilerV1,
    InMemoryBrokerStateProviderV1,
    LocalOrderStateV1,
    ReconciliationError,
    ReconciliationSession,
)


def _provider() -> InMemoryBrokerStateProviderV1:
    provider = InMemoryBrokerStateProviderV1()
    provider.setOrderState("co-001", "broker-001", "ACCEPTED")
    provider.setPosition("acc-001", "518880", "100.0000")
    provider.setCash("acc-001", "50000.00")
    return provider


def _orders() -> list[LocalOrderStateV1]:
    return [
        LocalOrderStateV1(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            state="ACCEPTED",
            cumulativeQuantity="0",
        )
    ]


def _positions() -> list[AccountPositionV1]:
    return [AccountPositionV1(symbol="518880", quantity="100.0000", cash="50000.00")]


class TestBrokerReconciler:
    def test_reconcile_orders_clean(self) -> None:
        reconciler = BrokerReconcilerV1(_provider())
        report = reconciler.reconcileOrders(
            session=ReconciliationSession.Open, accountId="acc-001", localOrders=_orders()
        )
        assert report.clean is True
        assert report.unexplainedDifferences == 0
        assert report.blocking is False
        assert len(report.items) == 1
        assert report.items[0].matched is True

    def test_order_mismatch_blocks_trading(self) -> None:
        """未解释差异阻止交易。"""
        provider = _provider()
        provider.setOrderState("co-001", "broker-001", "REJECTED")
        reconciler = BrokerReconcilerV1(provider)
        report = reconciler.reconcileOrders(
            session=ReconciliationSession.Close, accountId="acc-001", localOrders=_orders()
        )
        assert report.clean is False
        assert report.blocking is True
        assert report.unexplainedDifferences == 1
        assert report.items[0].detail == "本地与券商订单状态不一致"

    def test_order_without_broker_id_unexplained(self) -> None:
        reconciler = BrokerReconcilerV1(_provider())
        orders = [
            LocalOrderStateV1(
                clientOrderId="co-002", brokerOrderId=None, state="SUBMITTED"
            )
        ]
        report = reconciler.reconcileOrders(
            session=ReconciliationSession.Reconnect, accountId="acc-001", localOrders=orders
        )
        assert report.blocking is True
        assert report.items[0].brokerValue == "UNKNOWN"

    def test_position_and_cash_reconcile(self) -> None:
        reconciler = BrokerReconcilerV1(_provider())
        report = reconciler.reconcilePositions(
            session=ReconciliationSession.Close, accountId="acc-001", localPositions=_positions()
        )
        assert report.clean is True
        assert len(report.items) == 2  # POSITION + CASH

    def test_position_mismatch_blocks(self) -> None:
        provider = _provider()
        provider.setPosition("acc-001", "518880", "90.0000")
        reconciler = BrokerReconcilerV1(provider)
        report = reconciler.reconcilePositions(
            session=ReconciliationSession.Intraday,
            accountId="acc-001",
            localPositions=_positions(),
        )
        assert report.blocking is True
        assert report.unexplainedDifferences == 1

    def test_cash_mismatch_blocks(self) -> None:
        provider = _provider()
        provider.setCash("acc-001", "49999.99")
        reconciler = BrokerReconcilerV1(provider)
        report = reconciler.reconcilePositions(
            session=ReconciliationSession.Open,
            accountId="acc-001",
            localPositions=_positions(),
        )
        assert report.blocking is True

    def test_override_values(self) -> None:
        """测试/演练可用覆盖值模拟券商侧。"""
        reconciler = BrokerReconcilerV1(_provider())
        report = reconciler.reconcileOrders(
            session=ReconciliationSession.Reconnect,
            accountId="acc-001",
            localOrders=_orders(),
            brokerStateOverride={"co-001": "ACCEPTED"},
        )
        assert report.clean is True

    def test_reports_history(self) -> None:
        reconciler = BrokerReconcilerV1(_provider())
        reconciler.reconcileOrders(
            session=ReconciliationSession.Open, accountId="acc-001", localOrders=_orders()
        )
        reconciler.reconcilePositions(
            session=ReconciliationSession.Close,
            accountId="acc-001",
            localPositions=_positions(),
        )
        assert len(reconciler.reports()) == 2

    def test_empty_position_requires_symbol(self) -> None:
        with pytest.raises(ReconciliationError):
            AccountPositionV1(symbol="", quantity="0", cash="0")

    def test_requires_provider(self) -> None:
        with pytest.raises(ReconciliationError):
            BrokerReconcilerV1(None)  # type: ignore[arg-type]
