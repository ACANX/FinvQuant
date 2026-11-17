"""RiskSignal、标准化失败与 AlertEvent 强类型模型（技术方案第 9 章）。

RiskSignal 是检测器的原始事实，不直接改变交易权限；AlertEvent 是关联、
去重、分级后可被全系统消费的预警；AlertNormalizationFailureEvent 是独立
审计事件，不参与预警生命周期。失败原始载荷不得静默丢弃。
"""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any

from pydantic import field_validator, model_validator

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class RiskContractError(ValueError):
    """风险模型不满足可追溯或生命周期契约时抛出。"""


class AlertSeverity(StrEnum):
    """预警严重度。"""

    P0 = "P0"
    P1 = "P1"
    P2 = "P2"
    P3 = "P3"


class AlertStatus(StrEnum):
    """预警生命周期状态。"""

    Active = "ACTIVE"
    Acknowledged = "ACKNOWLEDGED"
    Suppressed = "SUPPRESSED"
    Resolved = "RESOLVED"
    Expired = "EXPIRED"


class AlertEventType(StrEnum):
    """AlertEvent 事件类型。"""

    Created = "alert.created"
    Updated = "alert.updated"
    Resolved = "alert.resolved"


class RiskScopeV1(EventPayloadV1):
    """预警作用域：账户、策略、标的与市场白名单。"""

    accountIds: tuple[str, ...] = PascalAlias("AccountIds", min_length=1)
    strategyIds: tuple[str, ...] = PascalAlias("StrategyIds", default=())
    symbols: tuple[str, ...] = PascalAlias("Symbols", default=())
    markets: tuple[str, ...] = PascalAlias("Markets", default=())

    @model_validator(mode="after")
    def validateScope(self) -> "RiskScopeV1":
        if not self.accountIds:
            raise RiskContractError("作用域必须至少包含一个账户")
        return self


class RiskSignalV1(EventPayloadV1):
    """检测器或适配器生成的不可变原始风险事实。"""

    signalId: str = PascalAlias("SignalId", min_length=1)
    signalType: str = PascalAlias("SignalType", min_length=1)
    observedAt: datetime = PascalAlias("ObservedAt")
    detectedAt: datetime = PascalAlias("DetectedAt")
    source: str = PascalAlias("Source", min_length=1)
    scopeCandidate: dict[str, Any] = PascalAlias("ScopeCandidate")
    payload: dict[str, Any] = PascalAlias("Payload")
    evidence: tuple[dict[str, Any], ...] = PascalAlias("Evidence", default=())
    confidence: float | None = PascalAlias("Confidence", default=None, ge=0, le=1)
    ruleId: str | None = PascalAlias("RuleId", default=None, min_length=1)
    ruleVersion: str | None = PascalAlias("RuleVersion", default=None, min_length=1)

    @field_validator("observedAt", "detectedAt", mode="before")
    @classmethod
    def validateTimes(cls, value: object) -> Any:
        if not isinstance(value, datetime):
            raise RiskContractError("时间必须是 datetime")
        validateUtcTimestamp(value, TsPrecision.Millisecond)
        return value

    def payloadHash(self) -> str:
        """原始载荷哈希：失败审计与隔离区引用。"""
        return canonicalHash(
            {
                "signal_id": self.signalId,
                "signal_type": self.signalType,
                "payload": self.payload,
                "evidence": list(self.evidence),
            }
        )


class AlertEventV1(EventPayloadV1):
    """关联、去重、分级后的可消费预警；生命周期版本严格单调递增。"""

    alertId: str = PascalAlias("AlertId", min_length=1)
    alertVersion: int = PascalAlias("AlertVersion", ge=1)
    previousEventId: str | None = PascalAlias("PreviousEventId", default=None, min_length=1)
    alertType: str = PascalAlias("AlertType", min_length=1)
    severity: AlertSeverity = PascalAlias("Severity")
    status: AlertStatus = PascalAlias("Status")
    scope: RiskScopeV1 = PascalAlias("Scope")
    dedupeKey: str = PascalAlias("DedupeKey", min_length=1)
    correlationId: str | None = PascalAlias("CorrelationId", default=None, min_length=1)
    ruleId: str | None = PascalAlias("RuleId", default=None, min_length=1)
    ruleVersion: str | None = PascalAlias("RuleVersion", default=None, min_length=1)
    trigger: dict[str, Any] = PascalAlias("Trigger")
    evidence: tuple[dict[str, Any], ...] = PascalAlias("Evidence", default=())
    recommendedActions: tuple[str, ...] = PascalAlias("RecommendedActions", default=())
    expiresAt: datetime | None = PascalAlias("ExpiresAt", default=None)
    rawEventIds: tuple[str, ...] = PascalAlias("RawEventIds", default=())

    @field_validator("severity", "status", mode="before")
    @classmethod
    def parseEnums(cls, value: object, info: Any) -> Any:
        mapping: dict[str, Any] = {"severity": AlertSeverity, "status": AlertStatus}
        enumType = mapping[info.field_name]
        if isinstance(value, enumType):
            return value
        if not isinstance(value, str):
            raise RiskContractError(f"{info.field_name}必须是受控字符串")
        try:
            return enumType(value)
        except ValueError as error:
            raise RiskContractError(f"未知{info.field_name}: {value}") from error

    @field_validator("expiresAt", mode="before")
    @classmethod
    def validateExpiry(cls, value: object) -> Any:
        if value is None:
            return None
        if not isinstance(value, datetime):
            raise RiskContractError("过期时间必须是 datetime")
        validateUtcTimestamp(value, TsPrecision.Millisecond)
        return value

    @model_validator(mode="after")
    def validateAlert(self) -> "AlertEventV1":
        if self.alertVersion == 1 and self.previousEventId is not None:
            raise RiskContractError("创建版本不得引用 previousEventId")
        if self.alertVersion > 1 and self.previousEventId is None:
            raise RiskContractError("生命周期更新必须引用 previousEventId")
        if self.status in (AlertStatus.Resolved, AlertStatus.Expired) and self.alertVersion < 2:
            raise RiskContractError("终态必须经过至少一次生命周期更新")
        return self

    @classmethod
    def create(cls, **values: Any) -> "AlertEventV1":
        """从 Python 内部字段创建，自动填入创建事件类型。"""
        values["alertVersion"] = values.get("alertVersion", 1)
        values["previousEventId"] = values.get("previousEventId")
        return cls.model_validate(
            {fieldInfo.validation_alias: values.get(fieldName) for fieldName, fieldInfo in cls.model_fields.items()}
        )


class AlertNormalizationFailureEventV1(EventPayloadV1):
    """标准化失败的独立审计事件；不参与预警去重或交易控制。"""

    normalizationFailureId: str = PascalAlias("NormalizationFailureId", min_length=1)
    riskSignalId: str = PascalAlias("RiskSignalId", min_length=1)
    attemptedSchemaVersion: str = PascalAlias("AttemptedSchemaVersion", min_length=1)
    ruleId: str | None = PascalAlias("RuleId", default=None, min_length=1)
    ruleVersion: str | None = PascalAlias("RuleVersion", default=None, min_length=1)
    errorCodes: tuple[str, ...] = PascalAlias("ErrorCodes", min_length=1)
    rawPayloadHash: str = PascalAlias("RawPayloadHash", pattern=r"^[0-9a-f]{64}$")
    quarantineReference: str = PascalAlias("QuarantineReference", min_length=1)
    retryable: bool = PascalAlias("Retryable")
