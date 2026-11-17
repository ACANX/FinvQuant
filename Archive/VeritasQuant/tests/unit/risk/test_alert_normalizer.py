from __future__ import annotations

from datetime import datetime, timezone

from veritasquant.risk.AlertModels import AlertSeverity, AlertStatus
from veritasquant.risk.AlertNormalizer import (
    AlertNormalizerV1,
    NormalizationResultKind,
)
from veritasquant.risk.AlertModels import RiskSignalV1

UTC = timezone.utc
SEVERITY_MAP = {
    "volatility.threshold_breached": AlertSeverity.P1,
    "order_rejected": AlertSeverity.P0,
}


def _signal(signalType: str = "volatility.threshold_breached", **overrides: object) -> RiskSignalV1:
    values: dict[str, object] = {
        "SignalId": "signal-1",
        "SignalType": signalType,
        "ObservedAt": datetime(2026, 8, 2, 10, 0, tzinfo=UTC),
        "DetectedAt": datetime(2026, 8, 2, 10, 0, tzinfo=UTC),
        "Source": "volatility-detector",
        "ScopeCandidate": {"account_ids": ["account-1"], "symbols": ["518880"]},
        "Payload": {"zscore": "3.20"},
        "Evidence": ({"bar": "bar-1"},),
        "RuleId": "rule-vol-1",
        "RuleVersion": "1.0.0",
    }
    values.update(overrides)
    return RiskSignalV1.model_validate(values)


def test_normalizes_known_signal_type_to_alert() -> None:
    outcome = AlertNormalizerV1().normalize(_signal(), SEVERITY_MAP)
    assert outcome.kind is NormalizationResultKind.Normalized
    assert outcome.alert is not None
    assert outcome.alert.alertVersion == 1
    assert outcome.alert.severity is AlertSeverity.P1
    assert outcome.alert.status is AlertStatus.Active
    assert outcome.alert.rawEventIds == ("signal-1",)
    assert outcome.alert.scope.accountIds == ("account-1",)


def test_failure_enters_isolation_with_audit_event() -> None:
    normalizer = AlertNormalizerV1()
    outcome = normalizer.normalize(_signal(signalType="unknown.type"), SEVERITY_MAP)
    assert outcome.kind is NormalizationResultKind.Failed
    assert outcome.failure is not None
    assert outcome.failure.riskSignalId == "signal-1"
    assert "NORMALIZATION_FAILED" in outcome.failure.errorCodes
    assert outcome.failure.rawPayloadHash == _signal(signalType="unknown.type").payloadHash()
    records = normalizer.isolationRecords()
    assert len(records) == 1
    assert records[0].retryable
    # 原始载荷不复制到失败事件中（仅哈希与隔离引用）
    assert "payload" not in outcome.failure.model_dump()


def test_missing_account_in_scope_fails() -> None:
    normalizer = AlertNormalizerV1()
    outcome = normalizer.normalize(_signal(ScopeCandidate={"symbols": ["518880"]}), SEVERITY_MAP)
    assert outcome.kind is NormalizationResultKind.Failed
    assert outcome.alert is None


def test_dedupe_key_is_stable_for_same_scope() -> None:
    normalizer = AlertNormalizerV1()
    first = normalizer.normalize(_signal(), SEVERITY_MAP)
    second = normalizer.normalize(_signal(), SEVERITY_MAP)
    assert first.alert is not None and second.alert is not None
    assert first.alert.dedupeKey == second.alert.dedupeKey
    assert first.alert.dedupeKey.startswith("volatility.threshold_breached|")


def test_dedupe_key_differs_for_different_scope() -> None:
    normalizer = AlertNormalizerV1()
    first = normalizer.normalize(_signal(), SEVERITY_MAP)
    second = normalizer.normalize(_signal(ScopeCandidate={"account_ids": ["account-2"], "symbols": ["518880"]}), SEVERITY_MAP)
    assert first.alert is not None and second.alert is not None
    assert first.alert.dedupeKey != second.alert.dedupeKey


def test_severity_map_drives_severity() -> None:
    outcome = AlertNormalizerV1().normalize(_signal(signalType="order_rejected"), SEVERITY_MAP)
    assert outcome.alert is not None
    assert outcome.alert.severity is AlertSeverity.P0


def test_rejects_missing_severity_mapping() -> None:
    outcome = AlertNormalizerV1().normalize(_signal(signalType="no.mapping.type"), SEVERITY_MAP)
    assert outcome.kind is NormalizationResultKind.Failed
    assert outcome.failure is not None
