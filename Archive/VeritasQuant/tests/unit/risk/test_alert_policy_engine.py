from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertSeverity,
    AlertStatus,
    RiskScopeV1,
)
from veritasquant.risk.AlertPolicyEngine import (
    AlertPolicyEngineV1,
    PolicyContextV1,
    PolicyEngineError,
)


def _alert(severity: AlertSeverity = AlertSeverity.P1, status: AlertStatus = AlertStatus.Active) -> AlertEventV1:
    return AlertEventV1.model_validate(
        {
            "AlertId": "alert-1",
            "AlertVersion": 1,
            "AlertType": "market.extreme_volatility",
            "Severity": severity,
            "Status": status,
            "Scope": RiskScopeV1(AccountIds=("account-1",)),
            "DedupeKey": "market.extreme_volatility|account-1|2026-08-02T10:00",
            "Trigger": {"zscore": "3.20"},
            "Evidence": ({"bar": "bar-1"},),
            "RawEventIds": ("signal-1",),
        }
    )


def _context(**overrides: object) -> PolicyContextV1:
    values: dict[str, object] = {
        "accountId": "account-1",
        "cashAvailable": Decimal("10000"),
        "exposure": Decimal("1000"),
        "activeControls": (),
        "openOrderQuantity": Decimal("100"),
        "equity": Decimal("20000"),
    }
    values.update(overrides)
    return PolicyContextV1(**values)  # type: ignore[call-arg]


def test_p0_active_alert_produces_pause_candidate() -> None:
    evaluation = AlertPolicyEngineV1().evaluate(_alert(severity=AlertSeverity.P0), _context())
    actions = {item.action for item in evaluation.actions}
    assert "PAUSE_SCOPE" in actions
    assert "rule.p0_active_pause" in evaluation.matchedRules


def test_insufficient_cash_rejects_new_orders() -> None:
    evaluation = AlertPolicyEngineV1().evaluate(
        _alert(), _context(cashAvailable=Decimal("50"), openOrderQuantity=Decimal("1000"))
    )
    actions = {item.action for item in evaluation.actions}
    assert "REJECT_NEW_ORDERS" in actions
    assert "INSUFFICIENT_CASH" in evaluation.actions[0].reasonCodes


def test_exposure_over_two_times_equity_limits() -> None:
    evaluation = AlertPolicyEngineV1().evaluate(
        _alert(), _context(exposure=Decimal("50000"), equity=Decimal("20000"))
    )
    actions = {item.action for item in evaluation.actions}
    assert "REDUCE_ONLY" in actions


def test_suppressed_alert_creates_no_new_action() -> None:
    evaluation = AlertPolicyEngineV1().evaluate(_alert(status=AlertStatus.Suppressed), _context())
    assert evaluation.actions == ()
    assert "rule.suppressed_no_action" in evaluation.matchedRules


def test_same_input_same_output_hash() -> None:
    engine = AlertPolicyEngineV1()
    first = engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    second = engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    assert first.outputHash == second.outputHash


def test_different_input_different_output_hash() -> None:
    engine = AlertPolicyEngineV1()
    p0 = engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    p1 = engine.evaluate(_alert(severity=AlertSeverity.P1), _context())
    assert p0.outputHash != p1.outputHash


def test_no_side_effects_across_evaluations() -> None:
    engine = AlertPolicyEngineV1()
    before = engine.evaluate(_alert(), _context())
    engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    after = engine.evaluate(_alert(), _context())
    # 纯函数：相同输入在其它求值前后输出一致
    assert before.outputHash == after.outputHash


def test_policy_version_is_stable_and_rejects_empty() -> None:
    assert AlertPolicyEngineV1().policyVersion == "P1-RISK-POLICY-V1"
    with pytest.raises(PolicyEngineError, match="版本"):
        AlertPolicyEngineV1(policyVersion="")
