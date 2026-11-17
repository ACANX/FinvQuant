"""回测应用服务：从配置创建、运行、继续、取消并查询（技术方案 8.1 节）。

退出码和错误信封符合应用契约；回测只能从已提交 checkpoint 继续；
取消不伪装未回滚副作用；结果可查询。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.core.BacktestRun import (
    BacktestRunStateMachineV1,
    BacktestRunStatus,
    BacktestRunV1,
)
from veritasquant.core.CanonicalJson import canonicalHash


class BacktestServiceError(ValueError):
    """回测服务配置或生命周期操作不合法时抛出。"""


@dataclass(frozen=True, slots=True)
class BacktestConfigV1:
    """回测运行配置；配置哈希进入运行清单。"""

    runId: str
    accountId: str
    strategyId: str
    strategyVersion: str
    dataRangeStart: str
    dataRangeEnd: str
    initialCash: Decimal
    executionMode: str
    executionModelVersion: str
    randomSeed: int

    def configHash(self) -> str:
        """配置身份哈希。"""
        return canonicalHash(
            {
                "run_id": self.runId,
                "account_id": self.accountId,
                "strategy_id": self.strategyId,
                "strategy_version": self.strategyVersion,
                "data_range_start": self.dataRangeStart,
                "data_range_end": self.dataRangeEnd,
                "initial_cash": self.initialCash,
                "execution_mode": self.executionMode,
                "execution_model_version": self.executionModelVersion,
                "random_seed": self.randomSeed,
            }
        )


@dataclass(frozen=True, slots=True)
class BacktestRunViewV1:
    """查询视图：运行状态、checkpoint 与失败原因。"""

    runId: str
    status: BacktestRunStatus
    checkpointSequence: int | None
    failureReason: str | None
    configHash: str


class BacktestApplicationServiceV1:
    """从配置创建、运行、继续、取消并查询回测运行。"""

    def __init__(self) -> None:
        self._stateMachines: dict[str, BacktestRunStateMachineV1] = {}
        self._configs: dict[str, BacktestConfigV1] = {}

    def createRun(self, config: BacktestConfigV1) -> BacktestRunViewV1:
        """从配置创建回测运行。"""
        if not config.runId or not config.accountId or not config.strategyId:
            raise BacktestServiceError("运行、账户和策略 ID 不能为空")
        if not config.initialCash or config.initialCash <= 0:
            raise BacktestServiceError("初始资金必须为正 Decimal")
        if config.dataRangeStart >= config.dataRangeEnd:
            raise BacktestServiceError("数据区间起点必须早于终点")
        if config.runId in self._stateMachines:
            raise BacktestServiceError("runId 已存在")
        machine = BacktestRunStateMachineV1(config.runId)
        self._stateMachines[config.runId] = machine
        self._configs[config.runId] = config
        return self._view(machine.current, config)

    def start(self, runId: str) -> BacktestRunViewV1:
        """开始或继续运行。"""
        machine = self._require(runId)
        return self._view(machine.start(), self._configs[runId])

    def pause(self, runId: str, checkpointSequence: int) -> BacktestRunViewV1:
        """在已提交 checkpoint 处暂停。"""
        machine = self._require(runId)
        return self._view(machine.pause(checkpointSequence), self._configs[runId])

    def succeed(self, runId: str, checkpointSequence: int) -> BacktestRunViewV1:
        """运行成功完成。"""
        machine = self._require(runId)
        return self._view(machine.succeed(checkpointSequence), self._configs[runId])

    def fail(self, runId: str, reason: str) -> BacktestRunViewV1:
        """运行失败；必须记录原因。"""
        machine = self._require(runId)
        return self._view(machine.fail(reason), self._configs[runId])

    def cancel(self, runId: str) -> BacktestRunViewV1:
        """取消运行；不得伪装未回滚副作用。"""
        machine = self._require(runId)
        return self._view(machine.cancel(), self._configs[runId])

    def query(self, runId: str) -> BacktestRunViewV1:
        """查询运行状态。"""
        machine = self._require(runId)
        return self._view(machine.current, self._configs[runId])

    def queryAll(self) -> tuple[BacktestRunViewV1, ...]:
        """查询全部运行视图。"""
        return tuple(
            self._view(machine.current, self._configs[runId])
            for runId, machine in sorted(self._stateMachines.items())
        )

    def _require(self, runId: str) -> BacktestRunStateMachineV1:
        machine = self._stateMachines.get(runId)
        if machine is None:
            raise BacktestServiceError("未知运行 ID")
        return machine

    def _view(self, run: BacktestRunV1, config: BacktestConfigV1) -> BacktestRunViewV1:
        return BacktestRunViewV1(
            runId=run.runId,
            status=run.status,
            checkpointSequence=run.checkpointSequence,
            failureReason=run.failureReason,
            configHash=config.configHash(),
        )
