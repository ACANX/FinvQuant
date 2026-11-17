"""P2-008 流传输、背压与重连单元测试。

验收标准映射：
- 传输元数据不进入事件哈希；
- 重复、积压和重连测试通过。
"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.infrastructure.messaging.StreamPolicy import (
    BackpressureLevel,
    ConnectionState,
    StreamBackpressurePolicyV1,
    StreamConnectionStateMachineV1,
)
from veritasquant.infrastructure.messaging.StreamTransport import (
    InMemoryStreamTransportV1,
    StreamTransportError,
    TransportMessageV1,
)


class NavPayloadV1(EventPayloadV1):
    nav: Decimal = PascalAlias("Nav")


def _makeEvent() -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId="evt-transport-1",
        eventType="FundNavPublishedEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        source="fund-source",
        producer="nav-importer",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=10,
        priority=0,
        sourceRank=0,
        sourceSequence=1,
        payload=NavPayloadV1.model_validate({"Nav": Decimal("1.2345")}),
    )


class TestTransportMetadataExcludedFromHash:
    def test_content_hash_does_not_include_stream_metadata(self) -> None:
        event = _makeEvent()
        messageA = TransportMessageV1.fromEvent("stream-a", "1-0", event)
        messageB = TransportMessageV1.fromEvent("stream-b", "999-999", event)
        # 不同 stream key / message id 不影响事件内容哈希
        assert messageA.contentHash == messageB.contentHash == event.contentHash

    def test_transport_message_roundtrip_preserves_event(self) -> None:
        event = _makeEvent()
        message = TransportMessageV1.fromEvent("stream-1", "1-0", event)
        assert message.contentHash == event.contentHash
        assert "stream-1" in message.streamKey


class TestInMemoryTransport:
    def test_publish_consume_acknowledge_lifecycle(self) -> None:
        transport = InMemoryStreamTransportV1()
        event = _makeEvent()
        message = TransportMessageV1.fromEvent("stream-1", "m1", event)
        transport.publish(message)
        assert transport.pendingCount("stream-1") == 1
        consumed = transport.consume("stream-1")
        assert len(consumed) == 1
        transport.acknowledge("stream-1", consumed[0].messageId)
        assert transport.pendingCount("stream-1") == 0

    def test_repeat_consume_returns_same_content_until_ack(self) -> None:
        transport = InMemoryStreamTransportV1()
        event = _makeEvent()
        transport.publish(TransportMessageV1.fromEvent("stream-1", "m1", event))
        first = transport.consume("stream-1")[0]
        second = transport.consume("stream-1")[0]
        assert first.contentHash == second.contentHash  # 至少一次：重复投递内容一致
        assert first.messageId == second.messageId

    def test_reconnect_keeps_published_events(self) -> None:
        transport = InMemoryStreamTransportV1()
        event = _makeEvent()
        transport.publish(TransportMessageV1.fromEvent("stream-1", "m1", event))
        transport.reconnect()  # 重连不丢已发布事件
        assert transport.pendingCount("stream-1") == 1
        assert transport.consume("stream-1")[0].contentHash == event.contentHash

    def test_invalid_limit_rejected(self) -> None:
        transport = InMemoryStreamTransportV1()
        with pytest.raises(StreamTransportError):
            transport.consume("stream-1", limit=0)


class TestBackpressure:
    def test_70_percent_warning_90_percent_critical(self) -> None:
        policy = StreamBackpressurePolicyV1(capacity=100)
        assert policy.evaluate(50).level is BackpressureLevel.Normal
        assert policy.evaluate(70).level is BackpressureLevel.Warning
        assert policy.evaluate(90).level is BackpressureLevel.Critical
        assert policy.evaluate(100).level is BackpressureLevel.Critical

    def test_critical_blocks_consumption_but_keeps_events(self) -> None:
        policy = StreamBackpressurePolicyV1(capacity=100)
        assert policy.canConsume(89)
        assert not policy.canConsume(90)  # 硬阈值：停止该分区消费，不丢事件

    def test_invalid_capacity_rejected(self) -> None:
        with pytest.raises(ValueError):
            StreamBackpressurePolicyV1(0)


class TestReconnectStateMachine:
    def test_disconnect_reconnect_lifecycle(self) -> None:
        machine = StreamConnectionStateMachineV1(maxRetries=5, baseBackoffSeconds=1.0)
        assert machine.state is ConnectionState.Connected
        machine.onDisconnect()
        assert machine.state is ConnectionState.Disconnected
        assert machine.nextBackoffSeconds() == 1.0
        assert machine.nextBackoffSeconds() == 2.0  # 指数退避
        assert machine.nextBackoffSeconds() == 4.0
        machine.onConnected()
        assert machine.state is ConnectionState.Connected

    def test_max_retries_exhausted_raises(self) -> None:
        machine = StreamConnectionStateMachineV1(maxRetries=2, baseBackoffSeconds=1.0)
        machine.onDisconnect()
        machine.nextBackoffSeconds()
        machine.nextBackoffSeconds()
        with pytest.raises(RuntimeError):
            machine.nextBackoffSeconds()
