"""P2-008 可替换跨进程流传输协议与内存实现。

传输层只负责可靠搬运事件，不改变事件内容：传输元数据（stream key、
message id、投递序号）是信封外元数据，绝不进入事件哈希（验收标准）。
内存实现用于本地确定性测试；Redis Streams 实现见 RedisStreamTransport。
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Protocol

from veritasquant.core.Events import EventEnvelopeV1


class StreamTransportError(RuntimeError):
    """传输发布/消费不合法。"""


@dataclass(frozen=True, slots=True)
class TransportMessageV1:
    """一条跨进程传输消息；contentHash 与传输元数据无关。"""

    streamKey: str
    messageId: str
    eventJson: str
    contentHash: str

    @classmethod
    def fromEvent(cls, streamKey: str, messageId: str, event: EventEnvelopeV1) -> "TransportMessageV1":
        """从事件构造传输消息；contentHash 来自事件自身，不含 stream/messageId。"""
        eventJson = json.dumps(
            event.model_dump(mode="json", by_alias=True),
            sort_keys=True,
            separators=(",", ":"),
        )
        return cls(streamKey, messageId, eventJson, event.contentHash)


class StreamTransportV1(Protocol):
    """跨进程传输契约：发布、消费、确认与状态查询。"""

    def publish(self, message: TransportMessageV1) -> str: ...

    def pendingCount(self, streamKey: str) -> int: ...

    def consume(self, streamKey: str, limit: int = 100) -> tuple[TransportMessageV1, ...]: ...

    def acknowledge(self, streamKey: str, messageId: str) -> None: ...


class InMemoryStreamTransportV1:
    """内存传输：确定性重放、积压可见、重复投递可被消费者去重。"""

    def __init__(self) -> None:
        self._streams: dict[str, dict[str, TransportMessageV1]] = {}
        self._acknowledged: set[tuple[str, str]] = set()

    def publish(self, message: TransportMessageV1) -> str:
        stream = self._streams.setdefault(message.streamKey, {})
        stream[message.messageId] = message
        return message.messageId

    def pendingCount(self, streamKey: str) -> int:
        """未确认消息数（积压可见性，供背压策略使用）。"""
        stream = self._streams.get(streamKey, {})
        return sum(1 for messageId in stream if (streamKey, messageId) not in self._acknowledged)

    def consume(self, streamKey: str, limit: int = 100) -> tuple[TransportMessageV1, ...]:
        """按发布顺序返回未确认消息；重复消费返回相同内容（至少一次语义）。"""
        if limit <= 0:
            raise StreamTransportError("limit 必须为正")
        stream = self._streams.get(streamKey, {})
        ordered = [
            stream[messageId]
            for messageId in sorted(stream)
            if (streamKey, messageId) not in self._acknowledged
        ]
        return tuple(ordered[:limit])

    def acknowledge(self, streamKey: str, messageId: str) -> None:
        self._acknowledged.add((streamKey, messageId))

    def reconnect(self) -> None:
        """内存传输重连为空操作；保持流内数据不变（重连不丢已发布事件）。"""
