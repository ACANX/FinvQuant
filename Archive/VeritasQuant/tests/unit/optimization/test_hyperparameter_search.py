"""P6-007b 确定性超参数搜索测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.optimization.ExperimentTracker import ExperimentTrackerV1
from veritasquant.optimization.HyperparameterSearch import (
    HyperparameterSearchV1,
    SearchResultV1,
    SearchSpaceV1,
    buildNumericSpace,
)


def _evaluator(parameters: dict, seed: int) -> Decimal:
    """确定性评估器：目标函数（越小越好转负，搜索取最大）。"""
    fast = int(parameters.get("fast", 1))
    slow = int(parameters.get("slow", 1))
    return Decimal(str(-(abs(fast - 3) + abs(slow - 10)) / 100.0))


def _tracker() -> ExperimentTrackerV1:
    return ExperimentTrackerV1()


class TestSearchSpace:
    def test_space_valid(self) -> None:
        space = SearchSpaceV1(name="fast", candidates=(1, 2, 3))
        assert space.name == "fast"

    def test_space_requires_name(self) -> None:
        with pytest.raises(ValueError, match="参数名"):
            SearchSpaceV1(name="", candidates=(1,))

    def test_space_requires_candidates(self) -> None:
        with pytest.raises(ValueError, match="候选值"):
            SearchSpaceV1(name="fast", candidates=())

    def test_build_numeric_space(self) -> None:
        space = buildNumericSpace("fast", (1, 2, 3))
        assert space.candidates == (1, 2, 3)


class TestHyperparameterSearch:
    def test_grid_search_finds_best(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(
            tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1",
        )
        result = searcher.gridSearch(
            space={
                "fast": SearchSpaceV1("fast", (1, 3, 5)),
                "slow": SearchSpaceV1("slow", (5, 10, 20)),
            },
            experimentId="exp-1",
        )
        # 最优：fast=3, slow=10 → 0
        assert result.bestParameters == {"fast": 3, "slow": 10}
        assert result.bestValidationScore == Decimal("0")
        assert result.trialsEvaluated == 9
        assert result.verify()
        assert len(tracker.completed()) == 9

    def test_grid_search_records_all_trials(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(
            tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1",
        )
        searcher.gridSearch(
            space={"fast": SearchSpaceV1("fast", (1, 2))},
            experimentId="exp-1",
        )
        assert len(tracker.all()) == 2
        for trial in tracker.all():
            assert trial.verify()
            assert trial.holdoutScore is None  # 留出保持锁定

    def test_random_search_deterministic(self) -> None:
        tracker1 = _tracker()
        tracker2 = _tracker()
        s1 = HyperparameterSearchV1(tracker=tracker1, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1", randomSeed=7)
        s2 = HyperparameterSearchV1(tracker=tracker2, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1", randomSeed=7)
        r1 = s1.randomSearch(space={"fast": SearchSpaceV1("fast", (1, 2, 3, 4, 5))}, experimentId="e", iterations=10)
        r2 = s2.randomSearch(space={"fast": SearchSpaceV1("fast", (1, 2, 3, 4, 5))}, experimentId="e", iterations=10)
        assert r1.bestParameters == r2.bestParameters
        assert r1.searchHash == r2.searchHash

    def test_random_search_integer_range(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1", randomSeed=3)
        result = searcher.randomSearch(
            space={"fast": SearchSpaceV1("fast", (), minValue=1, maxValue=10, isInteger=True)},
            experimentId="e",
            iterations=5,
        )
        assert result.trialsEvaluated == 5
        assert 1 <= int(result.bestParameters["fast"]) <= 10

    def test_random_search_rejects_zero_iterations(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        with pytest.raises(ValueError, match="为正"):
            searcher.randomSearch(space={"fast": SearchSpaceV1("fast", (1,))}, experimentId="e", iterations=0)

    def test_sequential_search_order(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        result = searcher.sequentialSearch(
            space={
                "fast": SearchSpaceV1("fast", (1, 3)),
                "slow": SearchSpaceV1("slow", (5, 10)),
            },
            experimentId="e",
        )
        assert result.trialsEvaluated == 4
        assert result.bestParameters == {"fast": 3, "slow": 10}

    def test_sequential_search_invalid_order(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        with pytest.raises(ValueError, match="order"):
            searcher.sequentialSearch(
                space={"fast": SearchSpaceV1("fast", (1,)), "slow": SearchSpaceV1("slow", (1,))},
                experimentId="e",
                order=("fast",),
            )

    def test_search_holdout_stays_locked(self) -> None:
        """搜索期间留出段保持锁定（隔离观察）。"""
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        searcher.gridSearch(
            space={"fast": SearchSpaceV1("fast", (1, 2))},
            experimentId="e",
        )
        assert tracker.holdoutLocked()
        for trial in tracker.completed():
            assert trial.holdoutScore is None

    def test_search_result_tamper_detected(self) -> None:
        tracker = _tracker()
        searcher = HyperparameterSearchV1(tracker=tracker, evaluator=_evaluator, dataVersionId="v1", implementationVersion="v1")
        result = searcher.gridSearch(space={"fast": SearchSpaceV1("fast", (1,))}, experimentId="e")
        tampered = SearchResultV1(
            searchId=result.searchId, strategy=result.strategy,
            bestParameters={"fast": 99}, bestValidationScore=result.bestValidationScore,
            trialsEvaluated=result.trialsEvaluated, dataVersionId=result.dataVersionId,
            randomSeed=result.randomSeed, implementationVersion=result.implementationVersion,
            searchHash=result.searchHash, createdAt=result.createdAt,
        )
        assert not tampered.verify()
