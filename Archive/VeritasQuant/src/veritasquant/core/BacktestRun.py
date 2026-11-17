"""回测运行生命周期状态机。"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class BacktestRunError(ValueError):
    """回测状态转换或结果记录不合法。"""


class BacktestRunStatus(StrEnum):
    Created = "CREATED"
    Running = "RUNNING"
    Paused = "PAUSED"
    Cancelled = "CANCELLED"
    Failed = "FAILED"
    Succeeded = "SUCCEEDED"


@dataclass(frozen=True, slots=True)
class BacktestRunV1:
    runId: str
    status: BacktestRunStatus
    checkpointSequence: int | None
    failureReason: str | None


class BacktestRunStateMachineV1:
    """回测仅能从已提交 checkpoint 继续。"""

    def __init__(self, runId: str) -> None:
        if not runId:
            raise BacktestRunError("runId 不能为空")
        self._run = BacktestRunV1(runId, BacktestRunStatus.Created, None, None)

    @property
    def current(self) -> BacktestRunV1:
        return self._run

    def start(self) -> BacktestRunV1:
        return self._transition({BacktestRunStatus.Created, BacktestRunStatus.Paused}, BacktestRunStatus.Running)

    def pause(self, checkpointSequence: int) -> BacktestRunV1:
        if checkpointSequence < 0:
            raise BacktestRunError("checkpointSequence 不得为负数")
        return self._transition({BacktestRunStatus.Running}, BacktestRunStatus.Paused, checkpointSequence)

    def succeed(self, checkpointSequence: int) -> BacktestRunV1:
        if checkpointSequence < 0:
            raise BacktestRunError("checkpointSequence 不得为负数")
        return self._transition({BacktestRunStatus.Running}, BacktestRunStatus.Succeeded, checkpointSequence)

    def fail(self, reason: str) -> BacktestRunV1:
        if not reason:
            raise BacktestRunError("失败必须记录原因")
        return self._transition({BacktestRunStatus.Running, BacktestRunStatus.Paused}, BacktestRunStatus.Failed, failureReason=reason)

    def cancel(self) -> BacktestRunV1:
        return self._transition({BacktestRunStatus.Created, BacktestRunStatus.Running, BacktestRunStatus.Paused}, BacktestRunStatus.Cancelled)

    def _transition(self, allowed: set[BacktestRunStatus], target: BacktestRunStatus, checkpointSequence: int | None = None, failureReason: str | None = None) -> BacktestRunV1:
        if self._run.status not in allowed:
            raise BacktestRunError(f"不允许从 {self._run.status} 转换到 {target}")
        self._run = BacktestRunV1(self._run.runId, target, checkpointSequence if checkpointSequence is not None else self._run.checkpointSequence, failureReason)
        return self._run
