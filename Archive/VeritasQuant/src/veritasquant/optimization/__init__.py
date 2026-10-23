"""离线优化与模型管理领域包（P6-007）。

训练/验证/留出三段隔离、确定性超参搜索与策略 Gate 隔离。
"""

from __future__ import annotations

from veritasquant.optimization.ExperimentTracker import (
    DatasetSplit,
    ExperimentTrackerV1,
    TrialStatus,
    TrialV1,
)
from veritasquant.optimization.HyperparameterSearch import (
    HyperparameterSearchV1,
    SearchResultV1,
    SearchSpaceV1,
    SearchStrategy,
    buildNumericSpace,
)
from veritasquant.optimization.OptimizationGate import (
    AcceptancePolicySnapshotV1,
    CandidateAdoptionV1,
    GateDecision,
    OptimizationAdoptionServiceV1,
    OptimizationGateV1,
    buildPolicySnapshot,
)

__all__ = [
    "DatasetSplit",
    "ExperimentTrackerV1",
    "TrialStatus",
    "TrialV1",
    "HyperparameterSearchV1",
    "SearchResultV1",
    "SearchSpaceV1",
    "SearchStrategy",
    "buildNumericSpace",
    "AcceptancePolicySnapshotV1",
    "CandidateAdoptionV1",
    "GateDecision",
    "OptimizationAdoptionServiceV1",
    "OptimizationGateV1",
    "buildPolicySnapshot",
]
