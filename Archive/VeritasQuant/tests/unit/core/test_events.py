from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.core.Events import CausalityTracker, EventContractError, EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision


class PricePayload(EventPayloadV1):
    """事件哈希和因果链测试所用的强类型价格载荷。"""

    price: Decimal = PascalAlias("Price")


def makeEvent(
    eventId: str,
    *,
    causationId: str | None = None,
    phase: int = 30,
    sourceSequence: int = 1,
    accountId: str | None = "account-1",
    subaccountId: str | None = None,
) -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc),
        source="fixture",
        producer="unit-test",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=causationId,
        accountId=accountId,
        subaccountId=subaccountId,
        eventOrderingVersion="V1",
        phase=phase,
        priority=0,
        sourceRank=0,
        sourceSequence=sourceSequence,
        payload=PricePayload.model_validate({"Price": Decimal("123.45")} ),
    )


def test_event_content_hash_detects_tampering() -> None:
    event = makeEvent("evt-1")
    assert len(event.contentHash) == 64
    tampered = event.model_dump(by_alias=True)
    tampered["Payload"] = event.payload
    tampered["ContentHash"] = "0" * 64
    with pytest.raises(ValidationError):
        EventEnvelopeV1.model_validate(tampered)


def test_causality_rejects_duplicate_ids_and_invalid_parent_reference() -> None:
    tracker = CausalityTracker()
    parent = makeEvent("evt-1")
    tracker.accept(parent, accountRequired=True)
    with pytest.raises(EventContractError):
        tracker.accept(parent)
    child = makeEvent("evt-2", causationId="evt-1", phase=40)
    tracker.accept(child, accountRequired=True)
    with pytest.raises(EventContractError):
        tracker.accept(makeEvent("evt-3", causationId="missing", phase=40))


def test_event_rejects_unscoped_subaccount_and_invalid_precision() -> None:
    event = makeEvent("evt-1")
    with pytest.raises(ValidationError):
        makeEvent("evt-2", accountId=None, subaccountId="sub-1")
    assert event.validateTsPrecision(TsPrecision.Second) is event
