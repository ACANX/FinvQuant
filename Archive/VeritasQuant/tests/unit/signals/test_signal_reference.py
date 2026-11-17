"""P3-001 SignalReference、人工审核与忽略原因契约测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest
from pydantic import ValidationError

from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    ManualExecutionV1,
    ManualReviewActionV1,
    SignalActionType,
    SignalContractError,
    SignalReferenceV1,
    SignalStatus,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _signal(**overrides: object) -> SignalReferenceV1:
    values: dict[str, object] = {
        "signalReferenceId": "sig-ref-001",
        "version": 1,
        "status": SignalStatus.Pending,
        "accountId": "acc-001",
        "strategyId": "strat-dual-ma",
        "strategyChecksum": "a" * 64,
        "sourceEventId": "evt-bar-001",
        "sourceEventType": "MarketBarEvent",
        "direction": "BUY",
        "quantity": "100.0000",
        "priceLimit": "5.0000",
        "operatorId": None,
        "generatedTs": _T0,
        "expiresAt": _T0 + timedelta(minutes=15),
        "previousSignalReferenceId": None,
    }
    values.update(overrides)
    return SignalReferenceV1.create(**values)


class TestSignalReferenceContract:
    def test_create_valid(self) -> None:
        signal = _signal()
        assert signal.signalReferenceId == "sig-ref-001"
        assert signal.version == 1
        assert signal.status is SignalStatus.Pending
        assert signal.accountId == "acc-001"
        assert signal.strategyId == "strat-dual-ma"
        assert signal.sourceEventId == "evt-bar-001"
        assert signal.operatorId is None

    def test_required_fields_complete(self) -> None:
        """状态、版本、账户、策略、来源事件字段必须完整。"""
        with pytest.raises(ValidationError, match="AccountId"):
            _signal(accountId="")
        with pytest.raises(ValidationError, match="StrategyChecksum"):
            _signal(strategyChecksum="short")
        with pytest.raises(ValidationError, match="Direction"):
            _signal(direction="HOLDX")

    def test_quantity_rejects_float(self) -> None:
        """金额/数量路径禁止 float：quantity 必须是字符串。"""
        with pytest.raises((ValidationError, TypeError)):
            _signal(quantity=100.0)  # type: ignore[arg-type]

    def test_version_chain(self) -> None:
        """版本 1 不得引用 previous；更高版本必须引用 previous。"""
        with pytest.raises(ValidationError, match="previousSignalReferenceId"):
            _signal(version=1, previousSignalReferenceId="sig-ref-000")
        with pytest.raises(ValidationError, match="previousSignalReferenceId"):
            _signal(version=2, previousSignalReferenceId=None)

    def test_expiry_after_generated(self) -> None:
        with pytest.raises(ValidationError, match="过期时间"):
            _signal(expiresAt=_T0 - timedelta(seconds=1))

    def test_transition_derives_new_version(self) -> None:
        signal = _signal()
        confirmed = signal.transition(
            status=SignalStatus.Confirmed, operatorId="op-alice", newId="sig-ref-002"
        )
        assert confirmed.version == 2
        assert confirmed.previousSignalReferenceId == signal.signalReferenceId
        assert confirmed.operatorId == "op-alice"
        assert confirmed.status is SignalStatus.Confirmed
        # 原记录不可变
        assert signal.status is SignalStatus.Pending
        assert signal.version == 1

    def test_transition_same_status_rejected(self) -> None:
        signal = _signal()
        with pytest.raises(SignalContractError):
            signal.transition(status=SignalStatus.Pending, operatorId="op", newId="sig-ref-002")

    def test_unknown_status_rejected(self) -> None:
        with pytest.raises(ValidationError, match="未知 SignalStatus"):
            _signal(status="BOGUS")  # type: ignore[arg-type]


class TestIgnoreReason:
    def test_valid(self) -> None:
        reason = IgnoreReasonV1.create(reasonCode="MANUAL_OVERRIDE", detail="人工判断", source="manual")
        assert reason.reasonCode == "MANUAL_OVERRIDE"

    def test_empty_code_rejected(self) -> None:
        with pytest.raises(ValidationError, match="忽略原因"):
            IgnoreReasonV1.create(reasonCode="  ", detail="x")


class TestManualReviewAction:
    def _action(self, **overrides: object) -> ManualReviewActionV1:
        values: dict[str, object] = {
            "actionId": "act-001",
            "signalReferenceId": "sig-ref-001",
            "actionType": SignalActionType.Confirm,
            "operatorId": "op-alice",
            "reason": "按信号执行",
            "ignoreReason": None,
            "actedAt": _T0 + timedelta(minutes=1),
            "version": 1,
            "auditTrail": ("created",),
        }
        values.update(overrides)
        return ManualReviewActionV1.create(**values)

    def test_confirm_valid(self) -> None:
        action = self._action()
        assert action.actionType is SignalActionType.Confirm
        assert action.operatorId == "op-alice"
        assert action.actedAt == _T0 + timedelta(minutes=1)

    def test_ignore_requires_reason(self) -> None:
        with pytest.raises(ValidationError, match="忽略动作"):
            self._action(actionType=SignalActionType.Ignore, ignoreReason=None)

    def test_ignore_with_reason_valid(self) -> None:
        reason = IgnoreReasonV1.create(reasonCode="MARKET_CLOSED", detail="休市")
        action = self._action(
            actionType=SignalActionType.Ignore, reason="休市忽略", ignoreReason=reason
        )
        assert action.ignoreReason is not None
        assert action.ignoreReason.reasonCode == "MARKET_CLOSED"

    def test_non_ignore_with_reason_rejected(self) -> None:
        reason = IgnoreReasonV1.create(reasonCode="X", detail="y")
        with pytest.raises(ValidationError, match="非忽略动作"):
            self._action(actionType=SignalActionType.Confirm, ignoreReason=reason)

    def test_missing_reason_for_register_rejected(self) -> None:
        with pytest.raises(ValidationError, match="理由"):
            self._action(actionType=SignalActionType.RegisterExecution, reason="")

    def test_unknown_action_type_rejected(self) -> None:
        with pytest.raises(ValidationError, match="未知 SignalActionType"):
            self._action(actionType="HACK")  # type: ignore[arg-type]


class TestManualExecution:
    def _execution(self, **overrides: object) -> ManualExecutionV1:
        values: dict[str, object] = {
            "executionId": "exec-001",
            "signalReferenceId": "sig-ref-001",
            "actionId": "act-001",
            "operatorId": "op-alice",
            "executedAt": _T0 + timedelta(minutes=2),
            "direction": "BUY",
            "quantity": "100.0000",
            "price": "5.0100",
            "deviationReason": None,
            "note": "",
        }
        values.update(overrides)
        return ManualExecutionV1.create(**values)

    def test_valid(self) -> None:
        execution = self._execution()
        assert execution.direction == "BUY"
        assert execution.quantity == "100.0000"
        assert execution.price == "5.0100"

    def test_with_deviation_reason(self) -> None:
        reason = IgnoreReasonV1.create(reasonCode="SLIPPAGE", detail="滑点超限")
        execution = self._execution(deviationReason=reason)
        assert execution.deviationReason is not None

    def test_rejects_float_price(self) -> None:
        with pytest.raises((ValidationError, TypeError)):
            self._execution(price=5.01)  # type: ignore[arg-type]

    def test_direction_restricted(self) -> None:
        with pytest.raises(ValidationError, match="Direction"):
            self._execution(direction="HOLD")
