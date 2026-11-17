"""版本化事件信封、内容哈希与因果链校验。"""

from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import field_validator, model_validator

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import EventPayloadV1, PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class EventContractError(ValueError):
    """事件不可变契约或因果链不满足时抛出。"""


class EventEnvelopeV1(StrictModel):
    """所有进入统一事件总线的强类型不可变信封。"""

    eventId: str = PascalAlias("EventId", min_length=1)
    eventType: str = PascalAlias("EventType", min_length=1)
    schemaVersion: str = PascalAlias("SchemaVersion", min_length=3)
    runId: str = PascalAlias("RunId", min_length=1)
    ts: datetime = PascalAlias("Ts")
    occurredAt: datetime | None = PascalAlias("OccurredAt", default=None)
    publishedAt: datetime | None = PascalAlias("PublishedAt", default=None)
    ingestedAt: datetime = PascalAlias("IngestedAt")
    source: str = PascalAlias("Source", min_length=1)
    producer: str = PascalAlias("Producer", min_length=1)
    producerVersion: str = PascalAlias("ProducerVersion", min_length=1)
    correlationId: str = PascalAlias("CorrelationId", min_length=1)
    causationId: str | None = PascalAlias("CausationId", default=None)
    accountId: str | None = PascalAlias("AccountId", default=None)
    subaccountId: str | None = PascalAlias("SubaccountId", default=None)
    eventOrderingVersion: str = PascalAlias("EventOrderingVersion", min_length=1)
    phase: int = PascalAlias("Phase", ge=10, le=60)
    priority: int = PascalAlias("Priority", ge=0)
    sourceRank: int = PascalAlias("SourceRank", ge=0)
    sourceSequence: int = PascalAlias("SourceSequence", ge=0)
    payload: Any = PascalAlias("Payload")
    contentHash: str = PascalAlias("ContentHash", min_length=64, max_length=64)

    @field_validator("ts", "occurredAt", "publishedAt", "ingestedAt")
    @classmethod
    def validateTimestamp(cls, value: datetime | None) -> datetime | None:
        if value is None:
            return None
        # 信封只接受已符合最细支持精度的 UTC datetime；运行精度另由事件循环校验。
        return validateUtcTimestamp(value, TsPrecision.Millisecond)

    @field_validator("payload")
    @classmethod
    def validatePayload(cls, value: Any) -> EventPayloadV1:
        if not isinstance(value, EventPayloadV1) or type(value) is EventPayloadV1:
            raise EventContractError("事件载荷必须是已声明字段的 EventPayloadV1 子类")
        return value

    @model_validator(mode="after")
    def validateEnvelope(self) -> "EventEnvelopeV1":
        if self.causationId == self.eventId:
            raise EventContractError("事件不得引用自身作为 causationId")
        if self.subaccountId is not None and self.accountId is None:
            raise EventContractError("subaccountId 存在时必须同时提供 accountId")
        if self.eventOrderingVersion != "V1":
            raise EventContractError("阶段 1 仅接受 EventOrderingVersion V1")
        if self.phase not in {10, 20, 30, 40, 50, 60}:
            raise EventContractError("phase 必须为 V1 固定六阶段之一")
        if self.contentHash != self.calculateContentHash():
            raise EventContractError("contentHash 与信封内容不一致")
        return self

    @classmethod
    def create(cls, **values: Any) -> "EventEnvelopeV1":
        """从 Python 内部字段创建并写入准确的内容哈希。"""
        values["contentHash"] = "0" * 64
        draft = cls.model_construct(**values)
        values["contentHash"] = draft.calculateContentHash()
        wireValues = {
            fieldInfo.validation_alias: values[fieldName]
            for fieldName, fieldInfo in cls.model_fields.items()
        }
        return cls.model_validate(wireValues)

    def calculateContentHash(self) -> str:
        """计算除 contentHash 外全部不可变字段和强类型载荷的哈希。"""
        content = self.model_dump(mode="python", by_alias=False, exclude_none=False)
        content.pop("contentHash", None)
        return canonicalHash(content, TsPrecision.Millisecond)

    def requireAccountScope(self) -> "EventEnvelopeV1":
        """供账户域事件注册表在入总线前强制账户作用域。"""
        if self.accountId is None:
            raise EventContractError("账户域事件必须填写 accountId")
        return self

    def validateTsPrecision(self, tsPrecision: TsPrecision) -> "EventEnvelopeV1":
        """校验信封所有事件时间都符合当前运行精度。"""
        for value in (self.ts, self.occurredAt, self.publishedAt, self.ingestedAt):
            if value is not None:
                validateUtcTimestamp(value, tsPrecision)
        return self


class CausalityTracker:
    """按已接收事件验证全局唯一 ID 与直接父引用。"""

    def __init__(self) -> None:
        self._events: dict[str, EventEnvelopeV1] = {}

    def accept(self, event: EventEnvelopeV1, accountRequired: bool = False) -> None:
        """登记一个事件；违反 ID、父引用或运行边界时拒绝。"""
        if event.eventId in self._events:
            raise EventContractError(f"重复 eventId: {event.eventId}")
        if accountRequired:
            event.requireAccountScope()
        if event.causationId is not None:
            parent = self._events.get(event.causationId)
            if parent is None:
                raise EventContractError("causationId 未引用已接收的直接父事件")
            if parent.runId != event.runId or parent.correlationId != event.correlationId:
                raise EventContractError("派生事件必须与直接父事件共享 runId 和 correlationId")
            if event.ts < parent.ts:
                raise EventContractError("派生事件 ts 不得早于直接父事件")
            if event.ts == parent.ts and event.phase <= parent.phase:
                raise EventContractError("同一 ts 的派生事件必须进入更后阶段")
            if parent.accountId is not None and event.accountId != parent.accountId:
                raise EventContractError("派生事件不得改变账户作用域")
        self._events[event.eventId] = event
