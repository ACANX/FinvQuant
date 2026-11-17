from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.execution.Orders import OrderSide, OrderType
from veritasquant.strategy.BaseStrategy import (
    BaseStrategy,
    ConsumedEventV1,
    MovingAverageCrossStrategy,
    StrategyContextV1,
    StrategyContractError,
    StrategyInstrumentV1,
    StrategySnapshotV1,
)

UTC = timezone.utc


def _snapshot() -> StrategySnapshotV1:
    return StrategySnapshotV1(
        accountId="account-1",
        subaccountId="strategy-1",
        cashAvailable=Decimal("10000"),
        positions={"518880": Decimal("0")},
        snapshotVersion=5,
    )


def _instrument() -> StrategyInstrumentV1:
    return StrategyInstrumentV1(
        symbol="518880",
        metadataVersion="meta-v1",
        tickSize=Decimal("0.001"),
        lotSize=Decimal("100"),
        currency="CNY",
    )


def _context(**overrides: object) -> StrategyContextV1:
    values: dict[str, object] = {
        "strategyId": "strategy-1",
        "strategyVersion": "1.0.0",
        "runId": "run-1",
        "accountId": "account-1",
        "subaccountId": "strategy-1",
        "snapshot": _snapshot(),
        "instrument": _instrument(),
    }
    values.update(overrides)
    return StrategyContextV1(**values)  # type: ignore[call-arg]


def _barEvent(ts: datetime, close: str = "1.200") -> ConsumedEventV1:
    return ConsumedEventV1(
        eventId=f"bar-{ts.isoformat()}",
        eventType="MarketBarEvent",
        ts=ts,
        payload={"close": close, "symbol": "518880"},
    )


class _OrderOnlyStrategy(BaseStrategy):
    strategyId = "order_only"
    strategyVersion = "1.0.0"

    def onBar(self, event: ConsumedEventV1) -> None:
        self.createOrder("518880", OrderSide.Buy, Decimal("100"), orderType=OrderType.Market)


def test_strategy_returns_intent_not_order() -> None:
    context = _context()
    context.consume(_barEvent(datetime(2026, 8, 2, 10, 0, tzinfo=UTC)))
    strategy = _OrderOnlyStrategy()
    strategy.bind(context)
    strategy.onBar(context.latestEvent())  # type: ignore[arg-type]
    intents = strategy.emitIntents()
    assert len(intents) == 1
    intent = intents[0]
    assert intent.strategyId == "order_only"
    assert intent.accountId == "account-1"
    assert intent.createdFromEventId.startswith("bar-")
    # 意图不含券商凭据或最终订单状态
    assert not hasattr(intent, "brokerOrderId")


def test_context_is_read_only() -> None:
    context = _context()
    snapshot = context.currentSnapshot
    # 快照是副本，修改不影响上下文
    snapshot.positions["518880"] = Decimal("99999")  # type: ignore[index]
    assert context.currentSnapshot.positions["518880"] == Decimal("0")


def test_context_does_not_expose_broker_or_db() -> None:
    context = _context()
    assert not hasattr(context, "broker")
    assert not hasattr(context, "database")
    assert not hasattr(context, "oms")
    assert not hasattr(context, "future_index")


def test_clock_only_advances_via_consumed_events() -> None:
    context = _context()
    assert context.clock is None  # 未消费事件时无时钟（禁止系统时间）
    context.consume(_barEvent(datetime(2026, 8, 2, 10, 0, tzinfo=UTC)))
    assert context.clock == datetime(2026, 8, 2, 10, 0, tzinfo=UTC)
    with pytest.raises(StrategyContractError, match="回退"):
        context.consume(_barEvent(datetime(2026, 8, 2, 9, 59, tzinfo=UTC)))


def test_strategy_cannot_order_without_consumed_event() -> None:
    context = _context()
    strategy = _OrderOnlyStrategy()
    strategy.bind(context)
    with pytest.raises(StrategyContractError, match="已消费事件"):
        strategy.onBar(None)  # type: ignore[arg-type]
    strategy.emitIntents()


def test_strategy_cannot_order_unknown_symbol() -> None:
    context = _context()
    context.consume(_barEvent(datetime(2026, 8, 2, 10, 0, tzinfo=UTC)))

    class _BadSymbolStrategy(BaseStrategy):
        strategyId = "bad_symbol"
        strategyVersion = "1.0.0"

        def onBar(self, event: ConsumedEventV1) -> None:
            self.createOrder("AAPL", OrderSide.Buy, Decimal("100"), orderType=OrderType.Market)

    strategy = _BadSymbolStrategy()
    strategy.bind(context)
    with pytest.raises(StrategyContractError, match="订阅的标的"):
        strategy.onBar(context.latestEvent())  # type: ignore[arg-type]


def test_strategy_rejects_non_lot_quantity_and_bad_type() -> None:
    context = _context()
    context.consume(_barEvent(datetime(2026, 8, 2, 10, 0, tzinfo=UTC)))
    strategy = _OrderOnlyStrategy()
    strategy.bind(context)

    class _BadQuantityStrategy(BaseStrategy):
        strategyId = "bad_quantity"
        strategyVersion = "1.0.0"

        def onBar(self, event: ConsumedEventV1) -> None:
            self.createOrder("518880", OrderSide.Buy, Decimal("150"), orderType=OrderType.Market)

    badQuantity = _BadQuantityStrategy()
    badQuantity.bind(context)
    with pytest.raises(StrategyContractError, match="最小手数"):
        badQuantity.onBar(context.latestEvent())  # type: ignore[arg-type]

    class _BadTypeStrategy(BaseStrategy):
        strategyId = "bad_type"
        strategyVersion = "1.0.0"

        def onBar(self, event: ConsumedEventV1) -> None:
            self.createOrder("518880", OrderSide.Buy, Decimal("100"), orderType=OrderType.Market, price=Decimal("1.2"))

    badType = _BadTypeStrategy()
    badType.bind(context)
    with pytest.raises(StrategyContractError, match="市价单"):
        badType.onBar(context.latestEvent())  # type: ignore[arg-type]


def test_unbound_strategy_rejects_orders() -> None:
    strategy = _OrderOnlyStrategy()
    with pytest.raises(StrategyContractError, match="未绑定"):
        strategy.createOrder("518880", OrderSide.Buy, Decimal("100"), orderType=OrderType.Market)


def test_suspend_and_stop_state_transitions() -> None:
    strategy = _OrderOnlyStrategy()
    context = _context()
    strategy.bind(context)
    assert strategy.state.value == "RUNNING"
    strategy.suspend()
    assert strategy.state.value == "SUSPENDED"
    strategy.stop()
    assert strategy.state.value == "STOPPED"


def test_moving_average_cross_generates_buy_on_golden_cross() -> None:
    context = _context()
    strategy = MovingAverageCrossStrategy(fastPeriod=2, slowPeriod=3)
    strategy.bind(context)
    # 构造下降后上升序列触发金叉
    closes = ["1.000", "0.990", "0.980", "0.970", "0.960", "0.970", "0.985", "1.000"]
    for index, close in enumerate(closes):
        strategy.onBar(_barEvent(datetime(2026, 8, 2, 10, index, tzinfo=UTC), close))
    intents = strategy.emitIntents()
    sides = {intent.side for intent in intents}
    assert OrderSide.Buy in sides
    # 策略只返回意图，没有直接修改账户
    assert context.currentSnapshot.positions["518880"] == Decimal("0")
