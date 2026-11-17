"""P4-002 统一 BrokerPort 与能力协商测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.BrokerPort import (
    BrokerCapabilityV1,
    BrokerPortError,
    CapabilityNegotiatorV1,
    OrderRequestV1,
)
from veritasquant.execution.Orders import OrderSide, OrderType, TimeInForce


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


class TestBrokerCapability:
    def test_valid(self) -> None:
        capability = _capability()
        assert capability.brokerId == "sim-broker"
        assert capability.supportsOrderType(OrderType.Limit) is True
        assert capability.supportsOrderType(OrderType.Stop) is False
        assert capability.supportsSymbol("518880") is True
        assert capability.supportsSymbol("600000") is False

    def test_requires_identity(self) -> None:
        with pytest.raises(BrokerPortError):
            _capability(brokerId="")
        with pytest.raises(BrokerPortError):
            _capability(orderTypes=frozenset())
        with pytest.raises(BrokerPortError):
            _capability(maxOrderRatePerSecond=0)


class TestCapabilityNegotiator:
    def test_negotiate_ok(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        negotiator.negotiate(_request())  # 不抛即通过

    def test_unsupported_order_type_rejected(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        with pytest.raises(BrokerPortError, match="不支持订单类型"):
            negotiator.negotiate(_request(orderType=OrderType.Stop, stopPrice="5.0"))

    def test_unsupported_time_in_force_rejected(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        with pytest.raises(BrokerPortError, match="不支持有效期"):
            negotiator.negotiate(_request(timeInForce=TimeInForce.FillOrKill))

    def test_unsupported_symbol_rejected(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        with pytest.raises(BrokerPortError, match="不支持标的"):
            negotiator.negotiate(_request(symbol="600000"))

    def test_unsupported_side_rejected(self) -> None:
        capability = _capability(orderSides=frozenset({OrderSide.Buy}))
        negotiator = CapabilityNegotiatorV1(capability)
        with pytest.raises(BrokerPortError, match="不支持方向"):
            negotiator.negotiate(_request(side=OrderSide.Sell))

    def test_limit_requires_price(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        with pytest.raises(BrokerPortError, match="limitPrice"):
            negotiator.negotiate(_request(limitPrice=None))

    def test_market_with_price_rejected(self) -> None:
        negotiator = CapabilityNegotiatorV1(_capability())
        with pytest.raises(BrokerPortError, match="MARKET"):
            negotiator.negotiate(
                _request(orderType=OrderType.Market, limitPrice="5.0", stopPrice=None)
            )

    def test_request_requires_identity(self) -> None:
        with pytest.raises(BrokerPortError):
            _request(clientOrderId="")
