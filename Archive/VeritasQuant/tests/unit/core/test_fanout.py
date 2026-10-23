"""P2-003 确定性分区扇出单元测试。

验收标准映射：相同 event 按固定 partition_rank 扇出；分区快慢不改事件内容。
"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Fanout import (
    DeterministicFanoutV1,
    FanoutError,
    FanoutTargetV1,
)
from veritasquant.core.Models import EventPayloadV1, PascalAlias


class BarPayloadV1(EventPayloadV1):
    symbol: str = PascalAlias("Symbol")
    price: Decimal = PascalAlias("Price")


def _makeEvent(eventId: str) -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 8, 2, 0, 0, 0, 100000, tzinfo=timezone.utc),
        source="test",
        producer="test-producer",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=10,
        priority=0,
        sourceRank=1,
        sourceSequence=1,
        payload=BarPayloadV1.model_validate({"Symbol": "TEST", "Price": Decimal("123.45")}),
    )


class TestDeterministicFanout:
    def test_targets_sorted_by_partition_rank(self) -> None:
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1("ag-b", 2),
                FanoutTargetV1("ag-a", 1),
                FanoutTargetV1("ag-c", 3),
            )
        )
        assert [target.accountGroupId for target in fanout.targets] == ["ag-a", "ag-b", "ag-c"]

    def test_plan_assigns_independent_delivery_sequence(self) -> None:
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1("ag-a", 1),
                FanoutTargetV1("ag-b", 2),
            )
        )
        event = _makeEvent("evt-1")
        deliveries = fanout.plan(event, {"ag-a": 5, "ag-b": 0})
        assert [d.deliverySequence for d in deliveries] == [6, 1]
        assert [d.partitionRank for d in deliveries] == [1, 2]

    def test_event_content_identical_across_partitions(self) -> None:
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1("ag-a", 1),
                FanoutTargetV1("ag-b", 2),
                FanoutTargetV1("ag-c", 3),
            )
        )
        event = _makeEvent("evt-1")
        deliveries = fanout.plan(event, {})
        hashes = {delivery.event.contentHash for delivery in deliveries}
        assert len(hashes) == 1, "同一共享事件在所有分区必须保持相同内容哈希"
        for delivery in deliveries:
            assert delivery.event.eventId == "evt-1"
            assert delivery.event.contentHash == event.contentHash

    def test_fast_partition_does_not_change_event_content(self) -> None:
        """分区快慢只影响 delivery_sequence，不改事件信封与哈希。"""
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1("ag-fast", 1),
                FanoutTargetV1("ag-slow", 2),
            )
        )
        event = _makeEvent("evt-1")
        fastPlan = fanout.plan(event, {"ag-fast": 100, "ag-slow": 0})
        assert fastPlan[0].deliverySequence == 101
        assert fastPlan[1].deliverySequence == 1
        assert fastPlan[0].event.contentHash == fastPlan[1].event.contentHash

    def test_plan_is_deterministic_for_same_input(self) -> None:
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1("ag-b", 2),
                FanoutTargetV1("ag-a", 1),
            )
        )
        event = _makeEvent("evt-1")
        first = fanout.plan(event, {"ag-a": 3, "ag-b": 7})
        second = fanout.plan(event, {"ag-a": 3, "ag-b": 7})
        assert [(d.accountGroupId, d.deliverySequence) for d in first] == [
            (d.accountGroupId, d.deliverySequence) for d in second
        ]

    def test_empty_targets_rejected(self) -> None:
        with pytest.raises(FanoutError):
            DeterministicFanoutV1(())

    def test_duplicate_account_group_rejected(self) -> None:
        with pytest.raises(FanoutError):
            DeterministicFanoutV1(
                (
                    FanoutTargetV1("ag-a", 1),
                    FanoutTargetV1("ag-a", 2),
                )
            )
