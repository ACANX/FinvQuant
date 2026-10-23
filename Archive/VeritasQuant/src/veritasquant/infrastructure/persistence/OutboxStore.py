"""P2-002 数据库端 outbox：领域提交后至少一次投递。

- 领域状态与待发布事件在同一 PostgreSQL 事务提交（由调用方控制事务边界）；
- 发布器按提交序号升序扫描 PENDING 消息，投递成功后标记 PUBLISHED；
- 失败条目保留，下一次以同一 message_id 重试，绝不重复副作用；
- 写入前必须通过租约 guard，旧 fencing token 的写入被持久层拒绝。
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from enum import StrEnum

from psycopg import Connection

from veritasquant.infrastructure.persistence.LeaseStore import LeaseStoreV1

_ENQUEUE_SQL = """
INSERT INTO outbox_records (
    outbox_id, message_id, sequence, topic, payload_hash, status,
    run_id, partition_id, created_at
) VALUES (%s, %s, nextval('outbox_sequence_seq'), %s, %s, 'PENDING', %s, %s, now())
ON CONFLICT (run_id, partition_id, message_id) DO NOTHING
"""

_PENDING_SQL = """
SELECT outbox_id, message_id, sequence, topic, payload_hash
FROM outbox_records
WHERE run_id = %s AND partition_id = %s AND status = 'PENDING'
ORDER BY sequence ASC
LIMIT %s
"""

_MARK_PUBLISHED_SQL = """
UPDATE outbox_records
SET status = 'PUBLISHED', published_at = now(), attempt_count = attempt_count + 1
WHERE outbox_id = %s AND status = 'PENDING'
"""


class OutboxError(RuntimeError):
    """outbox 写入或发布不合法。"""


class OutboxStatus(StrEnum):
    Pending = "PENDING"
    Published = "PUBLISHED"


@dataclass(frozen=True, slots=True)
class OutboxMessageV1:
    """一条待投递消息的持久化视图。"""

    outboxId: str
    messageId: str
    sequence: int
    topic: str
    payloadHash: str
    status: OutboxStatus


class OutboxStoreV1:
    """PostgreSQL 持久化 outbox，语义与 P1-028 TransactionStoreV1 一致。"""

    def __init__(self, connection: Connection, leaseStore: LeaseStoreV1) -> None:
        self._connection = connection
        self._leaseStore = leaseStore

    def enqueue(
        self,
        messageId: str,
        topic: str,
        payloadHash: str,
        runId: str,
        partitionId: str,
        accountGroupId: str,
        holder: str,
        fencingToken: int,
    ) -> OutboxMessageV1:
        """登记待发布消息；同 message_id 幂等（返回既有记录）。"""
        _validateInput(messageId, payloadHash)
        with self._connection.transaction():
            self._leaseStore.guard(accountGroupId, holder, fencingToken)
            outboxId = f"ob:{messageId}"
            self._connection.execute(
                _ENQUEUE_SQL, (outboxId, messageId, topic, payloadHash, runId, partitionId)
            )
            row = self._connection.execute(
                "SELECT outbox_id, message_id, sequence, topic, payload_hash, status "
                "FROM outbox_records WHERE run_id = %s AND partition_id = %s AND message_id = %s",
                (runId, partitionId, messageId),
            ).fetchone()
            assert row is not None, "outbox 幂等写入后记录必须存在"
        return OutboxMessageV1(row[0], row[1], row[2], row[3], row[4], OutboxStatus(row[5]))

    def publishPending(
        self,
        publisher: Callable[[OutboxMessageV1], None],
        runId: str,
        partitionId: str,
        batchSize: int = 100,
    ) -> int:
        """按提交序号投递 PENDING 消息；投递成功才标记 PUBLISHED。"""
        if batchSize <= 0:
            raise OutboxError("batchSize 必须为正")
        published = 0
        with self._connection.transaction():
            rows = self._connection.execute(
                _PENDING_SQL, (runId, partitionId, batchSize)
            ).fetchall()
            for row in rows:
                message = OutboxMessageV1(row[0], row[1], row[2], row[3], row[4], OutboxStatus.Pending)
                publisher(message)
                cursor = self._connection.execute(_MARK_PUBLISHED_SQL, (message.outboxId,))
                if cursor.rowcount == 1:
                    published += 1
        return published

    def pendingCount(self, runId: str, partitionId: str) -> int:
        """返回分区内尚未发布的消息数（监控/背压用）。"""
        row = self._connection.execute(
            "SELECT count(*) FROM outbox_records "
            "WHERE run_id = %s AND partition_id = %s AND status = 'PENDING'",
            (runId, partitionId),
        ).fetchone()
        assert row is not None
        return int(row[0])


def _validateInput(messageId: str, payloadHash: str) -> None:
    if not messageId:
        raise OutboxError("message_id 不能为空")
    if len(payloadHash) != 64 or any(character not in "0123456789abcdef" for character in payloadHash):
        raise OutboxError("载荷哈希必须为小写 SHA-256")
