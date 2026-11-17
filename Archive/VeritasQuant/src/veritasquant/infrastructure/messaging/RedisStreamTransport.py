"""P2-008 Redis Streams 跨进程传输实现（可替换内存传输）。

基于 redis-py 的 Streams（XADD/XREADGROUP/XACK）。传输元数据
（stream key、message id）不参与事件内容哈希；消费使用至少一次语义，
消费者通过 inbox 去重，保证重复投递无重复副作用。
"""

from __future__ import annotations


from redis import Redis
from typing import cast

from veritasquant.infrastructure.messaging.StreamTransport import (
    StreamTransportError,
    TransportMessageV1,
)


class RedisStreamTransportV1:
    """Redis Streams 传输：发布、消费、确认与积压查询。"""

    def __init__(self, client: Redis, groupName: str = "vq-workers") -> None:
        if client is None or not groupName:
            raise StreamTransportError("Redis 客户端与消费组名不能为空")
        self._client = client
        self._groupName = groupName

    def publish(self, message: TransportMessageV1) -> str:
        """发布到 stream；返回 Redis 分配的 message id。"""
        try:
            messageId = self._client.xadd(
                message.streamKey,
                {"event": message.eventJson, "content_hash": message.contentHash},
            )
        except Exception as error:  # noqa: BLE001
            raise StreamTransportError(f"Redis 发布失败: {error}") from error
        return str(messageId)

    def pendingCount(self, streamKey: str) -> int:
        """未确认消息数（积压可见性）。"""
        self.ensureGroup(streamKey)
        try:
            summary = self._client.xpending(streamKey, self._groupName)
            if not summary:
                return 0
            return int(summary["pending"])
        except Exception as error:  # noqa: BLE001
            raise StreamTransportError(f"Redis 积压查询失败: {error}") from error

    def ensureGroup(self, streamKey: str) -> None:
        """幂等创建消费组（重连/重启后安全）。"""
        try:
            self._client.xgroup_create(streamKey, self._groupName, id="0", mkstream=True)
        except Exception:  # noqa: BLE001
            # BUSYGROUP：消费组已存在，属预期
            pass

    def consume(
        self,
        streamKey: str,
        limit: int = 100,
        *,
        readPending: bool = False,
    ) -> tuple[TransportMessageV1, ...]:
        """读取消息（至少一次语义）。

        readPending=False 时读取组内新消息（id=">"）；True 时从 pending
        列表重新读取未确认消息（id="0"），用于消费者重连后恢复位点。
        """
        if limit <= 0:
            raise StreamTransportError("limit 必须为正")
        self.ensureGroup(streamKey)
        readId = "0" if readPending else ">"
        try:
            entries = self._client.xreadgroup(
                self._groupName, "consumer-1", {streamKey: readId}, count=limit
            )
        except Exception as error:  # noqa: BLE001
            raise StreamTransportError(f"Redis 消费失败: {error}") from error
        messages: list[TransportMessageV1] = []
        streamEntries = cast(
            "list[tuple[object, list[tuple[object, dict[str, str]]]]]", entries or []
        )
        for streamName, payloads in streamEntries:
            for messageId, fields in payloads:
                messages.append(
                    TransportMessageV1(
                        streamKey=str(streamName),
                        messageId=str(messageId),
                        eventJson=str(fields.get("event", "")),
                        contentHash=str(fields.get("content_hash", "")),
                    )
                )
        return tuple(messages)

    def acknowledge(self, streamKey: str, messageId: str) -> None:
        """确认已处理消息；重复确认是幂等的。"""
        try:
            self._client.xack(streamKey, self._groupName, messageId)
        except Exception as error:  # noqa: BLE001
            raise StreamTransportError(f"Redis 确认失败: {error}") from error
