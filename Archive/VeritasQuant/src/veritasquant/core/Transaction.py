"""领域事实与 outbox 的原子提交基线。"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from enum import StrEnum


class TransactionError(ValueError):
    """事务状态或 outbox 发布不合法。"""


class OutboxStatus(StrEnum):
    Pending = "PENDING"
    Published = "PUBLISHED"


@dataclass(frozen=True, slots=True)
class DomainFactV1:
    sequence: int
    factType: str
    payloadHash: str


@dataclass(frozen=True, slots=True)
class OutboxMessageV1:
    messageId: str
    sequence: int
    topic: str
    payloadHash: str
    status: OutboxStatus


class TransactionStoreV1:
    """阶段 1 内存事务存储；P2 迁移到同语义的数据库事务。"""

    def __init__(self) -> None:
        self._facts: list[DomainFactV1] = []
        self._outbox: list[OutboxMessageV1] = []
        self._nextSequence = 1

    def begin(self) -> "DomainTransactionV1":
        return DomainTransactionV1(self)

    @property
    def facts(self) -> tuple[DomainFactV1, ...]:
        return tuple(self._facts)

    @property
    def outbox(self) -> tuple[OutboxMessageV1, ...]:
        return tuple(self._outbox)

    def publishPending(self, publisher: Callable[[OutboxMessageV1], None]) -> int:
        """按提交序号发布；失败条目保留，下一次以同一 messageId 重试。"""
        published = 0
        for index, message in enumerate(self._outbox):
            if message.status is OutboxStatus.Published:
                continue
            publisher(message)
            self._outbox[index] = OutboxMessageV1(message.messageId, message.sequence, message.topic, message.payloadHash, OutboxStatus.Published)
            published += 1
        return published


class DomainTransactionV1:
    """提交前不修改事实源或 outbox。"""

    def __init__(self, store: TransactionStoreV1) -> None:
        self._store = store
        self._facts: list[tuple[str, str]] = []
        self._messages: list[tuple[str, str, str]] = []
        self._closed = False

    def appendFact(self, factType: str, payloadHash: str) -> None:
        self._ensureOpen()
        _validateNonEmpty(factType, "事实类型")
        _validateHash(payloadHash)
        self._facts.append((factType, payloadHash))

    def enqueue(self, messageId: str, topic: str, payloadHash: str) -> None:
        self._ensureOpen()
        _validateNonEmpty(messageId, "消息 ID")
        _validateNonEmpty(topic, "主题")
        _validateHash(payloadHash)
        if any(item[0] == messageId for item in self._messages):
            raise TransactionError("同一事务不得重复写入 outbox messageId")
        self._messages.append((messageId, topic, payloadHash))

    def commit(self) -> tuple[DomainFactV1, ...]:
        self._ensureOpen()
        if not self._facts:
            raise TransactionError("事务至少必须包含一个领域事实")
        knownIds = {item.messageId for item in self._store._outbox}
        if any(messageId in knownIds for messageId, _, _ in self._messages):
            raise TransactionError("outbox messageId 已提交，拒绝重复外部副作用")
        sequence = self._store._nextSequence
        facts = tuple(DomainFactV1(sequence + index, factType, payloadHash) for index, (factType, payloadHash) in enumerate(self._facts))
        outboxStart = sequence + len(facts)
        messages = tuple(OutboxMessageV1(messageId, outboxStart + index, topic, payloadHash, OutboxStatus.Pending) for index, (messageId, topic, payloadHash) in enumerate(self._messages))
        self._store._facts.extend(facts)
        self._store._outbox.extend(messages)
        self._store._nextSequence = outboxStart + len(messages)
        self._closed = True
        return facts

    def rollback(self) -> None:
        self._ensureOpen()
        self._facts.clear()
        self._messages.clear()
        self._closed = True

    def _ensureOpen(self) -> None:
        if self._closed:
            raise TransactionError("事务已关闭")


def _validateNonEmpty(value: str, label: str) -> None:
    if not value:
        raise TransactionError(f"{label}不能为空")


def _validateHash(value: str) -> None:
    if len(value) != 64 or any(character not in "0123456789abcdef" for character in value):
        raise TransactionError("内容哈希必须为小写 SHA-256")
