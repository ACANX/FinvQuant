from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.risk.ControlBook import (
    ControlBookV1,
    ControlMergeError,
    controlStrength,
)
from veritasquant.risk.RiskEngine import ControlAction, TradingControlEventV1


def _control(
    controlId: str = "control-1",
    scope: str = "account",
    action: ControlAction = ControlAction.PauseScope,
    version: int = 1,
    parameters: dict[str, object] | None = None,
    status: str = "ACTIVE",
) -> TradingControlEventV1:
    return TradingControlEventV1(
        controlId=controlId,
        controlVersion=version,
        controlRequestId=f"request-{controlId}",
        idempotencyKey=f"idem-{controlId}",
        scope=scope,
        action=action,
        strength=controlStrength(action),
        parameters=parameters or {},
        effectiveFrom="2026-08-02T00:00:00Z",
        expiresAt=None,
        sourceDecisionId="decision-0",
        riskPolicyVersion="P1-RISK-POLICY-V1",
        status=status,
        controlHash="0" * 64,
    )


def test_strength_partial_order() -> None:
    assert controlStrength(ControlAction.RejectNewOrders) < controlStrength(ControlAction.ReduceOnly)
    assert controlStrength(ControlAction.ReduceOnly) < controlStrength(ControlAction.PauseScope)
    assert controlStrength(ControlAction.PauseScope) < controlStrength(ControlAction.StopTrading)


def test_strongest_control_wins() -> None:
    book = ControlBookV1()
    book.publish(_control("reduce", action=ControlAction.ReduceOnly, parameters={"account_ids": ["account-1"]}))
    book.publish(_control("stop", action=ControlAction.StopTrading, parameters={"account_ids": ["account-1"]}))
    effective = book.effectiveFor("account-1")
    assert effective is not None
    assert effective.action is ControlAction.StopTrading
    assert effective.strength == 50


def test_scope_expansion_to_concrete_targets() -> None:
    book = ControlBookV1()
    book.publish(_control("c1", parameters={"account_ids": ["account-1", "account-2"]}))
    expansions = book.expansions()
    assert len(expansions) == 1
    targets = set(expansions[0].targets)
    assert targets == {"account:account-1", "account:account-2"}


def test_control_does_not_apply_to_unscoped_account() -> None:
    book = ControlBookV1()
    book.publish(_control("c1", parameters={"account_ids": ["account-1"]}))
    assert book.effectiveFor("account-9") is None


def test_release_only_removes_own_contribution() -> None:
    book = ControlBookV1()
    book.publish(_control("reduce", action=ControlAction.ReduceOnly, parameters={"account_ids": ["account-1"]}))
    book.publish(_control("stop", action=ControlAction.StopTrading, parameters={"account_ids": ["account-1"]}))
    book.release("stop", 2)
    effective = book.effectiveFor("account-1")
    # 解除 STOP 后 REDUCE_ONLY 仍然生效：不能放松到无控制
    assert effective is not None
    assert effective.action is ControlAction.ReduceOnly
    assert effective.strength == 30


def test_stale_version_does_not_overwrite_newer() -> None:
    book = ControlBookV1()
    book.publish(_control("c1", version=2))
    with pytest.raises(ControlMergeError, match="乱序"):
        book.publish(_control("c1", version=1))


def test_same_version_same_hash_idempotent() -> None:
    book = ControlBookV1()
    control = _control("c1", version=1)
    book.publish(control)
    book.publish(control)  # 幂等
    assert len(book.controls) == 1


def test_same_version_different_hash_conflict() -> None:
    book = ControlBookV1()
    book.publish(_control("c1", version=1))
    conflict = TradingControlEventV1(
        controlId="c1",
        controlVersion=1,
        controlRequestId="request-c1",
        idempotencyKey="idem-c1",
        scope="account",
        action=ControlAction.PauseScope,
        strength=40,
        parameters={},
        effectiveFrom="2026-08-02T00:00:00Z",
        expiresAt=None,
        sourceDecisionId="decision-0",
        riskPolicyVersion="P1-RISK-POLICY-V1",
        status="ACTIVE",
        controlHash="1" * 64,
    )
    with pytest.raises(ControlMergeError, match="协议冲突"):
        book.publish(conflict)


def test_boolean_parameters_or_merge() -> None:
    book = ControlBookV1()
    book.publish(_control("a", parameters={"account_ids": ["account-1"], "cancel_active_orders": True}))
    book.publish(_control("b", parameters={"account_ids": ["account-1"], "cancel_active_orders": False}))
    effective = book.effectiveFor("account-1")
    assert effective is not None
    assert effective.parameters["cancel_active_orders"] is True  # 取或


def test_min_parameters_take_minimum() -> None:
    book = ControlBookV1()
    book.publish(_control("a", parameters={"account_ids": ["account-1"], "quantity_cap": Decimal("500")}))
    book.publish(_control("b", parameters={"account_ids": ["account-1"], "quantity_cap": Decimal("200")}))
    effective = book.effectiveFor("account-1")
    assert effective is not None
    assert effective.parameters["quantity_cap"] == Decimal("200")  # 取最小


def test_global_scope_expands_wildcards() -> None:
    book = ControlBookV1()
    book.publish(_control("g", scope="global"))
    effective = book.effectiveFor("account-1", strategyId="strategy-1", symbol="518880")
    assert effective is not None
    assert effective.action is ControlAction.PauseScope


def test_effective_hash_is_deterministic() -> None:
    book = ControlBookV1()
    book.publish(_control("a", parameters={"account_ids": ["account-1"]}))
    book.publish(_control("b", parameters={"account_ids": ["account-1"]}))
    first = book.effectiveFor("account-1")
    second = book.effectiveFor("account-1")
    assert first is not None and second is not None
    assert first.controlHash == second.controlHash
    assert len(first.controlHash) == 64
