"""风控发布权限（R-004）与预警生命周期（R-007）契约测试。

R-004：非 RiskEngine 生产者发布决定/控制被拒绝并告警；纯求值器无副作用；
通知/确认不解除 P0/P1 控制。
R-007：预警版本重复/缺口/乱序、抑制/恢复/终态按契约处理。
"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.risk.AlertCorrelator import AlertCorrelatorV1
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
    RiskEngineV1,
    TradingControlEventV1,
)


@pytest.mark.stable_id("R-004-001")
def test_only_risk_engine_can_publish_decisions() -> None:
    """决定必须由 RiskEngine 生成；AlertPolicyEngine 只返回候选建议。"""
    policyEngine = AlertPolicyEngineV1()
    evaluation = policyEngine.evaluate(
        _alert(severity=AlertSeverity.P0),
        _context(),
    )
    # 纯求值器只产生候选动作，不产生决定事件
    assert evaluation.actions
    assert evaluation.outputHash
    # RiskEngine 是唯一决定发布者
    engine = RiskEngineV1(policyEngine=policyEngine)
    decision = engine.approveIntent(_intent(), _context(), alert=_alert(severity=AlertSeverity.P0))
    assert decision.decision is RiskDecision.Rejected
    assert decision.decisionId.startswith("decision-")


@pytest.mark.stable_id("R-004-002")
def test_pure_evaluator_has_no_side_effects() -> None:
    """相同输入两次求值输出哈希一致，且不改变任何状态。"""
    engine = AlertPolicyEngineV1()
    first = engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    second = engine.evaluate(_alert(severity=AlertSeverity.P0), _context())
    assert first.outputHash == second.outputHash
    # 求值前后策略引擎无内部状态变化
    assert engine.policyVersion == "P1-RISK-POLICY-V1"


@pytest.mark.stable_id("R-004-003")
def test_notification_confirmation_does_not_release_p0_control() -> None:
    """通知确认不能解除 P0/P1 控制；只有显式解除（新版本）才移除。"""
    engine = RiskEngineV1(policyEngine=AlertPolicyEngineV1())
    control = TradingControlEventV1(
        controlId="control-1",
        controlVersion=1,
        controlRequestId="request-1",
        idempotencyKey="idem-1",
        scope="account",
        action=ControlAction.PauseScope,
        strength=40,
        parameters={"account_ids": ["account-1"]},
        effectiveFrom="2026-08-02T00:00:00Z",
        expiresAt=None,
        sourceDecisionId="decision-0",
        riskPolicyVersion="P1-RISK-POLICY-V1",
        status="ACTIVE",
        controlHash="0" * 64,
    )
    engine.publishControl(control)
    # 通知/确认参数不解除控制
    acknowledged = TradingControlEventV1(
        controlId="control-1",
        controlVersion=1,
        controlRequestId="request-1",
        idempotencyKey="idem-1",
        scope="account",
        action=ControlAction.PauseScope,
        strength=40,
        parameters={"account_ids": ["account-1"], "notify_escalation": True},
        effectiveFrom="2026-08-02T00:00:00Z",
        expiresAt=None,
        sourceDecisionId="decision-0",
        riskPolicyVersion="P1-RISK-POLICY-V1",
        status="ACKNOWLEDGED",
        controlHash="1" * 64,
    )
    # 同版本不同哈希属于协议冲突，不会静默替换
    with pytest.raises(Exception):
        engine.publishControl(acknowledged)
    # 控制仍然生效：新订单被阻断
    blocked = engine.approveIntent(_intent(), _context())
    assert blocked.decision is RiskDecision.Rejected


@pytest.mark.stable_id("R-007-001")
def test_alert_lifecycle_version_gap_pause_and_recovery() -> None:
    """版本缺口暂停推进，权威快照核验后恢复；低版本不回退。"""
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    skipped = correlator.process(_alert_update(version=3))
    assert skipped.disposition == "GAP_PAUSED"
    assert correlator.isGapPaused("alert-1")
    correlator.applyVerifiedSnapshot("alert-1", 3)
    assert not correlator.isGapPaused("alert-1")
    assert correlator.lifecycle("alert-1").alertVersion == 3
    # 低版本新内容：冲突/旧版本处理，投影不回退
    stale = correlator.process(_alert(severity=AlertSeverity.P3))
    assert stale.disposition in ("CONFLICT", "STALE_VERSION")
    assert correlator.lifecycle("alert-1").alertVersion == 3


@pytest.mark.stable_id("R-007-002")
def test_alert_suppression_recovery_and_terminal_state() -> None:
    """抑制、恢复、终态（RESOLVED/EXPIRED）按契约流转。"""
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    correlator.process(_alert_update(status=AlertStatus.Suppressed, version=2))
    assert correlator.lifecycle("alert-1").suppressionKey is not None
    correlator.process(_alert_update(status=AlertStatus.Active, version=3))
    assert correlator.lifecycle("alert-1").status is AlertStatus.Active
    correlator.process(_alert_update(status=AlertStatus.Resolved, version=4))
    assert correlator.lifecycle("alert-1").isTerminal
    # 终态拒绝复活
    revival = correlator.process(_alert_update(status=AlertStatus.Active, version=5))
    assert revival.disposition == "TERMINAL_REVIVAL_REJECTED"


@pytest.mark.stable_id("R-007-003")
def test_alert_duplicate_and_conflict_handling() -> None:
    """同版本同哈希重复投递；同版本不同哈希协议冲突。"""
    correlator = AlertCorrelatorV1()
    correlator.process(_alert())
    assert correlator.process(_alert()).disposition == "DUPLICATE"
    conflict = correlator.process(_alert(severity=AlertSeverity.P0))
    assert conflict.disposition == "CONFLICT"
    assert correlator.lifecycle("alert-1").severity is AlertSeverity.P1


def _intent() -> object:
    from veritasquant.execution.Orders import (
        OrderIntentV1,
        OrderSide,
        OrderType,
        PositionEffect,
        TimeInForce,
    )
    from datetime import datetime, timezone

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
            "Quantity": Decimal("100"),
            "TimeInForce": TimeInForce.Day,
            "Ts": datetime(2026, 8, 2, tzinfo=timezone.utc),
            "CreatedFromEventId": "event-100",
            "ExpectedAccountVersion": 5,
        }
    )


def _context() -> ApprovalContextV1:
    return ApprovalContextV1(
        accountId="account-1",
        accountSnapshotVersion=1,
        orderSnapshotVersion=1,
        positionSnapshotVersion=1,
        cashAvailable=Decimal("10000"),
        exposure=Decimal("1000"),
        equity=Decimal("20000"),
        openOrderQuantity=Decimal("100"),
    )


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


def _alert_update(version: int = 2, status: AlertStatus = AlertStatus.Acknowledged) -> AlertEventV1:
    return AlertEventV1.model_validate(
        {
            "AlertId": "alert-1",
            "AlertVersion": version,
            "PreviousEventId": f"event-{version - 1}",
            "AlertType": "market.extreme_volatility",
            "Severity": AlertSeverity.P1,
            "Status": status,
            "Scope": RiskScopeV1(AccountIds=("account-1",)),
            "DedupeKey": "market.extreme_volatility|account-1|2026-08-02T10:00",
            "Trigger": {"zscore": "3.20"},
            "RawEventIds": ("signal-1",),
        }
    )
