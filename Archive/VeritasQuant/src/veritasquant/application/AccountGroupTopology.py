"""P2-004 账户组拓扑：分区、账户排名与执行模式隔离。

首期固定使用 `account_group_id` 作为事件循环分区键（TechSpec 3.2）：

- 每个账户在一次运行中必须且只能属于一个账户组；
- 同一账户组只能包含同一 `execution_mode` 和同一安全环境的账户；
  `LIVE` 不得与回测、模拟或仿真账户混组；
- 组内按照版本化 `account_rank` 串行更新，组间由独立 worker 并行运行；
- 账户组、账户排名、分区排名与执行模式绑定在运行开始后冻结。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class ExecutionModeV1(StrEnum):
    """与 TechSpec 环境链一致的执行模式。"""

    Backtest = "BACKTEST"
    PaperTrading = "PAPER_TRADING"
    BrokerSimulation = "BROKER_SIMULATION"
    LiveShadow = "LIVE_SHADOW"
    ControlledLive = "CONTROLLED_LIVE"

    @property
    def isLive(self) -> bool:
        """LIVE 类模式：与任何非 LIVE 模式混组一律拒绝。"""
        return self in (ExecutionModeV1.LiveShadow, ExecutionModeV1.ControlledLive)


class AccountGroupError(ValueError):
    """账户组拓扑或排名不满足契约。"""


@dataclass(frozen=True, slots=True)
class AccountGroupTopologyV1:
    """冻结的账户组拓扑：同一执行模式、组内账户排名唯一。"""

    accountGroupId: str
    executionMode: ExecutionModeV1
    partitionRank: int
    accountRanks: tuple[tuple[str, int], ...]

    def __post_init__(self) -> None:
        if not self.accountGroupId:
            raise AccountGroupError("账户组 ID 不能为空")
        if self.partitionRank < 0:
            raise AccountGroupError("分区排名必须非负")
        if not self.accountRanks:
            raise AccountGroupError("账户组必须至少包含一个账户")
        accountIds = [accountId for accountId, _ in self.accountRanks]
        if len(accountIds) != len(set(accountIds)):
            raise AccountGroupError("同一账户不得重复加入一个账户组")
        ranks = [rank for _, rank in self.accountRanks]
        if len(ranks) != len(set(ranks)):
            raise AccountGroupError("组内 account_rank 必须唯一")

    def accountRankFor(self, accountId: str) -> int:
        """返回账户组内排名；未知账户抛出契约错误。"""
        for candidate, rank in self.accountRanks:
            if candidate == accountId:
                return rank
        raise AccountGroupError(f"账户 {accountId} 不属于账户组 {self.accountGroupId}")

    def accountsByRank(self) -> tuple[str, ...]:
        """按 account_rank 升序返回账户列表（组内串行更新顺序）。"""
        return tuple(accountId for accountId, _ in sorted(self.accountRanks, key=lambda item: item[1]))


def validateGroupPartitioning(groups: tuple[AccountGroupTopologyV1, ...]) -> None:
    """全局校验：账户跨组唯一、LIVE 与非 LIVE 不混组、分区排名唯一。"""
    seenAccounts: dict[str, str] = {}
    seenPartitionRanks: dict[int, str] = {}
    for group in groups:
        if group.partitionRank in seenPartitionRanks:
            raise AccountGroupError(
                f"分区排名 {group.partitionRank} 被 {seenPartitionRanks[group.partitionRank]} 与 "
                f"{group.accountGroupId} 重复使用"
            )
        seenPartitionRanks[group.partitionRank] = group.accountGroupId
        for accountId, _ in group.accountRanks:
            if accountId in seenAccounts:
                raise AccountGroupError(
                    f"账户 {accountId} 同时属于 {seenAccounts[accountId]} 与 {group.accountGroupId}"
                )
            seenAccounts[accountId] = group.accountGroupId
        # LIVE 与非 LIVE 混组检查：同组模式一致由 __post_init__ 之外
        # 这里的全局约束是：LIVE 账户组必须与其它所有组同为 LIVE 或同为非 LIVE
        liveModes = {group.executionMode.isLive for group in groups}
        if len(liveModes) > 1:
            liveGroups = [g.accountGroupId for g in groups if g.executionMode.isLive]
            nonLiveGroups = [g.accountGroupId for g in groups if not g.executionMode.isLive]
            raise AccountGroupError(
                f"LIVE 账户组 {liveGroups} 不得与回测/模拟/仿真账户组 {nonLiveGroups} 在同一运行混组"
            )
