"""AlertNormalizer：把 RiskSignal 标准化为 AlertEvent（技术方案第 9 章）。

标准化失败生成 AlertNormalizationFailureEventV1 审计事件并进入隔离区；
失败原始载荷不得静默丢弃或直接驱动交易。敏感原始载荷不复制到事件中。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertNormalizationFailureEventV1,
    AlertSeverity,
    AlertStatus,
    RiskScopeV1,
    RiskSignalV1,
)


class NormalizationError(ValueError):
    """标准化输入或配置不满足严格 Schema/枚举契约时抛出。"""


class NormalizationResultKind(StrEnum):
    Normalized = "NORMALIZED"
    Failed = "FAILED"
    Quarantined = "QUARANTINED"


@dataclass(frozen=True, slots=True)
class NormalizationOutcomeV1:
    """单条信号标准化结果。"""

    kind: NormalizationResultKind
    alert: AlertEventV1 | None
    failure: AlertNormalizationFailureEventV1 | None
    signalId: str
    message: str = ""


@dataclass(slots=True)
class NormalizationIsolationRecordV1:
    """隔离区记录：失败信号的可追溯摘要。"""

    signalId: str
    rawPayloadHash: str
    quarantineReference: str
    errorCodes: tuple[str, ...]
    retryable: bool


class AlertNormalizerV1:
    """把受控风险信号映射为 AlertEvent；未知类型/严重度进入隔离区。"""

    def __init__(
        self,
        *,
        normalizationVersion: str = "V1",
        quarantinePrefix: str = "quarantine/risk",
    ) -> None:
        if not normalizationVersion or not quarantinePrefix:
            raise NormalizationError("标准化版本与隔离前缀不能为空")
        self._normalizationVersion = normalizationVersion
        self._quarantinePrefix = quarantinePrefix
        self._isolation: dict[str, NormalizationIsolationRecordV1] = {}
        self._counter = 0

    @property
    def normalizationVersion(self) -> str:
        return self._normalizationVersion

    def normalize(self, signal: RiskSignalV1, severityMap: dict[str, AlertSeverity] | None = None) -> NormalizationOutcomeV1:
        """标准化一条风险信号；任何失败均不静默丢弃。"""
        self._counter += 1
        try:
            severity = self._resolveSeverity(signal.signalType, severityMap or {})
            scope = self._resolveScope(signal)
            dedupeKey = f"{signal.signalType}|{self._scopeKey(scope)}"
            alert = AlertEventV1.model_validate(
                {
                    "AlertId": f"alert-{signal.signalId}",
                    "AlertVersion": 1,
                    "AlertType": signal.signalType,
                    "Severity": severity,
                    "Status": AlertStatus.Active,
                    "Scope": scope,
                    "DedupeKey": dedupeKey,
                    "CorrelationId": signal.signalId,
                    "RuleId": signal.ruleId,
                    "RuleVersion": signal.ruleVersion,
                    "Trigger": dict(signal.payload),
                    "Evidence": tuple(signal.evidence),
                    "RecommendedActions": (),
                    "ExpiresAt": None,
                    "RawEventIds": (signal.signalId,),
                }
            )
            return NormalizationOutcomeV1(NormalizationResultKind.Normalized, alert, None, signal.signalId)
        except (ValueError, TypeError) as error:
            return self._fail(signal, ("NORMALIZATION_FAILED",), str(error), retryable=True)

    def isolationRecords(self) -> tuple[NormalizationIsolationRecordV1, ...]:
        """返回隔离区记录。"""
        return tuple(self._isolation.values())

    def _resolveSeverity(self, signalType: str, severityMap: dict[str, AlertSeverity]) -> AlertSeverity:
        severity = severityMap.get(signalType)
        if severity is None:
            raise NormalizationError(f"未知信号类型缺少严重度映射: {signalType}")
        return severity

    def _resolveScope(self, signal: RiskSignalV1) -> RiskScopeV1:
        candidate = signal.scopeCandidate
        accountIds = candidate.get("account_ids") or candidate.get("accountIds")
        strategyIds = candidate.get("strategy_ids") or candidate.get("strategyIds") or ()
        symbols = candidate.get("symbols") or ()
        markets = candidate.get("markets") or ()
        if not accountIds:
            raise NormalizationError("作用域候选缺少 account_ids")
        return RiskScopeV1.model_validate(
            {
                "AccountIds": tuple(accountIds),
                "StrategyIds": tuple(strategyIds),
                "Symbols": tuple(symbols),
                "Markets": tuple(markets),
            }
        )

    @staticmethod
    def _scopeKey(scope: RiskScopeV1) -> str:
        return canonicalHash(
            {
                "accounts": list(scope.accountIds),
                "strategies": list(scope.strategyIds),
                "symbols": list(scope.symbols),
                "markets": list(scope.markets),
            }
        )

    def _fail(
        self, signal: RiskSignalV1, errorCodes: tuple[str, ...], message: str, retryable: bool
    ) -> NormalizationOutcomeV1:
        rawHash = signal.payloadHash()
        reference = f"{self._quarantinePrefix}/{_utcDate()}/{signal.signalId}"
        record = NormalizationIsolationRecordV1(
            signalId=signal.signalId,
            rawPayloadHash=rawHash,
            quarantineReference=reference,
            errorCodes=errorCodes,
            retryable=retryable,
        )
        self._isolation[signal.signalId] = record
        failure = AlertNormalizationFailureEventV1.model_validate(
            {
                "NormalizationFailureId": f"nf-{signal.signalId}",
                "RiskSignalId": signal.signalId,
                "AttemptedSchemaVersion": self._normalizationVersion,
                "RuleId": signal.ruleId,
                "RuleVersion": signal.ruleVersion,
                "ErrorCodes": errorCodes,
                "RawPayloadHash": rawHash,
                "QuarantineReference": reference,
                "Retryable": retryable,
            }
        )
        return NormalizationOutcomeV1(
            NormalizationResultKind.Failed,
            None,
            failure,
            signal.signalId,
            message,
        )


def _utcDate() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d")
