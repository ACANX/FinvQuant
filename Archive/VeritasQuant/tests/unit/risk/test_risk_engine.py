from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.execution.Orders import (
    OrderIntentV1,
    OrderSide,
    OrderType,
    PositionEffect,
    TimeInForce,
)
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertSeverity,
    AlertStatus,
    RiskScopeV1,
)
from veritasquant.risk.AlertPolicyEngine import AlertPolicyEngineV1
from veritasquant.risk.RiskEngine import (
    ApprovalContextV1,
    ControlAction,
    RiskDecision,
    RiskEngineError,
    RiskEngineV1,
    TradingControlEventV1,
)

UTC = timezone.utc


def _intent(quantity: Decimal = Decimal("100")) -> OrderIntentV1:
    return OrderIntentV1.model_validate(
        {
            "IntentId": "intent-1",
            "RunId": "run-1",
            "AccountId": "account-1",
            "SubaccountId": "strategy-1",
            "StrategyId": "strategy-1",
            "StrategyVersion": "1.0.0",
            "Symbol": "518880",
            "InstrumentMetadataVersion": "meta-v1",
            "Side": OrderSide.Buy,
            "PositionEffect": PositionEffect.Open,
            "OrderType": OrderType.Market,
            "Quantity": quantity,
            "TimeInForce": TimeInForce.Day,
            "Ts": datetime(2026, 8, 2, tzinfo=UTC),
            "CreatedFromEventId": "event-100",
            "ExpectedAccountVersion": 5,
        }
    )


def _context(**overrides: object) -> ApprovalContextV1:
    values: dict[str, object] = {
        "accountId": "account-1",
        "accountSnapshotVersion": 3,
        "orderSnapshotVersion": 2,
        "positionSnapshotVersion": 1,
        "cashAvailable": Decimal("10000"),
        "exposure": Decimal("1000"),
        "equity": Decimal("20000"),
        "openOrderQuantity": Decimal("100"),
    }
    values.update(overrides)
    return ApprovalContextV1(**values)  # type: ignore[call-arg]


def _alert(severity: AlertSeverity = AlertSeverity.P1) -> AlertEventV1:
    return AlertEventV1.model_validate(
        {
            "AlertId": "alert-1",
            "AlertVersion": 1,
            "AlertType": "market.extreme_volatility",
            "Severity": severity,
            "Status": AlertStatus.Active,
            "Scope": RiskScopeV1(AccountIds=("account-1",)),
            "DedupeKey": "market.extreme_volatility|account-1|2026-08-02T10:00",
            "Trigger": {"zscore": "3.20"},
            "RawEventIds": ("signal-1",),
        }
    )


def _engine() -> RiskEngineV1:
    return RiskEngineV1(policyEngine=AlertPolicyEngineV1())


def _control(strength: int = 40, controlId: str = "control-1", controlVersion: int = 1) -> TradingControlEventV1:
    return TradingControlEventV1(
        controlId=controlId,
        controlVersion=controlVersion,
        controlRequestId=f"request-{controlId}",
        idempotencyKey=f"idem-{controlId}",
        scope="account",
        action=ControlAction.PauseScope if strength == 40 else ControlAction.ReduceOnly,
        strength=strength,
        parameters={},
        effectiveFrom="2026-08-02T00:00:00Z",
        expiresAt=None,
        sourceDecisionId="decision-0",
        riskPolicyVersion="P1-RISK-POLICY-V1",
        status="ACTIVE",
        controlHash="0" * 64,
    )


def test_approves_healthy_intent() -> None:
    engine = _engine()
    decision = engine.approveIntent(_intent(), _context())
    assert decision.decision is RiskDecision.Approved
    assert decision.approvedQuantity == Decimal("100")
    assert decision.riskPolicyVersion == "P1-RISK-POLICY-V1"
    assert decision.decisionHash  # 可审计


def test_global_stop_trading_blocks_everything() -> None:
    engine = _engine()
    engine.publishControl(_control(strength=50, controlId="stop-1"))
    decision = engine.approveIntent(_intent(), _context())
    assert decision.decision is RiskDecision.Rejected
    assert decision.approvedQuantity == Decimal("0")
    assert "GLOBAL_CONTROL_BLOCK" in decision.reasonCodes


def test_p0_alert_rejects_via_policy_candidate() -> None:
    engine = _engine()
    decision = engine.approveIntent(_intent(), _context(), alert=_alert(severity=AlertSeverity.P0))
    assert decision.decision is RiskDecision.Rejected
    assert "POLICY_CANDIDATE_BLOCK" in decision.reasonCodes


def test_p1_alert_with_exposure_reduces() -> None:
    engine = _engine()
    decision = engine.approveIntent(
        _intent(quantity=Decimal("100")),
        _context(exposure=Decimal("39950"), equity=Decimal("20000")),
    )
    # 39950 + 100 = 40050 > 40000（2x equity）：REDUCED 到 50
    assert decision.decision is RiskDecision.Reduced
    assert decision.approvedQuantity == Decimal("50")


def test_insufficient_cash_rejects() -> None:
    engine = _engine()
    decision = engine.approveIntent(_intent(quantity=Decimal("5000")), _context(cashAvailable=Decimal("100")))
    assert decision.decision is RiskDecision.Rejected
    assert "INSUFFICIENT_CASH" in decision.reasonCodes


def test_control_version_must_increase() -> None:
    engine = _engine()
    engine.publishControl(_control())
    with pytest.raises(RiskEngineError, match="单调递增"):
        engine.publishControl(_control(controlVersion=1, controlId="control-1"))


def test_control_strength_cannot_weaken_in_place() -> None:
    engine = _engine()
    engine.publishControl(_control(strength=50, controlId="control-1"))
    with pytest.raises(RiskEngineError, match="放宽"):
        engine.publishControl(_control(strength=30, controlVersion=2, controlId="control-1"))


def test_release_control_uses_new_version() -> None:
    engine = _engine()
    engine.publishControl(_control())
    released = engine.releaseControl("control-1", 2)
    assert released.status == "RELEASED"
    assert engine.activeControls == {}
    with pytest.raises(RiskEngineError, match="高于"):
        engine.releaseControl("control-1", 1)


def test_account_mismatch_rejected() -> None:
    engine = _engine()
    with pytest.raises(RiskEngineError, match="不一致"):
        engine.approveIntent(_intent(), _context(accountId="account-2"))


def test_decisions_are_auditable_and_unique() -> None:
    engine = _engine()
    first = engine.approveIntent(_intent(), _context())
    second = engine.approveIntent(_intent(), _context())
    assert first.decisionId != second.decisionId
    assert len(engine.decisions()) == 2
    assert first.decisionHash != second.decisionHash
