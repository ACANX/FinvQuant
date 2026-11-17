from __future__ import annotations

from datetime import datetime, timezone

import pytest
from pydantic import ValidationError

from veritasquant.core.Models import EventPayloadV1
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertNormalizationFailureEventV1,
    AlertSeverity,
    AlertStatus,
    RiskScopeV1,
    RiskSignalV1,
)

UTC = timezone.utc


def _utc() -> datetime:
    return datetime(2026, 8, 2, 10, 0, tzinfo=UTC)


def _signal(**overrides: object) -> RiskSignalV1:
    values: dict[str, object] = {
        "SignalId": "signal-1",
        "SignalType": "volatility.threshold_breached",
        "ObservedAt": _utc(),
        "DetectedAt": _utc(),
        "Source": "volatility-detector",
        "ScopeCandidate": {"symbols": ["518880"]},
        "Payload": {"zscore": "3.20"},
        "Evidence": ({"bar": "bar-1"},),
        "Confidence": 0.9,
        "RuleId": "rule-vol-1",
        "RuleVersion": "1.0.0",
    }
    values.update(overrides)
    return RiskSignalV1.model_validate(values)


def test_risk_signal_is_traceable_and_payload_hashed() -> None:
    signal = _signal()
    assert isinstance(signal, EventPayloadV1)
    assert signal.ruleVersion == "1.0.0"
    assert len(signal.payloadHash()) == 64
    changed = _signal(Payload={"zscore": "3.30"})
    assert signal.payloadHash() != changed.payloadHash()


def test_risk_signal_rejects_missing_required_fields() -> None:
    with pytest.raises(ValidationError, match="SignalId"):
        RiskSignalV1.model_validate({})
    with pytest.raises(ValidationError, match="时间"):
        _signal(ObservedAt="2026-08-02")


def test_risk_signal_rejects_non_utc_or_float_confidence() -> None:
    with pytest.raises(ValidationError):
        _signal(DetectedAt=datetime(2026, 8, 2, 10, 0))
    with pytest.raises(ValidationError, match="Confidence"):
        _signal(Confidence=1.5)


def test_alert_scope_requires_account() -> None:
    scope = RiskScopeV1(AccountIds=("account-1",), StrategyIds=("strategy-1",))
    assert scope.accountIds == ("account-1",)
    with pytest.raises(ValidationError, match="AccountIds"):
        RiskScopeV1(AccountIds=())


def test_alert_create_version_one_no_previous() -> None:
    alert = AlertEventV1.model_validate(
        {
            "AlertId": "alert-1",
            "AlertVersion": 1,
            "AlertType": "market.extreme_volatility",
            "Severity": AlertSeverity.P1,
            "Status": AlertStatus.Active,
            "Scope": RiskScopeV1(AccountIds=("account-1",)),
            "DedupeKey": "market.extreme_volatility|518880|2026-08-02T10:00",
            "Trigger": {"zscore": "3.20", "direction": "up"},
            "Evidence": ({"bar": "bar-1"},),
            "RawEventIds": ("signal-1",),
        }
    )
    assert alert.alertVersion == 1
    assert alert.previousEventId is None


def test_alert_update_requires_previous_event() -> None:
    with pytest.raises(ValidationError, match="previousEventId"):
        AlertEventV1.model_validate(
            {
                "AlertId": "alert-1",
                "AlertVersion": 2,
                "AlertType": "market.extreme_volatility",
                "Severity": AlertSeverity.P0,
                "Status": AlertStatus.Acknowledged,
                "Scope": RiskScopeV1(AccountIds=("account-1",)),
                "DedupeKey": "market.extreme_volatility|518880|2026-08-02T10:00",
                "Trigger": {"zscore": "3.80"},
            }
        )
    with pytest.raises(ValidationError, match="previousEventId"):
        AlertEventV1.model_validate(
            {
                "AlertId": "alert-1",
                "AlertVersion": 1,
                "PreviousEventId": "event-0",
                "AlertType": "market.extreme_volatility",
                "Severity": AlertSeverity.P1,
                "Status": AlertStatus.Active,
                "Scope": RiskScopeV1(AccountIds=("account-1",)),
                "DedupeKey": "market.extreme_volatility|518880|2026-08-02T10:00",
                "Trigger": {"zscore": "3.20"},
            }
        )


def test_terminal_status_requires_lifecycle_update() -> None:
    with pytest.raises(ValidationError, match="终态"):
        AlertEventV1.model_validate(
            {
                "AlertId": "alert-1",
                "AlertVersion": 1,
                "AlertType": "market.extreme_volatility",
                "Severity": AlertSeverity.P1,
                "Status": AlertStatus.Resolved,
                "Scope": RiskScopeV1(AccountIds=("account-1",)),
                "DedupeKey": "market.extreme_volatility|518880|2026-08-02T10:00",
                "Trigger": {"zscore": "3.20"},
            }
        )


def test_alert_rejects_unknown_severity_and_status() -> None:
    with pytest.raises(ValidationError, match="未知"):
        AlertEventV1.model_validate(
            {
                "AlertId": "alert-1",
                "AlertVersion": 1,
                "AlertType": "market.extreme_volatility",
                "Severity": "P4",
                "Status": AlertStatus.Active,
                "Scope": RiskScopeV1(AccountIds=("account-1",)),
                "DedupeKey": "key",
                "Trigger": {},
            }
        )


def test_normalization_failure_is_audit_event_with_hash() -> None:
    failure = AlertNormalizationFailureEventV1.model_validate(
        {
            "NormalizationFailureId": "nf-1",
            "RiskSignalId": "signal-1",
            "AttemptedSchemaVersion": "1.0",
            "RuleId": "rule-vol-1",
            "RuleVersion": "1.0.0",
            "ErrorCodes": ("SCHEMA_VALIDATION_FAILED",),
            "RawPayloadHash": "0" * 64,
            "QuarantineReference": "quarantine/2026-08-02/signal-1",
            "Retryable": True,
        }
    )
    assert failure.retryable
    assert failure.rawPayloadHash == "0" * 64
    # 失败事件必须有错误码
    with pytest.raises(ValidationError, match="ErrorCodes"):
        AlertNormalizationFailureEventV1.model_validate(
            {
                "NormalizationFailureId": "nf-2",
                "RiskSignalId": "signal-1",
                "AttemptedSchemaVersion": "1.0",
                "ErrorCodes": (),
                "RawPayloadHash": "0" * 64,
                "QuarantineReference": "quarantine/1",
                "Retryable": False,
            }
        )


def test_payload_hash_distinguishes_evidence() -> None:
    first = _signal()
    second = _signal(Evidence=({"bar": "bar-2"},))
    assert first.payloadHash() != second.payloadHash()
