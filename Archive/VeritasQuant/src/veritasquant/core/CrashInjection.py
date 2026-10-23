"""提交边界的确定性崩溃注入工具。"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class CrashPoint(StrEnum):
    BeforeInbox = "BEFORE_INBOX"
    AfterInbox = "AFTER_INBOX"
    BeforeLedger = "BEFORE_LEDGER"
    AfterLedger = "AFTER_LEDGER"
    BeforeOrder = "BEFORE_ORDER"
    AfterOrder = "AFTER_ORDER"
    BeforeControl = "BEFORE_CONTROL"
    AfterControl = "AFTER_CONTROL"
    BeforeCheckpoint = "BEFORE_CHECKPOINT"
    AfterCheckpoint = "AFTER_CHECKPOINT"
    BeforeOutbox = "BEFORE_OUTBOX"
    AfterOutbox = "AFTER_OUTBOX"


class CrashInjectedError(RuntimeError):
    """测试注入的受控崩溃，不表示真实进程故障。"""


@dataclass(frozen=True, slots=True)
class CrashInjectionRecordV1:
    point: CrashPoint
    hitSequence: int


class CrashInjectorV1:
    """仅在明确配置的命中次数触发，默认不注入。"""

    def __init__(self, point: CrashPoint | None = None, triggerHit: int = 1) -> None:
        if triggerHit < 1:
            raise ValueError("triggerHit 必须大于零")
        self._point = point
        self._triggerHit = triggerHit
        self._hits: dict[CrashPoint, int] = {}
        self._records: list[CrashInjectionRecordV1] = []

    def hit(self, point: CrashPoint) -> None:
        count = self._hits.get(point, 0) + 1
        self._hits[point] = count
        if self._point is point and count == self._triggerHit:
            record = CrashInjectionRecordV1(point, count)
            self._records.append(record)
            raise CrashInjectedError(f"已在 {point} 第 {count} 次命中注入崩溃")

    @property
    def records(self) -> tuple[CrashInjectionRecordV1, ...]:
        return tuple(self._records)
