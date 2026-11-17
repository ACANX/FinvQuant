from __future__ import annotations

import random
from datetime import datetime, timezone

from veritasquant.core.EventOrdering import EventPhase, eventOrderingKey, sortEvents
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision


class OrderingPayload(EventPayloadV1):
    """只保留稳定数值字段，避免载荷内容干扰排序键断言。"""

    value: int = PascalAlias("Value")


def makeOrderingEvent(eventId: str, *, phase: int, priority: int, sourceRank: int, sourceSequence: int) -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=datetime(2026, 7, 31, 8, 15, 30, 123000, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 7, 31, 8, 15, 30, 123000, tzinfo=timezone.utc),
        source="fixture",
        producer="unit-test",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=phase,
        priority=priority,
        sourceRank=sourceRank,
        sourceSequence=sourceSequence,
        payload=OrderingPayload.model_validate({"Value": sourceSequence}),
    )


def test_v1_phase_codes_are_exactly_the_six_fixed_phases() -> None:
    assert [phase.value for phase in EventPhase] == [10, 20, 30, 40, 50, 60]


def test_event_id_is_final_stable_tiebreaker() -> None:
    second = makeOrderingEvent("evt-b", phase=30, priority=0, sourceRank=0, sourceSequence=1)
    first = makeOrderingEvent("evt-a", phase=30, priority=0, sourceRank=0, sourceSequence=1)
    assert [event.eventId for event in sortEvents([second, first], TsPrecision.Millisecond)] == ["evt-a", "evt-b"]


def test_generated_ordering_property_is_invariant_to_input_permutation() -> None:
    events = [
        makeOrderingEvent(
            f"evt-{index:02d}",
            phase=[10, 20, 30, 40, 50, 60][index % 6],
            priority=index % 3,
            sourceRank=index % 4,
            sourceSequence=index,
        )
        for index in range(48)
    ]
    # 固定种子重复生成输入排列，验证完整排序键与到达顺序无关。
    expected = [event.eventId for event in sortEvents(events, TsPrecision.Millisecond)]
    randomizer = random.Random(20260731)
    for _ in range(25):
        shuffled = list(events)
        randomizer.shuffle(shuffled)
        assert [event.eventId for event in sortEvents(shuffled, TsPrecision.Millisecond)] == expected
        assert [eventOrderingKey(event, TsPrecision.Millisecond) for event in sortEvents(shuffled, TsPrecision.Millisecond)] == sorted(
            eventOrderingKey(event, TsPrecision.Millisecond) for event in events
        )
