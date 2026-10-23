from __future__ import annotations

from datetime import datetime, timezone

from veritasquant.core.EventRegistry import EventSchemaRegistry, SchemaRegistration, SchemaVersion, UpgraderRegistration
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision


class V10Payload(EventPayloadV1):
    """升级器输入端的 1.0 载荷契约。"""

    value: int = PascalAlias("Value")


class V11Payload(EventPayloadV1):
    """升级器输出端新增带默认值字段的同主版本载荷契约。"""

    value: int = PascalAlias("Value")
    label: str = PascalAlias("Label", default="default")


def makeRawEvent() -> dict[str, object]:
    payload = V10Payload.model_validate({"Value": 7})
    event = EventEnvelopeV1.create(
        eventId="evt-1",
        eventType="TestEvent",
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
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=30,
        priority=0,
        sourceRank=0,
        sourceSequence=1,
        payload=payload,
    )
    return event.model_dump(by_alias=True)


def test_unknown_major_version_is_quarantined() -> None:
    registry = EventSchemaRegistry()
    registry.register(SchemaRegistration("TestEvent", SchemaVersion.parse("1.0"), V10Payload, "core", ">=1.0,<2.0"))
    raw = makeRawEvent()
    raw["SchemaVersion"] = "2.0"
    result = registry.decodeAndUpgrade(raw, TsPrecision.Second)
    assert result.envelope is None
    assert result.quarantine is not None
    assert result.quarantine.reasonCode == "UNKNOWN_MAJOR_VERSION"


def test_initial_registry_declares_all_stage_one_core_event_types() -> None:
    registry = EventSchemaRegistry.createInitialRegistry()
    assert len(registry._registrations) == 12
    assert all(registration.schemaVersion == SchemaVersion(1, 0) for registration in registry._registrations.values())


def test_deterministic_upgrader_returns_stable_current_payload() -> None:
    registry = EventSchemaRegistry()
    source = SchemaVersion.parse("1.0")
    target = SchemaVersion.parse("1.1")
    registry.register(SchemaRegistration("TestEvent", source, V10Payload, "core", ">=1.0,<2.0"))
    registry.register(SchemaRegistration("TestEvent", target, V11Payload, "core", ">=1.0,<2.0"))
    registry.registerUpgrader(
        UpgraderRegistration(
            "TestEvent",
            source,
            target,
            "upgrade-v1",
            lambda payload: V11Payload.model_validate({"Value": payload.value, "Label": "upgraded"}),
        )
    )
    first = registry.decodeAndUpgrade(makeRawEvent(), TsPrecision.Second)
    second = registry.decodeAndUpgrade(makeRawEvent(), TsPrecision.Second)
    assert first.quarantine is None
    assert first.currentVersion == target
    assert first.currentPayload == second.currentPayload
    assert first.currentPayload.model_dump(by_alias=True) == {"Value": 7, "Label": "upgraded"}
