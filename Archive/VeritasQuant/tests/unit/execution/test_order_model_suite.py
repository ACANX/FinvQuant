from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.execution.OrderModelSuite import (
    ARCHIVE_SEEDS,
    OrderModelSampleV1,
    runBarPathModel,
    runExecutionModelWalk,
    runLiquidityCompetitionModel,
    runOrderStateModel,
)


def test_state_model_passes_on_archive_seeds() -> None:
    for seed in ARCHIVE_SEEDS:
        report = runOrderStateModel(seed)
        assert report.passed, f"seed={seed} 失败: {report.failures[0] if report.failures else '未知'}"


def test_bar_path_model_passes_on_archive_seeds() -> None:
    for seed in ARCHIVE_SEEDS:
        report = runBarPathModel(seed)
        assert report.passed, f"seed={seed} 失败: {report.failures[0] if report.failures else '未知'}"


def test_liquidity_competition_model_passes_on_archive_seeds() -> None:
    for seed in ARCHIVE_SEEDS:
        report = runLiquidityCompetitionModel(seed)
        assert report.passed, f"seed={seed} 失败: {report.failures[0] if report.failures else '未知'}"


def test_execution_model_walk_passes_on_archive_seeds() -> None:
    for seed in ARCHIVE_SEEDS:
        report = runExecutionModelWalk(seed)
        assert report.passed, f"seed={seed} 失败: {report.failures[0] if report.failures else '未知'}"


def test_state_model_detects_duplicate_intent_rejection() -> None:
    """model-based 套件必须能捕获人为注入的不变量破坏。"""
    from veritasquant.execution.OrderStateMachine import OrderStateMachineV1

    machine = OrderStateMachineV1()
    machine.createIntent("order-1", "account-1", Decimal("100"), 0)
    from veritasquant.execution.OrderStateMachine import OrderStateMachineError

    with pytest.raises(OrderStateMachineError, match="唯一"):
        machine.createIntent("order-1", "account-1", Decimal("200"), 0)


def test_ten_thousand_state_sequences_have_no_invariant_failure() -> None:
    """验收标准：随机重复/乱序/撤单竞态不少于 10,000 组无状态或累计量失败。"""
    failures: list[str] = []
    for seed in range(10_000):
        report = runOrderStateModel(seed, steps=30)
        if not report.passed and report.failures:
            failures.append(f"seed={seed} sample={report.failures[0]}")
            if len(failures) >= 5:
                break
    assert not failures, f"发现不变量失败: {failures}"


def test_ten_thousand_bar_path_sequences_have_no_failure() -> None:
    failures: list[str] = []
    for seed in range(10_000):
        report = runBarPathModel(seed, steps=20)
        if not report.passed and report.failures:
            failures.append(f"seed={seed} sample={report.failures[0]}")
            if len(failures) >= 5:
                break
    assert not failures, f"发现路径失败: {failures}"


def test_failure_sample_carries_seed_and_step() -> None:
    """最小失败样本必须携带种子和步骤以便复现。"""
    sample = OrderModelSampleV1(seed=42, step=7, invariant="version_monotonic", message="版本回退")
    assert sample.seed == 42
    assert sample.step == 7
    assert sample.invariant == "version_monotonic"
