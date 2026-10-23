"""P2-002 数据库端 inbox：可重试输入的幂等与协议冲突隔离。

- 同幂等键 + 同内容哈希：返回原提交结果（DUPLICATE），不重复副作用；
- 同幂等键 + 异内容哈希：写入隔离审计记录并抛出 InboxError（CONFLICT）；
- 写入前必须通过租约 guard，保证旧 fencing token 的写入被持久层拒绝。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from psycopg import Connection

from veritasquant.infrastructure.persistence.LeaseStore import LeaseStoreV1

# 幂等写入：冲突时读取既有记录，由应用层判定 DUPLICATE/CONFLICT
_ACCEPT_SQL = """
INSERT INTO inbox_records (
    idempotency_key, content_hash, receipt_sequence, disposition,
    run_id, partition_id, created_at
) VALUES (%s, %s, nextval('inbox_receipt_seq'), 'APPLIED', %s, %s, now())
ON CONFLICT (idempotency_key) DO NOTHING
"""

_FETCH_SQL = """
SELECT content_hash, receipt_sequence, disposition
FROM inbox_records WHERE idempotency_key = %s
"""

_RECORD_CONFLICT_SQL = """
INSERT INTO inbox_conflicts (
    conflict_id, idempotency_key, existing_content_hash,
    conflicting_content_hash, isolated_at, run_id, partition_id
) VALUES (%s, %s, %s, %s, now(), %s, %s)
ON CONFLICT (conflict_id) DO NOTHING
"""


class InboxError(ValueError):
    """inbox 键或内容哈希不满足协议。"""


class InboxDisposition(StrEnum):
    Applied = "APPLIED"
    Duplicate = "DUPLICATE"
    Conflict = "CONFLICT"


@dataclass(frozen=True, slots=True)
class InboxReceiptV1:
    """一次投递的不可变处理结果。"""

    idempotencyKey: str
    contentHash: str
    receiptSequence: int
    disposition: InboxDisposition


@dataclass(frozen=True, slots=True)
class InboxConflictV1:
    """同键异内容的隔离审计记录。"""

    idempotencyKey: str
    existingContentHash: str
    conflictingContentHash: str


class InboxStoreV1:
    """PostgreSQL 持久化 inbox，语义与 P1-027 InboxV1 一致。"""

    def __init__(self, connection: Connection, leaseStore: LeaseStoreV1) -> None:
        self._connection = connection
        self._leaseStore = leaseStore

    def accept(
        self,
        idempotencyKey: str,
        contentHash: str,
        runId: str,
        partitionId: str,
        accountGroupId: str,
        holder: str,
        fencingToken: int,
    ) -> InboxReceiptV1:
        """幂等接受；同键异哈希隔离冲突；旧 token 写入被拒。"""
        _validateInput(idempotencyKey, contentHash)
        with self._connection.transaction():
            self._leaseStore.guard(accountGroupId, holder, fencingToken)
            row = self._connection.execute(_FETCH_SQL, (idempotencyKey,)).fetchone()
            if row is None:
                # 首次投递：写入 APPLIED 并返回原结果
                self._connection.execute(
                    _ACCEPT_SQL, (idempotencyKey, contentHash, runId, partitionId)
                )
                sequence = self._receiptSequence(idempotencyKey)
                return InboxReceiptV1(idempotencyKey, contentHash, sequence, InboxDisposition.Applied)
            existingHash, sequence, disposition = row
            if existingHash == contentHash:
                # 同键同哈希：重投返回原结果（DUPLICATE），不重复副作用
                return InboxReceiptV1(
                    idempotencyKey, contentHash, sequence, InboxDisposition.Duplicate
                )
        # 同键异哈希：在独立事务中写入隔离审计后抛出（TechSpec 6.1）
        with self._connection.transaction():
            conflictId = _conflictId(idempotencyKey)
            self._connection.execute(
                _RECORD_CONFLICT_SQL,
                (conflictId, idempotencyKey, existingHash, contentHash, runId, partitionId),
            )
        raise InboxError("同一幂等键对应不同内容哈希，已隔离协议冲突")

    def _receiptSequence(self, idempotencyKey: str) -> int:
        row = self._connection.execute(
            "SELECT receipt_sequence FROM inbox_records WHERE idempotency_key = %s",
            (idempotencyKey,),
        ).fetchone()
        assert row is not None
        return int(row[0])


def _validateInput(idempotencyKey: str, contentHash: str) -> None:
    if not idempotencyKey:
        raise InboxError("幂等键不能为空")
    if len(contentHash) != 64 or any(character not in "0123456789abcdef" for character in contentHash):
        raise InboxError("内容哈希必须为小写 SHA-256")


def _conflictId(idempotencyKey: str) -> str:
    return f"conflict:{idempotencyKey}"
