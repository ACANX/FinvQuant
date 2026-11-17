"""P5-007 实盘适配器、幂等下单和权威对账测试。"""

from __future__ import annotations


from veritasquant.broker.BrokerOrderGateway import OrderOutcome, SimBrokerOrderGatewayV1
from veritasquant.broker.BrokerPort import (
    BrokerCapabilityV1,
    OrderRequestV1,
)
from veritasquant.broker.BrokerSession import (
    BrokerCredentialV1,
    InMemoryCredentialResolverV1,
    SessionManagerV1,
)
from veritasquant.broker.LiveBroker import (
    AuthorityReconcilerV1,
    LiveBrokerAdapterV1,
    LiveOrderSubmissionV1,
)
from veritasquant.execution.Orders import OrderSide, OrderType, TimeInForce


def _capability() -> BrokerCapabilityV1:
    return BrokerCapabilityV1(
        brokerId="live-broker",
        capabilityVersion="1.0",
        orderTypes=frozenset({OrderType.Limit, OrderType.Market}),
        timeInForces=frozenset({TimeInForce.Day, TimeInForce.GoodTillCancelled}),
        orderSides=frozenset({OrderSide.Buy, OrderSide.Sell}),
        symbols=frozenset({"518880"}),
        markets=frozenset({"ETF_CN"}),
        supportsCancel=True,
        supportsOrderQuery=True,
        supportsPositionQuery=True,
        supportsCashQuery=True,
        minQuantity="100.0000",
        maxOrderRatePerSecond=10,
        sessionOpenSupported=True,
        sessionIntradaySupported=True,
        sessionCloseSupported=True,
    )


def _request(clientOrderId: str = "co-001") -> OrderRequestV1:
    return OrderRequestV1(
        clientOrderId=clientOrderId,
        accountId="acc-live-001",
        symbol="518880",
        side=OrderSide.Buy,
        orderType=OrderType.Limit,
        timeInForce=TimeInForce.Day,
        quantity="100.0000",
        limitPrice="5.0000",
        stopPrice=None,
    )


def _adapter() -> tuple[LiveBrokerAdapterV1, SimBrokerOrderGatewayV1, SessionManagerV1]:
    resolver = InMemoryCredentialResolverV1(
        {"cred-live-001": BrokerCredentialV1("cred-live-001", "secret")}
    )
    sessionManager = SessionManagerV1(resolver)
    gateway = SimBrokerOrderGatewayV1(_capability(), sessionManager)
    adapter = LiveBrokerAdapterV1(gateway, sessionManager)
    return adapter, gateway, sessionManager


class TestLiveBrokerAdapter:
    def test_submit_accepted(self) -> None:
        adapter, _, sessionManager = _adapter()
        session = sessionManager.authenticate("cred-live-001")
        result = adapter.submitOrder(session=session, request=_request())
        assert isinstance(result, LiveOrderSubmissionV1)
        assert result.outcome is OrderOutcome.Accepted
        assert result.brokerOrderId is not None
        assert result.reusedId is False

    def test_unknown_result_no_new_id(self) -> None:
        """发送结果未知不生成新 ID。"""
        adapter, _, sessionManager = _adapter()
        session = sessionManager.authenticate("cred-live-001")
        result = adapter.submitOrder(session=session, request=_request(), simulateUnknown=True)
        assert result.outcome is OrderOutcome.TimeoutUnknown
        assert result.brokerOrderId is None

    def test_retry_unknown_reuses_original(self) -> None:
        """未知结果重试：复用原结果，不生成新 ID（幂等下单）。"""
        adapter, _, sessionManager = _adapter()
        session = sessionManager.authenticate("cred-live-001")
        first = adapter.submitOrder(session=session, request=_request(), simulateUnknown=True)
        second = adapter.submitOrder(session=session, request=_request(), simulateUnknown=True)
        assert second.reusedId is True
        assert second.brokerOrderId == first.brokerOrderId

    def test_duplicate_submit_reuses_original_id(self) -> None:
        """同 clientOrderId 重试返回原 brokerOrderId（不生成新 ID）。"""
        adapter, _, sessionManager = _adapter()
        session = sessionManager.authenticate("cred-live-001")
        first = adapter.submitOrder(session=session, request=_request())
        second = adapter.submitOrder(session=session, request=_request())
        assert second.reusedId is True
        assert second.brokerOrderId == first.brokerOrderId

    def test_submission_lookup(self) -> None:
        adapter, _, sessionManager = _adapter()
        session = sessionManager.authenticate("cred-live-001")
        adapter.submitOrder(session=session, request=_request())
        submission = adapter.submissionFor("co-001")
        assert submission is not None
        assert submission.clientOrderId == "co-001"


class TestAuthorityReconciler:
    def test_clean_reconciliation(self) -> None:
        reconciler = AuthorityReconcilerV1()
        report = reconciler.reconcile(
            accountId="acc-live-001",
            localOrders={"co-001": "FILLED"},
            authorityOrders={"co-001": "FILLED"},
            localPositions={"518880": "100.0000"},
            authorityPositions={"518880": "100.0000"},
            localCash="50000.00",
            authorityCash="50000.00",
        )
        assert report.clean is True
        assert report.differences == 0
        assert len(report.items) == 3  # ORDER + POSITION + CASH

    def test_order_mismatch_blocks(self) -> None:
        """差异阻止交易。"""
        reconciler = AuthorityReconcilerV1()
        report = reconciler.reconcile(
            accountId="acc-live-001",
            localOrders={"co-001": "FILLED"},
            authorityOrders={"co-001": "WORKING"},
        )
        assert report.clean is False
        assert report.blocking is True
        assert report.differences == 1

    def test_position_mismatch_blocks(self) -> None:
        reconciler = AuthorityReconcilerV1()
        report = reconciler.reconcile(
            accountId="acc-live-001",
            localOrders={},
            authorityOrders={},
            localPositions={"518880": "100.0000"},
            authorityPositions={"518880": "90.0000"},
        )
        assert report.blocking is True

    def test_cash_mismatch_blocks(self) -> None:
        reconciler = AuthorityReconcilerV1()
        report = reconciler.reconcile(
            accountId="acc-live-001",
            localOrders={},
            authorityOrders={},
            localCash="50000.00",
            authorityCash="49999.99",
        )
        assert report.blocking is True

    def test_authority_order_unknown_locally(self) -> None:
        """券商有而本地无的订单：差异。"""
        reconciler = AuthorityReconcilerV1()
        report = reconciler.reconcile(
            accountId="acc-live-001",
            localOrders={},
            authorityOrders={"co-999": "FILLED"},
        )
        assert report.blocking is True
        assert report.items[0].localValue == "UNKNOWN"

    def test_report_history(self) -> None:
        reconciler = AuthorityReconcilerV1()
        reconciler.reconcile(accountId="acc-1", localOrders={}, authorityOrders={})
        reconciler.reconcile(accountId="acc-2", localOrders={}, authorityOrders={})
        assert len(reconciler.reports()) == 2
