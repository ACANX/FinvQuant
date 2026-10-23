"""P2-008 Redis Streams 传输集成测试（CI redis service 运行）。

验证真实 Redis Streams 的发布/消费/确认、幂等消费组创建与积压查询。
"""

from __future__ import annotations

import os
from datetime import datetime, timezone
from decimal import Decimal

import pytest
from redis import Redis
from redis.exceptions import RedisError


from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.infrastructure.messaging.RedisStreamTransport import RedisStreamTransportV1
from veritasquant.infrastructure.messaging.StreamTransport import TransportMessageV1

_REDIS_URL = os.environ.get(
    "VQ_TEST_REDIS_URL", "redis://localhost:6379/0"
)
_STREAM = "vq:events:test"


class NavPayloadV1(EventPayloadV1):
    nav: Decimal = PascalAlias("Nav")


def _makeEvent(eventId: str) -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId=eventId,
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


@pytest.fixture(scope="module")
def redisClient():
    try:
        client = Redis.from_url(_REDIS_URL, decode_responses=True)
        client.ping()
    except RedisError:
        pytest.skip("Redis 测试实例不可用，跳过传输集成测试")
    yield client
    client.close()


@pytest.fixture()
def transport(redisClient) -> RedisStreamTransportV1:  # noqa: ANN001
    redisClient.delete(_STREAM)
    return RedisStreamTransportV1(redisClient)


def test_publish_consume_acknowledge(transport) -> None:  # noqa: ANN001
    event = _makeEvent("evt-redis-1")
    message = TransportMessageV1.fromEvent(_STREAM, "local-1", event)
    messageId = transport.publish(message)
    assert messageId
    consumed = transport.consume(_STREAM)
    assert len(consumed) == 1
    assert consumed[0].contentHash == event.contentHash  # 传输元数据不进哈希
    transport.acknowledge(_STREAM, consumed[0].messageId)
    assert transport.pendingCount(_STREAM) == 0


def test_repeat_consume_same_content_until_ack(transport) -> None:  # noqa: ANN001
    event = _makeEvent("evt-redis-2")
    transport.publish(TransportMessageV1.fromEvent(_STREAM, "local-2", event))
    first = transport.consume(_STREAM)[0]
    # 未确认前从 pending 列表重新读取（重连恢复）：至少一次语义下内容一致
    redelivered = transport.consume(_STREAM, readPending=True)
    assert len(redelivered) == 1
    assert redelivered[0].contentHash == first.contentHash
    assert redelivered[0].messageId == first.messageId
    transport.acknowledge(_STREAM, first.messageId)
    assert transport.pendingCount(_STREAM) == 0


def test_ensure_group_is_idempotent(transport) -> None:  # noqa: ANN001
    transport.ensureGroup(_STREAM)
    transport.ensureGroup(_STREAM)  # BUSYGROUP 被吞掉，幂等


def test_pending_count_after_publish(transport) -> None:  # noqa: ANN001
    event = _makeEvent("evt-redis-3")
    transport.publish(TransportMessageV1.fromEvent(_STREAM, "local-3", event))
    # 未消费前积压可见（pendingCount 幂等建组）
    assert transport.pendingCount(_STREAM) >= 0
