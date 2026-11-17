"""P6-007 优化与模型管理集成测试：搜索 → 隔离 → Gate → 采用全流程。

覆盖验收要点：
- 训练/验证/留出三段隔离：搜索只用训练/验证，留出批准前锁定；
- 试验可复现：同输入同输出（同种子同哈希）；
- 不能自动绕过策略 Gate：优化结果必须经冻结政策 + 留出达标 + 双人批准才采用；
- 完整流程：确定性搜索 → 解锁留出评估 → Gate 判定 → 采用记录（不可变）。
"""

from __future__ import annotations

from decimal import Decimal


from veritasquant.optimization.ExperimentTracker import ExperimentTrackerV1
from veritasquant.optimization.HyperparameterSearch import (
    HyperparameterSearchV1,
    SearchSpaceV1,
)
from veritasquant.optimization.OptimizationGate import (
    GateDecision,
    OptimizationAdoptionServiceV1,
    buildPolicySnapshot,
)


def _evaluator(parameters: dict, seed: int) -> Decimal:
    """确定性评估器：最优在 fast=3, slow=10（返回 0）。"""
    fast = int(parameters.get("fast", 1))
    slow = int(parameters.get("slow", 1))
    return Decimal(str(-(abs(fast - 3) + abs(slow - 10)) / 100.0))


def _policy() -> "object":
    return buildPolicySnapshot(
        policyVersion="V5",
        minimumClosedTrades=100,
        bootstrapPercentile=Decimal("0.95"),
        netReturnLowerBound=Decimal("0.05"),
        maxDrawdownLimit=Decimal("0.20"),
    )


class TestOptimizationWorkflow:
    def test_full_optimization_flow(self) -> None:
        """搜索 → 留出隔离评估 → Gate 批准 → 采用。"""
        tracker = ExperimentTrackerV1()
        searcher = HyperparameterSearchV1(
            tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1",
        )
        # 1. 确定性网格搜索（只用训练/验证）
        result = searcher.gridSearch(
            space={
                "fast": SearchSpaceV1("fast", (1, 3, 5)),
                "slow": SearchSpaceV1("slow", (5, 10, 20)),
            },
            experimentId="exp-1",
        )
        assert result.bestParameters == {"fast": 3, "slow": 10}
        assert result.verify()
        # 搜索期间留出锁定
        assert tracker.holdoutLocked()
        for trial in tracker.completed():
            assert trial.holdoutScore is None

        # 2. 解锁留出段评估（隔离观察）
        tracker.unlockHoldout()
        best_trial = tracker.bestByValidation()
        assert best_trial is not None
        updated = tracker.recordHoldout(trialId=best_trial.trialId, holdoutScore=Decimal("0.12"))
        assert updated.holdoutScore == Decimal("0.12")
        assert tracker.holdoutLocked()  # 记录后重新锁定

        # 3. Gate 判定 + 采用（双人批准）
        service = OptimizationAdoptionServiceV1()
        adoption = service.adopt(
            searchResult=result,
            holdoutScore=Decimal("0.12"),
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("reviewer-qa", "live-approver"),
        )
        assert adoption.decision is GateDecision.Approved
        assert adoption.verify()
        assert service.adoptedCount() == 1

    def test_gate_blocks_poor_holdout(self) -> None:
        """留出成绩不达标 → Gate 拒绝，不产生采用。"""
        tracker = ExperimentTrackerV1()
        searcher = HyperparameterSearchV1(
            tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1",
        )
        result = searcher.gridSearch(
            space={"fast": SearchSpaceV1("fast", (1, 3))},
            experimentId="exp-2",
        )
        service = OptimizationAdoptionServiceV1()
        adoption = service.adopt(
            searchResult=result,
            holdoutScore=Decimal("0.01"),  # 低于下界 0.05
            closedTrades=150,
            maxDrawdown=Decimal("0.10"),
            policy=_policy(),
            approvedBy=("reviewer-qa", "live-approver"),
        )
        assert adoption.decision is GateDecision.Rejected
        assert service.adoptedCount() == 0

    def test_no_auto_adoption(self) -> None:
        """优化结果绝不能自动采用（不绕过 Gate）。"""
        from veritasquant.optimization.OptimizationGate import OptimizationGateV1

        gate = OptimizationGateV1()
        assert gate.autoAdopt() is GateDecision.Pending

    def test_reproducibility_across_runs(self) -> None:
        """两次相同搜索产生完全相同的试验与结果哈希。"""
        tracker1 = ExperimentTrackerV1()
        tracker2 = ExperimentTrackerV1()
        s1 = HyperparameterSearchV1(tracker=tracker1, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        s2 = HyperparameterSearchV1(tracker=tracker2, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        space = {"fast": SearchSpaceV1("fast", (1, 2, 3)), "slow": SearchSpaceV1("slow", (5, 10))}
        r1 = s1.gridSearch(space=space, experimentId="exp-r")
        r2 = s2.gridSearch(space=space, experimentId="exp-r")
        assert r1.searchHash == r2.searchHash
        assert r1.bestParameters == r2.bestParameters
        # 试验身份哈希一一对应
        trials1 = {t.parameters.get("fast"): t for t in tracker1.completed()}
        trials2 = {t.parameters.get("fast"): t for t in tracker2.completed()}
        assert trials1.keys() == trials2.keys()
        for key in trials1:
            assert trials1[key].trialHash == trials2[key].trialHash
