"""P4-004 订单发送、受理、拒绝、撤单和查询映射测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.BrokerOrderGateway import (
    OrderOutcome,
    OrderStatusMapperV1,
    QueryOutcomeV1,
    SimBrokerOrderGatewayV1,
)
from veritasquant.broker.BrokerPort import (
    BrokerCapabilityV1,
    BrokerPortError,
    OrderRequestV1,
)
from veritasquant.broker.BrokerSession import (
    BrokerCredentialV1,
    InMemoryCredentialResolverV1,
    SessionManagerV1,
)
from veritasquant.execution.Orders import BrokerState, OrderSide, OrderState, OrderType, TimeInForce


def _capability(**overrides: object) -> BrokerCapabilityV1:
    values: dict[str, object] = {
        "brokerId": "sim-broker",
        "capabilityVersion": "1.0",
        "orderTypes": frozenset({OrderType.Limit, OrderType.Market}),
        "timeInForces": frozenset({TimeInForce.Day, TimeInForce.GoodTillCancelled}),
        "orderSides": frozenset({OrderSide.Buy, OrderSide.Sell}),
        "symbols": frozenset({"518880"}),
        "markets": frozenset({"ETF_CN"}),
        "supportsCancel": True,
        "supportsOrderQuery": True,
        "supportsPositionQuery": True,
        "supportsCashQuery": True,
        "minQuantity": "100.0000",
        "maxOrderRatePerSecond": 10,
        "sessionOpenSupported": True,
        "sessionIntradaySupported": True,
        "sessionCloseSupported": True,
    }
    values.update(overrides)
    return BrokerCapabilityV1(**values)


def _request(**overrides: object) -> OrderRequestV1:
    values: dict[str, object] = {
        "clientOrderId": "co-001",
        "accountId": "acc-001",
        "symbol": "518880",
        "side": OrderSide.Buy,
        "orderType": OrderType.Limit,
        "timeInForce": TimeInForce.Day,
        "quantity": "100.0000",
        "limitPrice": "5.0000",
        "stopPrice": None,
    }
    values.update(overrides)
    return OrderRequestV1(**values)


def _session_manager() -> SessionManagerV1:
    resolver = InMemoryCredentialResolverV1(
        {"cred-sim-001": BrokerCredentialV1("cred-sim-001", "secret")}
    )
    return SessionManagerV1(resolver)


def _gateway(**overrides: object) -> SimBrokerOrderGatewayV1:
    values: dict[str, object] = {
        "capability": _capability(),
        "sessionManager": _session_manager(),
    }
    values.update(overrides)
    return SimBrokerOrderGatewayV1(**values)


class TestOrderStatusMapper:
    def test_mapping(self) -> None:
        mapper = OrderStatusMapperV1()
        assert mapper.map(BrokerState.Accepted) is OrderState.Accepted
        assert mapper.map(BrokerState.Partial) is OrderState.PartiallyFilled
        assert mapper.map(BrokerState.Filled) is OrderState.Filled
        assert mapper.map(BrokerState.Unknown) is OrderState.ReconciliationRequired


class TestSimBrokerOrderGateway:
    def test_submit_accepted_with_mapping(self) -> None:
        """client/broker ID 可追溯。"""
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        outcome = gateway.submit(session=session, request=_request())
        assert outcome.outcome is OrderOutcome.Accepted
        assert outcome.brokerOrderId is not None
        assert gateway.brokerOrderIdFor("co-001") == outcome.brokerOrderId
        assert gateway.clientOrderIdFor(outcome.brokerOrderId) == "co-001"
        assert gateway.mappingCount() == 1

    def test_submit_rejected(self) -> None:
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        outcome = gateway.submit(
            session=session,
            request=_request(),
            simulateAccept=False,
            simulateReject=True,
            reasonCode="INSUFFICIENT_BALANCE",
        )
        assert outcome.outcome is OrderOutcome.Rejected
        assert outcome.reasonCode == "INSUFFICIENT_BALANCE"

    def test_submit_timeout_unknown(self) -> None:
        """超时进入未知状态，不盲目重发。"""
        gateway = _gateway(allowUnknownAsAccepted=False)
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        outcome = gateway.submit(
            session=session, request=_request(), simulateAccept=False, simulateReject=False
        )
        assert outcome.outcome is OrderOutcome.TimeoutUnknown
        assert outcome.brokerOrderId is None
        assert gateway.mappingCount() == 0

    def test_unsupported_capability_rejected_before_send(self) -> None:
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        with pytest.raises(BrokerPortError, match="不支持订单类型"):
            gateway.submit(
                session=session,
                request=_request(orderType=OrderType.Stop, stopPrice="5.0"),
            )
        assert gateway.mappingCount() == 0

    def test_missing_permission_rejected(self) -> None:
        manager = SessionManagerV1(
            InMemoryCredentialResolverV1({"c": BrokerCredentialV1("c", "s")}),
            permissions=frozenset({"order:query"}),
        )
        gateway = SimBrokerOrderGatewayV1(_capability(), manager)
        session = manager.authenticate("c")
        with pytest.raises(BrokerPortError, match="order:submit"):
            gateway.submit(session=session, request=_request())

    def test_cancel_with_mapping(self) -> None:
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        outcome = gateway.submit(session=session, request=_request())
        cancelled = gateway.cancel(
            session=session,
            clientOrderId="co-001",
            brokerOrderId=outcome.brokerOrderId,  # type: ignore[arg-type]
        )
        assert cancelled is OrderOutcome.Cancelled

    def test_cancel_mapping_mismatch_rejected(self) -> None:
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        with pytest.raises(BrokerPortError, match="映射不一致"):
            gateway.cancel(
                session=session,
                clientOrderId="co-001",
                brokerOrderId="broker-unknown",
            )

    def test_query(self) -> None:
        gateway = _gateway()
        session = gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]
        outcome = gateway.submit(session=session, request=_request())
        query = gateway.query(
            session=session,
            clientOrderId="co-001",
            brokerOrderId=outcome.brokerOrderId,  # type: ignore[arg-type]
            brokerState=BrokerState.Partial,
        )
        assert isinstance(query, QueryOutcomeV1)
        assert query.brokerState is BrokerState.Partial
