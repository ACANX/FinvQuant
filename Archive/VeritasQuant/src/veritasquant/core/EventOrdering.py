"""EventOrderingVersion V1 的固定六阶段全序。"""

from __future__ import annotations

from enum import IntEnum
from typing import Iterable

from veritasquant.core.Events import EventContractError, EventEnvelopeV1
from veritasquant.core.Time import TsPrecision, serializeUtcTimestamp


EVENT_ORDERING_VERSION_V1 = "V1"


class EventPhase(IntEnum):
    MATCH_PRIOR_ORDERS = 10
    APPLY_EXECUTION_AND_LEDGER = 20
    DISPATCH_CURRENT_EVENT = 30
    GENERATE_ORDER_INTENT = 40
    EVALUATE_RISK = 50
    ENQUEUE_APPROVED_ORDER = 60


def eventOrderingKey(event: EventEnvelopeV1, tsPrecision: TsPrecision) -> tuple[str, int, int, int, int, str]:
    """返回技术方案固定字段顺序的 V1 排序键。"""
    if event.eventOrderingVersion != EVENT_ORDERING_VERSION_V1:
        raise EventContractError("当前排序器只支持 EventOrderingVersion V1")
    event.validateTsPrecision(tsPrecision)
    return (
        serializeUtcTimestamp(event.ts, tsPrecision),
        event.phase,
        event.priority,
        event.sourceRank,
        event.sourceSequence,
        event.eventId,
    )


def sortEvents(events: Iterable[EventEnvelopeV1], tsPrecision: TsPrecision) -> list[EventEnvelopeV1]:
    """按照完整 V1 排序键形成跨来源确定性全序。"""
    return sorted(events, key=lambda event: eventOrderingKey(event, tsPrecision))
