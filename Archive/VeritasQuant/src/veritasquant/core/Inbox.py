"""可重试输入的幂等与协议冲突基线。"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


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


class InboxV1:
    """阶段 1 进程内实现；持久化事务实现由后续任务替换。"""

    def __init__(self) -> None:
        self._accepted: dict[str, InboxReceiptV1] = {}
        self._conflicts: list[InboxConflictV1] = []
        self._nextSequence = 1

    def accept(self, idempotencyKey: str, contentHash: str) -> InboxReceiptV1:
        """同键同哈希去重；同键异哈希隔离并拒绝。"""
        _validateInput(idempotencyKey, contentHash)
        existing = self._accepted.get(idempotencyKey)
        if existing is None:
            receipt = InboxReceiptV1(idempotencyKey, contentHash, self._nextSequence, InboxDisposition.Applied)
            self._nextSequence += 1
            self._accepted[idempotencyKey] = receipt
            return receipt
        if existing.contentHash == contentHash:
            return InboxReceiptV1(idempotencyKey, contentHash, existing.receiptSequence, InboxDisposition.Duplicate)
        conflict = InboxConflictV1(idempotencyKey, existing.contentHash, contentHash)
        self._conflicts.append(conflict)
        raise InboxError("同一幂等键对应不同内容哈希，已隔离协议冲突")

    @property
    def conflicts(self) -> tuple[InboxConflictV1, ...]:
        """返回不可修改的冲突审计快照。"""
        return tuple(self._conflicts)


def _validateInput(idempotencyKey: str, contentHash: str) -> None:
    if not idempotencyKey:
        raise InboxError("幂等键不能为空")
    if len(contentHash) != 64 or any(character not in "0123456789abcdef" for character in contentHash):
        raise InboxError("内容哈希必须为小写 SHA-256")
