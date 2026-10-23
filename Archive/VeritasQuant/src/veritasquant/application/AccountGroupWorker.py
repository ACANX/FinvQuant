"""P2-004 账户组 worker：组内串行、组间并行与故障隔离。

- 组内按 `account_rank` 升序串行处理事件，保证可复现时序；
- 不同账户组由独立 worker 并行运行（单活租约保护各分区写入）；
- 单组失败只暂停该分区的新开仓与外部发送，其他分区继续消费；
  共享行情失效、全局控制等场景由上层统一进入保护状态。
"""

from __future__ import annotations

from collections.abc import Callable, Iterable
from concurrent.futures import Future, ThreadPoolExecutor
from dataclasses import dataclass
from enum import StrEnum

from veritasquant.application.AccountGroupTopology import AccountGroupTopologyV1
from veritasquant.core.Events import EventEnvelopeV1


class GroupWorkerError(RuntimeError):
    """账户组 worker 生命周期或隔离状态不合法。"""


class GroupState(StrEnum):
    Active = "ACTIVE"
    Isolated = "ISOLATED"  # 单组失败暂停；不影响其他组
    Stopped = "STOPPED"


@dataclass(frozen=True, slots=True)
class GroupProcessingResultV1:
    """单组处理结果，供调度与审计使用。"""

    accountGroupId: str
    state: GroupState
    processedAccounts: tuple[str, ...]


AccountHandler = Callable[[EventEnvelopeV1], None]


class AccountGroupWorkerV1:
    """一个账户组的串行事件处理器：组内按 account_rank 更新。"""

    def __init__(
        self,
        topology: AccountGroupTopologyV1,
        handlers: dict[str, AccountHandler],
    ) -> None:
        self._topology = topology
        missing = [accountId for accountId in topology.accountsByRank() if accountId not in handlers]
        if missing:
            raise GroupWorkerError(f"账户缺少处理器: {missing}")
        self._handlers = dict(handlers)
        self._state = GroupState.Active
        self._failedAccounts: list[str] = []

    @property
    def topology(self) -> AccountGroupTopologyV1:
        return self._topology

    @property
    def state(self) -> GroupState:
        return self._state

    def processEvent(self, event: EventEnvelopeV1) -> GroupProcessingResultV1:
        """组内按 account_rank 升序串行处理；账户失败隔离到本组。"""
        if self._state is GroupState.Stopped:
            raise GroupWorkerError(f"账户组 {self._topology.accountGroupId} 已停止")
        processed: list[str] = []
        for accountId in self._topology.accountsByRank():
            try:
                self._handlers[accountId](event)
                processed.append(accountId)
            except Exception:
                # 单账户失败：暂停整个分区（新开仓与外部发送），记录并继续隔离
                self._state = GroupState.Isolated
                self._failedAccounts.append(accountId)
                break
        return GroupProcessingResultV1(self._topology.accountGroupId, self._state, tuple(processed))

    @property
    def failedAccounts(self) -> tuple[str, ...]:
        return tuple(self._failedAccounts)

    def stop(self) -> None:
        """停止组处理；不可恢复（需重建 worker）。"""
        self._state = GroupState.Stopped


class GroupWorkerPoolV1:
    """组间并行调度：独立线程运行每个账户组 worker，单组失败不污染其他组。"""

    def __init__(self, workers: Iterable[AccountGroupWorkerV1], maxWorkers: int | None = None) -> None:
        self._workers = {worker.topology.accountGroupId: worker for worker in workers}
        if not self._workers:
            raise GroupWorkerError("worker 池不能为空")
        self._maxWorkers = maxWorkers or len(self._workers)

    def fanOut(self, event: EventEnvelopeV1) -> dict[str, GroupProcessingResultV1]:
        """并行分发同一事件到所有账户组；单组异常隔离，其余组继续。"""
        results: dict[str, GroupProcessingResultV1] = {}
        with ThreadPoolExecutor(max_workers=self._maxWorkers) as executor:
            futures: dict[str, Future[GroupProcessingResultV1]] = {
                groupId: executor.submit(worker.processEvent, event)
                for groupId, worker in self._workers.items()
                if worker.state is not GroupState.Stopped
            }
            for groupId, future in futures.items():
                try:
                    results[groupId] = future.result()
                except Exception:
                    # 未来异常（如 worker 已停止）不影响其他组结果
                    continue
        return results

    def workerFor(self, accountGroupId: str) -> AccountGroupWorkerV1:
        if accountGroupId not in self._workers:
            raise GroupWorkerError(f"未知账户组 {accountGroupId}")
        return self._workers[accountGroupId]

    @property
    def isolatedGroups(self) -> tuple[str, ...]:
        return tuple(
            groupId
            for groupId, worker in self._workers.items()
            if worker.state is GroupState.Isolated
        )
