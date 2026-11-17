from __future__ import annotations

from datetime import timezone

import pytest

from veritasquant.risk.AlertCorrelator import AlertCorrelatorV1, CorrelationError
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertSeverity,
    AlertStatus,
    RiskScopeV1,
)

UTC = timezone.utc


def _alert(**overrides: object) -> AlertEventV1:
    values: dict[str, object] = {
        "AlertId": "alert-1",
        "AlertVersion": 1,
        "AlertType": "market.extreme_volatility",
        "Severity": AlertSeverity.P1,
        "Status": AlertStatus.Active,
        "Scope": RiskScopeV1(AccountIds=("account-1",)),
        "DedupeKey": "market.extreme_volatility|account-1|2026-08-02T10:00",
        "Trigger": {"zscore": "3.20"},
        "Evidence": ({"bar": "bar-1"},),
        "RawEventIds": ("signal-1",),
    }
    values.update(overrides)
    return AlertEventV1.model_validate(values)


def _updated(status: AlertStatus = AlertStatus.Acknowledged, version: int = 2, severity: AlertSeverity = AlertSeverity.P1) -> AlertEventV1:
    return _alert(
        AlertVersion=version,
        PreviousEventId="event-1",
        Status=status,
        Severity=severity,
    )


def test_create_then_update_lifecycle() -> None:
    correlator = AlertCorrelatorV1()
    created = correlator.process(_alert())
    assert created.disposition == "APPLIED"
    updated = correlator.process(_updated())
    assert updated.disposition == "APPLIED"
    state = correlator.lifecycle("alert-1")
    assert state.alertVersion == 2
    assert state.status is AlertStatus.Acknowledged


def test_duplicate_same_version_same_hash() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    duplicate = correlator.process(_alert())
    assert duplicate.disposition == "DUPLICATE"
    assert correlator.lifecycle("alert-1").alertVersion == 1


def test_same_version_different_hash_is_conflict() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    conflict = correlator.process(_alert(Severity=AlertSeverity.P0))
    assert conflict.disposition == "CONFLICT"
    assert correlator.lifecycle("alert-1").severity is AlertSeverity.P1  # 投影不变


def test_stale_version_does_not_regress_projection() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    correlator.process(_updated())
    # 版本 1 的事件键已被占用：同版本不同内容属于协议冲突，投影必须保持不变
    stale = correlator.process(_alert(Severity=AlertSeverity.P3))
    assert stale.disposition == "CONFLICT"
    assert correlator.lifecycle("alert-1").alertVersion == 2
    assert correlator.lifecycle("alert-1").severity is AlertSeverity.P1


def test_version_gap_pauses_and_recovers() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    # 直接跳版本 3：缺口
    skipped = correlator.process(_updated(version=3))
    assert skipped.disposition == "GAP_PAUSED"
    assert correlator.isGapPaused("alert-1")
    assert correlator.pendingGapCount("alert-1") == 1
    # 权威快照核验补齐
    correlator.applyVerifiedSnapshot("alert-1", 3)
    assert not correlator.isGapPaused("alert-1")
    assert correlator.lifecycle("alert-1").alertVersion == 3


def test_terminal_state_rejects_revival() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    correlator.process(_updated(status=AlertStatus.Resolved, version=2))
    state = correlator.lifecycle("alert-1")
    assert state.isTerminal
    revival = correlator.process(_updated(status=AlertStatus.Active, version=3))
    assert revival.disposition == "TERMINAL_REVIVAL_REJECTED"


def test_suppression_records_key() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    correlator.process(_updated(status=AlertStatus.Suppressed, version=2))
    state = correlator.lifecycle("alert-1")
    assert state.suppressionKey is not None
    assert len(state.suppressionKey) == 64


def test_unknown_lifecycle_raises() -> None:
    correlator = AlertCorrelatorV1()
    with pytest.raises(CorrelationError, match="未知预警"):
        correlator.lifecycle("ghost")


def test_verified_snapshot_below_applied_version_rejected() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    with pytest.raises(CorrelationError, match="不得低于"):
        correlator.applyVerifiedSnapshot("alert-1", 0)


def test_audit_trail_records_all_dispositions() -> None:
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    correlator.process(_alert())
    correlator.process(_updated(version=3))
    dispositions = [item.disposition for item in correlator.audit]
    assert dispositions == ["APPLIED", "DUPLICATE", "GAP_PAUSED"]
