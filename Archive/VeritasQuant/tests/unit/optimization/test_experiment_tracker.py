"""P6-007a 试验追踪（训练/验证/留出隔离 + 可复现）测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.optimization.ExperimentTracker import (
    DatasetSplit,
    ExperimentTrackerV1,
    TrialStatus,
    TrialV1,
)


def _trial_params() -> dict:
    return {"fast_window": 5, "slow_window": 20, "threshold": Decimal("0.5")}


class TestDatasetSplit:
    def test_three_splits(self) -> None:
        assert {s.value for s in DatasetSplit} == {"TRAINING", "VALIDATION", "HOLDOUT"}


class TestTrial:
    def test_trial_hash_and_verify(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="exp-1", parameters=_trial_params(),
            dataVersionId="data-v1", randomSeed=42, implementationVersion="v1",
        )
        assert trial.verify()
        assert trial.status is TrialStatus.Running

    def test_trial_hash_changes_with_parameters(self) -> None:
        tracker = ExperimentTrackerV1()
        t1 = tracker.createTrial(
            experimentId="exp-1", parameters={"a": 1}, dataVersionId="v1",
            randomSeed=42, implementationVersion="v1",
        )
        t2 = tracker.createTrial(
            experimentId="exp-1", parameters={"a": 2}, dataVersionId="v1",
            randomSeed=42, implementationVersion="v1",
        )
        assert t1.trialHash != t2.trialHash

    def test_reproducible_with_same_inputs(self) -> None:
        tracker = ExperimentTrackerV1()
        t1 = tracker.createTrial(
            experimentId="exp-1", parameters={"a": 1}, dataVersionId="v1",
            randomSeed=42, implementationVersion="v1",
        )
        assert t1.reproducibleWith(parameters={"a": 1}, dataVersionId="v1", randomSeed=42, implementationVersion="v1")
        assert not t1.reproducibleWith(parameters={"a": 2}, dataVersionId="v1", randomSeed=42, implementationVersion="v1")

    def test_tamper_detected(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="exp-1", parameters={"a": 1}, dataVersionId="v1",
            randomSeed=42, implementationVersion="v1",
        )
        tampered = TrialV1(
            trialId=trial.trialId, experimentId=trial.experimentId,
            parameters={"a": 99}, dataVersionId=trial.dataVersionId,
            randomSeed=trial.randomSeed, implementationVersion=trial.implementationVersion,
            trainingScore=None, validationScore=None, holdoutScore=None,
            status=trial.status, trialHash=trial.trialHash, createdAt=trial.createdAt,
        )
        assert not tampered.verify()
        assert not tracker.verifyIntegrity(tampered)


class TestExperimentTracker:
    def test_create_and_complete(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="exp-1", parameters=_trial_params(),
            dataVersionId="v1", randomSeed=42, implementationVersion="v1",
        )
        completed = tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.6"), validationScore=Decimal("0.55"))
        assert completed.status is TrialStatus.Completed
        assert completed.verify()
        assert len(tracker.completed()) == 1

    def test_complete_requires_running(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1, implementationVersion="v1",
        )
        tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.5"), validationScore=Decimal("0.5"))
        with pytest.raises(ValueError, match="RUNNING"):
            tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.5"), validationScore=Decimal("0.5"))

    def test_holdout_locked_during_optimization(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1, implementationVersion="v1",
        )
        # 优化期间直接带 holdoutScore 完成 → 拒绝（隔离观察）
        with pytest.raises(ValueError, match="锁定"):
            tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.5"), validationScore=Decimal("0.5"), holdoutScore=Decimal("0.7"))

    def test_holdout_unlock_requires_explicit(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1, implementationVersion="v1",
        )
        tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.5"), validationScore=Decimal("0.5"))
        # 未解锁时记录留出 → 拒绝
        with pytest.raises(ValueError, match="锁定"):
            tracker.recordHoldout(trialId=trial.trialId, holdoutScore=Decimal("0.7"))

    def test_holdout_flow(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1, implementationVersion="v1",
        )
        tracker.completeTrial(trialId=trial.trialId, trainingScore=Decimal("0.5"), validationScore=Decimal("0.5"))
        tracker.unlockHoldout()
        updated = tracker.recordHoldout(trialId=trial.trialId, holdoutScore=Decimal("0.72"))
        assert updated.holdoutScore == Decimal("0.72")
        assert updated.verify()
        # 记录后重新锁定
        assert tracker.holdoutLocked()

    def test_best_by_validation(self) -> None:
        tracker = ExperimentTrackerV1()
        for i, score in enumerate([Decimal("0.4"), Decimal("0.6"), Decimal("0.5")]):
            trial = tracker.createTrial(
                experimentId="e", parameters={"i": i}, dataVersionId="v1",
                randomSeed=i, implementationVersion="v1",
            )
            tracker.completeTrial(trialId=trial.trialId, trainingScore=score, validationScore=score)
        best = tracker.bestByValidation()
        assert best is not None
        assert best.validationScore == Decimal("0.6")

    def test_best_by_validation_empty(self) -> None:
        tracker = ExperimentTrackerV1()
        assert tracker.bestByValidation() is None

    def test_fail_trial(self) -> None:
        tracker = ExperimentTrackerV1()
        trial = tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1, implementationVersion="v1",
        )
        failed = tracker.failTrial(trial.trialId, "不可复现")
        assert failed.status is TrialStatus.Failed
        assert failed.notes == "不可复现"
        assert len(tracker.completed()) == 0

    def test_duplicate_trial_rejected(self) -> None:
        tracker = ExperimentTrackerV1()
        tracker.createTrial(
            experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1,
            implementationVersion="v1", trialId="t1",
        )
        with pytest.raises(ValueError, match="已存在"):
            tracker.createTrial(
                experimentId="e", parameters={}, dataVersionId="v1", randomSeed=1,
                implementationVersion="v1", trialId="t1",
            )

    def test_all_trials_recorded(self) -> None:
        tracker = ExperimentTrackerV1()
        for i in range(5):
            tracker.createTrial(
                experimentId="e", parameters={"i": i}, dataVersionId="v1",
                randomSeed=i, implementationVersion="v1",
            )
        assert len(tracker.all()) == 5
