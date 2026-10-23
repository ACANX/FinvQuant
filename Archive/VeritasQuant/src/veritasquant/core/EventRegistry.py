"""事件 Schema 注册、隔离与确定性升级。"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Any, Callable, Mapping

from pydantic import create_model

from pydantic import ValidationError

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision, parseUtcTimestamp


# 技术方案要求阶段 1 即可识别的核心事件类型；领域任务会在此基础上补充语义字段。
_INITIAL_EVENT_TYPES = (
    "MarketBarEvent",
    "CorporateActionEvent",
    "OrderEvent",
    "CancelOrderEvent",
    "ReplaceOrderEvent",
    "ExecutionReportEvent",
    "RiskSignalEvent",
    "AlertEvent",
    "AlertNormalizationFailureEvent",
    "RiskDecisionEvent",
    "TradingControlEvent",
    "LedgerJournalEvent",
)


class SchemaRegistryError(ValueError):
    """Schema 注册表配置或升级路径不合法。"""


@dataclass(frozen=True, order=True)
class SchemaVersion:
    """受限的 MAJOR.MINOR Schema 版本。"""

    major: int
    minor: int

    _PATTERN = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)$")

    @classmethod
    def parse(cls, value: str) -> "SchemaVersion":
        match = cls._PATTERN.fullmatch(value)
        if match is None:
            raise SchemaRegistryError(f"SchemaVersion 非法: {value}")
        return cls(int(match.group(1)), int(match.group(2)))

    def __str__(self) -> str:
        return f"{self.major}.{self.minor}"


@dataclass(frozen=True)
class SchemaRegistration:
    """一个 event_type + schema_version 的不可变声明。"""

    eventType: str
    schemaVersion: SchemaVersion
    payloadModel: type[EventPayloadV1]
    ownerModule: str
    compatibleConsumerRange: str


@dataclass(frozen=True)
class UpgraderRegistration:
    """同一主版本内的纯函数升级器。"""

    eventType: str
    sourceVersion: SchemaVersion
    targetVersion: SchemaVersion
    upgraderVersion: str
    upgrade: Callable[[EventPayloadV1], EventPayloadV1]


@dataclass(frozen=True)
class QuarantinedEvent:
    """不能进入事件总线的原始事件摘要。"""

    reasonCode: str
    eventType: str | None
    schemaVersion: str | None
    rawHash: str
    message: str


@dataclass(frozen=True)
class RegistryResult:
    """注册表处理结果，成功时保留原始信封与当前载荷。"""

    envelope: EventEnvelopeV1 | None
    currentPayload: EventPayloadV1 | None
    currentVersion: SchemaVersion | None
    quarantine: QuarantinedEvent | None


class EventSchemaRegistry:
    """拒绝猜测、只允许显式升级路径的事件 Schema 注册表。"""

    def __init__(self) -> None:
        self._registrations: dict[tuple[str, SchemaVersion], SchemaRegistration] = {}
        self._upgraders: dict[tuple[str, SchemaVersion], UpgraderRegistration] = {}

    def register(self, registration: SchemaRegistration) -> None:
        key = (registration.eventType, registration.schemaVersion)
        if key in self._registrations:
            raise SchemaRegistryError(f"重复注册 Schema: {registration.eventType} {registration.schemaVersion}")
        if not issubclass(registration.payloadModel, EventPayloadV1):
            raise SchemaRegistryError("事件载荷模型必须继承 EventPayloadV1")
        self._registrations[key] = registration

    @classmethod
    def createInitialRegistry(cls) -> "EventSchemaRegistry":
        """注册阶段 1 必需的核心事件类型，后续任务再扩展各自领域载荷字段。"""
        registry = cls()
        for eventType in _INITIAL_EVENT_TYPES:
            # 每个类型拥有独立的冻结 Pydantic 模型，拒绝无约束 dict 作为最终载荷。
            payloadModel = create_model(
                f"{eventType}PayloadV1",
                __base__=EventPayloadV1,
                schemaMarker=(str, PascalAlias("SchemaMarker", default="V1")),
            )
            registry.register(
                SchemaRegistration(
                    eventType=eventType,
                    schemaVersion=SchemaVersion(1, 0),
                    payloadModel=payloadModel,
                    ownerModule="veritasquant.core",
                    compatibleConsumerRange=">=1.0,<2.0",
                )
            )
        return registry

    def registerUpgrader(self, registration: UpgraderRegistration) -> None:
        if registration.sourceVersion.major != registration.targetVersion.major:
            raise SchemaRegistryError("禁止自动跨主版本升级或降级")
        if registration.sourceVersion >= registration.targetVersion:
            raise SchemaRegistryError("升级器目标版本必须高于来源版本")
        if (registration.eventType, registration.sourceVersion) not in self._registrations:
            raise SchemaRegistryError("升级器来源 Schema 尚未注册")
        if (registration.eventType, registration.targetVersion) not in self._registrations:
            raise SchemaRegistryError("升级器目标 Schema 尚未注册")
        key = (registration.eventType, registration.sourceVersion)
        if key in self._upgraders:
            raise SchemaRegistryError("同一来源版本只能有一个确定性升级器")
        self._upgraders[key] = registration

    def registryHash(self) -> str:
        """为运行清单提供注册表内容身份。"""
        rows = [
            {
                "eventType": item.eventType,
                "schemaVersion": str(item.schemaVersion),
                "payloadSchemaHash": canonicalHash(item.payloadModel.model_json_schema()),
                "ownerModule": item.ownerModule,
                "compatibleConsumerRange": item.compatibleConsumerRange,
            }
            for item in self._registrations.values()
        ]
        return canonicalHash(rows)

    def decodeAndUpgrade(
        self, rawEvent: Mapping[str, Any], tsPrecision: TsPrecision
    ) -> RegistryResult:
        """校验原始信封并升级到当前同主版本载荷，失败即隔离。"""
        eventType = self._read(rawEvent, "EventType", "eventType")
        schemaVersionValue = self._read(rawEvent, "SchemaVersion", "schemaVersion")
        try:
            if not isinstance(eventType, str) or not isinstance(schemaVersionValue, str):
                raise SchemaRegistryError("缺少 EventType 或 SchemaVersion")
            sourceVersion = SchemaVersion.parse(schemaVersionValue)
            registration = self._registrations.get((eventType, sourceVersion))
            if registration is None:
                knownMajors = {
                    version.major
                    for registeredType, version in self._registrations
                    if registeredType == eventType
                }
                reason = "UNKNOWN_MAJOR_VERSION" if sourceVersion.major not in knownMajors else "UNREGISTERED_SCHEMA"
                return self._quarantine(reason, eventType, schemaVersionValue, rawEvent)
            payloadRaw = self._read(rawEvent, "Payload", "payload")
            payload = registration.payloadModel.model_validate(payloadRaw)
            envelopeInput = dict(rawEvent)
            for key in ("Ts", "OccurredAt", "PublishedAt", "IngestedAt"):
                if key in envelopeInput and envelopeInput[key] is not None:
                    envelopeInput[key] = parseUtcTimestamp(envelopeInput[key], tsPrecision)
            envelopeInput["Payload"] = payload
            envelope = EventEnvelopeV1.model_validate(envelopeInput)
            envelope.validateTsPrecision(tsPrecision)
            currentPayload, currentVersion = self._upgrade(eventType, sourceVersion, payload)
            return RegistryResult(envelope, currentPayload, currentVersion, None)
        except (SchemaRegistryError, ValidationError, ValueError, TypeError) as error:
            return self._quarantine("SCHEMA_VALIDATION_FAILED", eventType, schemaVersionValue, rawEvent, str(error))

    def _upgrade(
        self, eventType: str, sourceVersion: SchemaVersion, payload: EventPayloadV1
    ) -> tuple[EventPayloadV1, SchemaVersion]:
        currentPayload = payload
        currentVersion = sourceVersion
        while (eventType, currentVersion) in self._upgraders:
            upgrader = self._upgraders[(eventType, currentVersion)]
            upgraded = upgrader.upgrade(currentPayload)
            target = self._registrations[(eventType, upgrader.targetVersion)]
            if not isinstance(upgraded, target.payloadModel):
                raise SchemaRegistryError("升级器输出不符合目标强类型载荷")
            currentPayload = upgraded
            currentVersion = upgrader.targetVersion
        return currentPayload, currentVersion

    @staticmethod
    def _read(rawEvent: Mapping[str, Any], alias: str, internalName: str) -> Any:
        if alias in rawEvent:
            return rawEvent[alias]
        return rawEvent.get(internalName)

    @staticmethod
    def _quarantine(
        reasonCode: str,
        eventType: str | None,
        schemaVersion: str | None,
        rawEvent: Mapping[str, Any],
        message: str = "",
    ) -> RegistryResult:
        quarantine = QuarantinedEvent(
            reasonCode=reasonCode,
            eventType=eventType if isinstance(eventType, str) else None,
            schemaVersion=schemaVersion if isinstance(schemaVersion, str) else None,
            rawHash=canonicalHash(dict(rawEvent)),
            message=message,
        )
        return RegistryResult(None, None, None, quarantine)
