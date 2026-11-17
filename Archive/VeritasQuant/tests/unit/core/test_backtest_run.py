from __future__ import annotations

import pytest

from veritasquant.core.BacktestRun import BacktestRunError, BacktestRunStateMachineV1, BacktestRunStatus


def test_backtest_run_pauses_and_resumes_from_checkpoint() -> None:
    machine = BacktestRunStateMachineV1("run-1")
    assert machine.start().status is BacktestRunStatus.Running
    assert machine.pause(9).checkpointSequence == 9
    assert machine.start().status is BacktestRunStatus.Running
    assert machine.succeed(12).status is BacktestRunStatus.Succeeded


def test_backtest_run_rejects_terminal_regression_and_cancel_is_not_success() -> None:
    machine = BacktestRunStateMachineV1("run-1")
    machine.start()
    assert machine.cancel().status is BacktestRunStatus.Cancelled
    with pytest.raises(BacktestRunError):
        machine.start()
    failed = BacktestRunStateMachineV1("run-2")
    failed.start()
    assert failed.fail("data failure").status is BacktestRunStatus.Failed
    with pytest.raises(BacktestRunError):
        failed.succeed(1)
