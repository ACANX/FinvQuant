"""P1-022 顺序迭代器与多源最小堆归并验证。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.core.Events import EventEnvelopeV1, EventPayloadV1
from veritasquant.core.Models import PascalAlias
from veritasquant.core.Time import TsPrecision
from veritasquant.data.OrderedMerge import (
    MergeError,
    MinHeapMergerV1,
    OrderedSourceError,
    SequentialIteratorV1,
    makeEventSortKey,
)


class _PayloadV1(EventPayloadV1):
    value: int = PascalAlias("Value")


def _event(
    ts: str,
    *,
    phase: int = 30,
    priority: int = 0,
    sourceRank: int = 0,
    sourceSequence: int = 0,
    eventId: str = "e",
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
        priority=priority,
        sourceRank=sourceRank,
        sourceSequence=sourceSequence,
        payload=_PayloadV1(Value=sourceSequence),
    )


def _sortKey(event: EventEnvelopeV1) -> object:
    return makeEventSortKey(event, TsPrecision.Millisecond)


def test_sequential_iterator_is_bounded_and_in_order() -> None:
    events = [_event("2026-08-03T01:00:00.000Z", sourceSequence=1), _event("2026-08-03T01:01:00.000Z", sourceSequence=2)]
    iterator = SequentialIteratorV1(events, _sortKey)
    collected = list(iterator)
    assert [event.sourceSequence for event in collected] == [1, 2]
    assert iterator.invalidCount == 0
    assert iterator.exhausted


def test_sequential_iterator_rejects_out_of_order_in_strict_mode() -> None:
    events = [_event("2026-08-03T01:01:00.000Z", sourceSequence=2), _event("2026-08-03T01:00:00.000Z", sourceSequence=1)]
    iterator = SequentialIteratorV1(events, _sortKey)
    with pytest.raises(OrderedSourceError, match="严格模式"):
        list(iterator)


def test_sequential_iterator_isolates_out_of_order_in_lenient_mode() -> None:
    events = [_event("2026-08-03T01:01:00.000Z", sourceSequence=2), _event("2026-08-03T01:00:00.000Z", sourceSequence=1)]
    iterator = SequentialIteratorV1(events, _sortKey, strict=False)
    collected = list(iterator)
    assert [event.sourceSequence for event in collected] == [2]
    assert iterator.invalidCount == 1


def test_merge_combines_sources_in_full_sort_order() -> None:
    sourceA = [
        _event("2026-08-03T01:00:00.000Z", sourceRank=0, sourceSequence=1, eventId="a1"),
        _event("2026-08-03T01:02:00.000Z", sourceRank=0, sourceSequence=2, eventId="a2"),
    ]
    sourceB = [
        _event("2026-08-03T01:01:00.000Z", sourceRank=1, sourceSequence=1, eventId="b1"),
        _event("2026-08-03T01:03:00.000Z", sourceRank=1, sourceSequence=2, eventId="b2"),
    ]
    merger = MinHeapMergerV1(
        [SequentialIteratorV1(sourceA, _sortKey), SequentialIteratorV1(sourceB, _sortKey)],
        TsPrecision.Millisecond,
    )
    merged = merger.drain()
    assert [event.eventId for event in merged] == ["a1", "b1", "a2", "b2"]
    keys = [_sortKey(event) for event in merged]
    assert keys == sorted(keys)


def test_merge_respects_phase_before_source_rank() -> None:
    sourceA = [
        _event("2026-08-03T01:00:00.000Z", phase=10, sourceRank=9, sourceSequence=1, eventId="match"),
    ]
    sourceB = [
        _event("2026-08-03T01:00:00.000Z", phase=30, sourceRank=0, sourceSequence=1, eventId="dispatch"),
    ]
    merger = MinHeapMergerV1(
        [SequentialIteratorV1(sourceA, _sortKey), SequentialIteratorV1(sourceB, _sortKey)],
        TsPrecision.Millisecond,
    )
    merged = merger.drain()
    assert [event.eventId for event in merged] == ["match", "dispatch"]


def test_merge_is_deterministic_with_tied_keys() -> None:
    sources = [
        [_event("2026-08-03T01:00:00.000Z", sourceRank=0, sourceSequence=1, eventId=f"s{index}-1") for index in range(3)]
    ]
    first = MinHeapMergerV1(
        [SequentialIteratorV1(sources[0], _sortKey)],
        TsPrecision.Millisecond,
    ).drain()
    second = MinHeapMergerV1(
        [SequentialIteratorV1(sources[0], _sortKey)],
        TsPrecision.Millisecond,
    ).drain()
    assert [event.eventId for event in first] == [event.eventId for event in second]


def test_merge_requires_at_least_one_source() -> None:
    with pytest.raises(MergeError, match="至少需要一个输入来源"):
        MinHeapMergerV1([], TsPrecision.Millisecond)


def test_merge_memory_is_bounded_by_source_count() -> None:
    # 每个来源 1000 条，堆大小应恒为源数量
    bigSource = [_event("2026-08-03T01:00:00.000Z", sourceRank=0, sourceSequence=i, eventId=f"e{i}") for i in range(1000)]
    iterator = SequentialIteratorV1(bigSource, _sortKey)
    merger = MinHeapMergerV1([iterator], TsPrecision.Millisecond)
    count = 0
    while merger.next() is not None:
        count += 1
        assert len(merger._heap) <= 1  # noqa: SLF001 - 验证内存有界
    assert count == 1000
