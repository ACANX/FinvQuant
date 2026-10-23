from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.IdealExecution import (
    IdealExecutionAdapterV1,
    IdealExecutionError,
)
from veritasquant.execution.Orders import OrderSide, OrderState, OrderType

UTC = timezone.utc


def _bar(open: Decimal, high: Decimal, low: Decimal, close: Decimal) -> MinuteBarSchemaV1:
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": datetime(2026, 8, 2, 10, 1, tzinfo=UTC),
            "BarStart": datetime(2026, 8, 2, 10, 0, tzinfo=UTC),
            "BarEnd": datetime(2026, 8, 2, 10, 0, 59, tzinfo=UTC),
            "Symbol": "518880",
            "Market": "SSE",
            "Open": open,
            "High": high,
            "Low": low,
            "Close": close,
            "Volume": Decimal("1000000"),
            "Currency": "CNY",
            "SessionId": "cn-morning",
            "Source": "fixture",
            "SourceRecordId": "bar-1",
            "SourceSequence": 1,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "meta-v1",
            "QualityFlags": 0,
        }
    )


def _adapter(**overrides: object) -> IdealExecutionAdapterV1:
    values: dict[str, object] = {
        "clientOrderId": "client-1",
        "accountId": "account-1",
        "orderState": OrderState.Accepted,
        "orderVersion": 5,
        "side": OrderSide.Buy,
        "orderType": OrderType.Market,
        "quantity": Decimal("100"),
        "limitPrice": None,
        "symbol": "518880",
        "brokerOrderId": "broker-1",
        "effectiveAfterEventId": "event-200",
    }
    values.update(overrides)
    return values


def test_market_buy_fills_at_next_bar_open() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(**_adapter(), currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")))
    assert result is not None
    assert result.mode == "IDEAL"
    assert result.fillPrice == Decimal("1.200")
    assert result.fillQuantity == Decimal("100")
    assert result.report.cumulativeQuantity == Decimal("100")
    assert result.report.executionId == "ideal-exec-1"
    assert result.report.accountId == "account-1"


def test_limit_buy_triggers_when_low_touches_price() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(
        **_adapter(orderType=OrderType.Limit, limitPrice=Decimal("1.195")),
        currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")),
    )
    assert result is not None
    # 路径首次触及限价时按不劣于限价成交
    assert result.fillPrice == Decimal("1.195")
    assert result.fillQuantity == Decimal("100")


def test_limit_buy_improves_on_open() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(
        **_adapter(orderType=OrderType.Limit, limitPrice=Decimal("1.210")),
        currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")),
    )
    assert result is not None
    assert result.fillPrice == Decimal("1.200")  # 开盘价改善


def test_limit_sell_triggers_on_high() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(
        **_adapter(side=OrderSide.Sell, orderType=OrderType.Limit, limitPrice=Decimal("1.215")),
        currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")),
    )
    assert result is not None
    assert result.fillPrice == Decimal("1.215")


def test_limit_buy_not_triggered_returns_none() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(
        **_adapter(orderType=OrderType.Limit, limitPrice=Decimal("1.185")),
        currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")),
    )
    assert result is None


def test_partial_previous_fill_only_fills_remaining() -> None:
    adapter = IdealExecutionAdapterV1()
    result = adapter.matchOrder(
        **_adapter(), previouslyMatchedQuantity=Decimal("40"), currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    )
    assert result is not None
    assert result.fillQuantity == Decimal("60")
    assert result.report.cumulativeQuantity == Decimal("100")
    assert result.report.remainingQuantity == Decimal("0")


def test_rejects_non_active_order_states() -> None:
    adapter = IdealExecutionAdapterV1()
    with pytest.raises(IdealExecutionError, match="已生效"):
        adapter.matchOrder(**_adapter(orderState=OrderState.PendingRisk), currentBar=_bar(Decimal("1"), Decimal("2"), Decimal("1"), Decimal("2")))


def test_rejects_unsupported_order_types() -> None:
    adapter = IdealExecutionAdapterV1()
    with pytest.raises(IdealExecutionError, match="市价与限价"):
        adapter.matchOrder(**_adapter(orderType=OrderType.Stop), currentBar=_bar(Decimal("1"), Decimal("2"), Decimal("1"), Decimal("2")))


def test_rejects_invalid_bar_geometry() -> None:
    adapter = IdealExecutionAdapterV1()
    # MinuteBarSchemaV1 已拒绝非法 OHLC 几何，适配器层面不重复校验
    with pytest.raises(ValidationError, match="OHLC"):
        _bar(Decimal("1.200"), Decimal("1.180"), Decimal("1.190"), Decimal("1.210"))
    # 适配器仍拒绝空订单 ID 等输入边界
    with pytest.raises(IdealExecutionError, match="账户和订单"):
        adapter.matchOrder(**_adapter(clientOrderId=""), currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")))


def test_rejects_negative_fee_and_quantity_violations() -> None:
    with pytest.raises(IdealExecutionError, match="手续费"):
        IdealExecutionAdapterV1(feePerUnit=Decimal("-0.01"))
    adapter = IdealExecutionAdapterV1()
    with pytest.raises(IdealExecutionError, match="订单数量"):
        adapter.matchOrder(**_adapter(quantity=Decimal("0")), currentBar=_bar(Decimal("1"), Decimal("2"), Decimal("1"), Decimal("2")))
    with pytest.raises(IdealExecutionError, match="已成交数量"):
        adapter.matchOrder(**_adapter(), previouslyMatchedQuantity=Decimal("101"), currentBar=_bar(Decimal("1"), Decimal("2"), Decimal("1"), Decimal("2")))


def test_deterministic_report_sequence() -> None:
    adapter = IdealExecutionAdapterV1()
    first = adapter.matchOrder(**_adapter(), currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")))
    second = adapter.matchOrder(**_adapter(clientOrderId="client-2"), currentBar=_bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210")))
    assert first is not None and second is not None
    assert first.report.reportSequence == 1
    assert second.report.reportSequence == 2
    assert first.report.brokerReportId == "ideal-1"
    assert second.report.brokerReportId == "ideal-2"
