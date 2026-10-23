"""P1-025 UTC 逻辑时钟与阶段推进器验证。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.core.Events import EventEnvelopeV1, EventPayloadV1
from veritasquant.core.LogicalClock import (
    ClockPhase,
    LogicalClockError,
    PhaseAdvancerV1,
    UtcLogicalClockV1,
)
from veritasquant.core.Models import PascalAlias
from veritasquant.core.Time import TsPrecision


class _PayloadV1(EventPayloadV1):
    value: int = PascalAlias("Value")


def _event(
    ts: str,
    *,
    phase: int = 30,
    eventId: str = "e",
    sourceSequence: int = 1,
) -> EventEnvelopeV1:
    parsed = datetime.strptime(ts, "%Y-%m-%dT%H:%M:%S.%fZ").replace(tzinfo=timezone.utc)
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="TestEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=parsed,
        occurredAt=None,
        publishedAt=None,
        ingestedAt=parsed,
        source="test",
        producer="test-producer",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=phase,
        priority=0,
        sourceRank=0,
        sourceSequence=sourceSequence,
        payload=_PayloadV1(Value=sourceSequence),
    )


def test_clock_advances_monotonically() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    assert clock.now is None
    first = clock.advance(datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc))
    second = clock.advance(datetime(2026, 8, 3, 1, 1, 0, tzinfo=timezone.utc))
    assert first < second == clock.now


def test_clock_rejects_rollback() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    clock.advance(datetime(2026, 8, 3, 1, 1, 0, tzinfo=timezone.utc))
    with pytest.raises(LogicalClockError, match="只前进"):
        clock.advance(datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc))


def test_clock_accepts_same_instant() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    instant = datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc)
    clock.advance(instant)
    assert clock.advance(instant) == instant


def test_clock_rejects_naive_and_out_of_precision() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    with pytest.raises(Exception):
        clock.advance(datetime(2026, 8, 3, 1, 0, 0))  # 无时区
    secondClock = UtcLogicalClockV1(TsPrecision.Second)
    with pytest.raises(Exception):
        secondClock.advance(datetime(2026, 8, 3, 1, 0, 0, 123000, tzinfo=timezone.utc))


def test_check_not_beyond_blocks_future_queries() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    clock.advance(datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc))
    clock.checkNotBeyond(datetime(2026, 8, 3, 1, 0, 0, tzinfo=timezone.utc))
    with pytest.raises(LogicalClockError, match="超越"):
        clock.checkNotBeyond(datetime(2026, 8, 3, 1, 1, 0, tzinfo=timezone.utc))


def test_clock_observes_event_ts() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    event = _event("2026-08-03T01:00:00.000Z")
    assert clock.observe(event) == event.ts
    with pytest.raises(LogicalClockError, match="只前进"):
        clock.observe(_event("2026-08-03T00:59:00.000Z"))


def test_phase_advancer_derives_into_later_phase() -> None:
    advancer = PhaseAdvancerV1(TsPrecision.Millisecond)
    parent = _event("2026-08-03T01:00:00.000Z", phase=ClockPhase.DISPATCH_CURRENT_EVENT)
    derived = advancer.derive(
        parent,
        eventId="derived-1",
        eventType="OrderIntent",
        phase=ClockPhase.GENERATE_ORDER_INTENT,
        producer="strategy",
        producerVersion="1.0",
        payload=_PayloadV1(Value=1),
    )
    assert derived.ts == parent.ts
    assert derived.causationId == parent.eventId
    assert derived.phase == ClockPhase.GENERATE_ORDER_INTENT


def test_phase_advancer_rejects_earlier_or_same_phase() -> None:
    advancer = PhaseAdvancerV1(TsPrecision.Millisecond)
    parent = _event("2026-08-03T01:00:00.000Z", phase=ClockPhase.EVALUATE_RISK)
    with pytest.raises(LogicalClockError, match="更早或相同"):
        advancer.derive(
            parent,
            eventId="bad",
            eventType="OrderIntent",
            phase=ClockPhase.DISPATCH_CURRENT_EVENT,
            producer="strategy",
            producerVersion="1.0",
            payload=_PayloadV1(Value=1),
        )
    with pytest.raises(LogicalClockError, match="更早或相同"):
        advancer.derive(
            parent,
            eventId="bad",
            eventType="OrderIntent",
            phase=ClockPhase.EVALUATE_RISK,
            producer="strategy",
            producerVersion="1.0",
            payload=_PayloadV1(Value=1),
        )


def test_phase_advancer_rejects_unknown_phase() -> None:
    advancer = PhaseAdvancerV1(TsPrecision.Millisecond)
    parent = _event("2026-08-03T01:00:00.000Z", phase=ClockPhase.DISPATCH_CURRENT_EVENT)
    with pytest.raises(LogicalClockError, match="未知阶段"):
        advancer.derive(
            parent,
            eventId="bad",
            eventType="OrderIntent",
            phase=99,
            producer="strategy",
            producerVersion="1.0",
            payload=_PayloadV1(Value=1),
        )


def test_derive_with_clock_advances_together() -> None:
    clock = UtcLogicalClockV1(TsPrecision.Millisecond)
    advancer = PhaseAdvancerV1(TsPrecision.Millisecond)
    parent = _event("2026-08-03T01:00:00.000Z", phase=ClockPhase.DISPATCH_CURRENT_EVENT)
    derived = advancer.deriveWithClock(
        clock,
        parent,
        eventId="derived-2",
        eventType="OrderIntent",
        phase=ClockPhase.GENERATE_ORDER_INTENT,
        producer="strategy",
        producerVersion="1.0",
        payload=_PayloadV1(Value=2),
    )
    assert clock.now == derived.ts == parent.ts
