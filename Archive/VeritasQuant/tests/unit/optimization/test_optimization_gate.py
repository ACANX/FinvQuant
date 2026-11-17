"""P6-007c 优化结果 Gate 隔离测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.optimization.HyperparameterSearch import SearchResultV1
from veritasquant.optimization.OptimizationGate import (
    AcceptancePolicySnapshotV1,
    CandidateAdoptionV1,
    GateDecision,
    OptimizationAdoptionServiceV1,
    OptimizationGateV1,
    buildPolicySnapshot,
)


def _search_result(score: Decimal = Decimal("0.15")) -> SearchResultV1:
    return SearchResultV1(
        searchId="search-exp-1-abc123",
        strategy="GRID",
        bestParameters={"fast": 3, "slow": 10},
        bestValidationScore=score,
        trialsEvaluated=9,
        dataVersionId="v1",
        randomSeed=42,
        implementationVersion="v1",
        searchHash="",
    )


def _policy(**kw) -> AcceptancePolicySnapshotV1:
    defaults = dict(
        policyVersion="V5",
        minimumClosedTrades=100,
        bootstrapPercentile=Decimal("0.95"),
        netReturnLowerBound=Decimal("0.05"),
        maxDrawdownLimit=Decimal("0.20"),
    )
    defaults.update(kw)
    return buildPolicySnapshot(**defaults)


class TestAcceptancePolicySnapshot:
    def test_policy_verify(self) -> None:
        policy = _policy()
        assert policy.verify()

    def test_policy_tamper_detected(self) -> None:
        policy = _policy()
        tampered = AcceptancePolicySnapshotV1(
            policyVersion=policy.policyVersion,
            minimumClosedTrades=policy.minimumClosedTrades,
            bootstrapPercentile=policy.bootstrapPercentile,
            netReturnLowerBound=Decimal("0.99"),  # 篡改
            maxDrawdownLimit=policy.maxDrawdownLimit,
            policyHash=policy.policyHash,
        )
        assert not tampered.verify()


class TestOptimizationGate:
    def test_gate_approved_when_all_met(self) -> None:
        gate = OptimizationGateV1()
        decision = gate.evaluate(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.12"),
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert decision is GateDecision.Approved

    def test_gate_rejected_on_trades(self) -> None:
        gate = OptimizationGateV1()
        decision = gate.evaluate(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.12"),
            closedTrades=50,  # < 100
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert decision is GateDecision.Rejected

    def test_gate_rejected_on_holdout_below_bound(self) -> None:
        gate = OptimizationGateV1()
        decision = gate.evaluate(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.01"),  # < 0.05
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert decision is GateDecision.Rejected

    def test_gate_rejected_on_drawdown(self) -> None:
        gate = OptimizationGateV1()
        decision = gate.evaluate(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.12"),
            closedTrades=150,
            maxDrawdown=Decimal("0.35"),  # > 0.20
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert decision is GateDecision.Rejected

    def test_gate_requires_two_approvers(self) -> None:
        gate = OptimizationGateV1()
        with pytest.raises(ValueError, match="两名批准人"):
            gate.evaluate(
                searchResult=_search_result(),
                holdoutScore=Decimal("0.12"),
                closedTrades=150,
                maxDrawdown=Decimal("0.10"),
                policy=_policy(),
                approvedBy=("alice",),
            )

    def test_gate_rejects_same_approver(self) -> None:
        gate = OptimizationGateV1()
        with pytest.raises(ValueError, match="互不相同"):
            gate.evaluate(
                searchResult=_search_result(),
                holdoutScore=Decimal("0.12"),
                closedTrades=150,
                maxDrawdown=Decimal("0.10"),
                policy=_policy(),
                approvedBy=("alice", "alice"),
            )

    def test_gate_rejects_policy_tamper(self) -> None:
        gate = OptimizationGateV1()
        policy = _policy()
        tampered = AcceptancePolicySnapshotV1(
            policyVersion=policy.policyVersion,
            minimumClosedTrades=policy.minimumClosedTrades,
            bootstrapPercentile=policy.bootstrapPercentile,
            netReturnLowerBound=Decimal("0.99"),
            maxDrawdownLimit=policy.maxDrawdownLimit,
            policyHash=policy.policyHash,
        )
        with pytest.raises(ValueError, match="哈希不匹配"):
            gate.evaluate(
                searchResult=_search_result(),
                holdoutScore=Decimal("0.12"),
                closedTrades=150,
                maxDrawdown=Decimal("0.10"),
                policy=tampered,
                approvedBy=("alice", "bob"),
            )

    def test_auto_adopt_never_bypasses_gate(self) -> None:
        """优化结果不能自动绕过策略 Gate：任何自动晋级返回 PENDING。"""
        gate = OptimizationGateV1()
        assert gate.autoAdopt() is GateDecision.Pending


class TestOptimizationAdoptionService:
    def test_adopt_success(self) -> None:
        service = OptimizationAdoptionServiceV1()
        adoption = service.adopt(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.12"),
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert adoption.decision is GateDecision.Approved
        assert adoption.verify()
        assert service.adoptedCount() == 1

    def test_adopt_rejected_when_gate_fails(self) -> None:
        service = OptimizationAdoptionServiceV1()
        adoption = service.adopt(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.01"),  # 不达标
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        assert adoption.decision is GateDecision.Rejected
        assert service.adoptedCount() == 0

    def test_adoption_tamper_detected(self) -> None:
        service = OptimizationAdoptionServiceV1()
        adoption = service.adopt(
            searchResult=_search_result(),
            holdoutScore=Decimal("0.12"),
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("alice", "bob"),
        )
        # 篡改：把批准人换掉（但保留原哈希）
        tampered = CandidateAdoptionV1(
            adoptionId=adoption.adoptionId,
            searchResult=adoption.searchResult,
            policyVersion=adoption.policyVersion,
            decision=GateDecision.Approved,
            approvedBy=("carol", "dave"),
            adoptedAt=adoption.adoptedAt,
            adoptionHash=adoption.adoptionHash,
        )
        assert not tampered.verify()
        assert not service.verifyIntegrity(tampered)

    def test_unapproved_candidate_not_adopted(self) -> None:
        """未审批的优化结果不得成为默认：只有 Gate 通过 + 双人批准才记录采用。"""
        service = OptimizationAdoptionServiceV1()
        with pytest.raises(ValueError, match="两名批准人"):
            service.adopt(
                searchResult=_search_result(),
                holdoutScore=Decimal("0.12"),
                closedTrades=150,
                maxDrawdown=Decimal("0.10"),
                policy=_policy(),
                approvedBy=("alice",),
            )
        assert service.all() == ()
