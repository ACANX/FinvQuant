"""P1-025 UTC 逻辑时钟与阶段推进器。

时钟只前进：任何回退、非法跨阶段或以系统时间替代逻辑时钟的行为都必须被拒绝。
派生事件继承关联 ts 与排序版本，并进入更后的合法阶段，不得重新进入更早阶段。
"""

from __future__ import annotations

from datetime import datetime
from enum import IntEnum

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Time import TsPrecision, parseUtcTimestamp


class LogicalClockError(ValueError):
    """逻辑时钟推进或阶段派生违反因果契约。"""


class ClockPhase(IntEnum):
    """与 EventOrderingVersion V1 一致的固定阶段表。"""

    MATCH_PRIOR_ORDERS = 10
    APPLY_EXECUTION_AND_LEDGER = 20
    DISPATCH_CURRENT_EVENT = 30
    GENERATE_ORDER_INTENT = 40
    EVALUATE_RISK = 50
    ENQUEUE_APPROVED_ORDER = 60

    @classmethod
    def validate(cls, phase: int) -> "ClockPhase":
        try:
            return cls(phase)
        except ValueError as error:
            raise LogicalClockError(f"未知阶段: {phase}") from error


class UtcLogicalClockV1:
    """仅前进的 UTC 逻辑时钟；禁止读取系统时间作为推进依据。"""

    def __init__(self, tsPrecision: TsPrecision) -> None:
        self._tsPrecision = tsPrecision
        self._now: datetime | None = None

    @property
    def now(self) -> datetime | None:
        """当前逻辑时刻；未推进时为 None。"""
        return self._now

    def advance(self, ts: datetime) -> datetime:
        """推进到 `ts`；任何回退（早于当前逻辑时刻）都被拒绝。"""
        normalized = parseUtcTimestamp(ts, self._tsPrecision)
        if self._now is not None and normalized < self._now:
            raise LogicalClockError("逻辑时钟只前进，拒绝回退")
        if normalized == self._now:
            return self._now
        self._now = normalized
        return self._now

    def observe(self, event: EventEnvelopeV1) -> datetime:
        """按事件 ts 推进时钟；事件必须符合运行精度。"""
        event.validateTsPrecision(self._tsPrecision)
        return self.advance(event.ts)

    def checkNotBeyond(self, ts: datetime) -> datetime:
        """校验查询时间不大于逻辑时钟（防前视访问入口）。"""
        normalized = parseUtcTimestamp(ts, self._tsPrecision)
        if self._now is None or normalized > self._now:
            raise LogicalClockError("查询时间不得超越逻辑时钟")
        return normalized


class PhaseAdvancerV1:
    """阶段推进器：派生事件必须进入更后阶段，禁止回到更早阶段。"""

    def __init__(self, tsPrecision: TsPrecision) -> None:
        self._tsPrecision = tsPrecision

    def derive(
        self,
        parent: EventEnvelopeV1,
        *,
        eventId: str,
        eventType: str,
        phase: int,
        priority: int = 0,
        sourceRank: int = 0,
        sourceSequence: int = 0,
        producer: str,
        producerVersion: str,
        correlationId: str | None = None,
        payload: object,
    ) -> EventEnvelopeV1:
        """从父事件派生新事件；继承 ts、排序版本和因果引用。

        - ts 必须不早于父事件 ts
        - 同一 ts 下 phase 必须严格大于父事件 phase
        - 更早 ts 或更早 phase 均被拒绝
        """
        parentPhase = ClockPhase.validate(parent.phase)
        derivedPhase = ClockPhase.validate(phase)
        if derivedPhase <= parentPhase:
            raise LogicalClockError("派生事件不得回到更早或相同阶段")
        return EventEnvelopeV1.create(
            eventId=eventId,
            eventType=eventType,
            schemaVersion=parent.schemaVersion,
            runId=parent.runId,
            ts=parent.ts,
            occurredAt=parent.occurredAt,
            publishedAt=parent.publishedAt,
            ingestedAt=parent.ingestedAt,
            source=parent.source,
            producer=producer,
            producerVersion=producerVersion,
            correlationId=correlationId if correlationId is not None else parent.correlationId,
            causationId=parent.eventId,
            accountId=parent.accountId,
            subaccountId=parent.subaccountId,
            eventOrderingVersion=parent.eventOrderingVersion,
            phase=phase,
            priority=priority,
            sourceRank=sourceRank,
            sourceSequence=sourceSequence,
            payload=payload,
        )

    def deriveWithClock(
        self,
        clock: UtcLogicalClockV1,
        parent: EventEnvelopeV1,
        *,
        eventId: str,
        eventType: str,
        phase: int,
        priority: int = 0,
        sourceRank: int = 0,
        sourceSequence: int = 0,
        producer: str,
        producerVersion: str,
        correlationId: str | None = None,
        payload: object,
    ) -> EventEnvelopeV1:
        """派生并推进逻辑时钟；时钟与阶段约束同时生效。"""
        derived = self.derive(
            parent,
            eventId=eventId,
            eventType=eventType,
            phase=phase,
            priority=priority,
            sourceRank=sourceRank,
            sourceSequence=sourceSequence,
            producer=producer,
            producerVersion=producerVersion,
            correlationId=correlationId,
            payload=payload,
        )
        clock.observe(derived)
        return derived
