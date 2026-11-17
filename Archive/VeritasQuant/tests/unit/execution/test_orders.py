from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.core.Models import EventPayloadV1
from veritasquant.execution.Orders import (
    BrokerState,
    CancelOrderEventV1,
    ExecutionReportEventV1,
    ExecutionType,
    OrderEventV1,
    OrderIntentV1,
    OrderSide,
    OrderState,
    OrderType,
    PositionEffect,
    ReplaceOrderEventV1,
    TimeInForce,
)

UTC = timezone.utc


def _utc(hour: int = 0, minute: int = 0) -> datetime:
    return datetime(2026, 8, 2, hour, minute, tzinfo=UTC)


def _intent(**overrides: object) -> OrderIntentV1:
    values: dict[str, object] = {
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
        "OrderType": OrderType.Limit,
        "Quantity": Decimal("100"),
        "TimeInForce": TimeInForce.Day,
        "Ts": _utc(),
        "CreatedFromEventId": "event-100",
        "ExpectedAccountVersion": 5,
        "LimitPrice": Decimal("1.234"),
    }
    values.update(overrides)
    return OrderIntentV1.model_validate(values)


def test_intent_accepts_valid_limit_buy() -> None:
    intent = _intent()
    assert isinstance(intent, EventPayloadV1)
    assert intent.quantity == Decimal("100")
    assert intent.side is OrderSide.Buy
    assert intent.model_dump(by_alias=True)["IntentId"] == "intent-1"


def test_intent_rejects_missing_price_for_limit_and_stop() -> None:
    with pytest.raises(ValidationError, match="limitPrice"):
        _intent(OrderType=OrderType.Limit, LimitPrice=None)
    with pytest.raises(ValidationError, match="stopPrice"):
        _intent(OrderType=OrderType.Stop, StopPrice=None)
    with pytest.raises(ValidationError, match="limitPrice"):
        _intent(OrderType=OrderType.StopLimit, LimitPrice=None, StopPrice=Decimal("1.1"))


def test_intent_rejects_market_order_with_prices() -> None:
    with pytest.raises(ValidationError, match="MARKET"):
        _intent(OrderType=OrderType.Market, LimitPrice=Decimal("1.1"))


def test_intent_rejects_non_positive_quantity_and_unknown_enum() -> None:
    with pytest.raises(ValidationError, match="Quantity"):
        _intent(Quantity=Decimal("0"))
    with pytest.raises(ValidationError, match="未知"):
        _intent(Side="BOTTOM")


def test_intent_rejects_unknown_fields_and_float_amounts() -> None:
    with pytest.raises(ValidationError, match="Extra inputs are not permitted"):
        _intent(BrokerCredentials="secret")
    with pytest.raises(ValidationError):
        _intent(Quantity=100.5)  # float 必须被拒绝


def test_intent_requires_all_mandatory_fields() -> None:
    with pytest.raises(ValidationError, match="AccountId"):
        OrderIntentV1.model_validate({})


def test_order_event_validates_version_and_approved_quantity() -> None:
    order = OrderEventV1.model_validate(
        {
            "ClientOrderId": "client-1",
            "IntentId": "intent-1",
            "CommandId": "cmd-1",
            "OrderVersion": 1,
            "State": OrderState.PendingSubmit,
            "ApprovedQuantity": Decimal("100"),
            "OrderType": OrderType.Limit,
            "Side": OrderSide.Buy,
            "Quantity": Decimal("100"),
            "LimitPrice": Decimal("1.234"),
            "EffectiveAfterEventId": "event-200",
            "RiskDecisionId": "decision-1",
            "AccountId": "account-1",
            "Ts": _utc(),
        }
    )
    assert order.orderVersion == 1
    with pytest.raises(ValidationError, match="OrderVersion"):
        OrderEventV1.model_validate(order.model_dump(by_alias=True) | {"OrderVersion": 0})
    with pytest.raises(ValidationError, match="获批数量"):
        OrderEventV1.model_validate(order.model_dump(by_alias=True) | {"ApprovedQuantity": Decimal("101")})


def test_cancel_order_requires_reason_and_expected_version() -> None:
    cancel = CancelOrderEventV1.model_validate(
        {
            "CancelRequestId": "cancel-1",
            "ClientOrderId": "client-1",
            "BrokerOrderId": "broker-1",
            "ExpectedOrderVersion": 2,
            "Reason": "strategy signal",
            "RequestedBy": "strategy-1",
            "AccountId": "account-1",
            "Ts": _utc(),
        }
    )
    assert cancel.expectedOrderVersion == 2
    with pytest.raises(ValidationError, match="Reason"):
        CancelOrderEventV1.model_validate(cancel.model_dump(by_alias=True) | {"Reason": ""})
    with pytest.raises(ValidationError, match="ExpectedOrderVersion"):
        CancelOrderEventV1.model_validate(cancel.model_dump(by_alias=True) | {"ExpectedOrderVersion": 0})


def test_replace_order_requires_at_least_one_change() -> None:
    replace = ReplaceOrderEventV1.model_validate(
        {
            "ReplaceRequestId": "replace-1",
            "ClientOrderId": "client-1",
            "ExpectedOrderVersion": 2,
            "NewQuantity": Decimal("200"),
            "Reason": "increase size",
            "AccountId": "account-1",
            "Ts": _utc(),
        }
    )
    assert replace.newQuantity == Decimal("200")
    with pytest.raises(ValidationError, match="至少修改"):
        ReplaceOrderEventV1.model_validate(
            {
                "ReplaceRequestId": "replace-2",
                "ClientOrderId": "client-1",
                "ExpectedOrderVersion": 2,
                "Reason": "no-op",
                "AccountId": "account-1",
                "Ts": _utc(),
            }
        )


def _report(**overrides: object) -> ExecutionReportEventV1:
    values: dict[str, object] = {
        "BrokerReportId": "report-1",
        "ClientOrderId": "client-1",
        "BrokerOrderId": "broker-1",
        "ReportSequence": 1,
        "ExecutionType": ExecutionType.PartialFill,
        "ExecutionId": "exec-1",
        "LastQuantity": Decimal("40"),
        "LastPrice": Decimal("1.200"),
        "CumulativeQuantity": Decimal("40"),
        "RemainingQuantity": Decimal("60"),
        "BrokerState": BrokerState.Partial,
        "DiagnosticTs": _utc(),
        "AccountId": "account-1",
        "Ts": _utc(),
    }
    values.update(overrides)
    return ExecutionReportEventV1.model_validate(values)


def test_execution_report_requires_execution_id_for_fills() -> None:
    report = _report()
    assert report.cumulativeQuantity == Decimal("40")
    with pytest.raises(ValidationError, match="execution_id"):
        _report(ExecutionId=None)
    with pytest.raises(ValidationError, match="lastQuantity"):
        _report(LastQuantity=Decimal("0"))
    with pytest.raises(ValidationError, match="成交价格"):
        _report(LastPrice=None)


def test_execution_report_rejects_execution_id_on_non_fill() -> None:
    with pytest.raises(ValidationError, match="非成交类"):
        _report(ExecutionType=ExecutionType.Cancelled, ExecutionId="exec-1", LastQuantity=Decimal("0"))


def test_execution_report_rejects_cumulative_decrease() -> None:
    with pytest.raises(ValidationError, match="累计成交量"):
        _report(CumulativeQuantity=Decimal("39"))


def test_execution_report_rejects_float_and_unknown_broker_state() -> None:
    with pytest.raises(ValidationError):
        _report(LastQuantity=40.5)
    with pytest.raises(ValidationError, match="未知"):
        _report(BrokerState="HOLDING")


def test_execution_report_accepts_cancelled_report_without_price() -> None:
    cancelled = _report(
        ExecutionType=ExecutionType.Cancelled,
        ExecutionId=None,
        LastQuantity=Decimal("0"),
        LastPrice=None,
        CumulativeQuantity=Decimal("40"),
        RemainingQuantity=Decimal("0"),
        BrokerState=BrokerState.Cancelled,
    )
    assert cancelled.brokerState is BrokerState.Cancelled


def test_models_are_frozen_and_immutable() -> None:
    intent = _intent()
    with pytest.raises(ValidationError):
        intent.quantity = Decimal("999")  # type: ignore[misc]
