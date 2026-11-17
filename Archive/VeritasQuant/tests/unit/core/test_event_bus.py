"""P1-026 确定性事件总线与订阅路由验证。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.core.EventBus import (
    BusError,
    ConsumerFailurePolicy,
    DeterministicEventBusV1,
    SubscriptionOrder,
)
from veritasquant.core.Events import EventEnvelopeV1, EventPayloadV1
from veritasquant.core.LogicalClock import UtcLogicalClockV1
from veritasquant.core.Models import PascalAlias
from veritasquant.core.Time import TsPrecision


class _PayloadV1(EventPayloadV1):
    value: int = PascalAlias("Value")


def _event(eventId: str, ts: str = "2026-08-03T01:00:00.000Z") -> EventEnvelopeV1:
    parsed = datetime.strptime(ts, "%Y-%m-%dT%H:%M:%S.%fZ").replace(tzinfo=timezone.utc)
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=parsed,
        occurredAt=None,
        publishedAt=None,
        ingestedAt=parsed,
        source="fixture",
        producer="unit-test",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=30,
        priority=0,
        sourceRank=0,
        sourceSequence=1,
        payload=_PayloadV1(Value=1),
    )


def test_subscription_order_is_frozen_by_registration() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    received: list[str] = []
    bus.subscribe("first", "MarketBarEvent", lambda event: received.append("first"))
    bus.subscribe("second", "MarketBarEvent", lambda event: received.append("second"))
    bus.deliver(_event("e1"))
    bus.deliver(_event("e2"))
    assert received == ["first", "second", "first", "second"]
    assert bus.frozen


def test_subscription_cannot_change_after_delivery() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    bus.subscribe("first", "MarketBarEvent", lambda event: None)
    bus.deliver(_event("e1"))
    with pytest.raises(BusError, match="冻结"):
        bus.subscribe("late", "MarketBarEvent", lambda event: None)


def test_duplicate_consumer_id_rejected() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    bus.subscribe("dup", "MarketBarEvent", lambda event: None)
    with pytest.raises(BusError, match="重复消费者"):
        bus.subscribe("dup", "MarketBarEvent", lambda event: None)


def test_same_input_produces_same_delivery_order() -> None:
    def build() -> list[str]:
        bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
        received: list[str] = []
        bus.subscribe("a", "MarketBarEvent", lambda event: received.append(f"a:{event.eventId}"))
        bus.subscribe("b", "MarketBarEvent", lambda event: received.append(f"b:{event.eventId}"))
        bus.deliver(_event("e1"))
        bus.deliver(_event("e2"))
        return received

    assert build() == build() == ["a:e1", "b:e1", "a:e2", "b:e2"]


def test_source_rank_order_is_frozen() -> None:
    bus = DeterministicEventBusV1(
        UtcLogicalClockV1(TsPrecision.Millisecond),
        SubscriptionOrder.SourceRankOrder,
    )
    received: list[str] = []
    bus.subscribe("low", "MarketBarEvent", lambda event: received.append("low"), sourceRank=9)
    bus.subscribe("high", "MarketBarEvent", lambda event: received.append("high"), sourceRank=1)
    bus.deliver(_event("e1"))
    assert received == ["high", "low"]


def test_stop_run_policy_raises_and_halts() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))

    def boom(_event: EventEnvelopeV1) -> None:
        raise RuntimeError("consumer exploded")

    bus.subscribe("bad", "MarketBarEvent", boom, policy=ConsumerFailurePolicy.StopRun)
    with pytest.raises(BusError, match="失败"):
        bus.deliver(_event("e1"))


def test_isolate_consumer_policy_continues_others() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    received: list[str] = []

    def boom(_event: EventEnvelopeV1) -> None:
        raise RuntimeError("consumer exploded")

    bus.subscribe("bad", "MarketBarEvent", boom, policy=ConsumerFailurePolicy.IsolateConsumer)
    bus.subscribe("good", "MarketBarEvent", lambda event: received.append("good"))
    result = bus.deliver(_event("e1"))
    assert received == ["good"]
    assert result.isolatedConsumers == ("bad",)
    # 第二次投递：隔离消费者不再被调用
    result2 = bus.deliver(_event("e2"))
    assert result2.isolatedConsumers == ("bad",)


def test_retry_fixed_policy_retries_then_stops() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    attempts: list[int] = []

    def flaky(_event: EventEnvelopeV1) -> None:
        attempts.append(1)
        raise RuntimeError("flaky")

    bus.subscribe(
        "flaky",
        "MarketBarEvent",
        flaky,
        policy=ConsumerFailurePolicy.RetryFixed,
        maxRetries=2,
    )
    with pytest.raises(BusError, match="重试耗尽"):
        bus.deliver(_event("e1"))
    assert len(attempts) == 3  # 初始 + 2 次重试


def test_retry_policy_requires_positive_retries() -> None:
    bus = DeterministicEventBusV1(UtcLogicalClockV1(TsPrecision.Millisecond))
    with pytest.raises(BusError, match="正数重试"):
        bus.subscribe(
            "bad",
            "MarketBarEvent",
            lambda event: None,
            policy=ConsumerFailurePolicy.RetryFixed,
            maxRetries=0,
        )


def test_delivery_advances_logical_clock() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    bus = DeterministicEventBusV1(clock)
    bus.subscribe("a", "MarketBarEvent", lambda event: None)
    bus.deliver(_event("e1", "2026-08-03T01:00:00.000Z"))
    assert clock.now == datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc)
    # 回退时间的下一事件被时钟拒绝
    with pytest.raises(Exception):
        bus.deliver(_event("e2", "2026-08-03T00:59:00.000Z"))
