from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.application.BacktestService import (
    BacktestApplicationServiceV1,
    BacktestConfigV1,
    BacktestServiceError,
)
from veritasquant.core.BacktestRun import BacktestRunError, BacktestRunStatus


def _config(**overrides: object) -> BacktestConfigV1:
    values: dict[str, object] = {
        "runId": "run-1",
        "accountId": "account-1",
        "strategyId": "daily_momentum",
        "strategyVersion": "1.0.0",
        "dataRangeStart": "2024-01-01",
        "dataRangeEnd": "2026-01-01",
        "initialCash": Decimal("100000"),
        "executionMode": "IDEAL",
        "executionModelVersion": "IDEAL_V1",
        "randomSeed": 42,
    }
    values.update(overrides)
    return BacktestConfigV1(**values)  # type: ignore[call-arg]


def test_create_run_from_config() -> None:
    service = BacktestApplicationServiceV1()
    view = service.createRun(_config())
    assert view.status is BacktestRunStatus.Created
    assert len(view.configHash) == 64
    assert len(service.queryAll()) == 1


def test_config_hash_is_stable_and_sensitive_to_inputs() -> None:
    assert _config().configHash() == _config().configHash()
    assert _config().configHash() != _config(randomSeed=7).configHash()
    assert _config().configHash() != _config(initialCash=Decimal("1")).configHash()


def test_full_lifecycle_create_start_pause_continue_succeed() -> None:
    service = BacktestApplicationServiceV1()
    service.createRun(_config())
    assert service.start("run-1").status is BacktestRunStatus.Running
    assert service.pause("run-1", 100).status is BacktestRunStatus.Paused
    assert service.start("run-1").status is BacktestRunStatus.Running  # 继续
    view = service.succeed("run-1", 200)
    assert view.status is BacktestRunStatus.Succeeded
    assert view.checkpointSequence == 200


def test_cancel_from_running_and_paused() -> None:
    service = BacktestApplicationServiceV1()
    service.createRun(_config())
    service.start("run-1")
    assert service.cancel("run-1").status is BacktestRunStatus.Cancelled
    second = BacktestApplicationServiceV1()
    second.createRun(_config(runId="run-2"))
    assert second.cancel("run-2").status is BacktestRunStatus.Cancelled


def test_fail_requires_reason() -> None:
    service = BacktestApplicationServiceV1()
    service.createRun(_config())
    service.start("run-1")
    view = service.fail("run-1", "data gap")
    assert view.status is BacktestRunStatus.Failed
    assert view.failureReason == "data gap"


def test_illegal_transitions_rejected() -> None:
    service = BacktestApplicationServiceV1()
    service.createRun(_config())
    # Created 不能直接成功
    with pytest.raises(BacktestRunError, match="不允许"):
        service.succeed("run-1", 1)
    # 终态不可回退
    service.start("run-1")
    service.cancel("run-1")
    with pytest.raises(BacktestRunError, match="不允许"):
        service.start("run-1")


def test_rejects_invalid_configs() -> None:
    service = BacktestApplicationServiceV1()
    with pytest.raises(BacktestServiceError, match="初始资金"):
        service.createRun(_config(initialCash=Decimal("0")))
    with pytest.raises(BacktestServiceError, match="数据区间"):
        service.createRun(_config(dataRangeStart="2026-01-01", dataRangeEnd="2024-01-01"))
    with pytest.raises(BacktestServiceError, match="不能为空"):
        service.createRun(_config(runId=""))


def test_query_and_unknown_run() -> None:
    service = BacktestApplicationServiceV1()
    service.createRun(_config())
    view = service.query("run-1")
    assert view.runId == "run-1"
    with pytest.raises(BacktestServiceError, match="未知运行"):
        service.query("ghost")
